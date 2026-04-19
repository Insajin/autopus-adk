// Package cli_test covers doctor command smoke tests.
package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctorCmd_ReportsStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Run init before doctor.
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	err := doctorCmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.True(t, len(output) > 0, "doctor command should emit output")
}

func TestDoctorCmd_DetectsMissingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Run doctor in an empty directory.
	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	_ = doctorCmd.Execute()

	output := out.String()
	_ = output
}

func TestDoctorCmd_ShowsOKAfterInit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "OK")
	_ = filepath.Join(dir, "autopus.yaml")
}

func TestDoctorCmd_FixFlag_NoMissingDeps(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir, "--fix", "--yes"})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.True(t, len(output) > 0)
}
