package cli

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/shhac/agent-vercel/internal/credential"
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func authImportCLICmd(g *GlobalFlags) *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "import-cli",
		Short: "Import an access token from the Vercel CLI (after 'vercel login')",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			path, token, err := readVercelCLIToken()
			if err != nil {
				return err
			}
			store, err := newCredStore()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			if err := store.Upsert(credential.Auth{Label: label, Type: credential.AuthToken, Secret: token}); err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			return printSingle(g, map[string]any{"label": label, "imported_from": path, "stored": true})
		},
	}
	cmd.Flags().StringVar(&label, "label", "default", "label to store the imported credential under")
	return cmd
}

// readVercelCLIToken finds the Vercel CLI's auth.json and extracts its token.
func readVercelCLIToken() (path, token string, err error) {
	for _, p := range vercelCLIAuthCandidates() {
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			continue
		}
		var a struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(data, &a)
		if a.Token == "" {
			return p, "", agenterrors.Newf(agenterrors.FixableByHuman, "Vercel CLI auth file %q has no token", p).
				WithHint("run 'vercel login' first, or use 'agent-vercel auth add'")
		}
		return p, a.Token, nil
	}
	return "", "", agenterrors.New("no Vercel CLI auth file found", agenterrors.FixableByHuman).
		WithHint("run 'vercel login' first, or set VERCEL_TOKEN and use 'agent-vercel auth add'")
}

func vercelCLIAuthCandidates() []string {
	var out []string
	if env := os.Getenv("AGENT_VERCEL_CLI_AUTH"); env != "" {
		out = append(out, env)
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		out = append(out, filepath.Join(xdg, "com.vercel.cli", "auth.json"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out,
			filepath.Join(home, ".local", "share", "com.vercel.cli", "auth.json"),
			filepath.Join(home, "Library", "Application Support", "com.vercel.cli", "auth.json"),
		)
	}
	return out
}
