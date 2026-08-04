package detect

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opencodeProbeScript is the control platform placed next to the fake omp
// binary: the identity gate must never change what the other CLIs report.
const opencodeProbeScript = `#!/bin/sh
printf 'opencode 1.2.3\n'
`

var opencodeProbePlatform = Platform{Name: "opencode", Binary: "opencode", Version: "opencode 1.2.3"}

// TestDetectPlatformsOMPRequiresOhMyPiVersionShape pins SPEC-OMP-001 REQ-019 (S15).
// `omp` is a short binary name that collides with unrelated executables, so
// DetectPlatforms accepts it only when the version probe reports the exact
// release shape (`omp/x.y.z`). Every case runs against a PATH holding both the fake
// omp binary and an unrelated CLI to prove the gate is scoped to omp alone.
func TestDetectPlatformsOMPRequiresOhMyPiVersionShape(t *testing.T) {
	skipWithoutPOSIXShell(t)

	tests := []struct {
		name         string
		script       string
		wantAccepted bool
		wantVersion  string
	}{
		{
			name: "oh-my-pi version output is accepted",
			script: `#!/bin/sh
printf 'omp/17.1.8\n'
`,
			wantAccepted: true,
			wantVersion:  "omp/17.1.8",
		},
		{
			name: "oh-my-pi prerelease is not an exact release",
			script: `#!/bin/sh
printf 'omp/18.0.0-rc.1\n'
`,
			wantAccepted: false,
		},
		{
			name: "unrelated binary that merely starts with the name is rejected",
			script: `#!/bin/sh
printf 'omp 1.4.2\n'
`,
			wantAccepted: false,
		},
		{
			name: "unrelated openmp helper is rejected",
			script: `#!/bin/sh
printf 'OpenMP runtime helper 5.2\n'
`,
			wantAccepted: false,
		},
		{
			name: "different tool whose name shares the omp prefix is rejected",
			script: `#!/bin/sh
printf 'omptool/3.0.0\n'
`,
			wantAccepted: false,
		},
		{
			name: "unreadable version output is rejected instead of degrading to unknown",
			script: `#!/bin/sh
exit 3
`,
			wantAccepted: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeOMPProbeScript(t, filepath.Join(dir, "omp"), tc.script)
			writeOMPProbeScript(t, filepath.Join(dir, "opencode"), opencodeProbeScript)
			t.Setenv("PATH", dir)

			platforms := DetectPlatforms()

			// knownCLIs order places opencode before omp, so the expected slice
			// is exact: the control platform plus omp only when it is accepted.
			want := []Platform{opencodeProbePlatform}
			if tc.wantAccepted {
				want = append(want, Platform{Name: "omp", Binary: "omp", Version: tc.wantVersion})
			}
			require.Equal(t, want, platforms)

			detected, found := platformNamed(platforms, "omp")
			require.Equal(t, tc.wantAccepted, found,
				"omp membership must follow the oh-my-pi version shape")
			if tc.wantAccepted {
				assert.Equal(t, tc.wantVersion, detected.Version)
			}
		})
	}
}

// TestDetectInstalledPlatformsAppliesOMPIdentityGate pins REQ-019 on BOTH
// detection paths. The gate used to live only in DetectPlatforms, which left a
// bypass: `auto init` and `auto update` read the presence-only list, so an
// impostor binary named omp auto-activated the platform and the adapter then
// wrote into a foreign `.omp/`. Both paths must now reject it.
func TestDetectInstalledPlatformsAppliesOMPIdentityGate(t *testing.T) {
	skipWithoutPOSIXShell(t)

	dir := t.TempDir()
	marker := filepath.Join(dir, "probe-executed")
	writeOMPProbeScript(t, filepath.Join(dir, "omp"), `#!/bin/sh
printf executed > "$AUTOPUS_TEST_OMP_PROBE_MARKER"
printf 'omp 1.4.2\n'
`)
	t.Setenv("AUTOPUS_TEST_OMP_PROBE_MARKER", marker)
	t.Setenv("PATH", dir)

	assert.Empty(t, DetectInstalledPlatforms(),
		"an impostor must not reach the platform list `auto init` activates from")
	assert.FileExists(t, marker,
		"reaching the gate requires running the version probe")

	assert.Empty(t, DetectPlatforms(),
		"the version-probing path rejects the impostor as before")
}

// TestDetectInstalledPlatformsAcceptsGenuineOMP is the positive half: a real
// oh-my-pi version string still activates the platform on the init/update path.
func TestDetectInstalledPlatformsAcceptsGenuineOMP(t *testing.T) {
	skipWithoutPOSIXShell(t)

	dir := t.TempDir()
	writeOMPProbeScript(t, filepath.Join(dir, "omp"), "#!/bin/sh\nprintf 'omp/17.1.8\\n'\n")
	t.Setenv("PATH", dir)

	assert.Equal(t, []Platform{{Name: "omp", Binary: "omp"}}, DetectInstalledPlatforms(),
		"a genuine oh-my-pi binary still activates the platform")
}

// TestAgentRuntimeFromProcessArgsRecognizesOMP covers both omp recognition
// branches in runtime.go and keeps matching exact rather than substring-based.
func TestAgentRuntimeFromProcessArgsRecognizesOMP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args string
		want AgentRuntime
	}{
		{
			name: "omp native binary ancestor",
			args: "/opt/homebrew/bin/omp --mode rpc --no-session",
			want: AgentRuntimeOMP,
		},
		{
			name: "omp node entrypoint ancestor",
			args: "/opt/homebrew/bin/node /opt/homebrew/lib/node_modules/@oh-my-pi/cli/bin/omp.js --mode rpc",
			want: AgentRuntimeOMP,
		},
		{
			name: "unrelated binary whose name only starts with omp",
			args: "/usr/local/bin/ompd --daemon",
			want: AgentRuntimeUnknown,
		},
		{
			name: "node entrypoint outside the oh-my-pi package",
			args: "/opt/homebrew/bin/node /srv/tools/omp.js",
			want: AgentRuntimeUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, agentRuntimeFromProcessArgs(tc.args))
		})
	}
}

func platformNamed(platforms []Platform, name string) (Platform, bool) {
	for _, platform := range platforms {
		if platform.Name == name {
			return platform, true
		}
	}
	return Platform{}, false
}

func writeOMPProbeScript(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
}

func skipWithoutPOSIXShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixtures rely on POSIX shell scripts")
	}
}
