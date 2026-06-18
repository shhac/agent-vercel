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
	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "members")
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
	_, errOut, err := execCLI(t, srv.URL, "scope", "members")
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

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "member", "dev@acme.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["uid"] != "usr_dev" || m["role"] != "MEMBER" {
		t.Fatalf("member by email = %v", m)
	}
}

func TestScopeMemberNotFound(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "--scope", "acme", "scope", "member", "nobody@example.com")
	if err == nil {
		t.Fatalf("expected not-found error")
	}
	m := decodeJSON(t, errOut)
	if m["fixable_by"] != "agent" {
		t.Fatalf("expected fixable_by agent, got %v", m)
	}
}
