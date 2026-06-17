package vercel

import (
	"context"
	"encoding/json"
	"net/url"
)

// Page is the Vercel timestamp-cursor pagination block. Next is the ms cursor
// for the following page (nil when there are no more pages).
type Page struct {
	Count int    `json:"count"`
	Next  *int64 `json:"next"`
	Prev  *int64 `json:"prev"`
}

// listRaw fetches a list endpoint and returns the raw item objects under key
// plus the pagination block, leaving per-item decoding (compact vs --full) to
// the caller.
func (c *Client) listRaw(ctx context.Context, path, key string, q url.Values) ([]json.RawMessage, Page, error) {
	raw, err := c.Get(ctx, path, q)
	if err != nil {
		return nil, Page{}, err
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, Page{}, err
	}
	var items []json.RawMessage
	if v, ok := env[key]; ok {
		_ = json.Unmarshal(v, &items)
	}
	var p Page
	if v, ok := env["pagination"]; ok {
		_ = json.Unmarshal(v, &p)
	}
	return items, p, nil
}

// ListDeployments — GET /v6/deployments (org/scope-wide, filterable via q).
func (c *Client) ListDeployments(ctx context.Context, q url.Values) ([]json.RawMessage, Page, error) {
	return c.listRaw(ctx, "/v6/deployments", "deployments", q)
}

// GetDeployment — GET /v13/deployments/{idOrUrl} (single, richest payload).
func (c *Client) GetDeployment(ctx context.Context, idOrURL string) (json.RawMessage, error) {
	return c.Get(ctx, "/v13/deployments/"+url.PathEscape(idOrURL), nil)
}

// ListWebhooks — GET /v1/webhooks. The team/account webhooks (which events
// they subscribe to and which projects they target). Optional projectId filter
// via q. The payload may be a bare array or wrapped under "webhooks".
func (c *Client) ListWebhooks(ctx context.Context, q url.Values) ([]json.RawMessage, error) {
	raw, err := c.Get(ctx, "/v1/webhooks", q)
	if err != nil {
		return nil, err
	}
	return decodeKeyedArray(raw, "webhooks"), nil
}

// DeploymentChecks — GET /v1/deployments/{idOrUrl}/checks. The CI / integration
// checks attached to a deployment (e.g. a failing check blocking promotion).
// Not paginated; the items live under the "checks" key.
func (c *Client) DeploymentChecks(ctx context.Context, idOrURL string) ([]json.RawMessage, error) {
	items, _, err := c.listRaw(ctx, "/v1/deployments/"+url.PathEscape(idOrURL)+"/checks", "checks", nil)
	return items, err
}

// ListProjects — GET /v9/projects.
func (c *Client) ListProjects(ctx context.Context, q url.Values) ([]json.RawMessage, Page, error) {
	return c.listRaw(ctx, "/v9/projects", "projects", q)
}

// GetProject — GET /v9/projects/{idOrName}.
func (c *Client) GetProject(ctx context.Context, idOrName string) (json.RawMessage, error) {
	return c.Get(ctx, "/v9/projects/"+url.PathEscape(idOrName), nil)
}

// ProjectCustomEnvironments — GET /v9/projects/{idOrName}/custom-environments.
// The custom (non-standard) deployment environments a project defines, e.g. a
// "staging" env bound to a branch. The payload wraps them under
// "environments"; a bare array is also tolerated.
func (c *Client) ProjectCustomEnvironments(ctx context.Context, idOrName string) ([]json.RawMessage, error) {
	raw, err := c.Get(ctx, "/v9/projects/"+url.PathEscape(idOrName)+"/custom-environments", nil)
	if err != nil {
		return nil, err
	}
	return decodeKeyedArray(raw, "environments"), nil
}

// ProjectCrons — GET /v1/projects/{idOrName}/crons. The cron schedule a
// project's current production deployment runs (path + schedule per job), plus
// whether crons are enabled. The payload nests the data under a "crons" key.
func (c *Client) ProjectCrons(ctx context.Context, idOrName string) (json.RawMessage, error) {
	return c.Get(ctx, "/v1/projects/"+url.PathEscape(idOrName)+"/crons", nil)
}

// RollingRelease — GET /v1/projects/{idOrName}/rolling-release.
func (c *Client) RollingRelease(ctx context.Context, idOrName string) (json.RawMessage, error) {
	return c.Get(ctx, "/v1/projects/"+url.PathEscape(idOrName)+"/rolling-release", nil)
}
