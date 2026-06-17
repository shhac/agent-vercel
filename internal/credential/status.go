package credential

import (
	"encoding/json"
	"os"
)

// SecretStatus says where one stored token lives. "missing" means the file
// holds a Keychain placeholder whose backing entry is gone.
type SecretStatus string

const (
	SecretInKeychain SecretStatus = "keychain"
	SecretInFile     SecretStatus = "file"
	SecretMissing    SecretStatus = "missing"
)

// SecretStatuses reports, per credential label, where its secret lives. It reads
// the raw file (placeholders intact) and probes the Keychain WITHOUT returning
// any secret material — this is the read path `auth list` uses, and the reason
// the tool can report configuration without ever exposing a secret.
func (s *Store) SecretStatuses() (map[string]SecretStatus, error) {
	out := map[string]SecretStatus{}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	raw := &Credentials{}
	if err := json.Unmarshal(data, raw); err != nil {
		return out, nil // corrupt file reads as empty, matching Load
	}
	for _, a := range raw.Auths {
		out[a.Label] = s.secretStatus(a.Secret, secretAccount(a))
	}
	return out, nil
}

func (s *Store) secretStatus(rawValue, account string) SecretStatus {
	if !isPlaceholder(rawValue) {
		return SecretInFile
	}
	if _, ok := s.kc.Get(account); ok {
		return SecretInKeychain
	}
	return SecretMissing
}
