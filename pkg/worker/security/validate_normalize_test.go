// Package security tests for input normalization (REQ-04) and word-boundary matching (REQ-05).
package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeCommand_NullByteStripping verifies null bytes are stripped
// from commands before validation.
// RED: NormalizeCommand function does not exist yet.
func TestNormalizeCommand_NullByteStripping(t *testing.T) {
	t.Parallel()

	// Given: a command containing a null byte (injection attempt)
	input := "git\x00 rm -rf /"

	// When: normalized
	got, err := NormalizeCommand(input)
	require.NoError(t, err)

	// Then: null byte is stripped, result is safe
	assert.Equal(t, "git rm -rf /", got)
	assert.NotContains(t, got, "\x00", "null bytes must be stripped")
}

// TestNormalizeCommand_UnicodeNFC verifies that unicode characters are
// normalized to NFC form.
// RED: NormalizeCommand with unicode normalization does not exist yet.
func TestNormalizeCommand_UnicodeNFC(t *testing.T) {
	t.Parallel()

	// Given: a string with decomposed unicode (NFD form of "é")
	// "e" + combining acute accent (U+0301) → NFC "é" (U+00E9)
	nfdInput := "go\u0065\u0301 build"
	nfcExpected := "go\u00e9 build"

	// When: normalized
	got, err := NormalizeCommand(nfdInput)
	require.NoError(t, err)

	// Then: output is in NFC form
	assert.Equal(t, nfcExpected, got, "unicode must be NFC-normalized")
}

// TestNormalizeCommand_PathCanonicalization verifies symlink path resolution
// for command path normalization.
// RED: NormalizeCommand with path canonicalization does not exist yet.
func TestNormalizeCommand_PathCanonicalization(t *testing.T) {
	t.Parallel()

	// Given: a path with double slashes (common normalization case)
	input := "go//build ./..."

	// When: normalized
	got, err := NormalizeCommand(input)
	require.NoError(t, err)

	// Then: path is cleaned (double slash collapsed)
	assert.NotContains(t, got, "//", "double slashes must be canonicalized")
}

// TestValidateCommand_WordBoundary_GoNotGobusterWordBoundary verifies that
// exact word-boundary matching ("go" allowed) does NOT match "gobuster".
// RED: ValidateCommandWordBoundary does not exist yet.
func TestValidateCommand_WordBoundary_GoNotGobusterWordBoundary(t *testing.T) {
	t.Parallel()

	// Given: policy where "go" is in allowed list (no trailing space)
	policy := SecurityPolicy{
		AllowedCommands: []string{"go"},
	}

	// When: command is "gobuster scan" — word boundary must prevent this match
	ok, reason := policy.ValidateCommandWordBoundary("gobuster scan", "")

	// Then: MUST be denied — "gobuster" != word "go"
	assert.False(t, ok, "gobuster must be denied with word-boundary check on 'go'")
	assert.NotEmpty(t, reason)
}

// TestValidateCommand_WordBoundary_GoAllowedWordBoundary verifies that
// word-boundary "go" correctly matches the bare "go" command.
// RED: ValidateCommandWordBoundary does not exist yet.
func TestValidateCommand_WordBoundary_GoAllowedWordBoundary(t *testing.T) {
	t.Parallel()

	// Given: policy allowing "go" with word-boundary semantics
	policy := SecurityPolicy{
		AllowedCommands: []string{"go"},
	}

	// When: command is exactly "go"
	ok, _ := policy.ValidateCommandWordBoundary("go", "")

	// Then: command MUST be allowed
	assert.True(t, ok, "bare 'go' must be allowed when 'go' is in AllowedCommands")
}

// TestValidateCommand_ExactMatch_GoOnly verifies exact match "go" only matches
// the bare "go" command, not "gobuster".
// RED: NormalizeCommand-based word-boundary validation does not exist yet.
func TestValidateCommand_ExactMatch_GoOnly(t *testing.T) {
	t.Parallel()

	// Given: policy with exact "go" (no trailing space)
	policy := SecurityPolicy{
		AllowedCommands: []string{"go"},
	}

	tests := []struct {
		cmd    string
		wantOK bool
		desc   string
	}{
		{"go", true, "bare 'go' must match"},
		{"gobuster scan", false, "'gobuster' must NOT match exact 'go'"},
		{"go build", false, "'go build' must NOT match exact 'go'"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			ok, _ := policy.ValidateCommandWordBoundary(tt.cmd, "")
			assert.Equal(t, tt.wantOK, ok, tt.desc)
		})
	}
}
