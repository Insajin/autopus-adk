package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// A pack command that writes into HOME must not pollute the project tree. Go
// test runs did exactly that: HOME pointed at the project directory, so the
// toolchain left `Library/Application Support/go/telemetry` at the repo root.
func TestRunCommand_SandboxHomeWritesStayInsideCacheRoot(t *testing.T) {
	// The runner resolves the project dir through EvalSymlinks (/var -> /private/var
	// on macOS), so the expected HOME has to be resolved the same way.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	script := filepath.Join(dir, "write-home")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nmkdir -p \"$HOME/Library/state\"\nprintf tool-state > \"$HOME/Library/state/marker\"\nprintf 'home is %s\\n' \"$HOME\"\n"), 0o755))

	result := runCommand(dir, journey.Pack{
		ID:      "home-writer",
		Adapter: journey.AdapterRef{ID: "custom-command"},
		Command: journey.Command{Argv: []string{script}, CWD: "."},
	}, filepath.Join(dir, "artifacts"))

	require.Equal(t, "passed", result.Status, result.FailureSummary)
	assert.NoDirExists(t, filepath.Join(dir, "Library"), "sandbox HOME must not be the project root")
	sandbox := filepath.Join(dir, ".autopus", "qa", "cache", sandboxHomeDirName)
	assert.FileExists(t, filepath.Join(sandbox, "Library", "state", "marker"))
	assert.Contains(t, result.StdoutText, "home is "+sandbox)
}

// Allowlisting HOME is the documented escape hatch, and Playwright depends on
// browser caches under the real home, so the real value must still win.
func TestAllowedEnv_HomeAllowlistStillYieldsRealHome(t *testing.T) {
	cache, err := prepareCommandGoCache(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(cache.Cleanup)
	t.Setenv("HOME", "/tmp/qamesh-real-home")

	env := allowedEnv(cache.Paths, []string{"HOME"})

	assert.Contains(t, env, "HOME=/tmp/qamesh-real-home")
	playwright := envValue(t, env, "PLAYWRIGHT_BROWSERS_PATH")
	assert.NotEmpty(t, playwright)
	assert.False(t, strings.HasPrefix(playwright, cache.Paths.ProjectDir),
		"browser cache must resolve against the real user home, not the sandbox")
}

func envValue(t *testing.T, env []string, name string) string {
	t.Helper()
	for _, entry := range env {
		if strings.HasPrefix(entry, name+"=") {
			return strings.TrimPrefix(entry, name+"=")
		}
	}
	return ""
}
