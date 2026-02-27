package secrets

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// keychainStore wraps zalando/go-keyring for OS keychain access.
// On macOS this uses Keychain, on Linux it uses Secret Service (D-Bus).
type keychainStore struct{}

func newKeychainStore() *keychainStore {
	return &keychainStore{}
}

func (k *keychainStore) Get(key string) (string, error) {
	val, err := keyring.Get(serviceName, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return val, nil
}

func (k *keychainStore) Set(key, value string) error {
	return keyring.Set(serviceName, key, value)
}

func (k *keychainStore) Delete(key string) error {
	err := keyring.Delete(serviceName, key)
	if err != nil && errors.Is(err, keyring.ErrNotFound) {
		return nil // deleting non-existent key is fine
	}
	return err
}
