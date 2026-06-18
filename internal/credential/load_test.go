package credential

import (
	"errors"
	"os"
	"testing"
)

func TestLoadTreatsCorruptFileAsEmpty(t *testing.T) {
	s, _ := newTestStore(t)
	if err := os.WriteFile(s.Path(), []byte("{ not valid json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// A corrupt credentials file must not lock the user out with a fatal error;
	// it loads as empty so they can re-add credentials.
	creds, err := s.Load()
	if err != nil {
		t.Fatalf("Load on corrupt file should not error: %v", err)
	}
	if len(creds.Auths) != 0 {
		t.Fatalf("corrupt file should load as empty, got %d auths", len(creds.Auths))
	}
}

func TestSetDefaultAuthNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	// Pointing the default at a label that was never stored must surface
	// ErrAuthNotFound, not silently succeed.
	err := s.SetDefaultAuth("never-stored")
	if !errors.Is(err, ErrAuthNotFound) {
		t.Fatalf("want ErrAuthNotFound, got %v", err)
	}
}
