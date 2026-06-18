//go:build darwin

package credential

import (
	"errors"
	"reflect"
	"testing"
)

func TestSecurityKeychainGet(t *testing.T) {
	var gotArgs []string
	kc := &securityKeychain{run: func(args ...string) (string, error) {
		gotArgs = args
		return "tok-123\n", nil // security appends a trailing newline
	}}

	v, ok := kc.Get("token:work")
	if !ok || v != "tok-123" {
		t.Fatalf("Get = %q, %v; want tok-123, true", v, ok)
	}
	want := []string{"find-generic-password", "-s", keychainService, "-a", "token:work", "-w"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args = %v; want %v", gotArgs, want)
	}
}

func TestSecurityKeychainGetNotFound(t *testing.T) {
	// `security` exits non-zero when the item is absent.
	missing := &securityKeychain{run: func(...string) (string, error) {
		return "", errors.New("exit status 44")
	}}
	if v, ok := missing.Get("token:missing"); ok || v != "" {
		t.Fatalf("Get(missing) = %q, %v; want \"\", false", v, ok)
	}

	// Empty output with no error is also treated as not-found.
	empty := &securityKeychain{run: func(...string) (string, error) { return "\n", nil }}
	if _, ok := empty.Get("token:x"); ok {
		t.Fatal("empty output should be not-found")
	}
}

func TestSecurityKeychainSetDeletesThenAdds(t *testing.T) {
	var calls [][]string
	kc := &securityKeychain{run: func(args ...string) (string, error) {
		calls = append(calls, args)
		return "", nil
	}}

	if !kc.Set("token:work", "sekret") {
		t.Fatal("Set should report success when add succeeds")
	}
	if len(calls) != 2 {
		t.Fatalf("expected delete-then-add (2 calls), got %d: %v", len(calls), calls)
	}
	if calls[0][0] != "delete-generic-password" {
		t.Fatalf("first call = %v; want delete-generic-password first", calls[0])
	}
	wantAdd := []string{"add-generic-password", "-s", keychainService, "-a", "token:work", "-w", "sekret", "-U"}
	if !reflect.DeepEqual(calls[1], wantAdd) {
		t.Fatalf("add args = %v; want %v", calls[1], wantAdd)
	}
}

func TestSecurityKeychainSetReportsFailure(t *testing.T) {
	// A failing add must return false so the Store falls back to the file.
	kc := &securityKeychain{run: func(args ...string) (string, error) {
		if args[0] == "add-generic-password" {
			return "", errors.New("add failed")
		}
		return "", nil
	}}
	if kc.Set("token:x", "v") {
		t.Fatal("Set should report failure when add errors")
	}
}

func TestSecurityKeychainDelete(t *testing.T) {
	var gotArgs []string
	kc := &securityKeychain{run: func(args ...string) (string, error) {
		gotArgs = args
		return "", nil
	}}
	kc.Delete("token:work")
	want := []string{"delete-generic-password", "-s", keychainService, "-a", "token:work"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args = %v; want %v", gotArgs, want)
	}
}
