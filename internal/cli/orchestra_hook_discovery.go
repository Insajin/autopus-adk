package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// @AX:NOTE [AUTO]: hook discovery checks Claude and Codex Stop hooks and uses autopus.yaml as the project-root boundary
func isHookModeAvailable() bool {
	home, _ := os.UserHomeDir()
	cwd, err := os.Getwd()
	globalHooks := hookModeCandidates(home)
	if err != nil {
		return hookModeAvailableInCandidates(globalHooks...)
	}
	if hookModeAvailableInCandidates(globalHooks...) {
		return true
	}
	if hookModeAvailableInCandidates(hookModeCandidates(cwd)...) {
		return true
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "autopus.yaml")); err == nil {
			return hookModeAvailableInCandidates(hookModeCandidates(dir)...)
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}

type hookModeCandidate struct {
	path  string
	event string
}

func hookModeCandidates(root string) []hookModeCandidate {
	if root == "" {
		return nil
	}
	return []hookModeCandidate{
		{path: filepath.Join(root, ".claude", "settings.json"), event: "Stop"},
		{path: filepath.Join(root, ".codex", "hooks.json"), event: "Stop"},
	}
}

func hookModeAvailableInCandidates(candidates ...hookModeCandidate) bool {
	for _, candidate := range candidates {
		if candidate.path == "" || candidate.event == "" {
			continue
		}
		data, err := os.ReadFile(candidate.path)
		if err != nil {
			continue
		}
		settings := string(data)
		if strings.Contains(settings, "autopus") && strings.Contains(settings, candidate.event) {
			return true
		}
	}
	return false
}

// hookModeAvailableInDirs checks injected settings paths for both hook markers.
// Callers provide fixed paths; user-supplied path segments are not accepted.
func hookModeAvailableInDirs(globalPath, projectPath string) bool {
	candidates := make([]hookModeCandidate, 0, 2)
	for _, path := range []string{globalPath, projectPath} {
		candidates = append(candidates, hookModeCandidate{path: path, event: "Stop"})
	}
	return hookModeAvailableInCandidates(candidates...)
}
