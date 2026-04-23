package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePlist_ContainsExpectedElements(t *testing.T) {
	t.Parallel()

	cfg := LaunchdConfig{
		BinaryPath: "/usr/local/bin/autopus",
		Args:       []string{"worker", "start"},
		LogDir:     "/tmp/autopus-logs",
	}

	content, err := GeneratePlist(cfg)
	require.NoError(t, err)

	assert.Contains(t, content, "<key>Label</key>")
	assert.Contains(t, content, "<string>co.autopus.worker</string>")
	assert.Contains(t, content, "<key>ProgramArguments</key>")
	assert.Contains(t, content, "<string>/usr/local/bin/autopus</string>")
	assert.Contains(t, content, "<string>worker</string>")
	assert.Contains(t, content, "<string>start</string>")
	assert.Contains(t, content, "<key>KeepAlive</key>")
	assert.Contains(t, content, "<true/>")
	assert.Contains(t, content, "<key>RunAtLoad</key>")
	assert.Contains(t, content, "/tmp/autopus-logs/autopus-worker.out.log")
	assert.Contains(t, content, "/tmp/autopus-logs/autopus-worker.err.log")
}

func TestGeneratePlist_DefaultLogDir(t *testing.T) {
	t.Parallel()

	cfg := LaunchdConfig{
		BinaryPath: "/usr/local/bin/autopus",
	}

	content, err := GeneratePlist(cfg)
	require.NoError(t, err)

	// Should use the default log directory under ~/.config/autopus/logs
	assert.Contains(t, content, "autopus-worker.out.log")
	assert.Contains(t, content, "autopus-worker.err.log")
}

func TestGeneratePlist_NoArgs(t *testing.T) {
	t.Parallel()

	cfg := LaunchdConfig{
		BinaryPath: "/usr/local/bin/autopus",
		LogDir:     "/tmp/logs",
	}

	content, err := GeneratePlist(cfg)
	require.NoError(t, err)

	assert.Contains(t, content, "<string>/usr/local/bin/autopus</string>")
	assert.Contains(t, content, "<?xml version")
	assert.Contains(t, content, "<!DOCTYPE plist")
}

func TestLaunchdPlistPath_Format(t *testing.T) {
	t.Parallel()

	path := launchdPlistPath()
	assert.Contains(t, path, "co.autopus.worker.plist")
	assert.Contains(t, path, "LaunchAgents")
}

func TestInstallAndUninstallLaunchd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	launchctlPath := filepath.Join(binDir, "launchctl")
	require.NoError(t, os.WriteFile(launchctlPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	cfg := LaunchdConfig{
		BinaryPath: "/usr/local/bin/autopus",
		Args:       []string{"worker", "start"},
		LogDir:     filepath.Join(home, "logs"),
	}

	require.NoError(t, InstallLaunchd(cfg))
	assert.True(t, IsLaunchdInstalled())

	plistPath := launchdPlistPath()
	require.FileExists(t, plistPath)
	content, err := os.ReadFile(plistPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "/usr/local/bin/autopus")

	require.NoError(t, UninstallLaunchd())
	assert.False(t, IsLaunchdInstalled())
}
