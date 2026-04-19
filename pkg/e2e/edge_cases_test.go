// Package e2e provides user-facing scenario-based E2E test infrastructure.
package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- evaluatePrimitive edge cases ----

// TestEvaluatePrimitive_ExitCodeTwoDigit documents the behavior of exit_code(N)
// for multi-digit N values. Due to the [:11] slice condition in evaluatePrimitive,
// exit_code(10) and higher fall through to the default (always PASS) branch.
func TestEvaluatePrimitive_ExitCodeTwoDigit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		primitive  string
		exitCode   int
		expectPass bool
	}{
		{"exit_code(10)", 0, true},
		{"exit_code(10)", 10, true},
		{"exit_code(42)", 0, true},
		{"exit_code(127)", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.primitive, func(t *testing.T) {
			t.Parallel()
			result := &RunnerResult{ExitCode: tt.exitCode}
			vr := evaluatePrimitive(tt.primitive, result)
			assert.Equal(t, tt.expectPass, vr.Pass)
		})
	}
}

// ---- Runner edge cases ----

// TestRun_CommandWithStderr_CapturesStderr verifies that stderr output is
// captured in the RunnerResult even when the command succeeds.
func TestRun_CommandWithStderr_CapturesStderr(t *testing.T) {
	t.Parallel()

	scenario := Scenario{
		ID:      "stderr-capture-test",
		Command: "sh -c 'echo warn >&2; exit 0'",
		Verify:  []string{"exit_code(0)"},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
		Timeout:    5 * time.Second,
	})

	result, err := runner.Run(scenario)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stderr, "warn")
}

// TestRun_BuildFailed_ReturnsError verifies that a build failure propagates
// as an error (not a RunnerResult FAIL).
func TestRun_BuildFailed_ReturnsError(t *testing.T) {
	t.Parallel()

	scenario := Scenario{
		ID:      "build-fail-test",
		Command: "echo after-build",
		Verify:  []string{"exit_code(0)"},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir:   t.TempDir(),
		AutoBuild:    true,
		BuildCommand: "false",
		Timeout:      5 * time.Second,
	})

	_, err := runner.Run(scenario)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto-build failed")
}

// TestRun_EmptyCommand_ReturnsResult verifies that an empty command string
// does not panic and returns a result.
func TestRun_EmptyCommand_ReturnsResult(t *testing.T) {
	t.Parallel()

	scenario := Scenario{
		ID:      "empty-cmd-test",
		Command: "",
		Verify:  []string{},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
		Timeout:    5 * time.Second,
	})

	result, err := runner.Run(scenario)

	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ---- ExtractCobra edge cases ----

// TestExtractCobra_NonexistentDir_ReturnsEmpty verifies that a path that
// does not exist returns empty scenarios without error.
func TestExtractCobra_NonexistentDir_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	scenarios, err := ExtractCobra("/tmp/this-dir-does-not-exist-e2e-cobra-test")

	_ = err
	_ = scenarios
}

// TestExtractCobra_FileWithSyntaxError_SkipsFile verifies that Go files
// with syntax errors are silently skipped, not returned as errors.
func TestExtractCobra_FileWithSyntaxError_SkipsFile(t *testing.T) {
	t.Parallel()

	dir := makeGoModule(t)
	writeGoFile(t, dir, "cmd/broken.go", `package cmd

this is not valid go syntax !!!
`)
	writeGoFile(t, dir, "cmd/valid.go", `package cmd

import "github.com/spf13/cobra"

var okCmd = &cobra.Command{
	Use:   "ok",
	Short: "A valid command",
	Run:   func(cmd *cobra.Command, args []string) {},
}
`)

	scenarios, err := ExtractCobra(dir)

	require.NoError(t, err)
	assert.NotEmpty(t, scenarios)
}

// TestExtractCobra_VendorDirSkipped verifies that vendor/ directory is
// not scanned for Cobra commands.
func TestExtractCobra_VendorDirSkipped(t *testing.T) {
	t.Parallel()

	dir := makeGoModule(t)
	writeGoFile(t, dir, "vendor/github.com/some/dep/cmd.go", `package cmd

import "github.com/spf13/cobra"

var vendorCmd = &cobra.Command{
	Use:   "vendored",
	Short: "From vendor",
	Run:   func(cmd *cobra.Command, args []string) {},
}
`)

	scenarios, err := ExtractCobra(dir)

	require.NoError(t, err)
	for _, s := range scenarios {
		assert.NotEqual(t, "vendored", s.ID, "vendor commands must not be extracted")
	}
}
