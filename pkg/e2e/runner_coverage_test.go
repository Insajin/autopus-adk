// Package e2e provides user-facing scenario-based E2E test infrastructure.
package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvaluatePrimitive_StdoutContains_Pass verifies that the runner evaluates
// stdout_contains primitives correctly when the substring is present.
func TestEvaluatePrimitive_StdoutContains_Pass(t *testing.T) {
	t.Parallel()

	// Given: a RunnerResult with matching stdout
	result := &RunnerResult{ExitCode: 0, Stdout: "hello world", Stderr: ""}

	// When: evaluatePrimitive is called with a stdout_contains primitive
	vr := evaluatePrimitive(`stdout_contains("hello")`, result)

	// Then: the supported primitive executes its real check.
	assert.True(t, vr.Pass)
}

// TestEvaluatePrimitive_ExitCodeNonZero verifies parsing of exit_code(N) for N>1.
func TestEvaluatePrimitive_ExitCodeNonZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		primitive  string
		exitCode   int
		expectPass bool
	}{
		{"exit_code(0)", 0, true},
		{"exit_code(0)", 1, false},
		{"exit_code(1)", 1, true},
		{"exit_code(2)", 2, true},
		{"stderr_empty()", 0, true}, // uses Stderr="" from RunnerResult
	}

	for _, tt := range tests {
		t.Run(tt.primitive, func(t *testing.T) {
			t.Parallel()
			result := &RunnerResult{ExitCode: tt.exitCode, Stdout: "", Stderr: ""}
			vr := evaluatePrimitive(tt.primitive, result)
			assert.Equal(t, tt.expectPass, vr.Pass, "primitive=%s exitCode=%d", tt.primitive, tt.exitCode)
		})
	}
}

// TestEvaluatePrimitive_StderrEmpty_NotEmpty verifies stderr_empty fails when stderr has content.
func TestEvaluatePrimitive_StderrEmpty_NotEmpty(t *testing.T) {
	t.Parallel()

	result := &RunnerResult{ExitCode: 0, Stdout: "", Stderr: "some error output"}
	vr := evaluatePrimitive("stderr_empty()", result)
	assert.False(t, vr.Pass)
}

// TestEvaluatePrimitive_UnsafePrimitiveFailsClosed verifies that unsafe or
// unsupported primitives never default to PASS.
func TestEvaluatePrimitive_UnsafePrimitiveFailsClosed(t *testing.T) {
	t.Parallel()

	result := &RunnerResult{ExitCode: 0, Stdout: "", Stderr: ""}
	vr := evaluatePrimitive("file_exists(\"/some/path\")", result)
	assert.False(t, vr.Pass, "unsafe primitives must fail closed")
	assert.NotEmpty(t, vr.Message)
}

// TestWorkDirExists_ExistingDir verifies WorkDirExists returns true for a live directory.
func TestWorkDirExists_ExistingDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := &RunnerResult{WorkDir: dir}

	exists, err := result.WorkDirExists()
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestRun_VerifyPrimitivesInResult verifies that runner collects FailureDetails
// when a verify primitive fails during execution.
func TestRun_VerifyPrimitivesInResult(t *testing.T) {
	t.Parallel()

	// Given: a scenario expecting exit 0 but we run `exit 1`
	scenario := Scenario{
		ID:      "verify-fail-test",
		Command: "sh -c 'exit 1'",
		Verify:  []string{"exit_code(0)"},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
		Timeout:    5 * time.Second,
	})

	// When: Run is called
	result, err := runner.Run(scenario)

	// Then: Pass is false and FailureDetails contains expected/got message
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Pass)
	assert.Contains(t, result.FailureDetails, "expected exit_code(0)")
}

// TestRun_MultipleVerifyPrimitives_AllMustPass verifies that all verify
// primitives must pass for the overall result to be PASS.
func TestRun_MultipleVerifyPrimitives_AllMustPass(t *testing.T) {
	t.Parallel()

	// Given: a scenario with two verify conditions, both must pass
	// exit 0 with both exit_code(0) and stderr_empty() should PASS
	scenario := Scenario{
		ID:      "multi-verify-test",
		Command: "echo hello",
		Verify:  []string{"exit_code(0)", "stderr_empty()"},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
		Timeout:    5 * time.Second,
	})

	// When: Run is called
	result, err := runner.Run(scenario)

	// Then: overall result is PASS because both primitives pass
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Pass)
}

// TestRun_NoVerifyPrimitives_DefaultsToPass verifies that a scenario with no
// verify primitives returns PASS as long as the command runs without error.
func TestRun_NoVerifyPrimitives_DefaultsToPass(t *testing.T) {
	t.Parallel()

	// Given: a scenario with no verify primitives
	scenario := Scenario{
		ID:      "no-verify-test",
		Command: "echo ok",
		Verify:  []string{},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
		Timeout:    5 * time.Second,
	})

	// When: Run is called
	result, err := runner.Run(scenario)

	// Then: PASS (no primitives to fail)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Pass)
}
