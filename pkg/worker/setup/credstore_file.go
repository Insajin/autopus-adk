// Package setup - credstore_file.go: AES-256-GCM encrypted file backend.
//
// Derives a key from machine-id + username via PBKDF2. Each credential
// is stored in its own file under the configured directory.
package setup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	pbkdf2Iterations = 100_000
	saltLen          = 16
	keyLen           = 32 // AES-256
)

// encryptedFileStore implements CredentialStore using AES-256-GCM encrypted files.
type encryptedFileStore struct {
	dir string
}

// newEncryptedFileStore creates a file-based credential store in the given directory.
func newEncryptedFileStore(dir string) *encryptedFileStore {
	return &encryptedFileStore{dir: dir}
}

// Save encrypts and writes a credential value to a file named after the service key.
func (s *encryptedFileStore) Save(service, value string) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("create credential dir: %w", err)
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	key := deriveKey(salt)
	ciphertext, err := encryptGCM(key, []byte(value))
	if err != nil {
		return err
	}

	// File format: salt (16 bytes) + ciphertext (nonce + encrypted + tag)
	data := append(salt, ciphertext...)
	path := s.filePath(service)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write credential file: %w", err)
	}
	return nil
}

// Load decrypts and returns a credential value from its file.
func (s *encryptedFileStore) Load(service string) (string, error) {
	data, err := os.ReadFile(s.filePath(service))
	if err != nil {
		return "", fmt.Errorf("read credential file: %w", err)
	}

	if len(data) < saltLen {
		return "", fmt.Errorf("credential file too short")
	}

	salt := data[:saltLen]
	ciphertext := data[saltLen:]

	key := deriveKey(salt)
	plaintext, err := decryptGCM(key, ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Delete removes the credential file for the given service.
func (s *encryptedFileStore) Delete(service string) error {
	path := s.filePath(service)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete credential file: %w", err)
	}
	return nil
}

// rawBytes returns the raw encrypted bytes on disk for the given service key.
// Exposed for testing to verify ciphertext does not contain plaintext.
func (s *encryptedFileStore) rawBytes(service string) ([]byte, error) {
	return os.ReadFile(s.filePath(service))
}

// filePath returns the file path for a given service key.
func (s *encryptedFileStore) filePath(service string) string {
	// Sanitize service name for use as filename.
	safe := strings.ReplaceAll(service, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	return filepath.Join(s.dir, safe+".enc")
}

// deriveKey uses PBKDF2 with SHA-256 to derive an AES-256 key from
// machine identity (username) and the given salt.
func deriveKey(salt []byte) []byte {
	passphrase := machinePassphrase()
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, keyLen, sha256.New)
}

// machinePassphrase builds a passphrase from machine-local identity.
// Includes machine-id (non-public, per-machine unique value) for stronger entropy.
func machinePassphrase() string {
	u, err := user.Current()
	if err != nil {
		return "autopus-fallback-user"
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}
	base := u.Username + "@" + hostname

	// Append machine-id for additional entropy (not publicly exposed like hostname).
	mid := readMachineID()
	if mid != "" {
		base += ":" + mid
	}
	return base
}

// readMachineID reads a platform-specific machine identifier.
// Returns empty string if unavailable (fallback to username@hostname only).
func readMachineID() string {
	// Linux: /etc/machine-id (systemd)
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}
	// macOS: IOPlatformUUID via sysctl
	if runtime.GOOS == "darwin" {
		if data, err := exec.Command("sysctl", "-n", "kern.uuid").Output(); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id
			}
		}
	}
	return ""
}

// encryptGCM encrypts plaintext using AES-256-GCM. Returns nonce+ciphertext.
func encryptGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptGCM decrypts AES-256-GCM ciphertext (nonce prefixed).
func decryptGCM(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
