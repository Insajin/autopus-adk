package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// sanitizeUnsupportedClaudeTeamMappings is the final Antigravity platform
// boundary. Shared prose can describe orchestration semantics, but generated
// Antigravity files must never expose callable Claude team primitives or a
// dangling reference to a team skill that is not compiled for this platform.
func sanitizeUnsupportedClaudeTeamMappings(files []adapter.FileMapping) []adapter.FileMapping {
	sanitized := make([]adapter.FileMapping, len(files))
	for i, file := range files {
		file.Content = []byte(sanitizeUnsupportedClaudeTeamSurface(string(file.Content)))
		file.Checksum = checksum(string(file.Content))
		sanitized[i] = file
	}
	return sanitized
}

func sanitizeUnsupportedClaudeTeamSurface(body string) string {
	return strings.NewReplacer(
		"TeamCreate", "unsupported_claude_team_create",
		"TeamDelete", "unsupported_claude_team_delete",
		"SendMessage", "unsupported_claude_team_message",
		"agent-teams/SKILL.md", "unsupported-team-mode.md",
		"skills/autopus/agent-teams.md", "unsupported-team-mode.md",
	).Replace(body)
}

func rewriteSanitizedGeminiMappings(root string, files []adapter.FileMapping) ([]adapter.FileMapping, error) {
	files = sanitizeUnsupportedClaudeTeamMappings(files)
	for _, file := range files {
		path := filepath.Join(root, file.TargetPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("sanitized Antigravity directory creation failed %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, file.Content, geminiFileMode(file.TargetPath)); err != nil {
			return nil, fmt.Errorf("sanitized Antigravity surface write failed %s: %w", file.TargetPath, err)
		}
	}
	return files, nil
}
