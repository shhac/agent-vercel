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
		return labeledSecret(creds, g.Auth,
			agenterrors.Newf(agenterrors.FixableByAgent, "no stored credential labeled %q", g.Auth).
				WithHint("run 'agent-vercel auth list' to see stored labels"))
	}
	if env := os.Getenv("VERCEL_TOKEN"); env != "" {
		return env, nil, nil
	}
	if creds.DefaultAuth != "" {
		return labeledSecret(creds, creds.DefaultAuth,
			agenterrors.Newf(agenterrors.FixableByHuman, "default credential %q is not stored", creds.DefaultAuth).
				WithHint("run 'agent-vercel auth list', then set a valid default with 'agent-vercel auth default <label>'"))
	}
	return "", nil, agenterrors.New("no Vercel credential configured", agenterrors.FixableByHuman).
		WithHint("set VERCEL_TOKEN and run 'agent-vercel auth add', or pass --auth <label>")
}

// labeledSecret resolves a stored credential's token by label, sharing the
// lookup-and-branch between the explicit --auth and default-credential paths.
// notFound is returned when no credential carries the label; it differs by
// source (an explicit --auth miss is agent-fixable, a stale default is
// human-fixable), so the caller supplies it.
func labeledSecret(creds *credential.Credentials, label string, notFound error) (string, *credential.Auth, error) {
	a, err := findAuthByLabel(creds, label)
	if err != nil {
		return "", nil, err
	}
	if a == nil {
		return "", nil, notFound
	}
	return a.Secret, a, nil
}

// findAuthByLabel returns the stored credential with the given label, or
// (nil, nil) when none matches. It returns a fixable_by:human error when a
// match exists but its secret is only a Keychain placeholder (the secret could
// not be hydrated), so both the explicit-label and default-label paths report
// the missing secret identically.
func findAuthByLabel(creds *credential.Credentials, label string) (*credential.Auth, error) {
	for i := range creds.Auths {
		if creds.Auths[i].Label == label {
			a := &creds.Auths[i]
			if credential.IsPlaceholder(a.Secret) {
				return nil, missingSecretErr(a.Label)
			}
			return a, nil
		}
	}
	return nil, nil
}

func missingSecretErr(label string) error {
	return agenterrors.Newf(agenterrors.FixableByHuman, "credential %q has no secret in the Keychain", label).
		WithHint("re-add it: set VERCEL_TOKEN then run 'agent-vercel auth add " + label + "'")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
