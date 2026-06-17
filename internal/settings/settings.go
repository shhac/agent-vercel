// Package settings persists agent-vercel's non-secret configuration (e.g. cache
// TTLs) as a flat string→string JSON map under the user config dir
// (~/.config/agent-vercel/config.json), separate from credentials.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type Store struct {
	path string
}

// New returns a Store at the default config path ($AGENT_VERCEL_CONFIG, else
// $XDG_CONFIG_HOME/agent-vercel/config.json, else ~/.config/...).
func New() (*Store, error) {
	if env := os.Getenv("AGENT_VERCEL_CONFIG"); env != "" {
		return &Store{path: env}, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".config")
	}
	return &Store{path: filepath.Join(base, "agent-vercel", "config.json")}, nil
}

// NewWithPath builds a Store at an explicit path (tests).
func NewWithPath(path string) *Store { return &Store{path: path} }

// Path returns the config file path.
func (s *Store) Path() string { return s.path }

// Load returns the settings map (empty if the file is missing or corrupt).
func (s *Store) Load() (map[string]string, error) {
	out := map[string]string{}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]string{}, nil
	}
	return out, nil
}

// Get returns one setting and whether it is present.
func (s *Store) Get(key string) (string, bool, error) {
	m, err := s.Load()
	if err != nil {
		return "", false, err
	}
	v, ok := m[key]
	return v, ok, nil
}

// Set stores one key/value and persists.
func (s *Store) Set(key, value string) error {
	m, err := s.Load()
	if err != nil {
		return err
	}
	m[key] = value
	return s.save(m)
}

// Unset removes one key and persists.
func (s *Store) Unset(key string) error {
	m, err := s.Load()
	if err != nil {
		return err
	}
	delete(m, key)
	return s.save(m)
}

// Keys returns the setting keys in sorted order.
func (s *Store) Keys() ([]string, error) {
	m, err := s.Load()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *Store) save(m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}
