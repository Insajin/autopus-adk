package orchestra

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestBuildInteractiveLaunchCmd_GeminiNoRunStrip verifies Antigravity strips "run" (not args mode).
func TestBuildInteractiveLaunchCmd_GeminiNoRunStrip(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		Name:             "gemini",
		Binary:           "agy",
		PaneArgs:         []string{},
		Args:             []string{"--print", ""},
		PromptViaArgs:    true,
		InteractiveInput: "stdin",
	}

	cmd := buildInteractiveLaunchCmd(p, "test prompt")
	assert.Equal(t, "agy --dangerously-skip-permissions --prompt-interactive 'test prompt'", cmd,
		"gemini pane launch must use prompt-interactive instead of --print or a bare positional prompt")
}

func TestBuildInteractiveLaunchCmd_GeminiNoPrompt(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		Name:             "gemini",
		Binary:           "agy",
		Args:             []string{"--print", ""},
		PromptViaArgs:    true,
		InteractiveInput: "stdin",
	}

	cmd := buildInteractiveLaunchCmd(p, "")
	assert.Equal(t, "agy --dangerously-skip-permissions", cmd, "gemini relaunch without a prompt should open the TUI")
}

func TestBuildPaneLaunchCommand_AntigravityPromptUsesShortScript(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	p := ProviderConfig{
		Name:             "gemini",
		Binary:           "agy",
		PromptViaArgs:    true,
		InteractiveInput: "stdin",
	}

	cmd, launchFile, err := buildPaneLaunchCommand(workingDir, p, "open the Markdown file and reply PONG")
	require.NoError(t, err)
	defer cleanupPromptFiles([]string{launchFile})

	require.NotEmpty(t, launchFile)
	assert.Contains(t, cmd, "/bin/sh ")
	assert.Contains(t, cmd, launchFile)
	assert.NotContains(t, cmd, "open the Markdown file",
		"pane input must stay short; the long prompt belongs in the script")

	content, err := os.ReadFile(launchFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "cd "+shellQuote(workingDir)+" && agy")
	assert.Contains(t, string(content), "--dangerously-skip-permissions")
	assert.Contains(t, string(content), "--prompt-interactive")
	assert.Contains(t, string(content), "open the Markdown file and reply PONG")
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

func TestBuildInteractiveLaunchCmd_CodexPreservesConfigQuoting(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		Name:     "codex",
		Binary:   "codex",
		PaneArgs: []string{"-m", "gpt-5.5", "-c", `model_reasoning_effort="xhigh"`},
	}

	cmd := buildInteractiveLaunchCmd(p, "")
	assert.Equal(t, `codex -m gpt-5.5 -c 'model_reasoning_effort="xhigh"'`, cmd)
}
