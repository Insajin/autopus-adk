package cli_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

// setupConfigDir creates a temp directory with a minimal valid autopus.yaml.
func setupConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("test-project")
	require.NoError(t, config.Save(dir, cfg))
	return dir
}

// chdirTo changes to dir and returns a cleanup that restores the original cwd.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestConfigSet_HintsPlatform(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := setupConfigDir(t)
	chdirTo(t, dir)

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"config", "set", "hints.platform", "false"})
	cmd.SetOut(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	cfg, err := loadConfigFromDir(dir)
	require.NoError(t, err)
	assert.False(t, cfg.Hints.IsPlatformHintEnabled())
}

func TestConfigSet_UsageProfile(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := setupConfigDir(t)
	chdirTo(t, dir)

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"config", "set", "usage_profile", "fullstack"})
	cmd.SetOut(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	cfg, err := loadConfigFromDir(dir)
	require.NoError(t, err)
	assert.Equal(t, config.UsageProfile("fullstack"), cfg.UsageProfile)
}

func TestConfigSet_UnknownKey(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := setupConfigDir(t)
	chdirTo(t, dir)

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"config", "set", "nonexistent.key", "value"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown config key")
}

func TestConfigGet_HintsPlatform(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := setupConfigDir(t)
	chdirTo(t, dir)

	cmd := newTestRootCmd()
	var buf bytes.Buffer
	cmd.SetArgs([]string{"config", "get", "hints.platform"})
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "true")
}

func TestConfigGet_UsageProfile(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := setupConfigDir(t)

	// Write config with explicit fullstack profile.
	cfg, err := config.Load(dir)
	require.NoError(t, err)
	cfg.UsageProfile = config.UsageProfile("fullstack")
	require.NoError(t, config.Save(dir, cfg))

	chdirTo(t, dir)

	cmd := newTestRootCmd()
	var buf bytes.Buffer
	cmd.SetArgs([]string{"config", "get", "usage_profile"})
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "fullstack")
}

func TestConfigSet_UsageProfile_Invalid(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := setupConfigDir(t)
	chdirTo(t, dir)

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"config", "set", "usage_profile", "invalid"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage_profile must be")
}

func TestConfigSet_HintsPlatform_InvalidBool(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := setupConfigDir(t)
	chdirTo(t, dir)

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"config", "set", "hints.platform", "notabool"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hints.platform must be true or false")
}
