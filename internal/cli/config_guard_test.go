// Package cli_test covers the missing-workspace-config guard.
package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCLI executes one root command invocation with separated streams, so a JSON
// envelope on stdout stays parseable even when cobra also renders the error.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

// TestCommandsRequireExistingConfig pins the guard against operating on a
// synthesized configuration. config.Load returns DefaultFullConfig when
// autopus.yaml is absent, so doctor, update, and platform used to fail open:
// they reported on, and generated surfaces for, a project the user never
// configured, in whatever directory the command happened to be pointed at.
// The workspace-shaped commands now stop with an actionable error, and `auto
// init` remains the one command that may run without a config.
func TestCommandsRequireExistingConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"doctor", []string{"doctor", "--dir", dir}},
		{"update", []string{"update", "--dir", dir}},
		{"platform list", []string{"platform", "list", "--dir", dir}},
		{"platform add", []string{"platform", "add", "omp", "--dir", dir}},
		{"platform remove", []string{"platform", "remove", "omp", "--dir", dir}},
	} {
		stdout, stderr, err := runCLI(t, tc.args...)
		require.Errorf(t, err, "%s must refuse a directory that holds no autopus.yaml\n%s%s",
			tc.name, stdout, stderr)
		assert.Containsf(t, err.Error(), "autopus.yaml",
			"%s must name the missing file", tc.name)
		assert.Containsf(t, err.Error(), "auto init",
			"%s must name the command that recovers", tc.name)
		assert.NotContainsf(t, stdout, "autopus.yaml (mode:",
			"%s must not report on a synthesized config", tc.name)
	}

	// The JSON projection has to fail the same way instead of emitting a
	// healthy envelope full of checks about a project that does not exist.
	stdout, _, err := runCLI(t, "doctor", "--dir", dir, "--json")
	require.Error(t, err)
	var envelope struct {
		Status string `json:"status"`
		Error  *struct {
			Code string `json:"code"`
		} `json:"error"`
		Checks []struct {
			Status string `json:"status"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope), stdout)
	assert.Equal(t, "error", envelope.Status)
	require.NotNil(t, envelope.Error, "the envelope must carry a machine-readable cause")
	assert.Equal(t, "project_missing", envelope.Error.Code)
	assert.Empty(t, envelope.Checks,
		"no check may report a verdict on a project that does not exist")

	// The guard is not a blanket refusal: init still runs here, and every
	// guarded command works again once a real config exists.
	initOut, initErrOut, initErr := runCLI(t,
		"init", "--dir", dir, "--project", "guard-proj", "--platforms", "omp")
	require.NoError(t, initErr, initOut+initErrOut)
	require.FileExists(t, filepath.Join(dir, "autopus.yaml"))

	for _, args := range [][]string{
		{"update", "--dir", dir},
		{"platform", "add", "codex", "--dir", dir},
		{"doctor", "--dir", dir},
	} {
		stdout, stderr, err := runCLI(t, args...)
		require.NoErrorf(t, err, "%v must succeed once autopus.yaml exists\n%s%s",
			args, stdout, stderr)
	}
}
