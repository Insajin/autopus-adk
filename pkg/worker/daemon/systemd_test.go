package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateUnit_ContainsExpectedDirectives(t *testing.T) {
	t.Parallel()

	cfg := LaunchdConfig{
		BinaryPath: "/usr/local/bin/autopus",
		Args:       []string{"worker", "start"},
	}

	content, err := GenerateUnit(cfg)
	require.NoError(t, err)

	assert.Contains(t, content, "[Unit]")
	assert.Contains(t, content, "Description=Autopus Worker Daemon")
	assert.Contains(t, content, "After=network-online.target")
	assert.Contains(t, content, "[Service]")
	assert.Contains(t, content, "ExecStart=/usr/local/bin/autopus worker start")
	assert.Contains(t, content, "Restart=always")
	assert.Contains(t, content, "RestartSec=5")
	assert.Contains(t, content, "Type=simple")
	assert.Contains(t, content, "[Install]")
	assert.Contains(t, content, "WantedBy=default.target")
}

func TestGenerateUnit_NoArgs(t *testing.T) {
	t.Parallel()

	cfg := LaunchdConfig{
		BinaryPath: "/usr/local/bin/autopus",
	}

	content, err := GenerateUnit(cfg)
	require.NoError(t, err)

	assert.Contains(t, content, "ExecStart=/usr/local/bin/autopus")
	assert.NotContains(t, content, "ExecStart=/usr/local/bin/autopus ")
}

func TestGenerateUnit_MultipleArgs(t *testing.T) {
	t.Parallel()

	cfg := LaunchdConfig{
		BinaryPath: "/opt/autopus/bin/autopus",
		Args:       []string{"worker", "start", "--verbose", "--port", "8080"},
	}

	content, err := GenerateUnit(cfg)
	require.NoError(t, err)

	expected := "ExecStart=/opt/autopus/bin/autopus worker start --verbose --port 8080"
	assert.Contains(t, content, expected)
}

func TestSystemdUnitPath_Format(t *testing.T) {
	t.Parallel()

	path := systemdUnitPath()
	assert.Contains(t, path, "autopus-worker.service")
	assert.Contains(t, path, "systemd")
	assert.Contains(t, path, "user")
}

func TestInstallAndUninstallSystemd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	systemctlPath := filepath.Join(binDir, "systemctl")
	require.NoError(t, os.WriteFile(systemctlPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	cfg := LaunchdConfig{
		BinaryPath: "/usr/local/bin/autopus",
		Args:       []string{"worker", "start"},
		LogDir:     filepath.Join(home, "logs"),
	}

	require.NoError(t, InstallSystemd(cfg))
	assert.True(t, IsSystemdInstalled())

	unitPath := systemdUnitPath()
	require.FileExists(t, unitPath)
	content, err := os.ReadFile(unitPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ExecStart=/usr/local/bin/autopus worker start")

	require.NoError(t, UninstallSystemd())
	assert.False(t, IsSystemdInstalled())
}
