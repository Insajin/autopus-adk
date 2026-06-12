package setup

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// antigravityInstallCommand returns OS-appropriate installer commands.
func TestAntigravityInstallCommand_LinuxOrMac(t *testing.T) {
	t.Parallel()

	cmd := antigravityInstallCommand()
	if runtime.GOOS == "windows" {
		assert.Contains(t, cmd, "install.ps1")
	} else {
		assert.Contains(t, cmd, "install.sh")
		assert.Contains(t, cmd, "curl")
		assert.Contains(t, cmd, "bash")
	}
}

// shellCommand wraps the given command with the appropriate shell.
func TestShellCommand_NonWindows(t *testing.T) {
	t.Parallel()

	shell, args := shellCommand("echo hello")
	if runtime.GOOS == "windows" {
		assert.Equal(t, "powershell", shell)
		assert.Equal(t, "-Command", args[len(args)-2])
		assert.Equal(t, "echo hello", args[len(args)-1])
	} else {
		assert.Equal(t, "sh", shell)
		assert.Equal(t, []string{"-c", "echo hello"}, args)
	}
}

// InstallProvider returns error for unknown provider names.
func TestInstallProvider_UnknownProviderError(t *testing.T) {
	t.Parallel()

	err := InstallProvider("unknown-provider")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

// detectVersion falls back to "unknown" when the binary returns an error.
func TestDetectVersion_MissingBinaryReturnsUnknown(t *testing.T) {
	t.Parallel()

	v := detectVersion("/nonexistent/binary/path")
	assert.Equal(t, "unknown", v)
}
