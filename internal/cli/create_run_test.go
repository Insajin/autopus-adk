package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunAgentCreate_InvalidName rejects path separators in the name.
func TestRunAgentCreate_InvalidName(t *testing.T) {
	t.Parallel()

	cmd := newAgentCreateSubCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runAgentCreate(cmd, "../evil", "desc", "Read", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}

// TestRunAgentCreate_DryRunOutputsTemplate renders to stdout without writing.
func TestRunAgentCreate_DryRunOutputsTemplate(t *testing.T) {
	t.Parallel()

	cmd := newAgentCreateSubCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runAgentCreate(cmd, "spec-writer", "Writes specs", "Read,Write", false)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "name: spec-writer")
	assert.Contains(t, out, "description: Writes specs")
	// spec-writer is a max-effort role.
	assert.Contains(t, out, "effort: max")
	assert.Contains(t, out, "- Read")
	assert.Contains(t, out, "- Write")
}

// TestRunAgentCreate_WriteCreatesFile writes to .claude/agents/autopus.
func TestRunAgentCreate_WriteCreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cmd := newAgentCreateSubCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runAgentCreate(cmd, "my-agent", "Helper agent", "Read", true)
	require.NoError(t, err)

	outPath := filepath.Join(dir, ".claude", "agents", "autopus", "my-agent.md")
	assert.FileExists(t, outPath)
	assert.Contains(t, buf.String(), "Agent created:")
	content, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: my-agent")
	// medium effort for an ordinary role.
	assert.Contains(t, string(content), "effort: medium")

	// Second write must detect the duplicate.
	err = runAgentCreate(cmd, "my-agent", "Helper agent", "Read", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestRunSkillCreate_InvalidName rejects path separators.
func TestRunSkillCreate_InvalidName(t *testing.T) {
	t.Parallel()

	cmd := newSkillCreateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runSkillCreate(cmd, "a/b", "desc", "", "general", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}

// TestRunSkillCreate_DryRun renders the skill template with custom triggers.
func TestRunSkillCreate_DryRun(t *testing.T) {
	t.Parallel()

	cmd := newSkillCreateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runSkillCreate(cmd, "my-skill", "Does things", "go,build", "dev", false)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "name: my-skill")
	assert.Contains(t, out, "category: dev")
	assert.Contains(t, out, "- go")
	assert.Contains(t, out, "- build")
}

// TestRunSkillCreate_WriteAndConflict writes a skill then detects a trigger conflict.
func TestRunSkillCreate_WriteAndConflict(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cmd := newSkillCreateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runSkillCreate(cmd, "first", "First skill", "shared-trigger", "general", true)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, ".claude", "skills", "autopus", "first.md"))

	// A second skill reusing the same trigger conflicts.
	err = runSkillCreate(cmd, "second", "Second skill", "shared-trigger", "general", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trigger conflict")
}
