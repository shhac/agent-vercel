package vercel

import (
	"context"
	"encoding/json"
	"net/url"
)

// Drains — GET /v1/drains. The observability data exports (log / trace /
// analytics / speed_insights) configured for the scope; the only REST handle on
// where otherwise-unqueryable observability data is shipped. The payload may be
// a bare array or wrapped under "drains". Spec-documented but not live-validated.
func (c *Client) Drains(ctx context.Context, q url.Values) ([]json.RawMessage, error) {
	raw, err := c.Get(ctx, "/v1/drains", q)
	if err != nil {
		return nil, err
	}
	return decodeKeyedArray(raw, "drains"), nil
}

// GetDrain — GET /v1/drains/{id}.
func (c *Client) GetDrain(ctx context.Context, id string) (json.RawMessage, error) {
	return c.Get(ctx, "/v1/drains/"+url.PathEscape(id), nil)
}
