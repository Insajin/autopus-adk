// Package security — additional coverage tests for validate_normalize.go.
package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateCommandWordBoundary_NoAllowedCommands verifies fail-closed behavior.
func TestValidateCommandWordBoundary_NoAllowedCommands(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{}
	ok, reason := p.ValidateCommandWordBoundary("go", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "fail-closed")
}

// TestValidateCommandWordBoundary_DeniedPatternMatch verifies denied pattern blocks.
func TestValidateCommandWordBoundary_DeniedPatternMatch(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{
		AllowedCommands: []string{"git"},
		DeniedPatterns:  []string{`push.*--force`},
	}
	ok, reason := p.ValidateCommandWordBoundary("git push --force", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "denied pattern")
}

// TestValidateCommandWordBoundary_InvalidPattern verifies invalid regex denied.
func TestValidateCommandWordBoundary_InvalidPattern(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{
		AllowedCommands: []string{"go"},
		DeniedPatterns:  []string{`[invalid`},
	}
	ok, reason := p.ValidateCommandWordBoundary("go", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "invalid denied pattern")
}

// TestValidateCommandWordBoundary_NoMatch verifies no-match returns error.
func TestValidateCommandWordBoundary_NoMatch(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{AllowedCommands: []string{"go"}}
	ok, reason := p.ValidateCommandWordBoundary("git status", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "not in allowed list")
}

// TestNormalizeCommand_MultipleNullBytes verifies multiple nulls are all stripped.
func TestNormalizeCommand_MultipleNullBytes(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCommand("git\x00status\x00--short")
	require.NoError(t, err)
	assert.NotContains(t, got, "\x00")
	assert.Equal(t, "gitstatus--short", got)
}

// TestNormalizeCommand_NoModification verifies clean input without paths is unchanged.
func TestNormalizeCommand_NoModification(t *testing.T) {
	t.Parallel()

	// Use a command without path components (no "/" in args) so filepath.Clean
	// is not invoked on any argument.
	input := "echo hello"
	got, err := NormalizeCommand(input)
	require.NoError(t, err)
	assert.Equal(t, input, got)
}

// TestNormalizeCommand_LeadingTrailingWhitespace verifies trimming.
// Note: filepath.Clean removes leading "./" so "go build ./..." → "go build ..."
func TestNormalizeCommand_LeadingTrailingWhitespace(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCommand("  go build .  ")
	require.NoError(t, err)
	assert.Equal(t, "go build .", got)
}

// TestNormalizeCommand_EmptyString verifies empty input returns empty.
func TestNormalizeCommand_EmptyString(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCommand("")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}
