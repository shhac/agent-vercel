package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/shhac/agent-vercel/internal/credential"
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

// cacheData is the resolution cache: scopes (teams) and projects for the active
// scope, so name→id lookups and (future) completions don't re-hit the API. It
// is purely an optimization; commands work without it.
type cacheData struct {
	Scopes   []credential.Scope `json:"scopes"`
	Projects []cacheProject     `json:"projects"`
	WarmedAt string             `json:"warmed_at,omitempty"`
}

type cacheProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func cachePath() (string, error) {
	if env := os.Getenv("AGENT_VERCEL_CACHE"); env != "" {
		return env, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "agent-vercel", "cache.json"), nil
}

func registerCache(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Resolution cache (scopes, projects) for faster lookups",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	warm := &cobra.Command{
		Use:   "warm",
		Short: "Pre-fetch scopes and projects into the cache",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(func() error {
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				teams, err := r.client.ListTeams(cmd.Context())
				if err != nil {
					return err
				}
				data := cacheData{WarmedAt: time.Now().UTC().Format(time.RFC3339)}
				for _, t := range teams {
					data.Scopes = append(data.Scopes, credential.Scope{ID: t.ID, Slug: t.Slug, Name: t.Name})
				}
				q := url.Values{}
				q.Set("limit", "100")
				items, _, err := r.client.ListProjects(cmd.Context(), q)
				if err != nil {
					return err
				}
				for _, it := range items {
					var p struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}
					if json.Unmarshal(it, &p) == nil {
						data.Projects = append(data.Projects, cacheProject{ID: p.ID, Name: p.Name})
					}
				}
				if err := saveCache(data); err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				return printSingle(g, map[string]any{"scopes": len(data.Scopes), "projects": len(data.Projects)})
			})
		},
	}

	info := &cobra.Command{
		Use:   "info",
		Short: "Report what is cached",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(func() error {
				path, err := cachePath()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				data, _ := loadCache()
				return printSingle(g, map[string]any{
					"path":      path,
					"scopes":    len(data.Scopes),
					"projects":  len(data.Projects),
					"warmed_at": data.WarmedAt,
				})
			})
		},
	}

	purge := &cobra.Command{
		Use:   "purge",
		Short: "Delete the resolution cache",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(func() error {
				path, err := cachePath()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				return printSingle(g, map[string]any{"purged": true})
			})
		},
	}

	cmd.AddCommand(warm, info, purge)
	root.AddCommand(cmd)
}

func loadCache() (cacheData, error) {
	var data cacheData
	path, err := cachePath()
	if err != nil {
		return data, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return data, err
	}
	_ = json.Unmarshal(b, &data)
	return data, nil
}

func saveCache(data cacheData) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
