// Package setup - credstore.go: Credential store interface and factory.
//
// CredentialStore abstracts credential persistence behind a key-value
// interface. Backends include OS keychain and AES-256-GCM encrypted files.
package setup

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

// CredentialStore is the interface for credential persistence backends.
// Each credential is identified by a service key (e.g. "autopus-worker").
type CredentialStore interface {
	// Save stores a credential value under the given service key.
	Save(service, value string) error
	// Load retrieves a credential value by service key.
	Load(service string) (string, error)
	// Delete removes a credential by service key.
	Delete(service string) error
}

// Option configures NewCredentialStore behavior.
type Option func(*storeOptions)

type storeOptions struct {
	forceFile   bool
	warningFunc func(string)
}

// WithForceFileBackend forces the encrypted file backend, skipping keychain.
func WithForceFileBackend(force bool) Option {
	return func(o *storeOptions) {
		o.forceFile = force
	}
}

// WithWarningFunc sets a callback invoked when a startup warning is emitted.
func WithWarningFunc(fn func(string)) Option {
	return func(o *storeOptions) {
		o.warningFunc = fn
	}
}

// NewCredentialStore creates a CredentialStore, trying keychain first.
// Falls back to encrypted file if keychain is unavailable or force-file is set.
// Returns the store and a non-empty warning string when falling back.
func NewCredentialStore(opts ...Option) (CredentialStore, string) {
	o := &storeOptions{}
	for _, opt := range opts {
		opt(o)
	}

	warn := func(msg string) {
		if o.warningFunc != nil {
			o.warningFunc(msg)
		}
		slog.Warn(msg)
	}

	if !o.forceFile {
		ks := newKeychainStore()
		// Probe keychain availability with a test write/delete.
		testKey := "autopus-probe"
		if err := ks.Save(testKey, "probe"); err == nil {
			_ = ks.Delete(testKey)
			migratePlaintextCredentials(ks, warn)
			return ks, ""
		}
	}

	// Fallback to encrypted file backend.
	msg := "Using encrypted file storage (keychain unavailable)"
	warn(msg)
	store := newEncryptedFileStore(defaultCredentialDir())
	migratePlaintextCredentials(store, warn)
	return store, msg
}

// migratePlaintextCredentials migrates plaintext credentials.json to the new store.
// If the old file exists, its content is saved to the store and the file is
// zero-filled then removed.
func migratePlaintextCredentials(store CredentialStore, warn func(string)) {
	oldPath := DefaultCredentialsPath()
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return // no plaintext file — nothing to migrate
	}

	// Marshal the raw JSON back as a single string value under a known key.
	var raw json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return // not valid JSON — skip
	}

	if err := store.Save("autopus-worker", string(data)); err != nil {
		warn("Failed to migrate plaintext credentials: " + err.Error())
		return
	}

	// Secure delete: zero-fill then remove.
	zeros := make([]byte, len(data))
	_ = os.WriteFile(oldPath, zeros, 0600)
	_ = os.Remove(oldPath)

	slog.Info("Migrated credentials from plaintext to secure storage")
}

// defaultCredentialDir returns ~/.config/autopus.
func defaultCredentialDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".autopus-credentials")
	}
	return filepath.Join(home, ".config", "autopus")
}
