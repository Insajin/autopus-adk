//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package detect

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectBinaryFastVersion(t *testing.T) {
	script := writeVersionProbeScript(t, "#!/bin/sh\nprintf 'opencode 1.2.3\\n'\n")

	version, installed := detectBinary(script, "--version")

	assert.True(t, installed)
	assert.Equal(t, "opencode 1.2.3", version)
}

func TestDetectBinaryFailureDegradesToUnknown(t *testing.T) {
	script := writeVersionProbeScript(t, "#!/bin/sh\nexit 17\n")

	version, installed := detectBinary(script, "--version")

	assert.True(t, installed)
	assert.Equal(t, "unknown", version)
}

func TestDetectBinaryHungChildReturnsWithinBound(t *testing.T) {
	script := writeVersionProbeScript(t, "#!/bin/sh\n/bin/sleep 5\n")
	started := time.Now()

	version, installed := detectBinaryWithLimits(script, "--version", 100*time.Millisecond, 100*time.Millisecond)

	assert.True(t, installed)
	assert.Equal(t, "unknown", version)
	assert.Less(t, time.Since(started), 2*time.Second,
		"version probe must return after its context and pipe-drain bounds")
}

func TestDetectBinaryGrandchildPipeReturnsAndCleansProcessGroup(t *testing.T) {
	dir := t.TempDir()
	heartbeat := filepath.Join(dir, "heartbeat")
	t.Setenv("AUTOPUS_TEST_PROBE_HEARTBEAT", heartbeat)
	t.Setenv("PATH", dir)
	writeVersionProbeScriptAt(t, filepath.Join(dir, "opencode"), `#!/bin/sh
(
  count=0
  while [ "$count" -lt 50 ]; do
    printf x >> "$AUTOPUS_TEST_PROBE_HEARTBEAT"
    /bin/sleep 0.1
    count=$((count + 1))
  done
) &
exit 0
`)
	started := time.Now()

	platforms := DetectPlatforms()
	elapsed := time.Since(started)
	sizeAfterReturn := fileSize(t, heartbeat)
	time.Sleep(350 * time.Millisecond)

	require.Equal(t, []Platform{{Name: "opencode", Binary: "opencode", Version: "unknown"}}, platforms)
	assert.Less(t, elapsed, 2*time.Second,
		"version probe must not wait for a grandchild that inherited stdout")
	assert.Equal(t, sizeAfterReturn, fileSize(t, heartbeat),
		"version probe must terminate its orphan-prone process group")
}

func TestDetectInstalledPlatformsUsesPresenceOnlyInStableOrder(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	for _, cli := range knownCLIs {
		writeVersionProbeScriptAt(t, filepath.Join(dir, cli.binary),
			"#!/bin/sh\nprintf executed > \"$AUTOPUS_TEST_EXEC_MARKER\"\n")
	}
	t.Setenv("AUTOPUS_TEST_EXEC_MARKER", marker)
	t.Setenv("PATH", dir)

	platforms := DetectInstalledPlatforms()

	require.Len(t, platforms, len(knownCLIs))
	for i, cli := range knownCLIs {
		assert.Equal(t, Platform{Name: cli.name, Binary: cli.binary}, platforms[i])
	}
	assert.NoFileExists(t, marker, "presence detection must not execute provider binaries")
}

func writeVersionProbeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe")
	writeVersionProbeScriptAt(t, path, content)
	return path
}

func writeVersionProbeScriptAt(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return info.Size()
}
