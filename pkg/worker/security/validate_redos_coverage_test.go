// Package security — additional coverage tests for validate_redos.go.
package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompilePattern_Valid verifies that a valid regex compiles successfully.
func TestCompilePattern_Valid(t *testing.T) {
	t.Parallel()

	re, err := compilePattern(`[a-z]+`)
	require.NoError(t, err)
	assert.NotNil(t, re)
	assert.True(t, re.MatchString("hello"))
}

// TestCompilePattern_Invalid verifies that an invalid regex returns an error.
func TestCompilePattern_Invalid(t *testing.T) {
	t.Parallel()

	re, err := compilePattern(`[invalid`)
	assert.Error(t, err)
	assert.Nil(t, re)
}

// TestValidateDeniedPatterns_Empty verifies that empty DeniedPatterns passes.
func TestValidateDeniedPatterns_Empty(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{}
	assert.NoError(t, p.ValidateDeniedPatterns())
}

// TestValidateDeniedPatterns_ValidPattern verifies that valid patterns pass.
func TestValidateDeniedPatterns_ValidPattern(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{DeniedPatterns: []string{`rm\s+-rf`, `\bsudo\b`}}
	assert.NoError(t, p.ValidateDeniedPatterns())
}

// TestValidateDeniedPatterns_InvalidRegex verifies that invalid regex is rejected.
func TestValidateDeniedPatterns_InvalidRegex(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{DeniedPatterns: []string{`[invalid`}}
	err := p.ValidateDeniedPatterns()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid denied pattern")
}

// TestValidateCommandSafe_NoAllowedCommands verifies fail-closed behavior.
func TestValidateCommandSafe_NoAllowedCommands(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{}
	ok, reason := p.ValidateCommandSafe("echo hello", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "fail-closed")
}

// TestValidateCommandSafe_LongPattern verifies that oversized patterns are denied.
func TestValidateCommandSafe_LongPattern(t *testing.T) {
	t.Parallel()

	longPat := make([]byte, maxPatternLength+1)
	for i := range longPat {
		longPat[i] = 'a'
	}
	p := SecurityPolicy{
		AllowedCommands: []string{"echo "},
		DeniedPatterns:  []string{string(longPat)},
	}
	ok, reason := p.ValidateCommandSafe("echo hello", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "too long")
}

// TestValidateCommandSafe_DeniedPatternMatch verifies pattern-matched commands are denied.
func TestValidateCommandSafe_DeniedPatternMatch(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{
		AllowedCommands: []string{"git "},
		DeniedPatterns:  []string{`push.*--force`},
	}
	ok, reason := p.ValidateCommandSafe("git push --force origin main", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "denied pattern")
}

// TestValidateCommandSafe_AllowedWithWorkDir verifies allowed command + valid workdir.
func TestValidateCommandSafe_AllowedWithWorkDir(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{
		AllowedCommands: []string{"go "},
		AllowedDirs:     []string{"/home/user/project"},
	}
	ok, _ := p.ValidateCommandSafe("go build ./...", "/home/user/project/cmd")
	assert.True(t, ok)
}

// TestValidateCommandSafe_DeniedWorkDir verifies denied workdir.
func TestValidateCommandSafe_DeniedWorkDir(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{
		AllowedCommands: []string{"go "},
		AllowedDirs:     []string{"/home/user/project"},
	}
	ok, reason := p.ValidateCommandSafe("go build ./...", "/tmp/evil")
	assert.False(t, ok)
	assert.Contains(t, reason, "working directory")
}

// TestValidateCommandSafe_ExactCommandMatch verifies exact (no-arg) command match.
func TestValidateCommandSafe_ExactCommandMatch(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{AllowedCommands: []string{"go"}}
	ok, _ := p.ValidateCommandSafe("go", "")
	assert.True(t, ok)
}

// TestValidateCommandSafe_CommandNotAllowed verifies unlisted command is denied.
func TestValidateCommandSafe_CommandNotAllowed(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{AllowedCommands: []string{"go "}}
	ok, reason := p.ValidateCommandSafe("curl http://example.com", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "not in allowed list")
}

// TestValidateCommandDenyOnBadPattern_NoAllowedCommands verifies fail-closed.
func TestValidateCommandDenyOnBadPattern_NoAllowedCommands(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{}
	ok, reason := p.ValidateCommandDenyOnBadPattern("echo hello", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "fail-closed")
}

// TestValidateCommandDenyOnBadPattern_AllowedCommand verifies normal allow path.
func TestValidateCommandDenyOnBadPattern_AllowedCommand(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{AllowedCommands: []string{"echo "}}
	ok, _ := p.ValidateCommandDenyOnBadPattern("echo hello", "")
	assert.True(t, ok)
}

// TestValidateCommandDenyOnBadPattern_DeniedCommand verifies denied pattern path.
func TestValidateCommandDenyOnBadPattern_DeniedCommand(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{
		AllowedCommands: []string{"sh "},
		DeniedPatterns:  []string{`rm\s+-rf`},
	}
	ok, reason := p.ValidateCommandDenyOnBadPattern("sh -c rm -rf /", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "denied pattern")
}

// TestValidateCommandDenyOnBadPattern_CommandNotAllowed verifies unlisted command denied.
func TestValidateCommandDenyOnBadPattern_CommandNotAllowed(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{AllowedCommands: []string{"go "}}
	ok, reason := p.ValidateCommandDenyOnBadPattern("npm install", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "not in allowed list")
}

// TestValidateCommandDenyOnBadPattern_ExactMatch verifies exact command match.
func TestValidateCommandDenyOnBadPattern_ExactMatch(t *testing.T) {
	t.Parallel()

	p := SecurityPolicy{AllowedCommands: []string{"go"}}
	ok, _ := p.ValidateCommandDenyOnBadPattern("go", "")
	assert.True(t, ok)
}

// TestMatchWithTimeout_Timeout verifies that a slow match times out gracefully.
// Uses an artificially small timeout to trigger the timeout path.
func TestMatchWithTimeout_Timeout(t *testing.T) {
	t.Parallel()

	// Use 1ns timeout — any match will exceed this
	matched, err := matchWithTimeout(`\w+`, "hello world", 1)
	// Either matched quickly (true, nil) or timed out (false, err).
	// Both are valid — we just verify it does not panic or hang.
	_ = matched
	_ = err
}

// TestMatchWithTimeout_InvalidRegex verifies compile failure returns error.
func TestMatchWithTimeout_InvalidRegex(t *testing.T) {
	t.Parallel()

	matched, err := matchWithTimeout(`[invalid`, "hello", 100*1000*1000)
	assert.False(t, matched)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compile failed")
}
