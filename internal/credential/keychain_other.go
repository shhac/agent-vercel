//go:build !darwin

package credential

// defaultKeychain returns a no-op Keychain on platforms without a supported
// secret store; the Store then falls back to the 0600 credentials file.
func defaultKeychain() Keychain { return noopKeychain{} }
