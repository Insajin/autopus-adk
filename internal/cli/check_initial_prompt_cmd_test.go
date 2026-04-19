// Package cli_test verifies the --initial-prompt-guard CLI flag via black-box testing.
package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupAgentDir creates .claude/agents/autopus under root and returns its path.
func setupAgentDir(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, ".claude", "agents", "autopus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeAgentFile writes content to <dir>/<name>.
func writeAgentFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TC8: --initial-prompt-guard flag fails when a subagent defines initialPrompt.
func TestCheckCmd_InitialPromptGuard_CLIFlag_Fail(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := setupAgentDir(t, root)

	writeAgentFile(t, dir, "executor.md", `---
name: executor
model: sonnet
effort: medium
tools: Read
initialPrompt: "should be blocked"
---
`)

	cli := newTestRootCmd()
	var buf bytes.Buffer
	cli.SetOut(&buf)
	cli.SetErr(&buf)
	cli.SetArgs([]string{"check", "--initial-prompt-guard", "--quiet", "--dir", root})

	err := cli.Execute()
	if err == nil {
		t.Fatal("expected error for subagent with initialPrompt, got nil")
	}
	output := buf.String()
	if !strings.Contains(output, "executor.md") && !strings.Contains(err.Error(), "check failed") {
		t.Errorf("output must identify violation or report check failed; got: %s / err: %v", output, err)
	}
}

// TC9: --initial-prompt-guard flag passes when no subagent defines initialPrompt.
func TestCheckCmd_InitialPromptGuard_CLIFlag_Pass(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := setupAgentDir(t, root)

	writeAgentFile(t, dir, "executor.md", `---
name: executor
model: sonnet
effort: medium
tools: Read
---
`)

	cli := newTestRootCmd()
	var buf bytes.Buffer
	cli.SetOut(&buf)
	cli.SetErr(&buf)
	cli.SetArgs([]string{"check", "--initial-prompt-guard", "--quiet", "--dir", root})

	if err := cli.Execute(); err != nil {
		t.Fatalf("--initial-prompt-guard must pass when no initialPrompt is present, got: %v", err)
	}
}
