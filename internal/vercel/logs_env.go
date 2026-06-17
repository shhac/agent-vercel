package vercel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// DeploymentEvents — GET /v3/deployments/{idOrUrl}/events (build logs). The
// endpoint returns a JSON array (or, when streaming, NDJSON); both are handled.
func (c *Client) DeploymentEvents(ctx context.Context, idOrURL string, q url.Values) ([]json.RawMessage, error) {
	raw, err := c.Get(ctx, "/v3/deployments/"+url.PathEscape(idOrURL)+"/events", q)
	if err != nil {
		return nil, err
	}
	return decodeArrayOrStream(raw), nil
}

// RuntimeLogs — GET /v1/projects/{projectId}/deployments/{deploymentId}/runtime-logs.
// This is an open-ended NDJSON stream with no bounding query params, so it is
// read via StreamLines: collect logs for window, up to maxLines, then return.
func (c *Client) RuntimeLogs(ctx context.Context, projectID, deploymentID string, window time.Duration, maxLines int) ([]json.RawMessage, error) {
	path := "/v1/projects/" + url.PathEscape(projectID) + "/deployments/" + url.PathEscape(deploymentID) + "/runtime-logs"
	return c.StreamLines(ctx, path, url.Values{}, window, maxLines)
}

// ProjectEnv — GET /v10/projects/{idOrName}/env. Returns the env var objects
// (the API wraps them under "envs"; a bare array is also tolerated).
func (c *Client) ProjectEnv(ctx context.Context, idOrName string, q url.Values) ([]json.RawMessage, error) {
	raw, err := c.Get(ctx, "/v10/projects/"+url.PathEscape(idOrName)+"/env", q)
	if err != nil {
		return nil, err
	}
	return decodeKeyedArray(raw, "envs"), nil
}

// decodeArrayOrStream parses either a JSON array or an NDJSON stream of objects.
func decodeArrayOrStream(raw json.RawMessage) []json.RawMessage {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 {
		return nil
	}
	if t[0] == '[' {
		var a []json.RawMessage
		if json.Unmarshal(t, &a) == nil {
			return a
		}
	}
	var out []json.RawMessage
	for _, line := range bytes.Split(t, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		out = append(out, json.RawMessage(append([]byte(nil), line...)))
	}
	return out
}

// decodeKeyedArray returns the array under key from an object response, falling
// back to treating the whole payload as an array.
func decodeKeyedArray(raw json.RawMessage, key string) []json.RawMessage {
	var env map[string]json.RawMessage
	if json.Unmarshal(raw, &env) == nil {
		if v, ok := env[key]; ok {
			var a []json.RawMessage
			_ = json.Unmarshal(v, &a)
			return a
		}
	}
	return decodeArrayOrStream(raw)
}
