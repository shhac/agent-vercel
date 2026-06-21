package credential

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) (*Store, *MemoryKeychain) {
	t.Helper()
	dir := t.TempDir()
	kc := NewMemoryKeychain()
	return NewWithStore(filepath.Join(dir, "credentials.json"), kc), kc
}

// TestStore_Headless_FileFallback exercises the real credential-WRITE path
// non-interactively. Setting the per-CLI keychain opt-out (derived by
// lib-agent-cli from the "app.paulie.agent-vercel" service) makes the real
// keychain backend report Available()==false, so Save deterministically takes
// the 0600 file fallback on every platform — including darwin, where it would
// otherwise reach the `security` CLI and its GUI prompt. This is the path that
// previously could only be unit-tested with a MemoryKeychain.
func TestStore_Headless_FileFallback(t *testing.T) {
	t.Setenv("AGENT_VERCEL_NO_KEYCHAIN", "1")

	kc := defaultKeychain()
	if kc.Available() {
		t.Fatal("keychain still Available() with AGENT_VERCEL_NO_KEYCHAIN=1; opt-out not honoured")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	s := NewWithStore(path, kc)

	if err := s.Upsert(Auth{Label: "headless", Secret: "headless-token"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// With the keychain opted out, the raw secret must land in the 0600 file —
	// no placeholder, because nothing was pushed to a keychain.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("credentials file not written: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("credentials mode=%o, want 0600", mode)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(raw), "headless-token") {
		t.Fatalf("file fallback expected the raw secret on disk; got:\n%s", raw)
	}
	if strings.Contains(string(raw), keychainPlaceholder) {
		t.Fatalf("file unexpectedly contains keychain placeholder (keychain should be bypassed):\n%s", raw)
	}

	// Round-trip via the read path: Load must return the secret straight from
	// the file (no keychain hydration).
	creds, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(creds.Auths) != 1 || creds.Auths[0].Secret != "headless-token" {
		t.Fatalf("round-trip secret = %+v; want headless-token", creds.Auths)
	}

	if err := s.Remove("headless"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	after, err := s.Load()
	if err != nil {
		t.Fatalf("load after remove: %v", err)
	}
	if len(after.Auths) != 0 {
		t.Fatalf("auths after remove = %+v; want none", after.Auths)
	}
}

func TestUpsertStoresSecretInKeychainNotFile(t *testing.T) {
	s, kc := newTestStore(t)
	if err := s.Upsert(Auth{Label: "personal", Secret: "secret-token-abc"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// The secret must be in the Keychain, keyed by "<type>:<label>"...
	if got, ok := kc.Get("token:personal"); !ok || got != "secret-token-abc" {
		t.Fatalf("keychain secret = %q, %v; want secret-token-abc, true", got, ok)
	}

	// ...and the on-disk file must NOT contain the raw secret.
	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if strings.Contains(string(raw), "secret-token-abc") {
		t.Fatalf("credentials file leaked the raw secret:\n%s", raw)
	}
	if !strings.Contains(string(raw), keychainPlaceholder) {
		t.Fatalf("credentials file missing %s placeholder:\n%s", keychainPlaceholder, raw)
	}
}

func TestUpsertMergesAndRotatesExistingLabel(t *testing.T) {
	s, kc := newTestStore(t)
	if err := s.Upsert(Auth{Label: "work", Secret: "tok-1", UserID: "usr_1", Username: "bot"}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Re-add the same label with a rotated secret and no profile fields: the
	// merge branch must replace the secret but preserve the existing
	// UserID/Username (the auth-rotation path).
	if err := s.Upsert(Auth{Label: "work", Secret: "tok-2"}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	creds, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(creds.Auths) != 1 {
		t.Fatalf("auths = %+v; want exactly one (merge, not append)", creds.Auths)
	}
	if got, ok := kc.Get("token:work"); !ok || got != "tok-2" {
		t.Fatalf("rotated keychain secret = %q, %v; want tok-2, true", got, ok)
	}
	a := creds.Auths[0]
	if a.Secret != "tok-2" || a.UserID != "usr_1" || a.Username != "bot" {
		t.Fatalf("merged auth = %+v; want secret tok-2 with usr_1/bot preserved", a)
	}
	if creds.DefaultAuth != "work" {
		t.Fatalf("default auth = %q; want work (unchanged)", creds.DefaultAuth)
	}
}

func TestUpsertDefaultsTypeToToken(t *testing.T) {
	s, _ := newTestStore(t)
	_ = s.Upsert(Auth{Label: "x", Secret: "t"})
	creds, _ := s.Load()
	if creds.Auths[0].Type != AuthToken {
		t.Fatalf("type = %q; want %q", creds.Auths[0].Type, AuthToken)
	}
}

func TestLoadHydratesSecretFromKeychain(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Upsert(Auth{Label: "work", Secret: "tok-xyz"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	creds, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(creds.Auths) != 1 || creds.Auths[0].Secret != "tok-xyz" {
		t.Fatalf("hydrated secret = %+v; want tok-xyz", creds.Auths)
	}
	if creds.DefaultAuth != "work" {
		t.Fatalf("default auth = %q; want work", creds.DefaultAuth)
	}
}

func TestSecretStatusesNeverReturnsSecretMaterial(t *testing.T) {
	s, _ := newTestStore(t)
	_ = s.Upsert(Auth{Label: "personal", Secret: "top-secret"})

	st, err := s.SecretStatuses()
	if err != nil {
		t.Fatalf("statuses: %v", err)
	}
	if st["personal"] != SecretInKeychain {
		t.Fatalf("status = %q; want keychain", st["personal"])
	}
	// Round-trip the statuses through JSON; the secret must not appear.
	b, _ := json.Marshal(st)
	if strings.Contains(string(b), "top-secret") {
		t.Fatalf("SecretStatuses leaked secret material: %s", b)
	}
}

func TestRemoveDeletesKeychainEntryAndReassignsDefault(t *testing.T) {
	s, kc := newTestStore(t)
	_ = s.Upsert(Auth{Label: "a", Secret: "ta"})
	_ = s.Upsert(Auth{Label: "b", Secret: "tb"})

	if err := s.Remove("a"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := kc.Get("token:a"); ok {
		t.Fatalf("keychain entry for removed credential still present")
	}
	creds, _ := s.Load()
	if len(creds.Auths) != 1 || creds.Auths[0].Label != "b" {
		t.Fatalf("remaining auths = %+v; want [b]", creds.Auths)
	}
	if creds.DefaultAuth != "b" {
		t.Fatalf("default after removing default = %q; want b", creds.DefaultAuth)
	}
}

func TestSetDefaultScopeIsNonSecretAndPersists(t *testing.T) {
	s, _ := newTestStore(t)
	_ = s.Upsert(Auth{Label: "personal", Secret: "t"})
	if err := s.SetDefaultScope("acme"); err != nil {
		t.Fatalf("set scope: %v", err)
	}
	creds, _ := s.Load()
	if creds.DefaultScope != "acme" {
		t.Fatalf("default scope = %q; want acme", creds.DefaultScope)
	}
}

func TestFilePermissionsAre0600(t *testing.T) {
	s, _ := newTestStore(t)
	_ = s.Upsert(Auth{Label: "x", Secret: "t"})
	info, err := os.Stat(s.Path())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file perm = %o; want 600", perm)
	}
}
