package orchestra

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// OrchestraSession holds state for a yield-rounds orchestra session.
// Persisted to /tmp/autopus-orch-session-{ID}.json for collect/cleanup commands.
type OrchestraSession struct {
	ID        string                      `json:"id"`
	Panes     map[string]string           `json:"panes"`     // provider name -> pane ID
	Providers []SessionProviderConfig     `json:"providers"`
	Rounds    [][]SessionProviderResponse `json:"rounds"`
	CreatedAt time.Time                   `json:"created_at"`
}

// SessionProviderConfig is a serializable subset of ProviderConfig.
type SessionProviderConfig struct {
	Name   string `json:"name"`
	Binary string `json:"binary"`
}

// SessionProviderResponse is a serializable subset of ProviderResponse.
type SessionProviderResponse struct {
	Provider   string `json:"provider"`
	Output     string `json:"output"`
	DurationMs int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out"`
}

// NewSessionID generates a unique session ID: orch-{timestamp}-{random hex}.
func NewSessionID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("orch-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// sessionFilePath returns the path for a session persistence file.
func sessionFilePath(id string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("autopus-orch-session-%s.json", id))
}

// SaveSession writes session metadata to a temp file with 0600 permissions.
func SaveSession(session OrchestraSession) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return os.WriteFile(sessionFilePath(session.ID), data, 0o600)
}

// LoadSession reads and parses a session file by ID.
func LoadSession(id string) (*OrchestraSession, error) {
	data, err := os.ReadFile(sessionFilePath(id))
	if err != nil {
		return nil, fmt.Errorf("read session %s: %w", id, err)
	}
	var session OrchestraSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session %s: %w", id, err)
	}
	return &session, nil
}

// RemoveSession deletes the session persistence file.
// Returns nil if the file doesn't exist (idempotent).
func RemoveSession(id string) error {
	err := os.Remove(sessionFilePath(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
