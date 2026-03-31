package orchestra

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// sessionsDir is the directory where session persistence files are stored.
const sessionsDir = "autopus-sessions"

// OrchestraSession represents a persisted orchestra session state.
// Used by --yield-rounds to keep panes alive across commands, and by
// collect/cleanup commands to reference existing sessions.
type OrchestraSession struct {
	SessionID  string          `json:"session_id"`
	Strategy   Strategy        `json:"strategy"`
	Round      int             `json:"round"`
	TotalRounds int            `json:"total_rounds"`
	Panes      []SessionPane   `json:"panes"`
	CreatedAt  time.Time       `json:"created_at"`
}

// SessionPane stores per-provider pane state within a session.
type SessionPane struct {
	Provider   string          `json:"provider"`
	PaneID     terminal.PaneID `json:"pane_id"`
	OutputFile string          `json:"output_file,omitempty"`
}

// NewSessionID generates a unique session ID using timestamp and random suffix.
// Format: orch-{unix_ms}-{random_hex}
func NewSessionID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("orch-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// sessionFilePath returns the full path for a session persistence file.
func sessionFilePath(sessionID string) string {
	return filepath.Join(os.TempDir(), sessionsDir, sanitizeProviderName(sessionID)+".json")
}

// SaveSession persists an OrchestraSession to disk as JSON.
// Creates the sessions directory if it does not exist.
func SaveSession(session *OrchestraSession) error {
	dir := filepath.Join(os.TempDir(), sessionsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := sessionFilePath(session.SessionID)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

// LoadSession reads an OrchestraSession from disk by session ID.
func LoadSession(sessionID string) (*OrchestraSession, error) {
	path := sessionFilePath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session OrchestraSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session file: %w", err)
	}
	return &session, nil
}

// RemoveSession deletes the session persistence file for the given session ID.
func RemoveSession(sessionID string) error {
	path := sessionFilePath(sessionID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}
