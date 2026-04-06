// Package setup - credstore_keychain.go: OS keychain credential backend.
//
// Uses github.com/zalando/go-keyring to store credentials in the
// platform's native keychain (macOS Keychain, Linux Secret Service).
package setup

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

// keychainStore implements CredentialStore using the OS keychain.
type keychainStore struct{}

// newKeychainStore returns a keychain-backed CredentialStore.
func newKeychainStore() *keychainStore {
	return &keychainStore{}
}

// Save stores a credential value in the OS keychain.
func (k *keychainStore) Save(service, value string) error {
	if err := keyring.Set(service, "credentials", value); err != nil {
		return fmt.Errorf("keychain save %q: %w", service, err)
	}
	return nil
}

// Load retrieves a credential value from the OS keychain.
func (k *keychainStore) Load(service string) (string, error) {
	val, err := keyring.Get(service, "credentials")
	if err != nil {
		return "", fmt.Errorf("keychain load %q: %w", service, err)
	}
	return val, nil
}

// Delete removes a credential from the OS keychain.
func (k *keychainStore) Delete(service string) error {
	if err := keyring.Delete(service, "credentials"); err != nil {
		return fmt.Errorf("keychain delete %q: %w", service, err)
	}
	return nil
}
