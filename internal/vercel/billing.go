package vercel

import (
	"context"
	"encoding/json"
	"net/url"
)

// BillingCharges — GET /v1/billing/charges (FOCUS v1.3, JSONL). Returns the
// team's billing/usage charges between from and to (ISO 8601 UTC date-times,
// 1-day granularity). The response is newline-delimited JSON.
//
// NOTE: the FOCUS record field names are validated against the OpenAPI spec
// only, not against live data (billing access needs a billing-role token).
// Treat the compact mapping in the cli layer as spec-validated, not
// live-validated.
func (c *Client) BillingCharges(ctx context.Context, from, to string) ([]json.RawMessage, error) {
	q := url.Values{}
	q.Set("from", from)
	q.Set("to", to)
	raw, err := c.Get(ctx, "/v1/billing/charges", q)
	if err != nil {
		return nil, err
	}
	return decodeArrayOrStream(raw), nil
}
