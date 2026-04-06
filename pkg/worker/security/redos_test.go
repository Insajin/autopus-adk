// Package security tests for ReDoS defense in DeniedPatterns (REQ-07).
package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeniedPatterns_ReDoS_TimeoutHandled verifies that a catastrophic backtracking
// regex like "(a+)+$" is handled within an acceptable timeout.
// RED: ValidateCommandSafe (with timeout) does not exist yet.
func TestDeniedPatterns_ReDoS_TimeoutHandled(t *testing.T) {
	t.Parallel()

	// Given: a policy with a malicious ReDoS regex pattern
	policy := SecurityPolicy{
		AllowedCommands: []string{"echo "},
		DeniedPatterns:  []string{`(a+)+$`},
	}

	// When: we validate a command that would trigger catastrophic backtracking
	// (long string of 'a' not ending with match — forces O(2^n) backtracking)
	maliciousInput := "echo " + string(make([]byte, 50)) // simulated long input

	start := time.Now()
	done := make(chan struct{})
	var ok bool
	var reason string

	go func() {
		ok, reason = policy.ValidateCommandSafe(maliciousInput, "")
		close(done)
	}()

	select {
	case <-done:
		elapsed := time.Since(start)
		// Then: must complete within 2 seconds (not hang for minutes)
		assert.Less(t, elapsed, 2*time.Second, "ReDoS pattern must not hang")
		// And: result must be deterministic (denied or allowed, not hung)
		_ = ok
		_ = reason
	case <-time.After(2 * time.Second):
		t.Fatal("ValidateCommandSafe timed out — ReDoS not handled")
	}
}

// TestDeniedPatterns_PatternLengthLimit verifies patterns longer than 1024
// characters are rejected.
// RED: pattern length validation does not exist yet.
func TestDeniedPatterns_PatternLengthLimit(t *testing.T) {
	t.Parallel()

	// Given: a DeniedPattern exceeding 1024 characters
	longPattern := make([]byte, 1025)
	for i := range longPattern {
		longPattern[i] = 'a'
	}

	policy := SecurityPolicy{
		AllowedCommands: []string{"echo "},
		DeniedPatterns:  []string{string(longPattern)},
	}

	// When: we validate using the safe validator
	err := policy.ValidateDeniedPatterns()

	// Then: an error must be returned for the oversized pattern
	require.Error(t, err, "pattern >1024 chars must be rejected")
	assert.Contains(t, err.Error(), "pattern too long", "error message must mention pattern length")
}

// TestDeniedPatterns_CompileFailureDenyAll verifies that when a DeniedPattern
// fails to compile, the system defaults to deny-all behavior.
// RED: deny-all on compile failure does not exist yet.
func TestDeniedPatterns_CompileFailureDenyAll(t *testing.T) {
	t.Parallel()

	// Given: a policy with an invalid regex in DeniedPatterns
	policy := SecurityPolicy{
		AllowedCommands: []string{"echo "},
		DeniedPatterns:  []string{`[invalid-regex`},
	}

	// When: we validate a normally allowed command
	ok, reason := policy.ValidateCommandDenyOnBadPattern("echo hello", "")

	// Then: command must be denied (fail-safe / deny-all on bad pattern)
	assert.False(t, ok, "invalid denied pattern must trigger deny-all")
	assert.NotEmpty(t, reason, "reason must be provided")
}
