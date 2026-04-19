package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	selfVerifyLogName     = ".self-verify.log"
	maxSelfVerifyLogLines = 100
)

// SelfVerifyEntry is a structured self-verification log entry.
type SelfVerifyEntry struct {
	Timestamp time.Time       `json:"timestamp"`
	Dimension string          `json:"dimension"`
	Status    ChecklistStatus `json:"status"`
	Reason    string          `json:"reason,omitempty"`
}

// AppendSelfVerifyEntry appends one JSON Lines entry and keeps only the latest 100 lines.
func AppendSelfVerifyEntry(specDir string, entry SelfVerifyEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	if entry.Status != ChecklistStatusPass && entry.Status != ChecklistStatusFail {
		return fmt.Errorf("invalid self-verify status: %s", entry.Status)
	}

	path := filepath.Join(specDir, selfVerifyLogName)
	lines, err := loadSelfVerifyLines(path)
	if err != nil {
		return err
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal self-verify entry: %w", err)
	}

	lines = append(lines, string(data))
	if len(lines) > maxSelfVerifyLogLines {
		lines = lines[len(lines)-maxSelfVerifyLogLines:]
	}

	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write self-verify log: %w", err)
	}
	return nil
}

func loadSelfVerifyLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read self-verify log: %w", err)
	}

	rawLines := strings.Split(strings.TrimSpace(string(data)), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
}
