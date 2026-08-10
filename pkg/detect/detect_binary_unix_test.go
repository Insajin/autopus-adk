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

// probeFixtureLifetime is how long a probe fixture stays alive — the hung child
// below, and the backgrounded grandchild that holds the inherited stdout pipe.
// It doubles as the ceiling the elapsed assertions divide by. Every bound under
// test returns far sooner, so the assertions keep a wide margin instead of
// measuring how busy the machine is: the healthy paths finish in a few hundred
// milliseconds, while every regression — no context deadline, no pipe-drain
// bound — runs for the full lifetime. The contract is that those bounds are
// alive, not that the probe is quick.
const probeFixtureLifetime = 30 * time.Second

func TestDetectBinaryHungChildReturnsWithinBound(t *testing.T) {
	script := writeVersionProbeScript(t, "#!/bin/sh\n/bin/sleep 30\n")
	started := time.Now()

	version, installed := detectBinaryWithLimits(script, "--version", 100*time.Millisecond, 100*time.Millisecond)

	assert.True(t, installed)
	assert.Equal(t, "unknown", version)
	assert.Less(t, time.Since(started), probeFixtureLifetime/3,
		"version probe must return on its context and pipe-drain bounds, not when the child exits")
}

func TestDetectBinaryGrandchildPipeReturnsAndCleansProcessGroup(t *testing.T) {
	dir := t.TempDir()
	heartbeat := filepath.Join(dir, "heartbeat")
	t.Setenv("AUTOPUS_TEST_PROBE_HEARTBEAT", heartbeat)
	t.Setenv("PATH", dir)
	// The grandchild ticks for probeFixtureLifetime while holding stdout. The
	// production ceiling stays untouched here: without the pipe-drain bound the
	// probe blocks on the inherited pipe for the full lifetime regardless.
	writeVersionProbeScriptAt(t, filepath.Join(dir, "opencode"), `#!/bin/sh
(
  count=0
  while [ "$count" -lt 300 ]; do
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

	// The bounds come first: a fatal require on the degraded value would mask
	// which bound actually broke.
	assert.Less(t, elapsed, probeFixtureLifetime/3,
		"version probe must not wait for a grandchild that inherited stdout")
	assert.Equal(t, sizeAfterReturn, fileSize(t, heartbeat),
		"version probe must terminate its orphan-prone process group")
	require.Equal(t, []Platform{{Name: "opencode", Binary: "opencode", Version: "unknown"}}, platforms)
}

// TestDetectInstalledPlatformsExecutesOnlyTheAmbiguousIdentityProbe pins the
// narrowed contract. Presence alone still decides every platform whose binary
// name is unambiguous, and their binaries are never executed. `omp` is the
// exception: `auto init` and `auto update` activate platforms from this list, so
// an unrelated binary named omp must be rejected here rather than adopted and
// pointed at a `.omp/` directory it does not own.
func TestDetectInstalledPlatformsExecutesOnlyTheAmbiguousIdentityProbe(t *testing.T) {
	dir := t.TempDir()
	markers := make(map[string]string, len(knownCLIs))
	for _, cli := range knownCLIs {
		marker := filepath.Join(dir, cli.binary+".executed")
		markers[cli.name] = marker
		writeVersionProbeScriptAt(t, filepath.Join(dir, cli.binary),
			"#!/bin/sh\nprintf executed > \""+marker+"\"\n")
	}
	t.Setenv("PATH", dir)

	platforms := DetectInstalledPlatforms()

	names := make([]string, 0, len(platforms))
	for _, p := range platforms {
		names = append(names, p.Name)
	}
	assert.NotContains(t, names, "omp",
		"a binary named omp that prints no oh-my-pi version must not be adopted")

	expected := make([]Platform, 0, len(knownCLIs))
	for _, cli := range knownCLIs {
		if cli.name == "omp" {
			assert.FileExists(t, markers[cli.name],
				"omp is identity-probed, so its version command does run")
			continue
		}
		assert.NoFileExists(t, markers[cli.name],
			"presence detection must not execute %s", cli.binary)
		expected = append(expected, Platform{Name: cli.name, Binary: cli.binary})
	}
	require.Equal(t, expected, platforms, "remaining platforms keep knownCLIs order")
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
