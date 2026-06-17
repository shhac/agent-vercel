package vercel

import (
	"context"
	"encoding/json"
)

// User is the authenticated token owner (GET /v2/user).
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

// Team is one team the token can reach (GET /v2/teams).
type Team struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// GetUser identifies the token owner. Used by `auth test` / `whoami`.
func (c *Client) GetUser(ctx context.Context) (User, error) {
	raw, err := c.Get(ctx, "/v2/user", nil)
	if err != nil {
		return User{}, err
	}
	var env struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return User{}, err
	}
	return env.User, nil
}

// ListTeams lists the teams reachable by the token. Used by `scope list`.
func (c *Client) ListTeams(ctx context.Context) ([]Team, error) {
	raw, err := c.Get(ctx, "/v2/teams", nil)
	if err != nil {
		return nil, err
	}
	var env struct {
		Teams []Team `json:"teams"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	return env.Teams, nil
}
