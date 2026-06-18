package credential

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failingKeychain reports itself available but rejects every Set, exercising the
// plaintext-fallback branch in pushSecretsToKeychain (a secret the Keychain
// won't accept must never be silently lost).
type failingKeychain struct{}

func (failingKeychain) Get(string) (string, bool) { return "", false }
func (failingKeychain) Set(string, string) bool   { return false }
func (failingKeychain) Delete(string)             {}
func (failingKeychain) Available() bool           { return true }

func TestSaveRetainsSecretInFileWhenKeychainSetFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	s := NewWithStore(path, failingKeychain{})

	if err := s.Upsert(Auth{Label: "work", Secret: "plaintext-secret"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Set failed, so the real secret must be retained in the file rather than
	// replaced by the placeholder — otherwise it would be unrecoverable.
	if !strings.Contains(string(raw), "plaintext-secret") {
		t.Fatalf("expected secret retained in file on keychain failure:\n%s", raw)
	}
	if strings.Contains(string(raw), keychainPlaceholder) {
		t.Fatalf("file should not hold the placeholder when the secret was not stored:\n%s", raw)
	}
	// The file now holds a secret, so it must still be 0600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file perm = %o; want 600", perm)
	}
	// Load returns the secret straight from the file (the Keychain has nothing).
	creds, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(creds.Auths) != 1 || creds.Auths[0].Secret != "plaintext-secret" {
		t.Fatalf("loaded secret = %+v; want plaintext-secret", creds.Auths)
	}
}

func TestSaveWritesSecretToFileWhenKeychainUnavailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	// noopKeychain reports Available()==false, so Save skips the Keychain push
	// entirely and the secret lands in the file (the non-macOS path).
	s := NewWithStore(path, noopKeychain{})

	if err := s.Upsert(Auth{Label: "work", Secret: "plaintext-secret"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "plaintext-secret") {
		t.Fatalf("expected secret in file when keychain unavailable:\n%s", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file perm = %o; want 600", perm)
	}
}
