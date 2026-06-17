// Package vercel will hold the Vercel REST API client: a dependency-injected
// HTTP transport (Bearer auth against https://api.vercel.com), scope handling
// (teamId/slug query params), pagination helpers, 429/5xx retry with backoff,
// and error mapping to the agent-* fixable_by contract. It is intentionally
// empty in the scaffold — see design-docs/architecture.md for the planned
// shape and design-docs/cli-design.md for the command surface it serves.
package vercel
