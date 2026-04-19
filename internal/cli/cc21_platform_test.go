//go:build integration

// Package cli_test provides CC21 integration tests covering platform feature detection
// (TC7: AUTOPUS_PLATFORM=codex no-op).
// TC8 lives in pkg/platform/cc21_platform_test.go because execCommandFunc injection
// requires same-package access to the unexported variable.
package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/platform"
)

// --- TC7: AUTOPUS_PLATFORM=codex → no-op, all features false (S8-1) ----------------

// TestCC21_TC7_Platform_Codex_NoOp verifies that when AUTOPUS_PLATFORM=codex,
// DetectFeatures returns all-false (R8 no-op guarantee).
// Violation → S8-1: non-claude-code runtimes must not enable CC21 features.
func TestCC21_TC7_Platform_Codex_NoOp(t *testing.T) {
	t.Setenv("AUTOPUS_PLATFORM", "codex")

	rt := platform.DetectRuntime()
	assert.Equal(t, platform.RuntimeCodex, rt,
		"S8-1: AUTOPUS_PLATFORM=codex must resolve to RuntimeCodex, got %q", rt)

	f := platform.DetectFeatures()
	assert.False(t, f.Effort,
		"S8-1 violation: Effort must be false on non-claude-code runtime")
	assert.False(t, f.InitialPrompt,
		"S8-1 violation: InitialPrompt must be false on non-claude-code runtime")
	assert.False(t, f.Monitor,
		"S8-1 violation: Monitor must be false on non-claude-code runtime")
	assert.False(t, f.TaskCreated,
		"S8-1 violation: TaskCreated must be false on non-claude-code runtime")
}
