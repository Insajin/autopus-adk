//go:build integration

// Package platform provides CC21 integration tests for feature detection.
// TC8: AUTOPUS_PLATFORM=claude-code + version 2.1.113 → all CC21 features true.
// Same-package placement is required to access the unexported execCommandFunc.
package platform

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeVersionCmd returns an exec factory that echoes the given version string.
func fakeVersionCmd(version string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", version)
	}
}

// patchExecCommandIntegration overrides execCommandFunc for the duration of a test.
func patchExecCommandIntegration(t *testing.T, fn func(ctx context.Context, name string, args ...string) *exec.Cmd) {
	t.Helper()
	orig := execCommandFunc
	execCommandFunc = fn
	t.Cleanup(func() { execCommandFunc = orig })
}

// --- TC8: AUTOPUS_PLATFORM=claude-code + version 2.1.113 → all features true -------

// TestCC21_TC8_Platform_ClaudeCode_FeaturesTrue verifies that when AUTOPUS_PLATFORM=claude-code
// and claude --version returns 2.1.113, all CC21 features are enabled.
// Violation → spec.md R8 / acceptance.md TC8.
func TestCC21_TC8_Platform_ClaudeCode_FeaturesTrue(t *testing.T) {
	t.Setenv("AUTOPUS_PLATFORM", "claude-code")
	patchExecCommandIntegration(t, fakeVersionCmd("2.1.113"))
	withCC21Config(t, enabledCC21Features())

	f := DetectFeatures()
	assert.True(t, f.Effort,
		"TC8 violation: Effort must be true for claude-code v2.1.113")
	assert.True(t, f.InitialPrompt,
		"TC8 violation: InitialPrompt must be true for claude-code v2.1.113")
	assert.True(t, f.Monitor,
		"TC8 violation: Monitor must be true for claude-code v2.1.113")
	assert.True(t, f.TaskCreated,
		"TC8 violation: TaskCreated must be true for claude-code v2.1.113")
	assert.Equal(t, "2.1.113", f.ClaudeVersion,
		"TC8 violation: ClaudeVersion must be populated with the parsed version string")
}
