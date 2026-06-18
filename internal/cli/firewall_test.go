package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

// All firewall reads run under a SLUG scope (--scope acme). That exercises the
// slug→teamId resolution: the Firewall API requires an explicit teamId, and the
// mock 400s without it, so these also guard against regressing that resolution.

// --- in plan + data ---

func TestFirewallConfig(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "config", "prj_web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["enabled"] != true {
		t.Fatalf("expected enabled: %v", m)
	}
	rules, ok := m["custom_rules"].([]any)
	if !ok || len(rules) != 1 || rules[0] != "block-bad-bots" {
		t.Fatalf("custom_rules = %v; want [block-bad-bots]", m["custom_rules"])
	}
	if m["ip_rules"].(float64) != 1 {
		t.Fatalf("ip_rules = %v; want 1", m["ip_rules"])
	}
	managed, ok := m["managed_rulesets"].([]any)
	if !ok || len(managed) != 1 || managed[0] != "owasp" {
		t.Fatalf("managed_rulesets = %v; want [owasp]", m["managed_rulesets"])
	}
}

func TestFirewallConfigFull(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "config", "prj_web", "--full")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["firewallEnabled"] != true || m["version"].(float64) != 7 {
		t.Fatalf("--full should expose raw config: %v", m)
	}
}

func TestFirewallAttackStatus(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "attack-status", "prj_web", "--since", "7")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["under_attack"] != true || m["anomalies"].(float64) != 1 {
		t.Fatalf("attack-status = %v; want under_attack true, 1 anomaly", m)
	}
}

func TestFirewallBypass(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "bypass", "prj_web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["result"] == nil {
		t.Fatalf("bypass should print the raw payload: %v", m)
	}
}

// --- no data (firewall present but off / not under attack) ---

func TestFirewallConfigDisabled(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "config", "prj_api")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["enabled"] != false {
		t.Fatalf("expected enabled:false: %v", m)
	}
	for _, k := range []string{"custom_rules", "ip_rules", "managed_rulesets"} {
		if _, present := m[k]; present {
			t.Fatalf("empty %s should be omitted: %v", k, m)
		}
	}
}

func TestFirewallAttackStatusClear(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "attack-status", "prj_api")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["under_attack"] != false || m["anomalies"].(float64) != 0 {
		t.Fatalf("clear attack-status = %v; want under_attack false, 0 anomalies", m)
	}
}

// --- unavailable / out of plan ---

func TestFirewallBypassUnavailableOutOfPlan(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// prj_free's plan lacks IP Bypass → the API 404s; surface it as a clean
	// structured error, not a crash.
	_, errOut, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "bypass", "prj_free")
	if err == nil {
		t.Fatal("expected a plan-unavailable error")
	}
	m := decodeJSON(t, errOut)
	if m["fixable_by"] != "agent" {
		t.Fatalf("out-of-plan bypass should be a structured agent error: %v", m)
	}
}

func TestFirewallConfigNotFound(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "config", "prj_none")
	if err == nil {
		t.Fatal("expected a config-not-found error")
	}
	if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
		t.Fatalf("missing config should be a structured agent error: %v", m)
	}
}

// --- regression: firewall requires teamId, must work under a slug scope ---

func TestFirewallResolvesSlugScopeToTeamID(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// The bug fix: a slug scope must resolve to the explicit teamId the Firewall
	// API requires. The mock 400s on a slug-without-teamId, so success here proves
	// `firewall config --scope <slug>` sends teamId, not slug.
	out, _, err := execCLI(t, srv.URL, "--scope", "acme", "firewall", "config", "prj_web")
	if err != nil {
		t.Fatalf("slug-scope firewall should resolve to teamId and succeed: %v", err)
	}
	if decodeJSON(t, out)["enabled"] != true {
		t.Fatalf("slug-scope firewall config = %s", out)
	}

	// A team-id scope works too (resolveTeamID short-circuits on the team_ prefix).
	out, _, err = execCLI(t, srv.URL, "--scope", "team_abc", "firewall", "attack-status", "prj_web")
	if err != nil {
		t.Fatalf("team-id-scope firewall should succeed: %v", err)
	}
	if decodeJSON(t, out)["under_attack"] != true {
		t.Fatalf("team-id-scope attack-status = %s", out)
	}
}
