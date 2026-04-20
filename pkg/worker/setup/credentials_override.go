package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewPathCredentialStore returns a plain file-backed credential store for an explicit credentials path override.
func NewPathCredentialStore(path string) CredentialStore {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return &pathCredentialStore{path: path}
}

type pathCredentialStore struct {
	path string
}

func (s *pathCredentialStore) Save(_ string, value string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create credential dir: %w", err)
	}
	if err := os.WriteFile(s.path, []byte(value), 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

func (s *pathCredentialStore) Load(_ string) (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return "", fmt.Errorf("read credentials: %w", err)
	}
	return string(data), nil
}

func (s *pathCredentialStore) Delete(_ string) error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete credentials: %w", err)
	}
	return nil
}

func loadCredentialBytesFromPath(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return loadCredentialBytes()
	}
	return os.ReadFile(path)
}

func loadRawCredentialsFromPath(path string) (*rawCredentials, error) {
	data, err := loadCredentialBytesFromPath(path)
	if err != nil {
		return nil, err
	}

	var creds rawCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}
