package cli

import (
	"os"
	"os/exec"
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
		".autopus/txns/",
		".autopus/qa/runs/",
		".autopus/runtime/",
		"/.claude.json",
		"/.codex/",
		"/.gemini/",
		".agents/commands/",
		".agents/hooks.json",
		"/.mcp.json",
		"/config.toml",
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
	if !lines["/.codex/"] {
		t.Fatalf("expected root-anchored .codex/ pattern to be added despite existing .codex/skills/")
	}
	if !lines[".codex/skills/"] {
		t.Fatalf("expected existing .codex/skills/ line to be preserved")
	}
}

func TestUpdateGitignoreMigratesLegacyUnanchoredGeneratedPatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	err := os.WriteFile(path, []byte("# Custom ignores\n.gemini/\n.opencode/\n.claude.json\n.mcp.json\nconfig.toml\n.codex/skills/\n"), 0o644)
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
	if lines[".gemini/"] || lines[".opencode/"] || lines[".claude.json"] || lines[".mcp.json"] || lines["config.toml"] {
		t.Fatalf("legacy unanchored platform patterns must be migrated, got:\n%s", string(data))
	}
	if !lines["/.gemini/"] || !lines["/.opencode/"] || !lines["/.claude.json"] || !lines["/.mcp.json"] || !lines["/config.toml"] {
		t.Fatalf("expected canonical anchored platform patterns, got:\n%s", string(data))
	}
	if !lines[".codex/skills/"] {
		t.Fatalf("expected narrower custom/project ignore line to be preserved")
	}
}

func TestUpdateGitignoreAnchorsPlatformDogfoodSurface(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := updateGitignore(dir); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	writePolicyTestFile(t, dir, ".gemini/settings.json")
	writePolicyTestFile(t, dir, ".claude.json")
	writePolicyTestFile(t, dir, ".mcp.json")
	writePolicyTestFile(t, dir, "config.toml")
	writePolicyTestFile(t, dir, ".autopus/txns/20260620T010203-codex/journal.json")
	writePolicyTestFile(t, dir, "pkg/adapter/gemini/.gemini/settings.json")
	writePolicyTestFile(t, dir, "pkg/fixtures/.claude.json")
	writePolicyTestFile(t, dir, "pkg/fixtures/.mcp.json")
	writePolicyTestFile(t, dir, "pkg/fixtures/config.toml")

	rootGenerated := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", ".gemini/settings.json")
	if out, err := rootGenerated.CombinedOutput(); err != nil {
		t.Fatalf("expected root dogfood .gemini directory to be ignored: %v\n%s", err, out)
	}
	rootFiles := []string{".claude.json", ".mcp.json", "config.toml"}
	for _, rel := range rootFiles {
		cmd := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", rel)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("expected root generated file %s to be ignored: %v\n%s", rel, err, out)
		}
	}
	rootTxn := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", ".autopus/txns/20260620T010203-codex/journal.json")
	if out, err := rootTxn.CombinedOutput(); err != nil {
		t.Fatalf("expected root runtime transaction journal to be ignored: %v\n%s", err, out)
	}

	nestedFixture := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", "pkg/adapter/gemini/.gemini/settings.json")
	if err := nestedFixture.Run(); err == nil {
		t.Fatal("nested source fixture .gemini directory must not be ignored by root dogfood pattern")
	}
	nestedFiles := []string{"pkg/fixtures/.claude.json", "pkg/fixtures/.mcp.json", "pkg/fixtures/config.toml"}
	for _, rel := range nestedFiles {
		cmd := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", rel)
		if err := cmd.Run(); err == nil {
			t.Fatalf("nested source fixture %s must not be ignored by root file pattern", rel)
		}
	}
}

func writePolicyTestFile(t *testing.T, dir, rel string) {
	t.Helper()

	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
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
