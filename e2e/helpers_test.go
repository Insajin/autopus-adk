//go:build e2e

// Package e2e provides end-to-end test helpers for the auto CLI binary.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// RunResult holds the output and exit code of a binary invocation.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

// buildBinary builds the auto binary for testing and returns the path.
// It uses AUTOPUS_TEST_BINARY env var if set to skip the build.
func buildBinary(t *testing.T) string {
	t.Helper()

	if bin := os.Getenv("AUTOPUS_TEST_BINARY"); bin != "" {
		return bin
	}

	buildOnce.Do(func() {
		dir := t.TempDir()
		binName := "auto"
		if runtime.GOOS == "windows" {
			binName = "auto.exe"
		}
		builtBin = filepath.Join(dir, binName)
		cmd := exec.Command("go", "build", "-o", builtBin, "./cmd/auto")
		cmd.Dir = projectRoot(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = err
			t.Logf("build output: %s", out)
		}
	})

	require.NoError(t, buildErr, "failed to build auto binary")
	return builtBin
}

// runBinary executes the binary with args and returns the RunResult.
// It never calls t.Fatal on non-zero exit codes — callers assert as needed.
func runBinary(t *testing.T, bin string, args ...string) RunResult {
	t.Helper()

	cmd := exec.Command(bin, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Logf("runBinary unexpected error: %v", err)
		}
	}

	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: code,
	}
}

// projectRoot returns the absolute path to the repository root.
// It resolves upward from the e2e/ directory.
func projectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")

	// file is .../e2e/helpers_test.go — parent is the project root
	root := filepath.Dir(filepath.Dir(file))
	return root
}
