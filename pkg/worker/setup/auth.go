package setup

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
)

// GeneratePKCE creates a PKCE code verifier and its S256 challenge.
// The verifier is 32 random bytes, base64url-encoded.
// The challenge is SHA256(verifier), base64url-encoded.
func GeneratePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

// OpenBrowser opens the given URL in the default browser.
func OpenBrowser(u string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return exec.Command(cmd, u).Start()
}

// SaveCredentials writes credentials to ~/.config/autopus/credentials.json.
func SaveCredentials(creds map[string]any) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	return saveCredentialBytes(data)
}
