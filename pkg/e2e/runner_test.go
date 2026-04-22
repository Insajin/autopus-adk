// Package e2e provides user-facing scenario-based E2E test infrastructure.
package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_SimpleCommand_ReturnsResult verifies that Run executes a command and
// returns a populated RunnerResult with stdout, stderr, and exit code.
// S3: scenario execution with PASS/FAIL reporting.
func TestRun_SimpleCommand_ReturnsResult(t *testing.T) {
	t.Parallel()

	// Given: a scenario running a simple shell command
	scenario := Scenario{
		ID:      "echo-test",
		Command: "echo hello",
		Verify:  []string{"exit_code(0)", `stdout_contains("hello")`},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
	})

	// When: Run is called
	result, err := runner.Run(scenario)

	// Then: result is non-nil, stdout contains "hello", exit code is 0
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello")
}

// TestRun_BuildRequired_AutoBuilds verifies that when a scenario requires a
// binary, the runner detects the build command and builds it automatically.
// S4: binary auto-build from Makefile/go.mod.
func TestRun_BuildRequired_AutoBuilds(t *testing.T) {
	t.Parallel()

	// Given: a project directory with a go.mod indicating a buildable binary
	projectDir := t.TempDir()
	writeGoFile(t, projectDir, "go.mod", "module example.com/buildtest\n\ngo 1.21\n")
	writeGoFile(t, projectDir, "main.go", `package main

import "fmt"

func main() { fmt.Println("built") }
`)

	scenario := Scenario{
		ID:      "build-test",
		Command: "./buildtest",
		Verify:  []string{"exit_code(0)"},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir:   projectDir,
		AutoBuild:    true,
		BuildCommand: "go build -o buildtest .",
	})

	// When: Run is called with AutoBuild enabled
	result, err := runner.Run(scenario)

	// Then: build was triggered and result is available
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.BuildOccurred, "expected auto-build to have occurred")
}

// TestRun_FailingCommand_ReportsError verifies that a failing command produces
// a RunnerResult with a non-zero exit code and error details.
// S5: failure scenario error report with command, expected vs actual, exit code.
func TestRun_FailingCommand_ReportsError(t *testing.T) {
	t.Parallel()

	// Given: a scenario expecting exit 0 but command exits non-zero
	scenario := Scenario{
		ID:      "fail-test",
		Command: "exit 1",
		Verify:  []string{"exit_code(0)"},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
	})

	// When: Run is called
	result, err := runner.Run(scenario)

	// Then: result shows FAIL state with non-zero exit code
	require.NoError(t, err) // Run itself should not error; FAIL is in result
	require.NotNil(t, result)
	assert.NotEqual(t, 0, result.ExitCode)
	assert.False(t, result.Pass)
	assert.NotEmpty(t, result.FailureDetails)
}

// TestRun_Timeout_ReturnsTimeout verifies that a command exceeding the timeout
// is killed and RunnerResult reflects the timeout.
// NF2: per-scenario 30s timeout.
func TestRun_Timeout_ReturnsTimeout(t *testing.T) {
	t.Parallel()

	// Given: a scenario with a very short timeout and a long-running command
	scenario := Scenario{
		ID:      "timeout-test",
		Command: "sleep 60",
		Verify:  []string{"exit_code(0)"},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
		Timeout:    50 * time.Millisecond,
	})

	// When: Run is called
	result, err := runner.Run(scenario)

	// Then: result indicates timeout
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.TimedOut, "expected result to indicate timeout")
	assert.False(t, result.Pass)
}

func TestRun_ProviderScenario_UsesTimeoutFloor(t *testing.T) {
	t.Parallel()

	scenario := Scenario{
		ID:       "provider-timeout-floor",
		Command:  "sleep 0.1",
		Verify:   []string{"exit_code(0)"},
		Requires: "providers",
		Status:   "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
		Timeout:    10 * time.Millisecond,
	})

	result, err := runner.Run(scenario)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.TimedOut, "provider-required scenarios should use the timeout floor")
	assert.True(t, result.Pass)
}

// TestRun_IsolatedTempDir_CleanedUp verifies that each scenario runs in its own
// temporary directory and that directory is removed after execution.
// NF3: clean isolated temp directory per scenario.
func TestRun_IsolatedTempDir_CleanedUp(t *testing.T) {
	t.Parallel()

	// Given: a scenario that writes a file to its working directory
	scenario := Scenario{
		ID:      "isolation-test",
		Command: "touch marker.txt",
		Verify:  []string{"exit_code(0)"},
		Status:  "active",
	}
	runner := NewRunner(RunnerOptions{
		ProjectDir: t.TempDir(),
	})

	// When: Run is called
	result, err := runner.Run(scenario)

	// Then: run succeeds and the isolation dir was cleaned up
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.WorkDir)
	// WorkDir should no longer exist after cleanup
	_, statErr := result.WorkDirExists()
	assert.Error(t, statErr, "work dir should be removed after run")
}

// TestRun_BuildCache_OnlyBuildsOnce verifies that multiple Run calls within the
// same Runner session do not trigger repeated builds.
// T7: sync.Once build caching.
func TestRun_BuildCache_OnlyBuildsOnce(t *testing.T) {
	t.Parallel()

	// Given: a runner with AutoBuild enabled
	projectDir := t.TempDir()
	writeGoFile(t, projectDir, "go.mod", "module example.com/cachetest\n\ngo 1.21\n")
	writeGoFile(t, projectDir, "main.go", `package main
import "fmt"
func main() { fmt.Println("cached") }
`)

	runner := NewRunner(RunnerOptions{
		ProjectDir:   projectDir,
		AutoBuild:    true,
		BuildCommand: "go build -o cachetest .",
	})

	scenario := Scenario{
		ID:      "cache-test",
		Command: "./cachetest",
		Verify:  []string{"exit_code(0)"},
		Status:  "active",
	}

	// When: Run is called twice
	result1, err1 := runner.Run(scenario)
	result2, err2 := runner.Run(scenario)

	// Then: both succeed, but build only occurred once
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.True(t, result1.BuildOccurred, "first run should build")
	assert.False(t, result2.BuildOccurred, "second run should use cached build")
}
