package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	autopusClaudeStatusLineCommand         = ".claude/statusline.sh"
	autopusClaudeCombinedStatusLineCommand = ".claude/statusline-combined.sh"
	autopusClaudeUserCommandFile           = ".claude/statusline-user-command.txt"
)

type StatusLineState struct {
	Command string
}

func InspectStatusLine(root string) StatusLineState {
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return StatusLineState{}
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return StatusLineState{}
	}

	statusLine, ok := settings["statusLine"].(map[string]any)
	if !ok {
		return StatusLineState{}
	}

	command, _ := statusLine["command"].(string)
	return StatusLineState{Command: strings.TrimSpace(command)}
}

func statusLineStateFromValue(value any) StatusLineState {
	statusLine, ok := value.(map[string]any)
	if !ok {
		return StatusLineState{}
	}
	command, _ := statusLine["command"].(string)
	return StatusLineState{Command: strings.TrimSpace(command)}
}

func (s StatusLineState) HasCommand() bool {
	return s.Command != ""
}

func (s StatusLineState) IsAutopusReplace() bool {
	return s.Command == autopusClaudeStatusLineCommand
}

func (s StatusLineState) IsAutopusMerge() bool {
	return s.Command == autopusClaudeCombinedStatusLineCommand
}

func (s StatusLineState) IsUserManaged() bool {
	return s.HasCommand() && !s.IsAutopusReplace() && !s.IsAutopusMerge()
}

func resolveStatusLineMode(cfg *config.HarnessConfig, existing StatusLineState) config.StatusLineMode {
	if cfg != nil && cfg.Runtime.StatusLine.Mode.IsValid() {
		return cfg.Runtime.StatusLine.Mode
	}
	if existing.IsAutopusMerge() {
		return config.StatusLineModeMerge
	}
	if existing.IsAutopusReplace() || !existing.HasCommand() {
		return config.StatusLineModeReplace
	}
	return config.StatusLineModeKeep
}

func (a *Adapter) prepareStatuslineFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	mainData, err := contentfs.FS.ReadFile("statusline.sh")
	if err != nil {
		return nil, fmt.Errorf("statusline.sh 읽기 실패: %w", err)
	}

	existing := InspectStatusLine(a.root)
	mode := resolveStatusLineMode(cfg, existing)

	files := []adapter.FileMapping{
		{
			TargetPath:      filepath.Join(".claude", "statusline.sh"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(string(mainData)),
			Content:         mainData,
		},
		{
			TargetPath:      filepath.Join(".claude", "statusline-combined.sh"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(claudeCombinedStatuslineScript),
			Content:         []byte(claudeCombinedStatuslineScript),
		},
	}

	userCommand := a.resolveMergedUserStatusLineCommand(existing, mode)
	userCommandContent := userCommand
	if userCommandContent != "" && !strings.HasSuffix(userCommandContent, "\n") {
		userCommandContent += "\n"
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      filepath.Join(".claude", "statusline-user-command.txt"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(userCommandContent),
		Content:         []byte(userCommandContent),
	})

	return files, nil
}

func (a *Adapter) copyStatuslineFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files, err := a.prepareStatuslineFiles(cfg)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		targetPath := filepath.Join(a.root, file.TargetPath)
		perm := os.FileMode(0644)
		if strings.HasSuffix(file.TargetPath, ".sh") {
			perm = 0755
		}
		if err := os.WriteFile(targetPath, file.Content, perm); err != nil {
			return nil, fmt.Errorf("%s 쓰기 실패: %w", filepath.Base(file.TargetPath), err)
		}
	}

	return files, nil
}

func (a *Adapter) resolveMergedUserStatusLineCommand(existing StatusLineState, mode config.StatusLineMode) string {
	if mode != config.StatusLineModeMerge {
		return ""
	}
	if existing.IsUserManaged() {
		return existing.Command
	}
	if !existing.IsAutopusMerge() {
		return ""
	}

	data, err := os.ReadFile(filepath.Join(a.root, ".claude", "statusline-user-command.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

const claudeCombinedStatuslineScript = `#!/bin/sh
# Combined Claude statusline wrapper.
# Runs the preserved user command first, then the Autopus statusline.

set -eu

payload=$(cat)
user_command=""

if [ -f ".claude/statusline-user-command.txt" ]; then
  user_command=$(cat ".claude/statusline-user-command.txt")
fi

user_output=""
if [ -n "$user_command" ]; then
  user_output=$(printf '%s' "$payload" | sh -lc "$user_command" 2>/dev/null || true)
fi

autopus_output=$(printf '%s' "$payload" | .claude/statusline.sh 2>/dev/null || true)

if [ -n "$user_output" ] && [ -n "$autopus_output" ]; then
  printf "%s\n%s\n" "$user_output" "$autopus_output"
  exit 0
fi

if [ -n "$user_output" ]; then
  printf "%s\n" "$user_output"
  exit 0
fi

printf "%s\n" "$autopus_output"
`
