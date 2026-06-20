package cli

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/shhac/agent-vercel/internal/credential"
	"github.com/shhac/agent-vercel/internal/dialog"
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/vercel"
	"github.com/spf13/cobra"
)

// promptSecret opens a native OS dialog for secret entry. It is a package var so
// tests substitute a stub and never pop a real dialog.
var promptSecret = dialog.Secret

// storeNewToken verifies a freshly-entered token (best-effort GET /v2/user, to
// capture identity for `auth list` and catch a bad token early), then stores it.
// Verification failure never blocks the store — the token is saved and the
// returned map reports verified:false with a hint to run `auth test`.
func storeNewToken(g *GlobalFlags, label, tok string) (map[string]any, error) {
	username, userID, verified, note := verifyNewToken(g, tok)
	store, err := newCredStore()
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByHuman)
	}
	if err := store.Upsert(credential.Auth{
		Label: label, Type: credential.AuthToken, Secret: tok, Username: username, UserID: userID,
	}); err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByHuman)
	}
	out := map[string]any{"label": label, "type": string(credential.AuthToken), "stored": true, "verified": verified}
	if verified {
		out["username"] = username
	} else {
		out["hint"] = note
	}
	return out, nil
}

// verifyNewToken does a best-effort identity lookup with a not-yet-stored token.
func verifyNewToken(g *GlobalFlags, tok string) (username, userID string, verified bool, note string) {
	c, err := vercel.New(vercel.Config{BaseURL: g.BaseURL, Token: tok, Scope: g.Scope, Timeout: time.Duration(g.TimeoutMS) * time.Millisecond})
	if err != nil {
		return "", "", false, "stored, but the token could not be verified"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	u, err := c.GetUser(ctx)
	if err != nil {
		return "", "", false, "stored, but the token could not be verified — run 'agent-vercel auth test'"
	}
	return u.Username, u.ID, true, ""
}

// registerAuth wires the `auth` group: management of the credential(s) — the
// secret half of the credential/scope split. The secret lives in the Keychain
// and is never printed back. Each credential carries a type discriminator
// (currently always "token") so auth need not always be a bare access token.
func registerAuth(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage stored Vercel credential(s) (secret; kept in the Keychain)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var form bool

	add := &cobra.Command{
		Use:   "add [label]",
		Short: "Store a credential in the Keychain (from $VERCEL_TOKEN or --form dialog)",
		Long: "Stores a Vercel access token in the Keychain under the given label\n" +
			"(default \"default\"). With --form, a native OS dialog prompts the human for\n" +
			"the token so it never appears in the agent's conversation. Otherwise the\n" +
			"token is read from $VERCEL_TOKEN. The secret is never echoed or written to\n" +
			"the file.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			label := labelArg(args)
			tok, err := readNewToken(form)
			if err != nil {
				return err
			}
			out, err := storeNewToken(g, label, tok)
			if err != nil {
				return err
			}
			return printSingle(g, out)
		},
	}
	add.Flags().BoolVar(&form, "form", false, "prompt for the token via a native OS dialog (keeps it out of the conversation)")

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List stored credentials and where each secret lives (never the secret)",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			store, err := newCredStore()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			creds, err := store.Load()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			statuses, err := store.SecretStatuses()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			rows := make([]any, 0, len(creds.Auths))
			for _, a := range creds.Auths {
				rows = append(rows, map[string]any{
					"label":         a.Label,
					"type":          string(a.Type),
					"default":       a.Label == creds.DefaultAuth,
					"username":      a.Username,
					"secret_status": string(statuses[a.Label]),
				})
			}
			return emitList(g, rows, nil)
		},
	}

	test := &cobra.Command{
		Use:     "test",
		Aliases: []string{"whoami"},
		Short:   "Verify the active credential and show the token owner (GET /v2/user)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			user, err := r.client.GetUser(cmd.Context())
			if err != nil {
				return err
			}
			// Cache the resolved username back onto the stored credential
			// (best-effort) so `auth list` can show it.
			if r.auth != nil && user.Username != "" && r.auth.Username != user.Username {
				a := *r.auth
				a.Username = user.Username
				a.UserID = user.ID
				_ = r.store.Upsert(a)
			}
			scope := r.scope
			if scope == "" {
				scope = "(personal account)"
			}
			return printSingle(g, map[string]any{
				"user_id":  user.ID,
				"username": user.Username,
				"email":    user.Email,
				"scope":    scope,
			})
		},
	}

	setDefault := &cobra.Command{
		Use:   "set-default <label>",
		Short: "Set the default credential label",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := newCredStore()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			if err := store.SetDefaultAuth(args[0]); err != nil {
				return agenterrors.Newf(agenterrors.FixableByAgent, "%v", err).
					WithHint("run 'agent-vercel auth list' to see stored labels")
			}
			return printSingle(g, map[string]any{"default_auth": args[0]})
		},
	}

	remove := &cobra.Command{
		Use:   "remove <label>",
		Short: "Remove a stored credential and its Keychain secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := newCredStore()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			if err := store.Remove(args[0]); err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			return printSingle(g, map[string]any{"removed": args[0]})
		},
	}

	cmd.AddCommand(add, list, test, setDefault, remove, authImportCLICmd(g))
	root.AddCommand(cmd)
}

// labelArg returns the optional credential-label positional, defaulting to
// "default" when omitted.
func labelArg(args []string) string {
	if len(args) == 1 {
		if l := strings.TrimSpace(args[0]); l != "" {
			return l
		}
	}
	return "default"
}

// readNewToken obtains a token to store: via the OS dialog when form is set,
// else from $VERCEL_TOKEN. Either way the value never transits the agent.
func readNewToken(form bool) (string, error) {
	if form {
		v, err := promptSecret("agent-vercel", "Paste your Vercel access token:")
		if err != nil {
			fixableBy, hint := dialog.Classify(err)
			if hint == "" {
				hint = "rerun 'agent-vercel auth add --form' and paste the token into the dialog"
			}
			return "", agenterrors.Newf(fixableBy, "token entry failed: %v", err).WithHint(hint)
		}
		if v = strings.TrimSpace(v); v != "" {
			return v, nil
		}
		return "", agenterrors.New("empty token entered", agenterrors.FixableByHuman).
			WithHint("rerun 'agent-vercel auth add --form' and paste a non-empty token")
	}
	tok := strings.TrimSpace(os.Getenv("VERCEL_TOKEN"))
	if tok == "" {
		return "", agenterrors.New("no token provided", agenterrors.FixableByHuman).
			WithHint("use 'agent-vercel auth add --form' to enter it via dialog, or set VERCEL_TOKEN (create one at vercel.com/account/tokens)")
	}
	return tok, nil
}
