package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildInteractiveLaunchCmd_OpencodeWithArgs verifies opencode with InteractiveInput="args"
// keeps "run" and appends the prompt as a quoted CLI argument.
func TestBuildInteractiveLaunchCmd_OpencodeWithArgs(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		Name:             "opencode",
		Binary:           "opencode",
		PaneArgs:         []string{"run", "-m", "openai/gpt-5.4"},
		InteractiveInput: "args",
	}

	cmd := buildInteractiveLaunchCmd(p, "fix the bug")
	assert.Contains(t, cmd, "opencode run -m openai/gpt-5.4")
	assert.Contains(t, cmd, "'fix the bug'")
}

// TestBuildInteractiveLaunchCmd_OpencodeWithArgs_NoPrompt verifies no prompt appended when empty.
func TestBuildInteractiveLaunchCmd_OpencodeWithArgs_NoPrompt(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		Name:             "opencode",
		Binary:           "opencode",
		PaneArgs:         []string{"run", "-m", "openai/gpt-5.4"},
		InteractiveInput: "args",
	}

	cmd := buildInteractiveLaunchCmd(p, "")
	assert.Contains(t, cmd, "opencode run -m openai/gpt-5.4")
	assert.NotContains(t, cmd, "'")
}

// TestBuildInteractiveLaunchCmd_Claude verifies claude skips "run" and adds permission bypass.
func TestBuildInteractiveLaunchCmd_Claude(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		Name:     "claude",
		Binary:   "claude",
		PaneArgs: []string{"-p", "--model", "opus", "--effort", "max"},
	}

	cmd := buildInteractiveLaunchCmd(p, "review this code")
	// -p (print flag) should be stripped; verify it doesn't appear as a standalone arg
	assert.NotContains(t, cmd, " -p ")
	assert.Contains(t, cmd, "--model opus")
	assert.Contains(t, cmd, "--dangerously-skip-permissions")
	// Claude is not args-mode, so prompt should NOT be appended
	assert.NotContains(t, cmd, "review this code")
}

// TestBuildInteractiveLaunchCmd_GeminiNoRunStrip verifies gemini strips "run" (not args mode).
func TestBuildInteractiveLaunchCmd_GeminiNoRunStrip(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		Name:     "gemini",
		Binary:   "gemini",
		PaneArgs: []string{"run", "-m", "gemini-3.1-pro-preview"},
	}

	cmd := buildInteractiveLaunchCmd(p, "test prompt")
	// "run" should be stripped for non-args providers
	assert.NotContains(t, cmd, " run ")
	assert.Contains(t, cmd, "gemini -m gemini-3.1-pro-preview")
}

// TestBuildInteractiveLaunchCmd_ShellQuoteEscape verifies single quotes in prompt are escaped.
func TestBuildInteractiveLaunchCmd_ShellQuoteEscape(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		Name:             "opencode",
		Binary:           "opencode",
		PaneArgs:         []string{"run", "-m", "gpt-5.4"},
		InteractiveInput: "args",
	}

	cmd := buildInteractiveLaunchCmd(p, "it's a test")
	assert.Contains(t, cmd, "'it'\\''s a test'")
}
