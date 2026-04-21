package setup

import (
	"fmt"
	"os"
	"path/filepath"
)

const workerCredentialService = "autopus-worker"

var newCredentialStoreFunc = func() (CredentialStore, string) {
	return NewCredentialStore()
}

func loadCredentialBytes() ([]byte, error) {
	payload, err := loadCredentialPayload()
	if err != nil {
		return nil, err
	}
	return payload.data, nil
}

func saveCredentialBytes(data []byte) error {
	store, _ := newCredentialStoreFunc()
	if store != nil {
		if err := store.Save(workerCredentialService, string(data)); err == nil {
			// Best effort cleanup for legacy plaintext credentials after secure save.
			_ = os.Remove(DefaultCredentialsPath())
			return nil
		}
	}

	path := DefaultCredentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}
