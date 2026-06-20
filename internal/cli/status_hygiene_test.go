package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusHygieneCollectsGitSignals(t *testing.T) {
	t.Parallel()

	dir := setupStatusHygieneRepo(t)

	report := collectStatusHygiene(dir)

	assert.True(t, report.Available)
	assert.Equal(t, "warn", report.Status)
	assert.Contains(t, report.GeneratedDrift, ".codex/agents/reviewer.toml")
	assert.Contains(t, report.GeneratedDrift, ".opencode/agents/executor.md")
	assert.Contains(t, report.TrackedButIgnored, ".codex/agents/reviewer.toml")
	assert.Contains(t, report.RuntimeUnignored, ".opencode/agents/executor.md")

	payload := report.payload()
	assert.Equal(t, "warn", payload.GeneratedDrift.Status)
	assert.Equal(t, "warn", payload.TrackedButIgnored.Status)
	assert.Equal(t, "warn", payload.RuntimeUnignored.Status)
	assert.Equal(t, 1, payload.TrackedButIgnored.Count)

	checks := hygieneJSONChecks("status", report)
	require.Len(t, checks, 3)
	assert.Equal(t, "status.hygiene.generated_drift", checks[0].ID)
	assert.Equal(t, "warn", checks[0].Status)
}

func TestStatusHygieneCollectsUnstagedTrackedGeneratedDrift(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	runStatusHygieneGit(t, dir, "init")
	runStatusHygieneGit(t, dir, "config", "user.email", "test@test.com")
	runStatusHygieneGit(t, dir, "config", "user.name", "Test")
	writeStatusHygieneFile(t, dir, ".codex/agents/reviewer.toml", "name = \"reviewer\"\n")
	runStatusHygieneGit(t, dir, "add", ".codex/agents/reviewer.toml")
	runStatusHygieneGit(t, dir, "commit", "-m", "seed")
	writeStatusHygieneFile(t, dir, ".codex/agents/reviewer.toml", "name = \"reviewer2\"\n")

	report := collectStatusHygiene(dir)

	assert.True(t, report.Available)
	assert.Contains(t, report.GeneratedDrift, ".codex/agents/reviewer.toml")
}

func TestStatusHygieneCollectsTrackedRuntimeDrift(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	runStatusHygieneGit(t, dir, "init")
	runStatusHygieneGit(t, dir, "config", "user.email", "test@test.com")
	runStatusHygieneGit(t, dir, "config", "user.name", "Test")
	writeStatusHygieneFile(t, dir, ".autopus/runtime/memindex/state.json", "{}\n")
	runStatusHygieneGit(t, dir, "add", ".autopus/runtime/memindex/state.json")
	runStatusHygieneGit(t, dir, "commit", "-m", "seed")
	writeStatusHygieneFile(t, dir, ".autopus/runtime/memindex/state.json", "{\"dirty\":true}\n")

	report := collectStatusHygiene(dir)

	assert.True(t, report.Available)
	assert.Contains(t, report.GeneratedDrift, ".autopus/runtime/memindex/state.json")
}

func TestStatusHygieneUnavailableOutsideGit(t *testing.T) {
	t.Parallel()

	report := collectStatusHygiene(t.TempDir())

	assert.False(t, report.Available)
	assert.Equal(t, "unavailable", report.Status)
	assert.NotEmpty(t, report.Diagnostic)

	warnings := hygieneJSONWarnings(report)
	require.Len(t, warnings, 1)
	assert.Equal(t, "hygiene_unavailable", warnings[0].Code)

	checks := hygieneJSONChecks("doctor", report)
	require.Len(t, checks, 3)
	for _, check := range checks {
		assert.Equal(t, "warn", check.Status)
		assert.Equal(t, "warning", check.Severity)
	}
}

func TestHygieneMetricCheckLimitsPathDetail(t *testing.T) {
	t.Parallel()

	paths := []string{
		".codex/0", ".codex/1", ".codex/2", ".codex/3", ".codex/4", ".codex/5",
		".codex/6", ".codex/7", ".codex/8", ".codex/9", ".codex/10", ".codex/11",
	}

	check := hygieneMetricCheck("status", "generated_drift", "generated", true, "", paths)

	assert.Contains(t, check.Detail, "... and 2 more")
	assert.NotContains(t, check.Detail, ".codex/11")
}

func TestDoctorJSONReportCollectHygieneChecks_AddsPayloadAndChecks(t *testing.T) {
	t.Parallel()

	dir := setupStatusHygieneRepo(t)
	report := doctorJSONReport{status: jsonStatusOK}

	report.collectHygieneChecks(dir)

	assert.Equal(t, jsonStatusWarn, report.status)
	require.NotNil(t, report.data.Hygiene)
	assert.Equal(t, "warn", report.data.Hygiene.Status)
	assert.Contains(t, report.data.Hygiene.TrackedButIgnored.Paths, ".codex/agents/reviewer.toml")

	ids := map[string]bool{}
	for _, check := range report.checks {
		ids[check.ID] = true
	}
	assert.True(t, ids["doctor.hygiene.generated_drift"])
	assert.True(t, ids["doctor.hygiene.tracked_but_ignored"])
	assert.True(t, ids["doctor.hygiene.runtime_unignored"])
}

func TestRunStatusJSON_HygieneWarnsWithCompletedSpecs(t *testing.T) {
	t.Parallel()

	dir := setupStatusHygieneRepo(t)
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-HYGIENE-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("---\nstatus: completed\ntitle: Hygiene\n---\n"), 0o644))

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--dir", dir, "--json"})

	require.NoError(t, cmd.Execute())

	var env jsonEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, jsonStatusWarn, env.Status)

	raw, err := json.Marshal(env.Data)
	require.NoError(t, err)
	var data statusJSONData
	require.NoError(t, json.Unmarshal(raw, &data))
	assert.Equal(t, 0, data.Summary.Open)
	require.NotNil(t, data.Hygiene)
	assert.Equal(t, "warn", data.Hygiene.Status)
	assert.Contains(t, data.Hygiene.RuntimeUnignored.Paths, ".opencode/agents/executor.md")
}

func setupStatusHygieneRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runStatusHygieneGit(t, dir, "init")
	runStatusHygieneGit(t, dir, "config", "user.email", "test@test.com")
	runStatusHygieneGit(t, dir, "config", "user.name", "Test")

	writeStatusHygieneFile(t, dir, ".gitignore", ".codex/\n")
	writeStatusHygieneFile(t, dir, ".codex/agents/reviewer.toml", "name = \"reviewer\"\n")
	writeStatusHygieneFile(t, dir, ".opencode/agents/executor.md", "# executor\n")
	runStatusHygieneGit(t, dir, "add", ".gitignore")
	runStatusHygieneGit(t, dir, "add", "-f", ".codex/agents/reviewer.toml")
	return dir
}

func runStatusHygieneGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeStatusHygieneFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
