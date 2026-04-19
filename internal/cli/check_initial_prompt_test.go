// Package cli tests the initialPrompt guard check for subagent frontmatter.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentDir creates a .claude/agents/autopus directory under root and returns
// its absolute path.
func agentDir(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, ".claude", "agents", "autopus")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	return dir
}

// writeAgent writes content to <dir>/<name>.
func writeAgent(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// TC1: subagent with initialPrompt → error, check returns false.
func TestCheckInitialPrompt_SubagentWithField_Error(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := agentDir(t, root)

	writeAgent(t, dir, "executor.md", `---
name: executor
description: test executor
model: sonnet
effort: medium
tools: Read, Write
initialPrompt: "Hello from executor"
---

# Executor
`)

	var buf bytes.Buffer
	result := checkInitialPrompt(root, &buf, false)

	assert.False(t, result, "subagent with initialPrompt must cause check failure")
	assert.Contains(t, buf.String(), "executor.md", "output must name the offending file")
	assert.Contains(t, buf.String(), "initialPrompt", "output must mention the forbidden field")
	assert.Contains(t, buf.String(), "R11b", "output must reference the spec requirement")
	assert.Contains(t, buf.String(), "AGENT-GUIDE.md", "output must point to the guide")
}

// TC2: subagent without initialPrompt → pass, check returns true.
func TestCheckInitialPrompt_SubagentWithoutField_Pass(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := agentDir(t, root)

	writeAgent(t, dir, "executor.md", `---
name: executor
description: test executor
model: sonnet
effort: medium
tools: Read, Write
---

# Executor
`)

	var buf bytes.Buffer
	result := checkInitialPrompt(root, &buf, false)

	assert.True(t, result, "subagent without initialPrompt must pass")
}

// TC3: main-session auto.md with initialPrompt → allowed (pass).
func TestCheckInitialPrompt_MainSessionAutoMd_Allowed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Place auto.md directly in .claude/agents/ (main-session path).
	mainDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))

	writeAgent(t, mainDir, "auto.md", `---
name: auto
description: main session agent
model: opus
effort: high
tools: Read, Write
initialPrompt: "Bootstrap the main session"
---

# Auto
`)

	var buf bytes.Buffer
	result := checkInitialPrompt(root, &buf, false)

	assert.True(t, result, "auto.md is the main-session agent and must be exempt from the guard")
}

// TC4: multiple subagents with initialPrompt → all listed, non-zero failure.
func TestCheckInitialPrompt_MultipleViolations_AllListed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := agentDir(t, root)

	writeAgent(t, dir, "executor.md", `---
name: executor
model: sonnet
effort: medium
tools: Read
initialPrompt: "Executor init"
---
`)
	writeAgent(t, dir, "tester.md", `---
name: tester
model: sonnet
effort: medium
tools: Read
initialPrompt: "Tester init"
---
`)

	var buf bytes.Buffer
	result := checkInitialPrompt(root, &buf, false)

	assert.False(t, result, "multiple violations must cause overall failure")

	output := buf.String()
	assert.Contains(t, output, "executor.md", "executor.md violation must be listed")
	assert.Contains(t, output, "tester.md", "tester.md violation must be listed")
}

// TC5: file with no frontmatter → treated as clean, pass.
func TestCheckInitialPrompt_NoFrontmatter_Pass(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := agentDir(t, root)

	writeAgent(t, dir, "readme.md", `# Just a readme

No frontmatter here.
`)

	var buf bytes.Buffer
	result := checkInitialPrompt(root, &buf, false)

	assert.True(t, result, "file with no frontmatter must pass")
}

// TC6: parseFrontmatterInitialPrompt correctly detects initialPrompt and its line.
func TestParseFrontmatterInitialPrompt_DetectsLine(t *testing.T) {
	t.Parallel()

	content := `---
name: executor
description: test
initialPrompt: "hello"
---

body
`
	fm, line, found := parseFrontmatterInitialPrompt(content)
	assert.True(t, found)
	assert.Equal(t, "hello", fm.InitialPrompt, "InitialPrompt field must be extracted")
	assert.Equal(t, 4, line, "initialPrompt is on line 4 (1-based)")
}

// TC7: parseFrontmatterInitialPrompt returns false when field is absent.
func TestParseFrontmatterInitialPrompt_NotFound(t *testing.T) {
	t.Parallel()

	content := `---
name: executor
effort: medium
---
`
	_, _, found := parseFrontmatterInitialPrompt(content)
	assert.False(t, found)
}

