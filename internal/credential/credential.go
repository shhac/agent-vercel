// Package credential manages agent-vercel's stored Vercel credentials.
//
// Vercel's model differs from a per-workspace token: ONE credential reaches
// MANY teams, and the team is a per-request scope parameter (teamId/slug), not
// a property of the credential. So this package stores credentials (the secret)
// and scope metadata (teams — non-secret) on separate axes.
//
// A credential is not assumed to always be a bare access token: each Auth
// carries a Type discriminator (currently only "token") so other auth kinds can
// be added later without reshaping the store.
//
// Non-secret metadata lives in a JSON file under the user config dir
// (~/.config/agent-vercel/credentials.json by default). The secret is stored in
// the macOS Keychain when available; the file then holds a "__KEYCHAIN__"
// placeholder in its place. On platforms without a supported Keychain the secret
// is written to the file directly (0600).
//
// Security boundary: nothing in this package serializes a secret to stdout. The
// secret is loaded only to populate an Authorization header inside the binary.
// There is deliberately no "get secret" path — callers inspect *where* a secret
// lives (SecretStatuses) but never read the secret back out.
package credential

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AuthType discriminates the kind of secret an Auth holds. Today only access
// tokens exist; the field exists so future kinds (e.g. OAuth) don't require a
// schema change.
type AuthType string

const (
	AuthToken AuthType = "token"
)

// Auth is one stored credential, addressed by a human-chosen label. Secret
// normally holds the keychain placeholder; the real value lives in the Keychain
// and is hydrated only by Load.
type Auth struct {
	Label    string   `json:"label"`
	Type     AuthType `json:"type"`
	Secret   string   `json:"secret,omitempty"`
	UserID   string   `json:"user_id,omitempty"`
	Username string   `json:"username,omitempty"`
}

// normalizeType defaults a blank discriminator to AuthToken (back-compat / the
// common case) so callers can omit it.
func (a Auth) normalizeType() AuthType {
	if a.Type == "" {
		return AuthToken
	}
	return a.Type
}

type Credentials struct {
	Version      int    `json:"version"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	DefaultAuth  string `json:"default_auth,omitempty"`  // credential label
	DefaultScope string `json:"default_scope,omitempty"` // team slug; "" means personal account
	Auths        []Auth `json:"auths"`
}

// ErrAuthNotFound is returned when no stored credential matches a request.
var ErrAuthNotFound = errors.New("credential not found")

// AmbiguousSelectorError is returned when an --auth selector matches more than
// one stored credential label.
type AmbiguousSelectorError struct {
	Selector string
	Matches  []string
}

func (e *AmbiguousSelectorError) Error() string {
	return fmt.Sprintf("auth selector %q is ambiguous; matches: %s", e.Selector, strings.Join(e.Matches, ", "))
}

// Store reads and writes the credentials file plus the backing Keychain.
type Store struct {
	path string
	kc   Keychain
	now  func() time.Time
}

// New returns a Store using the default credentials path and platform Keychain.
func New() (*Store, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	return &Store{path: path, kc: defaultKeychain(), now: time.Now}, nil
}

// NewWithStore builds a Store with an explicit file path and Keychain — used by
// tests to avoid touching the real config dir or Keychain.
func NewWithStore(path string, kc Keychain) *Store {
	return &Store{path: path, kc: kc, now: time.Now}
}

// Load reads the credentials file and hydrates each secret from the Keychain.
func (s *Store) Load() (*Credentials, error) {
	creds := &Credentials{Version: 1, Auths: []Auth{}}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return creds, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, creds); err != nil {
		// A corrupt file is treated as empty rather than fatal.
		return &Credentials{Version: 1, Auths: []Auth{}}, nil
	}
	if creds.Version == 0 {
		creds.Version = 1
	}
	for i := range creds.Auths {
		a := &creds.Auths[i]
		if v, ok := s.kc.Get(secretAccount(*a)); ok {
			a.Secret = v
		}
	}
	return creds, nil
}

// Save writes the credentials, pushing secrets to the Keychain where possible
// and replacing them with a placeholder in the file.
func (s *Store) Save(creds *Credentials) error {
	out := *creds
	out.Version = 1
	out.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	out.Auths = make([]Auth, len(creds.Auths))
	copy(out.Auths, creds.Auths)
	for i := range out.Auths {
		out.Auths[i].Type = out.Auths[i].normalizeType()
	}

	if s.kc.Available() {
		s.pushSecretsToKeychain(out.Auths)
	}

	data, err := json.MarshalIndent(&out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// pushSecretsToKeychain stores each secret in the Keychain and replaces the
// in-place file copy with the placeholder — but only for secrets the Keychain
// actually accepted; a failed Set leaves the real value in the file so it is
// never lost. The caller is responsible for checking s.kc.Available() first.
func (s *Store) pushSecretsToKeychain(auths []Auth) {
	for i := range auths {
		a := &auths[i]
		if !isPlaceholder(a.Secret) && s.kc.Set(secretAccount(*a), a.Secret) {
			a.Secret = keychainPlaceholder
		}
	}
}

// Upsert inserts or replaces a credential by label and persists. The first
// credential added becomes the default.
func (s *Store) Upsert(auth Auth) error {
	auth.Type = auth.normalizeType()
	creds, err := s.Load()
	if err != nil {
		return err
	}
	idx := -1
	for i, existing := range creds.Auths {
		if existing.Label == auth.Label {
			idx = i
			break
		}
	}
	if idx == -1 {
		creds.Auths = append(creds.Auths, auth)
	} else {
		merged := creds.Auths[idx]
		merged.Type = auth.Type
		merged.Secret = auth.Secret
		if auth.UserID != "" {
			merged.UserID = auth.UserID
		}
		if auth.Username != "" {
			merged.Username = auth.Username
		}
		creds.Auths[idx] = merged
	}
	if creds.DefaultAuth == "" {
		creds.DefaultAuth = auth.Label
	}
	return s.Save(creds)
}

// SetDefaultAuth sets the default credential label.
func (s *Store) SetDefaultAuth(label string) error {
	creds, err := s.Load()
	if err != nil {
		return err
	}
	found := false
	for _, a := range creds.Auths {
		if a.Label == label {
			found = true
			break
		}
	}
	if !found {
		return ErrAuthNotFound
	}
	creds.DefaultAuth = label
	return s.Save(creds)
}

// SetDefaultScope sets the default team scope (slug; "" for personal account).
func (s *Store) SetDefaultScope(scope string) error {
	creds, err := s.Load()
	if err != nil {
		return err
	}
	creds.DefaultScope = scope
	return s.Save(creds)
}

// Remove deletes a credential and its Keychain secret.
func (s *Store) Remove(label string) error {
	creds, err := s.Load()
	if err != nil {
		return err
	}
	kept := creds.Auths[:0]
	for _, a := range creds.Auths {
		if a.Label == label {
			s.kc.Delete(secretAccount(a))
			continue
		}
		kept = append(kept, a)
	}
	creds.Auths = kept
	if creds.DefaultAuth == label {
		creds.DefaultAuth = ""
		if len(creds.Auths) > 0 {
			creds.DefaultAuth = creds.Auths[0].Label
		}
	}
	return s.Save(creds)
}
