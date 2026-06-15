package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeReleaseReadinessFixture creates a hermetic project dir with exactly one
// browser surface signal (playwright.config.ts) and an empty Journey Pack root,
// so regeneration proposes at least one added pack.
func writeReleaseReadinessFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "playwright.config.ts"),
		[]byte("export default {};\n"), 0o600))
	require.NoError(t, os.MkdirAll(
		filepath.Join(dir, ".autopus", "qa", "journeys"), 0o755))
	return dir
}

func runReleaseReadiness(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return decodeJSONMap(t, out.Bytes()), err
}

// AC-005: regeneration produces a diff but the unapproved run writes nothing and
// executes no lanes.
func TestQAReleaseReadiness_DiffPresentedNoSideEffects(t *testing.T) {
	t.Parallel()

	dir := writeReleaseReadinessFixture(t)
	payload, err := runReleaseReadiness(t,
		"qa", "release-readiness", "--project-dir", dir, "--json")
	require.NoError(t, err)
	assertCommonJSONEnvelope(t, payload, "auto qa release-readiness")

	data := payload["data"].(map[string]any)
	assert.Equal(t, "diff_presented", data["phase"])
	assert.Equal(t, float64(0), data["files_written"])
	assert.Equal(t, float64(0), data["lanes_executed"])

	diff := data["diff"].(map[string]any)
	require.GreaterOrEqual(t, diff["added_count"].(float64), float64(1))

	// Nothing was persisted: the empty journeys dir stays empty.
	entries, err := os.ReadDir(filepath.Join(dir, ".autopus", "qa", "journeys"))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// Observable contract: the JSON envelope exposes every documented field.
func TestQAReleaseReadiness_PayloadFields(t *testing.T) {
	t.Parallel()

	dir := writeReleaseReadinessFixture(t)
	payload, err := runReleaseReadiness(t,
		"qa", "release-readiness", "--project-dir", dir, "--json")
	require.NoError(t, err)

	data := payload["data"].(map[string]any)
	assert.Equal(t, "qamesh.release_readiness.v1", data["schema_version"])
	assert.Contains(t, data, "analyzed_surfaces")
	assert.Contains(t, data, "phase")
	assert.Contains(t, data, "files_written")
	assert.Contains(t, data, "lanes_executed")

	diff := data["diff"].(map[string]any)
	for _, field := range []string{"added_count", "changed_count", "removed_count"} {
		assert.Contains(t, diff, field)
	}
	verdict := data["verdict"].(map[string]any)
	assert.Contains(t, verdict, "status")
	assert.Contains(t, verdict, "deterministic_authority")

	surfaces := data["analyzed_surfaces"].([]any)
	assert.Contains(t, surfaces, "web")
}

// AC-011 (Should): phases are distinct across operator signals. Without
// --approve the run reports diff_presented; with --decline it reports declined
// and still performs no write or execution.
func TestQAReleaseReadiness_PhaseDistinctness(t *testing.T) {
	t.Parallel()

	dir := writeReleaseReadinessFixture(t)

	presented, err := runReleaseReadiness(t,
		"qa", "release-readiness", "--project-dir", dir, "--json")
	require.NoError(t, err)
	assert.Equal(t, "diff_presented", presented["data"].(map[string]any)["phase"])

	declined, err := runReleaseReadiness(t,
		"qa", "release-readiness", "--project-dir", dir, "--decline", "--json")
	require.NoError(t, err)
	declinedData := declined["data"].(map[string]any)
	assert.Equal(t, "declined", declinedData["phase"])
	assert.Equal(t, float64(0), declinedData["files_written"])
	assert.Equal(t, float64(0), declinedData["lanes_executed"])

	entries, err := os.ReadDir(filepath.Join(dir, ".autopus", "qa", "journeys"))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// AC-006: the only entry point is the explicit subcommand. The command is
// registered under qa, reachable only via Find, and the source file wires no
// init()/cron/hook/scheduler/auto-trigger.
func TestQAReleaseReadiness_OnlyExplicitEntryPoint(t *testing.T) {
	t.Parallel()

	qa := newQACmd()
	var found *cobra.Command
	for _, sub := range qa.Commands() {
		if sub.Name() == "release-readiness" {
			found = sub
			break
		}
	}
	require.NotNil(t, found, "release-readiness must be registered under qa")
	assert.False(t, found.Hidden, "command must be explicitly reachable")
	assert.Empty(t, found.Commands(), "command is a leaf, not a parent namespace")

	root := NewRootCmd()
	reached, _, err := root.Find([]string{"qa", "release-readiness"})
	require.NoError(t, err)
	require.NotNil(t, reached)
	assert.Equal(t, "release-readiness", reached.Name())

	src, err := os.ReadFile("qa_release_readiness.go")
	require.NoError(t, err)
	// Inspect only executable code, not explanatory comments, so the assertion
	// proves the absence of real auto-trigger wiring rather than prose mentions.
	var code strings.Builder
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		code.WriteString(line)
		code.WriteString("\n")
	}
	text := code.String()
	for _, forbidden := range []string{"func init(", "cron", "Cron", "Schedule", "scheduler", "Hook", "PersistentPreRun"} {
		assert.False(t, strings.Contains(text, forbidden),
			"source must not register %q auto-trigger", forbidden)
	}
}
