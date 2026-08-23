package antigravity

import (
	"encoding/json"
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
		body := string(file.Content)
		if filepath.ToSlash(file.TargetPath) == ".gemini/settings.json" {
			body = dedupeAntigravitySettingsPermissions(body)
		} else {
			body = sanitizeUnsupportedClaudeTeamSurface(body)
		}
		file.Content = []byte(body)
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

func dedupeAntigravitySettingsPermissions(body string) string {
	var settings map[string]any
	if err := json.Unmarshal([]byte(body), &settings); err != nil {
		return body
	}
	permissions, ok := settings["permissions"].(map[string]any)
	if !ok {
		return body
	}
	for _, key := range []string{"allow", "deny"} {
		values, ok := permissions[key].([]any)
		if !ok {
			continue
		}
		seen := make(map[string]bool, len(values))
		deduped := make([]any, 0, len(values))
		for _, value := range values {
			text, isString := value.(string)
			if isString && seen[text] {
				continue
			}
			if isString {
				seen[text] = true
			}
			deduped = append(deduped, value)
		}
		permissions[key] = deduped
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return body
	}
	return string(out) + "\n"
}

func rewriteSanitizedAntigravityMappings(root string, files []adapter.FileMapping) ([]adapter.FileMapping, error) {
	files = sanitizeUnsupportedClaudeTeamMappings(files)
	for _, file := range files {
		path := filepath.Join(root, file.TargetPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("sanitized Antigravity directory creation failed %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, file.Content, antigravityFileMode(file.TargetPath)); err != nil {
			return nil, fmt.Errorf("sanitized Antigravity surface write failed %s: %w", file.TargetPath, err)
		}
	}
	return files, nil
}
