package vercel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// CancelDeployment — PATCH /v12/deployments/{id}/cancel.
func (c *Client) CancelDeployment(ctx context.Context, id string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPatch, "/v12/deployments/"+url.PathEscape(id)+"/cancel", nil, nil)
}

// PromoteDeployment — POST /v10/projects/{projectId}/promote/{deploymentId}.
// Does not rebuild; repoints production traffic to an existing deployment.
func (c *Client) PromoteDeployment(ctx context.Context, projectID, deploymentID string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPost, "/v10/projects/"+url.PathEscape(projectID)+"/promote/"+url.PathEscape(deploymentID), nil, nil)
}

// RollbackDeployment — POST /v1/projects/{projectId}/rollback/{deploymentId}.
func (c *Client) RollbackDeployment(ctx context.Context, projectID, deploymentID, description string) (json.RawMessage, error) {
	q := url.Values{}
	if description != "" {
		q.Set("description", description)
	}
	return c.Do(ctx, http.MethodPost, "/v1/projects/"+url.PathEscape(projectID)+"/rollback/"+url.PathEscape(deploymentID), q, nil)
}

// CreateDeployment — POST /v13/deployments. Used for redeploy (rebuild) by
// passing {deploymentId, name, target}.
func (c *Client) CreateDeployment(ctx context.Context, body any) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPost, "/v13/deployments", nil, body)
}

// CreateEnv — POST /v10/projects/{idOrName}/env.
func (c *Client) CreateEnv(ctx context.Context, project string, body any) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPost, "/v10/projects/"+url.PathEscape(project)+"/env", nil, body)
}

// DeleteEnv — DELETE /v9/projects/{idOrName}/env/{id}.
func (c *Client) DeleteEnv(ctx context.Context, project, envID string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodDelete, "/v9/projects/"+url.PathEscape(project)+"/env/"+url.PathEscape(envID), nil, nil)
}

// AddProjectDomain — POST /v10/projects/{idOrName}/domains.
func (c *Client) AddProjectDomain(ctx context.Context, project string, body any) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPost, "/v10/projects/"+url.PathEscape(project)+"/domains", nil, body)
}

// RemoveProjectDomain — DELETE /v9/projects/{idOrName}/domains/{domain}.
func (c *Client) RemoveProjectDomain(ctx context.Context, project, domain string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodDelete, "/v9/projects/"+url.PathEscape(project)+"/domains/"+url.PathEscape(domain), nil, nil)
}

// VerifyProjectDomain — POST /v9/projects/{idOrName}/domains/{domain}/verify.
func (c *Client) VerifyProjectDomain(ctx context.Context, project, domain string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPost, "/v9/projects/"+url.PathEscape(project)+"/domains/"+url.PathEscape(domain)+"/verify", nil, nil)
}

// AssignAlias — POST /v2/deployments/{id}/aliases.
func (c *Client) AssignAlias(ctx context.Context, deploymentID, alias string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPost, "/v2/deployments/"+url.PathEscape(deploymentID)+"/aliases", nil, map[string]any{"alias": alias})
}

// DeleteAlias — DELETE /v2/aliases/{aliasId}.
func (c *Client) DeleteAlias(ctx context.Context, aliasID string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodDelete, "/v2/aliases/"+url.PathEscape(aliasID), nil, nil)
}

// SetAliasProtectionBypass — PATCH /aliases/{id}/protection-bypass. Creates a
// shareable bypass link for a deployment-protected alias (optionally with a TTL)
// or revokes an existing one. The returned bypass secret is the share link the
// caller asked to create — distinct from the agent-vercel auth token, which is
// never emitted.
func (c *Client) SetAliasProtectionBypass(ctx context.Context, id string, body any) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPatch, "/aliases/"+url.PathEscape(id)+"/protection-bypass", nil, body)
}
