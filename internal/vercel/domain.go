package vercel

import (
	"context"
	"encoding/json"
	"net/url"
)

// ListDomains — GET /v5/domains (account/scope domains).
func (c *Client) ListDomains(ctx context.Context, q url.Values) ([]json.RawMessage, Page, error) {
	return c.listRaw(ctx, "/v5/domains", "domains", q)
}

// GetDomain — GET /v5/domains/{domain}. Unwraps the {domain:{…}} envelope.
func (c *Client) GetDomain(ctx context.Context, name string) (json.RawMessage, error) {
	raw, err := c.Get(ctx, "/v5/domains/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	return unwrap(raw, "domain"), nil
}

// DomainConfig — GET /v6/domains/{domain}/config (misconfiguration check).
func (c *Client) DomainConfig(ctx context.Context, name string) (json.RawMessage, error) {
	return c.Get(ctx, "/v6/domains/"+url.PathEscape(name)+"/config", nil)
}

// DomainRecords — GET /v5/domains/{domain}/records.
func (c *Client) DomainRecords(ctx context.Context, name string, q url.Values) ([]json.RawMessage, Page, error) {
	return c.listRaw(ctx, "/v5/domains/"+url.PathEscape(name)+"/records", "records", q)
}

// GetCert — GET /v8/certs/{id}.
func (c *Client) GetCert(ctx context.Context, id string) (json.RawMessage, error) {
	return c.Get(ctx, "/v8/certs/"+url.PathEscape(id), nil)
}

// ListCerts — GET /v9/certs. The scope's TLS certificates, for bulk
// expiry/renewal triage. The payload may be a bare array or wrapped under
// "certs". Spec-plausible but not live-validated; Vercel may not expose a
// scope-wide certs list (cf. the absent scope-wide alias list) — callers that
// 404 should fall back to GetCert by id.
func (c *Client) ListCerts(ctx context.Context, q url.Values) ([]json.RawMessage, error) {
	raw, err := c.Get(ctx, "/v9/certs", q)
	if err != nil {
		return nil, err
	}
	return decodeKeyedArray(raw, "certs"), nil
}

// DeploymentAliases — GET /v2/deployments/{id}/aliases.
func (c *Client) DeploymentAliases(ctx context.Context, idOrURL string, q url.Values) ([]json.RawMessage, Page, error) {
	return c.listRaw(ctx, "/v2/deployments/"+url.PathEscape(idOrURL)+"/aliases", "aliases", q)
}

// unwrap returns the sub-object under key, or the raw payload if absent.
func unwrap(raw json.RawMessage, key string) json.RawMessage {
	var env map[string]json.RawMessage
	if json.Unmarshal(raw, &env) == nil {
		if v, ok := env[key]; ok {
			return v
		}
	}
	return raw
}
