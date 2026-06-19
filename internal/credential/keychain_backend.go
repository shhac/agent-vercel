package credential

import "github.com/shhac/lib-agent-cli/creds"

// defaultKeychain returns the platform Keychain backed by the shared
// creds.Keychain (macOS `security` CLI). On non-macOS platforms creds reports
// Available() == false and its mutations return creds.ErrKeychainUnavailable,
// which this adapter maps to the Store's "not stored" fallback so the secret
// lands in the 0600 credentials file instead.
func defaultKeychain() Keychain {
	return credsKeychain{kc: creds.NewKeychain(keychainService)}
}

// credsKeychain adapts the shared *creds.Keychain (whose mutations return
// errors) to the credential.Keychain interface the Store depends on (whose Set
// returns a bool and whose Delete is fire-and-forget). A failed Set reports
// false so the Store retains the plaintext secret in the file rather than
// losing it.
type credsKeychain struct {
	kc *creds.Keychain
}

func (c credsKeychain) Get(account string) (string, bool) {
	return c.kc.Get(account)
}

func (c credsKeychain) Set(account, value string) bool {
	return c.kc.Set(account, value) == nil
}

func (c credsKeychain) Delete(account string) {
	_ = c.kc.Delete(account)
}

func (c credsKeychain) Available() bool {
	return c.kc.Available()
}
