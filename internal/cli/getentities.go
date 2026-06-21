package cli

import (
	"context"
	"encoding/json"
	"os"

	libcli "github.com/shhac/lib-agent-cli/cli"
	"github.com/shhac/agent-vercel/internal/vercel"
)

// GetEntities runs the family's multi-capable get for the vercel domain: it
// resolves a client once and streams each id through getOne per the shared get
// contract (NDJSON by default — one record or {"@unresolved":…} per id in
// input order; item-level misses stay on stdout, command-level failures bubble
// to the single sink). getOne must return a classified *output.Error (via
// agenterrors) so a 404/bad input becomes an @unresolved record.
func GetEntities(
	g *GlobalFlags,
	ctx context.Context,
	args []string,
	getOne func(ctx context.Context, c *vercel.Client, id string) (any, error),
) error {
	r, err := resolveClient(g)
	if err != nil {
		return err
	}
	return libcli.EntityGet(os.Stdout, g.Format, args, func(id string) (any, error) {
		return getOne(ctx, r.client, id)
	})
}

// resolveRawAsAny fetches a raw JSON record from the API and applies the compact
// projection fn (or returns the raw object under --full), converting the result
// to any so it can be handed to EntityGet.
func resolveRawAsAny(g *GlobalFlags, raw json.RawMessage, fn func(json.RawMessage) (map[string]any, error)) (any, error) {
	if g.Full {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, wrapAgent(err)
		}
		return v, nil
	}
	return fn(raw)
}
