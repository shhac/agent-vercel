package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func registerAPI(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Raw Vercel REST escape hatch",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var query, body string
	var yes bool
	call := &cobra.Command{
		Use:   "call <METHOD> <path>",
		Short: "Call any Vercel REST endpoint with the active credential and scope",
		Long: "GET is ungated. Any other method changes state and requires --yes.\n" +
			"--query takes a urlencoded string (k=v&k2=v2); --body takes JSON (or - for stdin).",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]
			if method != http.MethodGet {
				if err := requireYes(yes, method+" "+path,
					"agent-vercel api call "+method+" "+path+" … --yes"); err != nil {
					return err
				}
			}
			q, err := url.ParseQuery(query)
			if err != nil {
				return agenterrors.Newf(agenterrors.FixableByAgent, "invalid --query %q: %v", query, err)
			}
			var payload any
			if body == "-" {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByAgent)
				}
				payload = json.RawMessage(b)
			} else if body != "" {
				payload = json.RawMessage(body)
			}
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			raw, err := r.client.Do(cmd.Context(), method, path, q, payload)
			if err != nil {
				return err
			}
			if len(raw) == 0 {
				return printSingle(g, map[string]any{"ok": true})
			}
			return printRaw(g, raw)
		},
	}
	call.Flags().StringVar(&query, "query", "", "urlencoded query string (k=v&k2=v2)")
	call.Flags().StringVar(&body, "body", "", "request body as JSON, or - to read stdin")
	call.Flags().BoolVar(&yes, "yes", false, "confirm a non-GET (state-changing) call")

	cmd.AddCommand(call)
	root.AddCommand(cmd)
}
