package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestScopeMembers(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --scope acme is a slug; it is resolved to the team id before hitting the
	// members endpoint.
	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "member", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %s", len(rows), out)
	}
	if rows[0]["role"] != "OWNER" || rows[0]["confirmed"] != true {
		t.Fatalf("owner row = %v", rows[0])
	}
	// an unconfirmed (invited, not joined) member still lists, confirmed=false
	if rows[2]["confirmed"] != false {
		t.Fatalf("pending member should be confirmed=false: %v", rows[2])
	}
}

func TestScopeMembersPersonalAccountErrors(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// No scope = personal account, which has no team members; this must be a
	// human-fixable error, not a crash or empty list.
	_, errOut, err := execCLI(t, srv.URL, "scope", "member", "list")
	if err == nil {
		t.Fatalf("expected an error for personal account")
	}
	m := decodeJSON(t, errOut)
	if m["fixable_by"] != "human" {
		t.Fatalf("expected fixable_by human, got %v", m)
	}
}

func TestScopeMemberByEmail(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "member", "get", "dev@acme.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["uid"] != "usr_dev" || m["role"] != "MEMBER" {
		t.Fatalf("member by email = %v", m)
	}
}

func TestScopeMemberByIDAndUsername(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// The same match arm accepts uid or username, not just email.
	byID, _, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "member", "get", "usr_owner")
	if err != nil {
		t.Fatalf("by id err: %v", err)
	}
	if decodeJSON(t, byID)["role"] != "OWNER" {
		t.Fatalf("by id = %s", byID)
	}

	byUser, _, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "member", "get", "newbie")
	if err != nil {
		t.Fatalf("by username err: %v", err)
	}
	if decodeJSON(t, byUser)["uid"] != "usr_pending" {
		t.Fatalf("by username = %s", byUser)
	}
}

func TestScopeMembersFull(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --full returns raw member objects (camelCase createdAt) instead of the
	// compact projection's "joined".
	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "member", "list", "--full")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if _, projected := rows[0]["joined"]; projected {
		t.Fatalf("--full should not carry compact 'joined': %v", rows[0])
	}
	if _, raw := rows[0]["createdAt"]; !raw {
		t.Fatalf("--full should expose raw createdAt: %v", rows[0])
	}
}

func TestScopeMemberNotFound(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "member", "get", "nobody@example.com")
	if err != nil {
		t.Fatalf("expected exit 0 on item-level miss, got: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 {
		t.Fatalf("want 1 @unresolved row, got %d: %s", len(rows), out)
	}
	u, ok := rows[0]["@unresolved"].(map[string]any)
	if !ok {
		t.Fatalf("expected @unresolved record, got %v", rows[0])
	}
	if u["fixable_by"] != "agent" {
		t.Fatalf("expected fixable_by agent, got %v", u)
	}
}
