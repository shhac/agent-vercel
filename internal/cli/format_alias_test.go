package cli

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

// The output-format parser moved to lib-agent-output during the libs migration
// and is deliberately more lenient than the pre-migration validator: it accepts
// "ndjson"/"yml" aliases and is case-insensitive. These tests pin that as
// intended contract so a future tightening (or a surprise loosening) is a
// conscious choice, not a silent regression.

func TestFormatAliasesAccepted(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// shape: how we recognize each canonical format from `scope current` output.
	isYAML := func(s string) bool {
		s = strings.TrimSpace(s)
		return strings.Contains(s, "default_scope:") && !strings.HasPrefix(s, "{")
	}
	isPrettyJSON := func(s string) bool {
		s = strings.TrimSpace(s)
		return strings.HasPrefix(s, "{") && strings.Contains(s, "\n")
	}
	isNDJSON := func(s string) bool {
		s = strings.TrimSpace(s)
		return strings.HasPrefix(s, "{") && !strings.Contains(s, "\n")
	}

	cases := []struct {
		flag string
		want func(string) bool
		desc string
	}{
		{"yaml", isYAML, "canonical yaml"},
		{"yml", isYAML, "yml alias"},
		{"YAML", isYAML, "uppercase yaml"},
		{"json", isPrettyJSON, "canonical json"},
		{"JSON", isPrettyJSON, "uppercase json"},
		{"jsonl", isNDJSON, "canonical jsonl"},
		{"ndjson", isNDJSON, "ndjson alias"},
	}
	for _, tc := range cases {
		out, _, err := execCLI(t, srv.URL, "--format", tc.flag, "scope", "current")
		if err != nil {
			t.Fatalf("--format %s (%s): %v", tc.flag, tc.desc, err)
		}
		if !tc.want(out) {
			t.Fatalf("--format %s (%s) produced unexpected shape:\n%s", tc.flag, tc.desc, out)
		}
	}
}

func TestFormatInvalidStillRejected(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "--format", "toml", "scope", "current")
	if err == nil {
		t.Fatal("--format toml should be rejected")
	}
	if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
		t.Fatalf("invalid format should be fixable_by agent: %v", m)
	}
}

// config set format also runs through the lenient parser, so the same aliases
// are accepted there and stored verbatim.
func TestConfigSetFormatAcceptsAliases(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	for _, v := range []string{"ndjson", "yml", "YAML"} {
		if _, _, err := execCLI(t, srv.URL, "config", "set", "format", v); err != nil {
			t.Fatalf("config set format %q should be accepted: %v", v, err)
		}
	}
	if _, errOut, err := execCLI(t, srv.URL, "config", "set", "format", "toml"); err == nil {
		t.Fatalf("config set format toml should fail")
	} else if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
		t.Fatalf("invalid config format should be fixable_by agent: %v", m)
	}
}
