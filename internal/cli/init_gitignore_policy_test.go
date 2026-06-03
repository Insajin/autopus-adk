package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitignorePatternsUseNarrowAutopusPolicy(t *testing.T) {
	t.Parallel()

	patterns := map[string]bool{}
	for _, pattern := range gitignorePatterns {
		patterns[pattern] = true
	}

	forbidden := []string{
		".autopus/",
		".autopus/project/",
		".autopus/specs/",
		".autopus/qa/",
		".autopus/qa/journeys/",
	}
	for _, pattern := range forbidden {
		if patterns[pattern] {
			t.Fatalf("gitignore pattern %q would hide human-managed Autopus docs", pattern)
		}
	}

	required := []string{
		".autopus/*-manifest.json",
		".autopus/plugins/",
		".autopus/qa/runs/",
		".autopus/runtime/",
		".codex/",
		".agents/commands/",
		".agents/hooks.json",
		"config.toml",
	}
	for _, pattern := range required {
		if !patterns[pattern] {
			t.Fatalf("missing generated/runtime gitignore pattern %q", pattern)
		}
	}
}

func TestUpdateGitignoreMatchesExactPatternLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	err := os.WriteFile(path, []byte(".codex/skills/\n.autopus/runtime/\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if err := updateGitignore(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := gitignoreLineSet(string(data))
	if !lines[".codex/"] {
		t.Fatalf("expected broad .codex/ pattern to be added despite existing .codex/skills/")
	}
	if !lines[".codex/skills/"] {
		t.Fatalf("expected existing .codex/skills/ line to be preserved")
	}
}

func gitignoreLineSet(content string) map[string]bool {
	lines := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines[line] = true
	}
	return lines
}
