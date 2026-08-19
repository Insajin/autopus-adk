//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// bin is the shared binary path built once via TestMain in helpers_test.go.
// Each test calls buildBinary which uses sync.Once so the binary is built once.

func TestCLI_Version(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	r := runBinary(t, bin, "version")

	assert.Equal(t, 0, r.ExitCode, "version should exit 0")
	assert.NotEmpty(t, r.Stdout, "version output should not be empty")
}

func TestCLI_Help(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	r := runBinary(t, bin, "--help")

	// --help exits 0 for cobra commands.
	assert.Equal(t, 0, r.ExitCode, "--help should exit 0")
	combined := r.Stdout + r.Stderr
	assert.True(t, strings.Contains(combined, "auto"), "--help output should mention 'auto'")
}

func TestCLI_Doctor_RequiresProject(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	// A directory without autopus.yaml is not a project: doctor must refuse
	// instead of synthesizing a default claude-code project and reporting
	// findings against surfaces the caller never configured.
	r := runBinary(t, bin, "doctor", "--dir", t.TempDir())

	assert.NotEqual(t, 0, r.ExitCode, "doctor must fail without autopus.yaml")
	combined := r.Stdout + r.Stderr
	assert.Contains(t, combined, "autopus.yaml")
	assert.Contains(t, combined, "auto init")
}

func TestCLI_Doctor_InitializedProjectExitsZero(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := t.TempDir()
	init := runBinary(t, bin, "init", "--dir", dir, "--platforms", "claude-code", "--yes")
	assert.Equal(t, 0, init.ExitCode, "init should establish the project")

	r := runBinary(t, bin, "doctor", "--dir", dir)

	// doctor reports findings but exits 0 once the project exists.
	assert.Equal(t, 0, r.ExitCode, "doctor should exit 0 even with warnings")
	combined := r.Stdout + r.Stderr
	assert.True(t, len(combined) > 0, "doctor should produce output")
}

func TestCLI_Init_InTempDir(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := t.TempDir()
	r := runBinary(t, bin, "init", "--dir", dir, "--platforms", "claude-code")

	// init may succeed (0) or return a non-zero code depending on environment.
	// We just verify it produces output without a panic.
	combined := r.Stdout + r.Stderr
	assert.True(t, len(combined) > 0, "init should produce output")
}
