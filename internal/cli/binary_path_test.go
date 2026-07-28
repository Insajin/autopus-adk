package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/selfupdate"
)

func TestResolveCurrentBinaryPath_ResolvesSymlinkTarget(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "auto-target")
	symlinkPath := filepath.Join(tmpDir, "auto-link")
	require.NoError(t, os.WriteFile(targetPath, []byte("bin"), 0o755))
	require.NoError(t, os.Symlink(targetPath, symlinkPath))

	originalExecutable := currentExecutablePath
	originalEval := evalBinarySymlinks
	t.Cleanup(func() {
		currentExecutablePath = originalExecutable
		evalBinarySymlinks = originalEval
	})

	currentExecutablePath = func() (string, error) { return symlinkPath, nil }
	evalBinarySymlinks = filepath.EvalSymlinks

	info, err := resolveCurrentBinaryPath()
	require.NoError(t, err)
	expectedPath, err := filepath.EvalSymlinks(targetPath)
	require.NoError(t, err)
	assert.Equal(t, symlinkPath, info.ExecutablePath)
	assert.Equal(t, expectedPath, info.ManagedPath())
	assert.True(t, info.IsSymlinked())
}

func TestResolveCurrentBinaryPath_FailsClosedWhenSymlinkEvalFails(t *testing.T) {
	originalExecutable := currentExecutablePath
	originalEval := evalBinarySymlinks
	t.Cleanup(func() {
		currentExecutablePath = originalExecutable
		evalBinarySymlinks = originalEval
	})

	currentExecutablePath = func() (string, error) { return "/tmp/auto", nil }
	evalBinarySymlinks = func(string) (string, error) { return "", assert.AnError }

	_, err := resolveCurrentBinaryPath()
	require.ErrorIs(t, err, assert.AnError)
}

func TestBinaryPathInfo_IsManagerOwnedRecognizesDesktopAuthorities(t *testing.T) {
	tests := []struct {
		name string
		info binaryPathInfo
		want bool
	}{
		{
			name: "immutable volume reference",
			info: binaryPathInfo{ExecutablePath: "/.vol/16777231/99123"},
			want: true,
		},
		{
			name: "desktop managed adk directory",
			info: binaryPathInfo{
				ExecutablePath: "/Users/example/.local/bin/auto",
				ResolvedPath:   "/Users/example/Library/Application Support/Autopus/managed-adk/current/auto",
			},
			want: true,
		},
		{
			name: "macOS Homebrew Cellar binary",
			info: binaryPathInfo{
				ExecutablePath: "/opt/homebrew/bin/auto",
				ResolvedPath:   "/opt/homebrew/Cellar/auto/0.50.90/bin/auto",
			},
			want: true,
		},
		{
			name: "macOS Homebrew opt target",
			info: binaryPathInfo{
				ExecutablePath: "/opt/homebrew/bin/auto",
				ResolvedPath:   "/opt/homebrew/opt/auto/bin/auto",
			},
			want: true,
		},
		{
			name: "macOS Homebrew Caskroom binary",
			info: binaryPathInfo{
				ExecutablePath: "/opt/homebrew/bin/auto",
				ResolvedPath:   "/opt/homebrew/Caskroom/auto/0.50.90/auto",
			},
			want: true,
		},
		{
			name: "Linuxbrew Cellar binary",
			info: binaryPathInfo{
				ExecutablePath: "/home/linuxbrew/.linuxbrew/bin/auto",
				ResolvedPath:   "/home/linuxbrew/.linuxbrew/Cellar/auto/0.50.90/bin/auto",
			},
			want: true,
		},
		{
			name: "Linuxbrew opt target",
			info: binaryPathInfo{
				ExecutablePath: "/home/example/.linuxbrew/bin/auto",
				ResolvedPath:   "/home/example/.linuxbrew/opt/auto/bin/auto",
			},
			want: true,
		},
		{
			name: "Homebrew formula alias",
			info: binaryPathInfo{
				ExecutablePath: "/opt/homebrew/bin/auto",
				ResolvedPath:   "/opt/homebrew/Cellar/autopus-adk/0.50.90/bin/auto",
			},
			want: true,
		},
		{
			name: "standalone binary",
			info: binaryPathInfo{
				ExecutablePath: "/usr/local/bin/auto",
				ResolvedPath:   "/usr/local/lib/autopus-adk/0.50.90/auto",
			},
			want: false,
		},
		{
			name: "generic opt directory",
			info: binaryPathInfo{
				ExecutablePath: "/usr/local/bin/auto",
				ResolvedPath:   "/opt/auto/0.50.90/auto",
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.info.IsManagerOwned())
		})
	}
}

func TestInstallSelfUpdateRelease_HomebrewOwnedBinaryStopsBeforeReplacement(t *testing.T) {
	pathInfo := binaryPathInfo{
		ExecutablePath: "/opt/homebrew/bin/auto",
		ResolvedPath:   "/opt/homebrew/Cellar/auto/0.50.90/bin/auto",
	}
	info := &selfupdate.ReleaseInfo{
		TagName:     "v0.50.91",
		ArchiveURL:  "https://example.test/auto.tar.gz",
		ChecksumURL: "https://example.test/checksums.txt",
	}
	replacementCalled := false

	err := installSelfUpdateReleaseWithOperation(
		&cobra.Command{Use: "update"},
		"0.50.90",
		info,
		pathInfo,
		func(*selfupdate.ReleaseInfo, string) error {
			replacementCalled = true
			return nil
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "manager-required")
	assert.False(t, replacementCalled)
}
