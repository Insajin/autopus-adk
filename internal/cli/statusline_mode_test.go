package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

// TestValidateStatusLineMode_ValidValues verifies accepted mode strings.
func TestValidateStatusLineMode_ValidValues(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"", "keep", "merge", "replace", "KEEP", "Merge"} {
		assert.NoError(t, validateStatusLineMode(mode), "mode=%q", mode)
	}
}

// TestValidateStatusLineMode_InvalidValue returns an explanatory error.
func TestValidateStatusLineMode_InvalidValue(t *testing.T) {
	t.Parallel()

	err := validateStatusLineMode("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --statusline-mode")
	assert.Contains(t, err.Error(), "keep, merge, or replace")
}

// TestDescribeStatusLineDecision_AllModes covers every description branch.
func TestDescribeStatusLineDecision_AllModes(t *testing.T) {
	t.Parallel()

	// Keep — no existing command.
	noneState := claude.StatusLineState{}
	got := describeStatusLineDecision(noneState, config.StatusLineModeKeep)
	assert.Contains(t, got, "no existing command")

	// Replace — no existing user command.
	got = describeStatusLineDecision(noneState, config.StatusLineModeReplace)
	assert.Contains(t, got, "Autopus statusLine enabled")

	// Merge — no existing user command.
	got = describeStatusLineDecision(noneState, config.StatusLineModeMerge)
	assert.Contains(t, got, "combined mode enabled")

	// Default empty mode.
	got = describeStatusLineDecision(noneState, config.StatusLineMode(""))
	assert.Empty(t, got)
}
