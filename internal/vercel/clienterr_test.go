package vercel

import (
	"context"
	"testing"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
)

func TestDoRejectsUnmarshalableBodyAsAgentError(t *testing.T) {
	c := mustClient(t, Config{BaseURL: "https://example.invalid", Token: "tok"})
	// A channel can't be JSON-encoded; the body marshal fails before any HTTP
	// request is built, and must surface as fixable_by:agent — the caller
	// constructed an invalid body, not a transient/network condition.
	_, err := c.Do(context.Background(), "POST", "/v1/anything", nil, make(chan int))
	var aerr *agenterrors.APIError
	if !agenterrors.As(err, &aerr) {
		t.Fatalf("error is not *APIError: %v", err)
	}
	if aerr.FixableBy != agenterrors.FixableByAgent {
		t.Fatalf("fixable_by = %q; want agent", aerr.FixableBy)
	}
}
