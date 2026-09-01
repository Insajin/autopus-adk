package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCanaryFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func canaryTargetIDs(targets []canaryCommandTarget) []string {
	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.ID)
	}
	return ids
}

// TestCanaryBuildUnits_RootWithStackWins proves a normal single-stack project is
// resolved from its own root and never scanned for sibling directories.
func TestCanaryBuildUnits_RootWithStackWins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeCanaryFile(t, dir, "go.mod", "module example.test\n")
	writeCanaryFile(t, dir, "examples/widget/package.json", `{"scripts":{"build":"x"}}`)

	units := canaryBuildUnits(dir)

	require.Len(t, units, 1)
	assert.Equal(t, ".", units[0].Rel)
	assert.Equal(t, []string{"go"}, units[0].Stacks)
}

// TestCanaryBuildUnits_UmbrellaCheckoutFindsMembers covers the Autopus monorepo
// shape: the root declares no stack, so the members two levels down must still
// be built.
func TestCanaryBuildUnits_UmbrellaCheckoutFindsMembers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeCanaryFile(t, dir, "adk/go.mod", "module example.test/adk\n")
	writeCanaryFile(t, dir, "monorepo/backend/go.mod", "module example.test/backend\n")
	writeCanaryFile(t, dir, "monorepo/frontend/package.json", `{"scripts":{"build":"vite build"}}`)
	writeCanaryFile(t, dir, "desktop/package.json", `{"scripts":{"build":"tauri build"}}`)
	writeCanaryFile(t, dir, "desktop/src-tauri/Cargo.toml", "[package]\nname=\"shell\"\n")
	// Neither of these may become a unit: one is vendored, the other lives inside
	// a directory that already declared a stack.
	writeCanaryFile(t, dir, "node_modules/dep/package.json", `{"scripts":{"build":"x"}}`)
	writeCanaryFile(t, dir, "adk/fixtures/package.json", `{"scripts":{"build":"x"}}`)

	targets, skips := canaryBuildTargets(dir)

	assert.Equal(t, []string{
		"build:adk:go",
		"build:desktop:node",
		"build:desktop:tauri",
		"build:monorepo/backend:go",
		"build:monorepo/frontend:node",
	}, canaryTargetIDs(targets))
	assert.Empty(t, skips)
}

// TestCanaryBuildTargets_StatesReasonForContributingNothing proves a detected
// stack that yields no command is reported, not silently dropped.
func TestCanaryBuildTargets_StatesReasonForContributingNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeCanaryFile(t, dir, "package.json", `{"scripts":{"lint":"eslint ."}}`)
	writeCanaryFile(t, dir, "pyproject.toml", "[project]\nname=\"x\"\n")

	targets, skips := canaryBuildTargets(dir)

	assert.Empty(t, targets)
	require.Len(t, skips, 2)
	assert.Equal(t, "build:.:node", skips[0].Area)
	assert.Equal(t, "package.json declares no build script", skips[0].Reason)
	assert.Equal(t, "build:.:python", skips[1].Area)
	assert.NotEmpty(t, skips[1].Reason)
}

// TestCanaryBuildTargets_UsesDeclaredPackageManager keeps the canary build on
// the same package manager the QA scaffold writes into Journey Packs.
func TestCanaryBuildTargets_UsesDeclaredPackageManager(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeCanaryFile(t, dir, "package.json", `{"scripts":{"build":"vite build"}}`)
	writeCanaryFile(t, dir, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")

	targets, _ := canaryBuildTargets(dir)

	require.Len(t, targets, 1)
	assert.Equal(t, []string{"pnpm", "run", "build"}, targets[0].Args)
}

// TestCanaryCheckFailure_NeverEmptyDetail is the direct regression guard for
// `H1 failed: ` - an exec error before the process starts leaves no output, and
// a blank reason is unactionable.
func TestCanaryCheckFailure_NeverEmptyDetail(t *testing.T) {
	t.Parallel()

	missingDir := filepath.Join(t.TempDir(), "absent")
	run := runCanaryExternal(t.Context(), "build:.:go", "go build ./...", missingDir, "go", "build", "./...")

	assert.Equal(t, "FAIL", run.Status)
	assert.NotEmpty(t, run.Detail)
	assert.Contains(t, run.Detail, "absent")
}

// TestCanaryBuildPhase_NoTargetSkipsWithReason proves a project with nothing to
// build satisfies the check instead of failing it.
func TestCanaryBuildPhase_NoTargetSkipsWithReason(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := canaryResult{Verdict: "PASS", Build: "SKIPPED"}

	require.NoError(t, runCanaryBuildPhase(t.Context(), dir, nil, &result))

	assert.Equal(t, "SKIPPED", result.Build)
	assert.Equal(t, "PASS", result.Verdict)
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, "build", result.Skipped[0].Area)
	assert.Contains(t, result.Skipped[0].Reason, "no buildable stack detected")
}

// TestCanaryBuildPhase_LaterChecksReportNotReached proves the summary no longer
// claims PASS for checks a bail-out never ran.
func TestCanaryBuildPhase_LaterChecksReportNotReached(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeCanaryFile(t, dir, "go.mod", "module example.test\n")
	writeCanaryFile(t, dir, "broken.go", "package main\nfunc main() { undefinedSymbol() }\n")
	targets, _ := canaryBuildTargets(dir)
	result := canaryResult{Verdict: "PASS", Build: "SKIPPED", E2E: "SKIPPED", Doctor: "SKIPPED"}

	err := runCanaryBuildPhase(t.Context(), dir, targets, &result)

	require.Error(t, err)
	assert.NotEqual(t, "", strings.TrimSpace(strings.TrimPrefix(err.Error(), "build:.:go failed:")))
	assert.Equal(t, "FAIL", result.Build)
	assert.Equal(t, "SKIPPED", result.E2E)
	assert.Equal(t, "SKIPPED", result.Doctor)
	reasons := map[string]string{}
	for _, skipped := range result.Skipped {
		reasons[skipped.Area] = skipped.Reason
	}
	for _, area := range []string{"e2e", "doctor", "endpoint", "browser"} {
		assert.Equal(t, "not reached: build:.:go failed", reasons[area], area)
	}
}

// TestCanaryDoctorSkip_RequiresAutopusProject proves `auto doctor` is gated on
// the harness it audits instead of failing every project without autopus.yaml.
func TestCanaryDoctorSkip_RequiresAutopusProject(t *testing.T) {
	t.Parallel()

	plain := t.TempDir()
	assert.Contains(t, canaryDoctorSkip(plain), "autopus.yaml")

	managed := t.TempDir()
	writeCanaryFile(t, managed, "autopus.yaml", "project: managed\n")
	assert.Empty(t, canaryDoctorSkip(managed))
}

// TestCanaryBrowserTarget_SkipsWithReasonWithoutBrowserProject proves a
// non-browser project gets a stated skip rather than a dangling directory name.
func TestCanaryBrowserTarget_SkipsWithReasonWithoutBrowserProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeCanaryFile(t, dir, "go.mod", "module example.test\n")

	browserDir, reason := canaryBrowserTarget(dir, canaryBuildUnits(dir))

	assert.Empty(t, browserDir)
	assert.NotEmpty(t, reason)
}

func TestCanaryBrowserTarget_PicksProjectBrowserDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeCanaryFile(t, dir, "web/package.json", `{"devDependencies":{"@playwright/test":"^1"}}`)

	browserDir, reason := canaryBrowserTarget(dir, canaryBuildUnits(dir))

	assert.Equal(t, filepath.Join(dir, "web"), browserDir)
	assert.Empty(t, reason)
}

// TestCanaryRemoteBrowser_SkipsWithoutURL proves the browser check no longer
// falls back to a hardcoded staging host.
func TestCanaryRemoteBrowser_SkipsWithoutURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeCanaryFile(t, dir, "package.json", `{"devDependencies":{"@playwright/test":"^1"}}`)

	result := canaryResult{}
	status := runCanaryRemoteBrowser(t.Context(), dir, resolveCanaryTargets(canaryOptions{}), &result)

	assert.Equal(t, "SKIPPED", status)
	require.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped[0].Reason, "--frontend-url")
	assert.Empty(t, result.Targets)
}

// TestCanaryTextOutput_OrderIsStableAcrossRenders is the D14 guard: canary
// stdout is captured verbatim as canary-explicit evidence, so repeated renders
// of one result must be byte-identical.
func TestCanaryTextOutput_OrderIsStableAcrossRenders(t *testing.T) {
	t.Parallel()

	result := canaryResult{
		Verdict: "WARN",
		Summary: canarySummary(canaryResult{
			Build: "PASS", E2E: "PASS", Doctor: "PASS", Endpoint: "SKIPPED", Browser: "SKIPPED",
		}),
	}
	const want = "canary WARN\nbuild: PASS\ne2e: PASS\ndoctor: PASS\nendpoint: SKIPPED\nbrowser: SKIPPED\n"

	for range 20 {
		cmd := newCanaryCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		printCanaryText(cmd, result)
		require.Equal(t, want, buf.String())
	}
}

// TestCanaryChecks_OrderIsStableAcrossRenders pins the JSON envelope ordering,
// which the release gate diffs run over run.
func TestCanaryChecks_OrderIsStableAcrossRenders(t *testing.T) {
	t.Parallel()

	result := canaryResult{Summary: canarySummary(canaryResult{
		Build: "PASS", E2E: "FAIL", Doctor: "PASS", Endpoint: "SKIPPED", Browser: "SKIPPED",
	})}
	want := []string{"canary.build", "canary.e2e", "canary.doctor", "canary.endpoint", "canary.browser"}

	for range 20 {
		ids := make([]string, 0, len(want))
		for _, check := range canaryChecks(result) {
			ids = append(ids, check.ID)
		}
		require.Equal(t, want, ids)
	}
}

// TestCanarySummaryRows_AppendsUnknownKeysSorted keeps a future summary field
// visible without letting it land in a random position.
func TestCanarySummaryRows_AppendsUnknownKeysSorted(t *testing.T) {
	t.Parallel()

	rows := canarySummaryRows(map[string]string{"zeta": "PASS", "build": "PASS", "alpha": "WARN"})

	assert.Equal(t, [][2]string{{"build", "PASS"}, {"alpha", "WARN"}, {"zeta", "PASS"}}, rows)
}
