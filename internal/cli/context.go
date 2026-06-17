package cli

import (
	"os"
	"time"

	"github.com/shhac/agent-vercel/internal/credential"
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/vercel"
)

// newCredStore constructs the credential store. It is a package var so tests
// can substitute an in-memory Keychain + temp file and never touch the real
// macOS Keychain. Production uses credential.New.
var newCredStore = credential.New

// resolved bundles the active client with the store and the resolved selectors,
// so commands can both call the API and persist resolution metadata (e.g. cache
// the username or team list) without re-resolving.
type resolved struct {
	client *vercel.Client
	store  *credential.Store
	creds  *credential.Credentials
	auth   *credential.Auth // the stored credential used, or nil when token came from env
	scope  string
}

// resolveClient resolves the active credential and scope (per the documented
// order) and builds a Vercel client. It never returns secret material to the
// caller beyond what the client needs internally.
//
//	auth:  --auth label → VERCEL_TOKEN env → stored default credential
//	scope: --scope flag → VERCEL_SCOPE / VERCEL_TEAM_ID env → stored default
func resolveClient(g *GlobalFlags) (*resolved, error) {
	store, err := newCredStore()
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByHuman)
	}
	creds, err := store.Load()
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByHuman)
	}

	token, usedAuth, err := resolveToken(g, creds)
	if err != nil {
		return nil, err
	}

	scope := g.Scope
	if scope == "" {
		scope = firstNonEmpty(os.Getenv("VERCEL_SCOPE"), os.Getenv("VERCEL_TEAM_ID"))
	}
	if scope == "" {
		scope = creds.DefaultScope
	}

	cfg := vercel.Config{
		BaseURL: g.BaseURL,
		Token:   token,
		Scope:   scope,
		Timeout: time.Duration(g.TimeoutMS) * time.Millisecond,
	}
	if g.Debug {
		cfg.Debug = os.Stderr
	}
	client, err := vercel.New(cfg)
	if err != nil {
		return nil, err
	}
	return &resolved{client: client, store: store, creds: creds, auth: usedAuth, scope: scope}, nil
}

// resolveToken returns the active token and, when it came from the store, the
// credential it belongs to (nil for an env-provided token).
func resolveToken(g *GlobalFlags, creds *credential.Credentials) (string, *credential.Auth, error) {
	if g.Auth != "" {
		for i := range creds.Auths {
			if creds.Auths[i].Label == g.Auth {
				a := &creds.Auths[i]
				if isPlaceholder(a.Secret) {
					return "", nil, missingSecretErr(a.Label)
				}
				return a.Secret, a, nil
			}
		}
		return "", nil, agenterrors.Newf(agenterrors.FixableByAgent, "no stored credential labeled %q", g.Auth).
			WithHint("run 'agent-vercel auth list' to see stored labels")
	}
	if env := os.Getenv("VERCEL_TOKEN"); env != "" {
		return env, nil, nil
	}
	if creds.DefaultAuth != "" {
		for i := range creds.Auths {
			if creds.Auths[i].Label == creds.DefaultAuth {
				a := &creds.Auths[i]
				if isPlaceholder(a.Secret) {
					return "", nil, missingSecretErr(a.Label)
				}
				return a.Secret, a, nil
			}
		}
	}
	return "", nil, agenterrors.New("no Vercel credential configured", agenterrors.FixableByHuman).
		WithHint("set VERCEL_TOKEN and run 'agent-vercel auth add', or pass --auth <label>")
}

func missingSecretErr(label string) error {
	return agenterrors.Newf(agenterrors.FixableByHuman, "credential %q has no secret in the Keychain", label).
		WithHint("re-add it: set VERCEL_TOKEN then run 'agent-vercel auth add --label " + label + "'")
}

// isPlaceholder mirrors credential.isPlaceholder for the hydrated-but-empty case
// (an unhydrated keychain placeholder means the backing entry is gone).
func isPlaceholder(v string) bool { return v == "" || v == "__KEYCHAIN__" }

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
