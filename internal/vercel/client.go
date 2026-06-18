// Package vercel is the Vercel REST API client: a dependency-injected HTTP
// transport (Bearer auth against https://api.vercel.com), scope handling
// (teamId/slug query params), 429/5xx retry with backoff, and error mapping to
// the agent-* fixable_by contract.
package vercel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
)

// DefaultBaseURL is the public Vercel REST API host.
const DefaultBaseURL = "https://api.vercel.com"

// Doer is the injected HTTP seam so tests run without real network access.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Config configures a Client. Only Token is required for authenticated calls.
type Config struct {
	BaseURL    string
	Token      string
	Scope      string // team slug or id; "" = personal account
	HTTP       Doer
	Timeout    time.Duration
	UserAgent  string
	Debug      io.Writer                       // if non-nil, log redacted request lines
	MaxRetries int                             // default 3
	Backoff    func(attempt int) time.Duration // injectable; default exponential
}

type Client struct {
	cfg     Config
	http    Doer
	base    *url.URL
	backoff func(int) time.Duration
}

// New builds a Client, applying defaults for any unset Config field.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	base, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || base.Host == "" {
		return nil, agenterrors.Newf(agenterrors.FixableByAgent, "invalid base URL %q", cfg.BaseURL)
	}
	if cfg.HTTP == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		cfg.HTTP = &http.Client{Timeout: timeout}
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "agent-vercel"
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	backoff := cfg.Backoff
	if backoff == nil {
		backoff = func(attempt int) time.Duration {
			d := time.Duration(1<<uint(attempt)) * 500 * time.Millisecond
			if d > 30*time.Second {
				d = 30 * time.Second
			}
			return d
		}
	}
	return &Client{cfg: cfg, http: cfg.HTTP, base: base, backoff: backoff}, nil
}

// Get is a convenience wrapper for Do with the GET method and no body.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil)
}

// Do performs a request, injecting auth + scope, retrying transient failures,
// and mapping errors to the fixable_by taxonomy. body, if non-nil, is JSON
// encoded. The decoded raw response body is returned on 2xx.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any) (json.RawMessage, error) {
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
		}
		bodyBytes = b
	}

	u := c.buildURL(path, query)

	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, agenterrors.Wrap(ctx.Err(), agenterrors.FixableByRetry)
			case <-time.After(c.backoff(attempt - 1)):
			}
		}

		var reader io.Reader
		if bodyBytes != nil {
			reader = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
		if err != nil {
			return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
		}
		c.setAuthHeaders(req)
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		c.debugf("%s %s", method, u.RequestURI())

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = agenterrors.Wrap(err, agenterrors.FixableByRetry)
			continue // network errors are retryable
		}
		raw, apiErr := readResponse(resp)
		if apiErr == nil {
			return raw, nil
		}
		if apiErr.FixableBy == agenterrors.FixableByRetry && attempt < c.cfg.MaxRetries {
			lastErr = apiErr
			if d, ok := retryAfter(resp); ok {
				select {
				case <-ctx.Done():
					return nil, agenterrors.Wrap(ctx.Err(), agenterrors.FixableByRetry)
				case <-time.After(d):
				}
			}
			continue
		}
		return nil, apiErr
	}
	if lastErr == nil {
		lastErr = agenterrors.New("request failed after retries", agenterrors.FixableByRetry)
	}
	return nil, lastErr
}

// buildURL joins the request path onto the base URL and encodes the query with
// scope (teamId/slug) applied. Returns the url.URL so callers can use both
// String() (request target) and RequestURI() (debug logging).
func (c *Client) buildURL(path string, query url.Values) url.URL {
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = c.withScope(query).Encode()
	return u
}

// setAuthHeaders applies the Bearer token and User-Agent shared by every request.
func (c *Client) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
}

// withScope adds teamId or slug to the query when a scope is set. A "team_"
// prefix marks an id; anything else is treated as a slug.
func (c *Client) withScope(query url.Values) url.Values {
	out := url.Values{}
	for k, v := range query {
		out[k] = v
	}
	if c.cfg.Scope != "" {
		if strings.HasPrefix(c.cfg.Scope, "team_") {
			out.Set("teamId", c.cfg.Scope)
		} else {
			out.Set("slug", c.cfg.Scope)
		}
	}
	return out
}

func (c *Client) debugf(format string, args ...any) {
	if c.cfg.Debug == nil {
		return
	}
	_, _ = fmt.Fprintf(c.cfg.Debug, "[vercel] "+format+"\n", args...)
}

// readResponse reads the body and maps non-2xx to a structured APIError. The
// Vercel error envelope is {error:{code,message}}.
func readResponse(resp *http.Response) (json.RawMessage, *agenterrors.APIError) {
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByRetry)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return json.RawMessage(data), nil
	}

	fixable := mapStatus(resp.StatusCode)
	msg := strings.TrimSpace(string(data))
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &env) == nil && env.Error.Message != "" {
		msg = env.Error.Message
		if env.Error.Code != "" {
			msg = env.Error.Code + ": " + msg
		}
	}
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	aerr := agenterrors.Newf(fixable, "vercel API %d: %s", resp.StatusCode, msg)
	return nil, withHint(aerr, resp.StatusCode)
}

func mapStatus(code int) agenterrors.FixableBy {
	switch {
	case code == 429 || code >= 500:
		return agenterrors.FixableByRetry
	case code == 401 || code == 402 || code == 403:
		return agenterrors.FixableByHuman
	default:
		return agenterrors.FixableByAgent
	}
}

func withHint(e *agenterrors.APIError, code int) *agenterrors.APIError {
	switch code {
	case 401:
		return e.WithHint("token is invalid or expired; run 'agent-vercel auth add' with a fresh token")
	case 402:
		return e.WithHint("the account has a billing issue; resolve payment in the Vercel dashboard")
	case 403:
		return e.WithHint("token lacks access to this resource or scope; check --scope and the token's team access")
	}
	return e
}

// retryAfter returns the Retry-After delay if the server set one (seconds).
func retryAfter(resp *http.Response) (time.Duration, bool) {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	return 0, false
}

// StreamLines GETs a streaming (NDJSON) endpoint and collects whole-object lines
// until the window elapses, maxLines is reached, or the server closes the
// stream. Vercel exposes runtime logs as an open-ended stream with no bounding
// query params, so a plain buffered read would block until the client timeout
// and lose partial data; this bounds the read and keeps whatever arrived.
func (c *Client) StreamLines(ctx context.Context, path string, query url.Values, window time.Duration, maxLines int) ([]json.RawMessage, error) {
	u := c.buildURL(path, query)

	ctx, cancel := context.WithTimeout(ctx, window)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	c.setAuthHeaders(req)
	c.debugf("GET %s (stream, window=%s)", u.RequestURI(), window)

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil // window elapsed before any response: no logs
		}
		return nil, agenterrors.Wrap(err, agenterrors.FixableByRetry)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, apiErr := readResponse(resp) // closes body, maps status → fixable_by
		return nil, apiErr
	}
	defer func() { _ = resp.Body.Close() }()

	lines := make(chan json.RawMessage)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			b := bytes.TrimSpace(sc.Bytes())
			if len(b) == 0 || b[0] != '{' {
				continue
			}
			cp := append(json.RawMessage(nil), b...)
			select {
			case lines <- cp:
			case <-ctx.Done():
				return
			}
		}
	}()

	var out []json.RawMessage
	for {
		select {
		case <-ctx.Done():
			return out, nil // window elapsed: return what arrived
		case line, ok := <-lines:
			if !ok {
				return out, nil // stream closed (EOF)
			}
			out = append(out, line)
			if maxLines > 0 && len(out) >= maxLines {
				return out, nil
			}
		}
	}
}
