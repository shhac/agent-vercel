package cli

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestCleanRef(t *testing.T) {
	cases := map[string]string{
		"dpl_abc123":                             "dpl_abc123",
		"web-ready.vercel.app":                   "web-ready.vercel.app",
		"https://web-ready.vercel.app":           "web-ready.vercel.app",
		"http://web-ready.vercel.app/some/path":  "web-ready.vercel.app",
		"  https://web-ready.vercel.app/x?y=z  ": "web-ready.vercel.app",
	}
	for in, want := range cases {
		if got := cleanRef(in); got != want {
			t.Fatalf("cleanRef(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestUnknownTopLevelCommandHasHint(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "token", "nope")
	if err == nil {
		t.Fatal("expected unknown-command error")
	}
	m := decodeJSON(t, errOut)
	if m["fixable_by"] != "agent" {
		t.Fatalf("unknown command should be agent error: %v", m)
	}
	if h, _ := m["hint"].(string); !strings.Contains(h, "usage") {
		t.Fatalf("unknown command hint should mention usage: %q", h)
	}
}

func TestUsageJSONCatalog(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "usage", "--json")
	if err != nil {
		t.Fatalf("usage --json: %v", err)
	}
	cmds, ok := decodeJSON(t, out)["commands"].([]any)
	if !ok || len(cmds) == 0 {
		t.Fatalf("no commands catalog: %s", out)
	}
	names := map[string]bool{}
	for _, c := range cmds {
		if m, ok := c.(map[string]any); ok {
			names[m["name"].(string)] = true
		}
	}
	for _, want := range []string{"auth", "scope", "deployment", "env", "domain", "alias"} {
		if !names[want] {
			t.Fatalf("catalog missing domain %q: %v", want, names)
		}
	}
}
