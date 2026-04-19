// Package cli_test covers doctor quality gate tests.
package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestDoctorCmd_ShowsQualityGateSection(t *testing.T) {
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
	assert.Contains(t, output, "Quality Gate")
}

func TestDoctorCmd_QualityGateShowsMethodology(t *testing.T) {
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
	assert.Contains(t, output, "methodology")
}

func TestDoctorCmd_QualityGate_NoPreset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := config.DefaultFullConfig("test-proj")
	cfg.Quality.Default = ""
	require.NoError(t, config.Save(dir, cfg))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "quality preset: not configured")
}

func TestDoctorCmd_QualityGate_ReviewGateDisabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := config.DefaultFullConfig("test-proj")
	cfg.Spec.ReviewGate.Enabled = false
	require.NoError(t, config.Save(dir, cfg))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "review gate: disabled")
}

func TestDoctorCmd_QualityGate_ValidPreset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := config.DefaultFullConfig("test-proj")
	require.NoError(t, config.Save(dir, cfg))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "quality preset: balanced")
}

func TestDoctorCmd_QualityGate_ReviewGateEnabled_WithProviders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := config.DefaultFullConfig("test-proj")
	cfg.Spec.ReviewGate.Enabled = true
	cfg.Spec.ReviewGate.Providers = []string{}
	require.NoError(t, config.Save(dir, cfg))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "review gate: enabled")
	assert.Contains(t, output, "fewer than 2 providers available")
}

func TestDoctorCmd_QualityGate_ReviewGate_InstalledProviders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := config.DefaultFullConfig("test-proj")
	cfg.Spec.ReviewGate.Enabled = true
	cfg.Spec.ReviewGate.Providers = []string{"claude", "gemini"}
	require.NoError(t, config.Save(dir, cfg))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "review gate: enabled")
}
