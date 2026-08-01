package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCmd_HygieneBlocksStagedGeneratedDrift(t *testing.T) {
	dir := t.TempDir()
	initHygieneTestGitRepo(t, dir)
	writeNestedTestFile(t, dir, ".codex/agents/reviewer.toml", "name = \"reviewer\"\n")
	runHygieneGitCommand(t, dir, "add", ".codex/agents/reviewer.toml")

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

	if err := root.Execute(); err == nil {
		t.Fatal("expected staged generated drift to fail")
	}
	if got := buf.String(); !strings.Contains(got, ".codex/agents/reviewer.toml") {
		t.Fatalf("expected generated drift path in output, got: %s", got)
	}
}

func TestCheckCmd_HygieneAllowsRootProjectDocs(t *testing.T) {
	dir := t.TempDir()
	initHygieneTestGitRepo(t, dir)
	writeNestedTestFile(t, dir, ".autopus/project/workspace.md", "# Workspace\n")
	writeNestedTestFile(t, dir, ".autopus/specs/SPEC-EXAMPLE/spec.md", "# Spec\n")
	runHygieneGitCommand(t, dir, "add", ".autopus/project/workspace.md", ".autopus/specs/SPEC-EXAMPLE/spec.md")

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

	if err := root.Execute(); err != nil {
		t.Fatalf("human-managed project/spec docs should pass hygiene: %v\n%s", err, buf.String())
	}
}

func TestCheckCmd_HygieneAllowsGeneratedWithSourceOfTruthChange(t *testing.T) {
	dir := t.TempDir()
	initHygieneTestGitRepo(t, dir)
	writeNestedTestFile(t, dir, ".codex/agents/reviewer.toml", "name = \"reviewer\"\n")
	writeNestedTestFile(t, dir, "templates/codex/agents/reviewer.toml.tmpl", "name = \"reviewer\"\n")
	runHygieneGitCommand(t, dir, "add", ".codex/agents/reviewer.toml", "templates/codex/agents/reviewer.toml.tmpl")

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

	if err := root.Execute(); err != nil {
		t.Fatalf("generated drift with staged source-of-truth change should pass: %v\n%s", err, buf.String())
	}
}

func TestCheckCmd_HygieneAllowsGeneratedWithMatchingContentSourceChange(t *testing.T) {
	dir := t.TempDir()
	initHygieneTestGitRepo(t, dir)
	writeNestedTestFile(t, dir, ".codex/agents/reviewer.toml", "name = \"reviewer\"\n")
	writeNestedTestFile(t, dir, "content/agents/reviewer.md", "---\nname: reviewer\n---\n")
	runHygieneGitCommand(t, dir, "add", ".codex/agents/reviewer.toml", "content/agents/reviewer.md")

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

	if err := root.Execute(); err != nil {
		t.Fatalf("generated drift with matching content source should pass: %v\n%s", err, buf.String())
	}
}

func TestCheckCmd_HygieneBlocksGeneratedWithUnrelatedSourceChange(t *testing.T) {
	dir := t.TempDir()
	initHygieneTestGitRepo(t, dir)
	writeNestedTestFile(t, dir, ".codex/agents/reviewer.toml", "name = \"reviewer\"\n")
	writeNestedTestFile(t, dir, "internal/cli/check.go", "package cli\n")
	runHygieneGitCommand(t, dir, "add", ".codex/agents/reviewer.toml", "internal/cli/check.go")

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

	if err := root.Execute(); err == nil {
		t.Fatal("expected unrelated source change not to allow generated drift")
	}
	if got := buf.String(); !strings.Contains(got, ".codex/agents/reviewer.toml") {
		t.Fatalf("expected generated drift path in output, got: %s", got)
	}
}

func TestCheckCmd_HygieneBlocksGeneratedWithUnrelatedContentSourceChange(t *testing.T) {
	dir := t.TempDir()
	initHygieneTestGitRepo(t, dir)
	writeNestedTestFile(t, dir, ".codex/agents/reviewer.toml", "name = \"reviewer\"\n")
	writeNestedTestFile(t, dir, "content/workflows/route_a.md", "# Route A\n")
	runHygieneGitCommand(t, dir, "add", ".codex/agents/reviewer.toml", "content/workflows/route_a.md")

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

	if err := root.Execute(); err == nil {
		t.Fatal("expected unrelated content source change not to allow generated drift")
	}
	if got := buf.String(); !strings.Contains(got, ".codex/agents/reviewer.toml") {
		t.Fatalf("expected generated drift path in output, got: %s", got)
	}
}

func TestCheckCmd_HygieneBlocksRuntimeEvenWithSourceOfTruthChange(t *testing.T) {
	dir := t.TempDir()
	initHygieneTestGitRepo(t, dir)
	writeNestedTestFile(t, dir, ".autopus/txns/20260620T010203-codex/journal.json", "{}\n")
	writeNestedTestFile(t, dir, "templates/codex/agents/reviewer.toml.tmpl", "name = \"reviewer\"\n")
	runHygieneGitCommand(t, dir, "add", ".autopus/txns/20260620T010203-codex/journal.json", "templates/codex/agents/reviewer.toml.tmpl")

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

	if err := root.Execute(); err == nil {
		t.Fatal("expected runtime drift to stay blocked")
	}
	if got := buf.String(); !strings.Contains(got, ".autopus/txns/20260620T010203-codex/journal.json") {
		t.Fatalf("expected runtime drift path in output, got: %s", got)
	}
}

func TestCheckCmd_HygieneAllowsGeneratedRuntimeUntrackCleanup(t *testing.T) {
	dir := t.TempDir()
	initHygieneTestGitRepo(t, dir)
	writeNestedTestFile(t, dir, ".autopus/brainstorms/BS-001.md", "# Brainstorm\n")
	runHygieneGitCommand(t, dir, "add", "-f", ".autopus/brainstorms/BS-001.md")
	runHygieneGitCommand(t, dir, "commit", "-m", "test fixture")
	runHygieneGitCommand(t, dir, "rm", "--cached", ".autopus/brainstorms/BS-001.md")

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

	if err := root.Execute(); err != nil {
		t.Fatalf("generated/runtime index-only cleanup should pass hygiene: %v\n%s", err, buf.String())
	}
}

// conditionalGeneratedPaths are the SPEC-CONDRULE-001 artifacts that live under
// .claude/hooks/ but are compiled from the rule source.
var conditionalGeneratedPaths = []string{
	".claude/hooks/autopus/conditional/lore-commit.md",
	".claude/hooks/autopus/conditional-rules.json",
}

func TestCheckCmd_HygieneAllowsConditionalRulesWithRuleSourceChange(t *testing.T) {
	for _, generated := range conditionalGeneratedPaths {
		t.Run(generated, func(t *testing.T) {
			dir := t.TempDir()
			initHygieneTestGitRepo(t, dir)
			writeNestedTestFile(t, dir, generated, "# Lore Commit\n")
			writeNestedTestFile(t, dir, "content/rules/lore-commit.md", "---\nname: lore-commit\n---\n")
			runHygieneGitCommand(t, dir, "add", generated, "content/rules/lore-commit.md")

			root := newTestRootCmd()
			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetErr(&buf)
			root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

			if err := root.Execute(); err != nil {
				t.Fatalf("conditional artifact with matching rule source should pass: %v\n%s", err, buf.String())
			}
		})
	}
}

// TestCheckCmd_HygieneBlocksConditionalRulesWithHookSourceChange is the
// discriminating case: before the conditional case was inserted ahead of the
// generic .claude/hooks/ case, a content/hooks/ edit wrongly attributed these
// artifacts and let them through.
func TestCheckCmd_HygieneBlocksConditionalRulesWithHookSourceChange(t *testing.T) {
	for _, generated := range conditionalGeneratedPaths {
		t.Run(generated, func(t *testing.T) {
			dir := t.TempDir()
			initHygieneTestGitRepo(t, dir)
			writeNestedTestFile(t, dir, generated, "# Lore Commit\n")
			writeNestedTestFile(t, dir, "content/hooks/task-created-validate.sh", "#!/bin/sh\n")
			runHygieneGitCommand(t, dir, "add", generated, "content/hooks/task-created-validate.sh")

			root := newTestRootCmd()
			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetErr(&buf)
			root.SetArgs([]string{"check", "--hygiene", "--quiet", "--dir", dir})

			if err := root.Execute(); err == nil {
				t.Fatal("expected hook source change not to attribute a conditional rule artifact")
			}
			if got := buf.String(); !strings.Contains(got, generated) {
				t.Fatalf("expected generated drift path in output, got: %s", got)
			}
		})
	}
}

func initHygieneTestGitRepo(t *testing.T, dir string) {
	t.Helper()

	runHygieneGitCommand(t, dir, "init")
	runHygieneGitCommand(t, dir, "config", "user.email", "test@test.com")
	runHygieneGitCommand(t, dir, "config", "user.name", "Test")
}

func runHygieneGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeNestedTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
