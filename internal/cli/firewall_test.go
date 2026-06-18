package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestFirewallConfig(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "firewall", "config", "prj_web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["enabled"] != true {
		t.Fatalf("expected enabled: %v", m)
	}
	// only the active custom rule is surfaced
	rules, ok := m["custom_rules"].([]any)
	if !ok || len(rules) != 1 || rules[0] != "block-bad-bots" {
		t.Fatalf("custom_rules = %v; want [block-bad-bots]", m["custom_rules"])
	}
	if m["ip_rules"].(float64) != 1 {
		t.Fatalf("ip_rules = %v; want 1", m["ip_rules"])
	}
	// only active managed rulesets
	managed, ok := m["managed_rulesets"].([]any)
	if !ok || len(managed) != 1 || managed[0] != "owasp" {
		t.Fatalf("managed_rulesets = %v; want [owasp]", m["managed_rulesets"])
	}
}

func TestFirewallConfigFull(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --full returns the raw config (camelCase firewallEnabled, version).
	out, _, err := execCLI(t, srv.URL, "firewall", "config", "prj_web", "--full")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["firewallEnabled"] != true || m["version"].(float64) != 7 {
		t.Fatalf("--full should expose raw config: %v", m)
	}
}

func TestFirewallAttackStatus(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "firewall", "attack-status", "prj_web", "--since", "7")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["under_attack"] != true || m["anomalies"].(float64) != 1 {
		t.Fatalf("attack-status = %v; want under_attack true, 1 anomaly", m)
	}
}

func TestFirewallBypass(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "firewall", "bypass", "prj_web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// printed raw; the bypass result array round-trips
	m := decodeJSON(t, out)
	if _, ok := m["result"].([]any); !ok {
		t.Fatalf("bypass should print the raw payload: %v", m)
	}
}
