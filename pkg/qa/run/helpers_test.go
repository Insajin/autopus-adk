package run

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
)

func TestHelperBranches(t *testing.T) {
	assert.Equal(t, "npm test", defaultCommand("node-script").Run)
	assert.Equal(t, "pytest", defaultCommand("pytest").Run)
	assert.Equal(t, "cargo test", defaultCommand("cargo-test").Run)
	assert.Empty(t, defaultCommand("playwright").Run)
	assert.Equal(t, "package", surfaceForAdapter("node-script"))
	assert.Equal(t, "frontend", surfaceForAdapter("playwright"))
	assert.Equal(t, "custom", surfaceForAdapter("custom-command"))
	assert.Equal(t, "multi", surfaceForAdapter("auto-test-run"))
	assert.Equal(t, "cli", surfaceForAdapter("go-test"))
	assert.Equal(t, []string{"go", "test", "./..."}, commandArgs(journeyPack("go-test", "")))
	assert.Equal(t, []string{"echo", "ok"}, commandArgs(journeyPack("custom-command", "echo ok")))
	assert.Nil(t, commandArgs(journeyPack("playwright", "")))
	t.Setenv("QAMESH_ALLOWED", "yes")
	env := allowedEnv(t.TempDir(), []string{"QAMESH_ALLOWED", "QAMESH_MISSING"})
	assert.Contains(t, env, "QAMESH_ALLOWED=yes")
	assert.NotContains(t, strings.Join(allowedEnv(t.TempDir(), nil), "\n"), "OPENAI_API_KEY=")
	assert.Equal(t, "blocked", aggregateStatus(Result{AdapterResults: []AdapterResult{{Status: "blocked"}}}))
	assert.Equal(t, "warning", aggregateStatus(Result{SetupGaps: []SetupGap{{Adapter: "playwright", Reason: "missing"}}}))
	assert.Equal(t, "failed", aggregateStatus(Result{FailedChecks: []string{"unit"}}))
	assert.Equal(t, "warning", indexStatus(Result{Status: "warning"}))
	assert.NotNil(t, setupGapFor(Options{Profile: "standalone"}, journeyPack("missing", "")))
	assert.NotNil(t, setupGapFor(Options{Profile: "standalone"}, journeyPack("canary-template", "")))
	assert.NotNil(t, setupGapFor(Options{Profile: "standalone"}, journey.Pack{
		ID:                  "profile",
		Adapter:             journey.AdapterRef{ID: "go-test"},
		ProfileRequirements: journey.ProfileRequirements{Capabilities: []string{"frontend-server"}},
	}))
	assert.Equal(t, []string{"a"}, appendUnique([]string{"a"}, "a"))
	assert.Equal(t, []string{"a", "b"}, appendUnique([]string{"a"}, "b"))
	assert.Equal(t, []string{"a"}, appendUnique([]string{"a"}, ""))
	assert.Equal(t, "item", safeSegment(""))
	assert.Equal(t, "a-b-c", safeSegment("a/b:c"))
	assert.True(t, includePack(journeyPack("go-test", ""), Options{Lane: "fast", AdapterID: "go-test"}))
	assert.False(t, includePack(journeyPack("go-test", ""), Options{Lane: "fast", AdapterID: "pytest"}))
	defaulted := normalizeOptions(Options{})
	assert.Equal(t, ".", defaulted.ProjectDir)
	assert.Equal(t, "standalone", defaulted.Profile)
	assert.Equal(t, "fast", defaulted.Lane)
}

func TestManifestHelpersPreserveExplicitSourceRefs(t *testing.T) {
	t.Parallel()

	pack := journey.Pack{
		ID:      "explicit",
		Adapter: journey.AdapterRef{ID: "pytest"},
		Checks:  []journey.Check{},
		SourceRefs: journey.SourceRefs{
			SourceSpec:       "SPEC-X",
			AcceptanceRefs:   []string{"AC-X"},
			OwnedPaths:       []string{"src"},
			DoNotModifyPaths: []string{"vendor/**"},
		},
	}

	assert.Equal(t, "explicit", firstCheckID(pack))
	assert.Equal(t, "unit_test", firstCheckType(pack))
	refs := sourceRefs(pack)
	assert.Equal(t, "SPEC-X", refs.SourceSpec)
	assert.Equal(t, []string{"AC-X"}, refs.AcceptanceRefs)
	assert.Equal(t, "explicit", refs.JourneyID)
	assert.Equal(t, "pytest", refs.Adapter)
}

func TestManifestHelpersDefaultSourceRefs(t *testing.T) {
	t.Parallel()

	refs := sourceRefs(journeyPack("go-test", "go test ./..."))

	assert.Equal(t, "SPEC-QAMESH-002", refs.SourceSpec)
	assert.Equal(t, []string{"AC-QAMESH2-005"}, refs.AcceptanceRefs)
	assert.Equal(t, []string{"."}, refs.OwnedPaths)
	assert.Contains(t, refs.DoNotModifyPaths, ".autopus/plugins/**")
	assert.Equal(t, map[string]any{"exit_code": 0}, refs.OracleThresholds)
}
