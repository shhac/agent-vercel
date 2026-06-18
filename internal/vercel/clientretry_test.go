package vercel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
)

// errDoer always fails with a network error, exercising the network-error retry
// branch and its eventual exhaustion.
type errDoer struct{ calls int32 }

func (d *errDoer) Do(*http.Request) (*http.Response, error) {
	atomic.AddInt32(&d.calls, 1)
	return nil, errors.New("dial tcp: connection refused")
}

func TestNetworkErrorsRetryThenFail(t *testing.T) {
	d := &errDoer{}
	c := mustClient(t, Config{BaseURL: "https://example.invalid", Token: "tok", HTTP: d, MaxRetries: 3})

	_, err := c.Get(context.Background(), "/v2/user", nil)
	var aerr *agenterrors.APIError
	if !agenterrors.As(err, &aerr) || aerr.FixableBy != agenterrors.FixableByRetry {
		t.Fatalf("want retry-fixable APIError, got %v", err)
	}
	if got := atomic.LoadInt32(&d.calls); got != 4 {
		t.Fatalf("Do calls = %d; want 4 (initial + 3 retries)", got)
	}
}

func TestRetryClassExhaustsAndReturnsLastError(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"down","message":"server error"}}`))
	}))
	defer srv.Close()
	c := mustClient(t, Config{BaseURL: srv.URL, Token: "tok", MaxRetries: 2})

	_, err := c.Get(context.Background(), "/v2/user", nil)
	var aerr *agenterrors.APIError
	if !agenterrors.As(err, &aerr) || aerr.FixableBy != agenterrors.FixableByRetry {
		t.Fatalf("want retry-fixable APIError after exhaustion, got %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("hits = %d; want 3 (initial + 2 retries)", got)
	}
}

// cancelOnCallDoer cancels the request context on the first call and returns a
// retryable status, so the next iteration's backoff select observes ctx.Done.
type cancelOnCallDoer struct{ cancel context.CancelFunc }

func (d *cancelOnCallDoer) Do(*http.Request) (*http.Response, error) {
	d.cancel()
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"boom"}}`)),
		Header:     make(http.Header),
	}, nil
}

func TestContextCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := mustClient(t, Config{
		BaseURL:    "https://example.invalid",
		Token:      "tok",
		HTTP:       &cancelOnCallDoer{cancel: cancel},
		MaxRetries: 3,
		Backoff:    func(int) time.Duration { return time.Hour }, // never elapses; ctx.Done wins
	})

	_, err := c.Get(ctx, "/v2/user", nil)
	var aerr *agenterrors.APIError
	if !agenterrors.As(err, &aerr) || aerr.FixableBy != agenterrors.FixableByRetry {
		t.Fatalf("want retry-fixable APIError from cancellation, got %v", err)
	}
}

func TestStreamLinesNon2xxReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"down","message":"boom"}}`))
	}))
	defer srv.Close()
	c := mustClient(t, Config{BaseURL: srv.URL, Token: "tok"})

	_, err := c.StreamLines(context.Background(), "/v1/projects/p/deployments/d/runtime-logs", nil, time.Second, 10)
	var aerr *agenterrors.APIError
	if !agenterrors.As(err, &aerr) || aerr.FixableBy != agenterrors.FixableByRetry {
		t.Fatalf("want retry-fixable APIError from non-2xx stream, got %v", err)
	}
	if !strings.Contains(aerr.Message, "boom") {
		t.Fatalf("message = %q; want it to include the API message", aerr.Message)
	}
}
