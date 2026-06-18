package vercel

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// The Vercel Firewall endpoints require an explicit teamId in the query (they
// reject the slug that withScope would otherwise add). Callers resolve the
// active scope to a teamId and pass it; an empty teamID means the personal
// account. Requests go through the unscoped client so only the explicit teamId
// is sent. Spec-documented reads; validate live behind the integration tag.

func firewallQuery(teamID, projectID string) url.Values {
	q := url.Values{}
	q.Set("projectId", projectID)
	if teamID != "" {
		q.Set("teamId", teamID)
	}
	return q
}

// FirewallConfig — GET /v1/security/firewall/config/active. The active WAF
// configuration for a project: custom rules, IP blocklist, managed rulesets,
// bot/attack-challenge state.
func (c *Client) FirewallConfig(ctx context.Context, teamID, projectID string) (json.RawMessage, error) {
	return c.unscoped().Get(ctx, "/v1/security/firewall/config/active", firewallQuery(teamID, projectID))
}

// FirewallAttackStatus — GET /v1/security/firewall/attack-status. Active-attack
// / DDoS anomalies detected for a project over the last sinceDays (default 1).
func (c *Client) FirewallAttackStatus(ctx context.Context, teamID, projectID string, sinceDays int) (json.RawMessage, error) {
	q := firewallQuery(teamID, projectID)
	if sinceDays > 0 {
		q.Set("since", strconv.Itoa(sinceDays))
	}
	return c.unscoped().Get(ctx, "/v1/security/firewall/attack-status", q)
}

// FirewallBypass — GET /v1/security/firewall/bypass. System bypass rules
// (sources allowed to skip the firewall) for a project.
func (c *Client) FirewallBypass(ctx context.Context, teamID, projectID string) (json.RawMessage, error) {
	return c.unscoped().Get(ctx, "/v1/security/firewall/bypass", firewallQuery(teamID, projectID))
}
