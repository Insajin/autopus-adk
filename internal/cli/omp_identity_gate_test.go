package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakeOMPBinary installs an executable named omp on a scratch PATH whose
// --version output the caller controls.
func writeFakeOMPBinary(t *testing.T, versionLine string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell probe script is not portable to windows")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '" + versionLine + "\\n'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "omp"), []byte(script), 0o755))
	t.Setenv("PATH", dir)
}

// TestDetectInstalledPlatforms_RejectsImpostorOMP covers the init/update half of
// REQ-019. `auto init` and `auto update` activate platforms from this list, so
// an unrelated binary named omp must not reach it — otherwise the adapter is
// pointed at a `.omp/` directory owned by something else.
func TestDetectInstalledPlatforms_RejectsImpostorOMP(t *testing.T) {
	writeFakeOMPBinary(t, "omp 1.4.2")

	assert.NotContains(t, detectInstalledPlatforms(), "omp",
		"a binary whose version lacks the oh-my-pi prefix must not activate the platform")
}

// TestDetectInstalledPlatforms_AcceptsGenuineOMP is the positive half: a real
// oh-my-pi binary still auto-activates.
func TestDetectInstalledPlatforms_AcceptsGenuineOMP(t *testing.T) {
	writeFakeOMPBinary(t, "omp/17.1.8")

	assert.Contains(t, detectInstalledPlatforms(), "omp",
		"a genuine oh-my-pi binary still activates the platform")
}

// TestAppendDetectedPlatforms_SkipsImpostorOMP drives the same gate through the
// `auto update` entry point that appends newly detected platforms to the config.
func TestAppendDetectedPlatforms_SkipsImpostorOMP(t *testing.T) {
	writeFakeOMPBinary(t, "definitely-not-oh-my-pi 9.9")

	cfg := ompDispatchConfig("claude-code")
	added := appendDetectedPlatforms(cfg)

	assert.NotContains(t, added, "omp")
	assert.NotContains(t, cfg.Platforms, "omp",
		"auto update must not silently adopt an unverified omp binary")
}
