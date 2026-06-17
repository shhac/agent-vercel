package cli

import (
	"os"

	"github.com/shhac/agent-vercel/internal/credential"
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/output"
	"github.com/spf13/cobra"
)

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

	var label string

	add := &cobra.Command{
		Use:   "add",
		Short: "Store a credential from $VERCEL_TOKEN into the Keychain",
		Long: "Reads the access token from $VERCEL_TOKEN and stores it in the Keychain under --label.\n" +
			"The secret is never echoed and never written to the credentials file.\n" +
			"(A native-dialog --form entry path is planned; see design-docs.)",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(func() error {
				tok := os.Getenv("VERCEL_TOKEN")
				if tok == "" {
					return agenterrors.New("no token provided", agenterrors.FixableByHuman).
						WithHint("set VERCEL_TOKEN (create one at vercel.com/account/tokens) then rerun 'agent-vercel auth add'")
				}
				store, err := credential.New()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				if err := store.Upsert(credential.Auth{Label: label, Type: credential.AuthToken, Secret: tok}); err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				return printSingle(g, map[string]any{
					"label":  label,
					"type":   string(credential.AuthToken),
					"stored": true,
					"hint":   "run 'agent-vercel auth test' to verify",
				})
			})
		},
	}
	add.Flags().StringVar(&label, "label", "default", "label to store the credential under")

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List stored credentials and where each secret lives (never the secret)",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(func() error {
				store, err := credential.New()
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
				w := output.NewNDJSONWriter(os.Stdout)
				for _, a := range creds.Auths {
					_ = w.WriteItem(map[string]any{
						"label":         a.Label,
						"type":          string(a.Type),
						"default":       a.Label == creds.DefaultAuth,
						"username":      a.Username,
						"secret_status": string(statuses[a.Label]),
					})
				}
				return nil
			})
		},
	}

	test := &cobra.Command{
		Use:     "test",
		Aliases: []string{"whoami"},
		Short:   "Verify the active credential and show the token owner (GET /v2/user)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(func() error {
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
			})
		},
	}

	setDefault := &cobra.Command{
		Use:   "set-default <label>",
		Short: "Set the default credential label",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return run(func() error {
				store, err := credential.New()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				if err := store.SetDefaultAuth(args[0]); err != nil {
					return agenterrors.Newf(agenterrors.FixableByAgent, "%v", err).
						WithHint("run 'agent-vercel auth list' to see stored labels")
				}
				return printSingle(g, map[string]any{"default_auth": args[0]})
			})
		},
	}

	remove := &cobra.Command{
		Use:   "remove <label>",
		Short: "Remove a stored credential and its Keychain secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return run(func() error {
				store, err := credential.New()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				if err := store.Remove(args[0]); err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				return printSingle(g, map[string]any{"removed": args[0]})
			})
		},
	}

	cmd.AddCommand(add, list, test, setDefault, remove, authImportCLICmd(g))
	root.AddCommand(cmd)
}
