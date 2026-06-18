package vercel

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// FirewallConfig — GET /v1/security/firewall/config/active. The active WAF
// configuration for a project: custom rules, IP blocklist, managed rulesets,
// bot/attack-challenge state. projectId is required. Spec-documented read;
// validate live behind the integration build tag before relying on the shape.
func (c *Client) FirewallConfig(ctx context.Context, projectID string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("projectId", projectID)
	return c.Get(ctx, "/v1/security/firewall/config/active", q)
}

// FirewallAttackStatus — GET /v1/security/firewall/attack-status. Active-attack
// / DDoS anomalies detected for a project over the last sinceDays (default 1).
func (c *Client) FirewallAttackStatus(ctx context.Context, projectID string, sinceDays int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("projectId", projectID)
	if sinceDays > 0 {
		q.Set("since", strconv.Itoa(sinceDays))
	}
	return c.Get(ctx, "/v1/security/firewall/attack-status", q)
}

// FirewallBypass — GET /v1/security/firewall/bypass. System bypass rules
// (sources allowed to skip the firewall) for a project.
func (c *Client) FirewallBypass(ctx context.Context, projectID string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("projectId", projectID)
	return c.Get(ctx, "/v1/security/firewall/bypass", q)
}
