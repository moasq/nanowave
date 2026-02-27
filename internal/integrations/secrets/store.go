// Package secrets provides secure storage for sensitive credentials like PATs.
// It uses the OS keychain (macOS Keychain, Linux Secret Service) when available,
// with a file-based fallback for environments without a keychain (CI, containers).
//
// Pattern: zalando/go-keyring — the industry standard for Go keychain access.
// Used by GitHub CLI, Docker credential helpers. Has MockInit() for testing.
package secrets

import "fmt"

// serviceName is the keychain service identifier for all nanowave secrets.
const serviceName = "nanowave"

// SecretStore provides secure credential storage.
type SecretStore interface {
	// Get retrieves a secret by key. Returns ErrNotFound if not present.
	Get(key string) (string, error)
	// Set stores a secret under the given key, replacing any existing value.
	Set(key, value string) error
	// Delete removes a secret. No error if the key doesn't exist.
	Delete(key string) error
}

// ErrNotFound is returned when a secret key does not exist.
var ErrNotFound = fmt.Errorf("secret not found")

// SecretKey builds a canonical key for provider secrets.
// Format: "provider/appName/field" (e.g. "supabase/MyApp/pat").
func SecretKey(provider, appName, field string) string {
	return provider + "/" + appName + "/" + field
}

// New returns the best available SecretStore for the current environment.
// It tries the OS keychain first, falling back to a file-based store.
func New(dir string) SecretStore {
	ks := newKeychainStore()
	// Probe: try a set+get+delete cycle to verify keychain availability.
	probeKey := "__nanowave_probe__"
	if err := ks.Set(probeKey, "ok"); err != nil {
		return newFileStore(dir)
	}
	_ = ks.Delete(probeKey)
	return ks
}
