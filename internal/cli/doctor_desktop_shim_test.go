package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const desktopShimFixture = `#!/bin/sh
# Autopus Desktop managed auto launcher v1
# sha256:0000
exec '/Applications/Autopus Desktop.app/Contents/MacOS/autopus-desktop' --autopus-managed-adk-cli-broker "$@"
`

// TestDiagnoseDesktopShim_LauncherOnPathIsDetected pins the core detection:
// the launcher installed over the PATH entry is reported together with the
// managed slot it brokers into.
func TestDiagnoseDesktopShim_LauncherOnPathIsDetected(t *testing.T) {
	requireDarwinDesktopShim(t)
	binDir := installFakeAutoOnPath(t, desktopShimFixture)
	home := stubDesktopShimHome(t, true)

	diagnosis := diagnoseDesktopShim()

	assert.True(t, diagnosis.Found)
	assert.Equal(t, filepath.Join(binDir, "auto"), diagnosis.ShimPath)
	assert.Equal(t, filepath.Join(home, filepath.FromSlash(desktopManagedSlotRelPath)), diagnosis.ManagedPath)
	assert.True(t, diagnosis.ManagedExists)
}

// TestDiagnoseDesktopShim_MissingManagedSlotIsReported covers the worst case:
// the launcher shadows PATH while the managed binary it delegates to is gone.
func TestDiagnoseDesktopShim_MissingManagedSlotIsReported(t *testing.T) {
	requireDarwinDesktopShim(t)
	installFakeAutoOnPath(t, desktopShimFixture)
	stubDesktopShimHome(t, false)

	diagnosis := diagnoseDesktopShim()

	require.True(t, diagnosis.Found)
	assert.False(t, diagnosis.ManagedExists)
	assert.Contains(t, desktopShimManagedLine(diagnosis), "missing")
}

// TestDiagnoseDesktopShim_RealBinaryIsNotALauncher guards the false positive
// that would nag every non-Desktop install: a genuine executable, and a
// symlink to it, must not look like the launcher.
func TestDiagnoseDesktopShim_RealBinaryIsNotALauncher(t *testing.T) {
	binDir := t.TempDir()
	realPath := filepath.Join(binDir, "auto-real")
	require.NoError(t, os.WriteFile(realPath, []byte("\x7fELF not a script"), 0o755))
	require.NoError(t, os.Symlink(realPath, filepath.Join(binDir, "auto")))
	t.Setenv("PATH", binDir)

	diagnosis := diagnoseDesktopShim()

	assert.False(t, diagnosis.Found)
	assert.Equal(t, filepath.Join(binDir, "auto"), diagnosis.ShimPath)
}

// TestDiagnoseDesktopShim_UnmarkedShellScriptIsNotALauncher keeps the marker
// requirement honest: any wrapper script must carry a Desktop marker.
func TestDiagnoseDesktopShim_UnmarkedShellScriptIsNotALauncher(t *testing.T) {
	installFakeAutoOnPath(t, "#!/bin/sh\nexec /usr/local/bin/auto-real \"$@\"\n")

	assert.False(t, diagnoseDesktopShim().Found)
}

func TestDiagnoseDesktopShim_AutoMissingFromPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	diagnosis := diagnoseDesktopShim()

	assert.False(t, diagnosis.Found)
	assert.Empty(t, diagnosis.ShimPath)
}

func TestCheckDesktopShimText_LauncherWarnsWithRecovery(t *testing.T) {
	diagnosis := desktopShimDiagnosis{
		Found:         true,
		ShimPath:      "/Users/example/.local/bin/auto",
		ManagedPath:   "/Users/example/Library/Application Support/co.autopus.desktop/managed-adk/current/auto",
		ManagedExists: true,
	}
	var out bytes.Buffer

	checkDesktopShimText(&out, diagnosis)

	text := out.String()
	assert.Contains(t, text, "WARN")
	assert.Contains(t, text, diagnosis.ShimPath)
	assert.Contains(t, text, diagnosis.ManagedPath)
	assert.Contains(t, text, "exists")
	assert.Contains(t, text, "alias auto=")
	assert.Contains(t, text, desktopShimBrokerRejection)
}

// TestCheckDesktopShimText_RunningManagedSlotIsNamed proves the current-binary
// input reaches the report, so a user already on the managed binary is told the
// launcher only affects future PATH invocations.
func TestCheckDesktopShimText_RunningManagedSlotIsNamed(t *testing.T) {
	managed := filepath.Join(t.TempDir(), "managed-adk", "current", "auto")
	var out bytes.Buffer

	checkDesktopShimText(&out, desktopShimDiagnosis{
		Found:         true,
		ShimPath:      "/Users/example/.local/bin/auto",
		ManagedPath:   managed,
		ManagedExists: true,
		SelfPath:      managed,
	})

	assert.Contains(t, out.String(), "currently running")
}

func TestCheckDesktopShimText_NoLauncherIsQuiet(t *testing.T) {
	var out bytes.Buffer

	checkDesktopShimText(&out, desktopShimDiagnosis{ShimPath: "/usr/local/bin/auto"})

	text := out.String()
	assert.Contains(t, text, "PATH auto is not a Desktop launcher")
	assert.NotContains(t, text, "WARN")
	assert.NotContains(t, text, desktopShimBrokerRejection)
}

func TestCollectDesktopShimCheck_LauncherWarnsEnvelope(t *testing.T) {
	report := doctorJSONReport{status: jsonStatusOK}

	report.collectDesktopShimCheck(desktopShimDiagnosis{
		Found:       true,
		ShimPath:    "/Users/example/.local/bin/auto",
		ManagedPath: "/Users/example/Library/Application Support/co.autopus.desktop/managed-adk/current/auto",
	})

	assert.Equal(t, jsonStatusWarn, report.status)
	require.Len(t, report.checks, 1)
	assert.Equal(t, doctorDesktopShimCheckID, report.checks[0].ID)
	assert.Equal(t, "warning", report.checks[0].Severity)
	assert.Equal(t, "warn", report.checks[0].Status)
	assert.Contains(t, report.checks[0].Detail, "missing")
	require.Len(t, report.warnings, 1)
	assert.Equal(t, "desktop_managed_launcher", report.warnings[0].Code)
}

func TestCollectDesktopShimCheck_StatusesWithoutLauncher(t *testing.T) {
	cases := map[string]struct {
		diagnosis desktopShimDiagnosis
		status    string
	}{
		"auto missing from PATH": {diagnosis: desktopShimDiagnosis{}, status: "skip"},
		"real CLI on PATH": {
			diagnosis: desktopShimDiagnosis{ShimPath: "/usr/local/bin/auto"},
			status:    "pass",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			report := doctorJSONReport{status: jsonStatusOK}

			report.collectDesktopShimCheck(tc.diagnosis)

			assert.Equal(t, jsonStatusOK, report.status)
			require.Len(t, report.checks, 1)
			assert.Equal(t, doctorDesktopShimCheckID, report.checks[0].ID)
			assert.Equal(t, tc.status, report.checks[0].Status)
			assert.Empty(t, report.warnings)
		})
	}
}

// TestDesktopShimProjectionsAgreeOnOneDiagnosis keeps `auto doctor` and
// `auto doctor --json` from drifting: both surfaces must name the launcher,
// the managed slot and the broker rejection code from the same diagnosis.
func TestDesktopShimProjectionsAgreeOnOneDiagnosis(t *testing.T) {
	diagnosis := desktopShimDiagnosis{
		Found:         true,
		ShimPath:      "/Users/example/.local/bin/auto",
		ManagedPath:   "/Users/example/Library/Application Support/co.autopus.desktop/managed-adk/current/auto",
		ManagedExists: true,
	}
	var out bytes.Buffer
	report := doctorJSONReport{status: jsonStatusOK}

	checkDesktopShimText(&out, diagnosis)
	report.collectDesktopShimCheck(diagnosis)

	require.Len(t, report.checks, 1)
	text := stripANSI(out.String())
	for _, fragment := range []string{diagnosis.ShimPath, diagnosis.ManagedPath, desktopShimBrokerRejection} {
		assert.Contains(t, text, fragment)
		assert.Contains(t, report.checks[0].Detail, fragment)
	}
}

func requireDarwinDesktopShim(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("the managed slot path is only defined on macOS")
	}
}

// installFakeAutoOnPath writes an `auto` script into an isolated PATH. The
// script is never executed; the check only reads its head.
func installFakeAutoOnPath(t *testing.T, content string) string {
	t.Helper()
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "auto"), []byte(content), 0o755))
	t.Setenv("PATH", binDir)
	return binDir
}

// stubDesktopShimHome points the managed slot lookup at a temp home, optionally
// materializing the managed binary.
func stubDesktopShimHome(t *testing.T, withManagedBinary bool) string {
	t.Helper()
	home := t.TempDir()
	original := desktopShimUserHomeDir
	t.Cleanup(func() { desktopShimUserHomeDir = original })
	desktopShimUserHomeDir = func() (string, error) { return home, nil }

	if withManagedBinary {
		managed := filepath.Join(home, filepath.FromSlash(desktopManagedSlotRelPath))
		require.NoError(t, os.MkdirAll(filepath.Dir(managed), 0o755))
		require.NoError(t, os.WriteFile(managed, []byte("managed"), 0o755))
	}
	return home
}

func stripANSI(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == 0x1b {
			for i < len(value) && value[i] != 'm' {
				i++
			}
			continue
		}
		out.WriteByte(value[i])
	}
	return out.String()
}
