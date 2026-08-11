package omp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// skipWithoutPOSIXShellOMP guards the PATH fixtures used by the Detect gate.
func skipWithoutPOSIXShellOMP(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixtures rely on POSIX shell scripts")
	}
}

// writeFakeOMPBinary installs an executable named `omp` that prints the given
// version string, and points PATH at it exclusively.
func writeFakeOMPBinary(t *testing.T, versionOutput string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '" + versionOutput + "\\n'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, cliBinary), []byte(script), 0o755))
	t.Setenv("PATH", dir)
}

// TestOMPAdapter_REQ010_IdentityContract pins the identifier values the five
// platform tables and ten CLI dispatch sites key on. A silent rename here makes
// every dispatch site fall through to its unknown-platform branch while the
// adapter itself keeps working, so the constants are asserted by value.
func TestOMPAdapter_REQ010_IdentityContract(t *testing.T) {
	a := NewWithRoot(t.TempDir())

	assert.Equal(t, "omp", a.Name(), "manifest name and dispatch key")
	assert.Equal(t, "omp", a.CLIBinary(), "PATH probe target")
	assert.Equal(t, "1.0.0", a.Version())
	assert.False(t, a.SupportsHooks(),
		"REQ-001: omp has no hook concept, so the lifecycle must advertise none")

	// New() roots the adapter at the working directory; every other constructor
	// path in the CLI relies on that default.
	assert.Equal(t, ".", New().root)
	assert.NotNil(t, New().engine, "the template engine must be wired by the constructor")
}

// TestOMPAdapter_S8_ValidateCleanAfterGenerate is the automated half of the S8
// oracle "auto doctor의 omp 검증 결과는 실패 항목 0건": a workspace omp just
// generated must validate with zero findings.
func TestOMPAdapter_S8_ValidateCleanAfterGenerate(t *testing.T) {
	dir := generateOMPOnly(t)

	errs, err := NewWithRoot(dir).Validate(context.Background())
	require.NoError(t, err)
	assert.Empty(t, errs,
		"a freshly generated omp workspace must report zero validation failures, got %+v", errs)
}

// TestOMPAdapter_Validate_NamesEachMissingSurface proves Validate identifies the
// specific missing surface rather than reporting a generic failure. Each case
// removes exactly one generated surface and expects exactly one finding naming
// it, so a Validate that collapsed to a single "something is wrong" check fails.
func TestOMPAdapter_Validate_NamesEachMissingSurface(t *testing.T) {
	tests := []struct {
		name        string
		remove      string
		wantFile    string
		wantMessage string
	}{
		{
			name:        "config removed",
			remove:      configFile,
			wantFile:    configFile,
			wantMessage: ".omp/config.yml이 없음",
		},
		{
			name:        "agent directory removed",
			remove:      filepath.Join(".omp", "agents"),
			wantFile:    filepath.Join(".omp", "agents"),
			wantMessage: "omp agent 디렉터리가 없음",
		},
		{
			name:        "rule directory removed",
			remove:      filepath.Join(".agents", "rules", "autopus"),
			wantFile:    filepath.Join(".agents", "rules", "autopus"),
			wantMessage: "omp rule 디렉터리가 없음",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := generateOMPOnly(t)
			require.NoError(t, os.RemoveAll(filepath.Join(dir, tc.remove)))

			errs, err := NewWithRoot(dir).Validate(context.Background())
			require.NoError(t, err)
			require.Len(t, errs, 1, "exactly the removed surface must be reported, got %+v", errs)
			assert.Equal(t, adapter.ValidationError{
				File:    tc.wantFile,
				Message: tc.wantMessage,
				Level:   "error",
			}, errs[0])
		})
	}
}

// TestOMPAdapter_Validate_EmptyWorkspaceReportsAllSurfaces covers the
// never-generated workspace: all three required surfaces are reported at error
// level so `auto doctor` fails closed instead of declaring omp healthy.
func TestOMPAdapter_Validate_EmptyWorkspaceReportsAllSurfaces(t *testing.T) {
	errs, err := NewWithRoot(t.TempDir()).Validate(context.Background())
	require.NoError(t, err)
	require.Len(t, errs, 3)

	files := make([]string, 0, len(errs))
	for _, e := range errs {
		files = append(files, e.File)
		assert.Equal(t, "error", e.Level, "a missing surface is never advisory")
		assert.NotEmpty(t, e.Message)
	}
	assert.ElementsMatch(t, []string{
		configFile,
		filepath.Join(".omp", "agents"),
		filepath.Join(".agents", "rules", "autopus"),
	}, files)
}

// widenOMPVersionCeiling raises the Detect deadline for one test and restores it
// afterwards. Identity tests assert on what a version string resolves to, not on
// how fast the probe returns, so the production bound must not preempt them.
func widenOMPVersionCeiling(t *testing.T) {
	t.Helper()
	previous := ompVersionCeiling
	ompVersionCeiling = 60 * time.Second
	t.Cleanup(func() { ompVersionCeiling = previous })
}

// TestOMPAdapter_REQ019_DetectRequiresOhMyPiVersionShape pins the identity gate
// at the adapter boundary. `omp` is a short name that collides with unrelated
// executables, so the adapter adopts only an exact `omp/x.y.z` release shape.
func TestOMPAdapter_REQ019_DetectRequiresOhMyPiVersionShape(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	widenOMPVersionCeiling(t)

	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{name: "oh-my-pi release", version: "omp/17.1.8", want: true},
		{name: "oh-my-pi prerelease", version: "omp/18.0.0-rc.1", want: false},
		{name: "space separated impostor", version: "omp 1.4.2", want: false},
		{name: "unrelated openmp helper", version: "openmp wrapper 4.5", want: false},
		{name: "empty output", version: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writeFakeOMPBinary(t, tc.version)

			got, err := NewWithRoot(t.TempDir()).Detect(context.Background())
			require.NoError(t, err, "a failed probe is reported as not-detected, never as an error")
			assert.Equal(t, tc.want, got,
				"version output %q must resolve to detected=%v", tc.version, tc.want)
		})
	}
}

func TestOMPAdapter_REQ019_DetectUsesCredentialFreeCanonicalEnvironment(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	widenOMPVersionCeiling(t)

	base := t.TempDir()
	home := filepath.Join(base, "home")
	profile := filepath.Join(base, "profile")
	require.NoError(t, os.Mkdir(home, 0o700))
	require.NoError(t, os.Mkdir(profile, 0o700))
	homeLink := filepath.Join(base, "home-link")
	profileLink := filepath.Join(base, "profile-link")
	require.NoError(t, os.Symlink(home, homeLink))
	require.NoError(t, os.Symlink(profile, profileLink))
	canonicalHome, err := filepath.EvalSymlinks(homeLink)
	require.NoError(t, err)
	canonicalProfile, err := filepath.EvalSymlinks(profileLink)
	require.NoError(t, err)

	const sentinel = "OMP-DETECT-PROVIDER-CREDENTIAL-SENTINEL"
	for _, key := range []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY",
		"OMP_ACCESS_TOKEN", "PI_PROVIDER_TOKEN",
	} {
		t.Setenv(key, sentinel)
	}
	t.Setenv("HOME", homeLink)
	t.Setenv("PI_CODING_AGENT_DIR", profileLink)

	marker := filepath.Join(base, "probe-environment")
	binDir := t.TempDir()
	script := `#!/bin/sh
{
printf 'home=%s\n' "$HOME"
printf 'profile=%s\n' "$PI_CODING_AGENT_DIR"
printf 'tmp=%s\n' "$TMPDIR"
printf 'config=%s\n' "$XDG_CONFIG_HOME"
printf 'data=%s\n' "$XDG_DATA_HOME"
printf 'state=%s\n' "$XDG_STATE_HOME"
printf 'cache=%s\n' "$XDG_CACHE_HOME"
printf 'locale=%s/%s\n' "$LANG" "$LC_ALL"
printf 'credentials=%s/%s/%s/%s/%s\n' "${OPENAI_API_KEY-unset}" "${ANTHROPIC_API_KEY-unset}" "${GOOGLE_API_KEY-unset}" "${OMP_ACCESS_TOKEN-unset}" "${PI_PROVIDER_TOKEN-unset}"
} > ` + strconv.Quote(marker) + `
printf 'omp/17.2.6\n'
`
	require.NoError(t, os.WriteFile(filepath.Join(binDir, cliBinary), []byte(script), 0o755))
	t.Setenv("PATH", binDir)

	detected, err := NewWithRoot(t.TempDir()).Detect(context.Background())
	require.NoError(t, err)
	require.True(t, detected)
	recorded, err := os.ReadFile(marker)
	require.NoError(t, err)
	environment := string(recorded)
	assert.Contains(t, environment, "home="+canonicalHome+"\n")
	assert.Contains(t, environment, "profile="+canonicalProfile+"\n")
	assert.Contains(t, environment, "credentials=unset/unset/unset/unset/unset\n")
	assert.NotContains(t, environment, sentinel)
	for _, locator := range []struct{ key, leaf string }{
		{"tmp", "tmp"}, {"config", "config"}, {"data", "data"},
		{"state", "state"}, {"cache", "cache"},
	} {
		assert.Regexp(t, `(?m)^`+locator.key+`=.*/autopus-omp-detect-[^/]+/`+locator.leaf+`$`, environment)
	}
	assert.Contains(t, environment, "locale=C/C\n")
}

// TestOMPAdapter_REQ019_DetectAbsentBinary covers the no-binary case: detection
// returns false without an error so `auto doctor` reports omp as unavailable
// rather than failing the whole run.
func TestOMPAdapter_REQ019_DetectAbsentBinary(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	t.Setenv("PATH", t.TempDir())

	got, err := NewWithRoot(t.TempDir()).Detect(context.Background())
	require.NoError(t, err)
	assert.False(t, got)
}

// TestOMPAdapter_REQ019_DetectRejectsFailingProbe covers a binary that exists
// but exits non-zero: no version output means no identity proof.
func TestOMPAdapter_REQ019_DetectRejectsFailingProbe(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, cliBinary), []byte("#!/bin/sh\nexit 1\n"), 0o755))
	t.Setenv("PATH", dir)

	got, err := NewWithRoot(t.TempDir()).Detect(context.Background())
	require.NoError(t, err)
	assert.False(t, got, "a non-zero version probe must not be adopted as omp")
}

// TestOMPAdapter_InstallHooksWritesNothing pins the REQ-001 no-op contract: omp
// has no hook surface, and InstallHooks must not create a file that Clean would
// then have no manifest record for.
func TestOMPAdapter_InstallHooksWritesNothing(t *testing.T) {
	dir := t.TempDir()
	before := snapshotTreeContract(t, dir)

	require.NoError(t, NewWithRoot(dir).InstallHooks(
		context.Background(),
		[]adapter.HookConfig{{Event: "Stop", Command: "echo hi"}},
		nil,
	))

	assert.Equal(t, before, snapshotTreeContract(t, dir),
		"InstallHooks must leave the workspace byte-identical")
}

// TestOMPAdapter_GenerateRejectsNilConfig proves the adapter fails closed rather
// than writing a partial surface when no config is supplied.
func TestOMPAdapter_GenerateRejectsNilConfig(t *testing.T) {
	dir := t.TempDir()

	_, err := NewWithRoot(dir).Generate(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "하네스 설정이 필요합니다")

	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	assert.Empty(t, entries, "a rejected Generate must not leave partial output behind")
}
