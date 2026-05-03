package run

import (
	"os"
	"path/filepath"
	"testing"

	qacompile "github.com/insajin/autopus-adk/pkg/qa/compile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCandidatePacksFiltersAndPreservesThresholds(t *testing.T) {
	t.Parallel()

	plan := Plan{
		SelectedLane:     "fast",
		SelectedJourneys: []string{"compiled-ok", "compiled-manual", "compiled-missing"},
		CandidateJourneys: []CandidateJourney{
			{
				JourneyID:        "compiled-ok",
				StepID:           "step-a",
				Adapter:          "go-test",
				Command:          []string{"go", "test", "./..."},
				AcceptanceRefs:   []string{"AC-1"},
				OracleThresholds: map[string]any{"exit_code": 1, "stdout_contains": "ok"},
			},
			{JourneyID: "compiled-manual", Adapter: "go-test", ManualOrDeferred: true},
			{JourneyID: "compiled-missing"},
			{JourneyID: "compiled-unselected", Adapter: "go-test"},
		},
	}

	packs := candidatePacks(plan)

	require.Len(t, packs, 1)
	assert.Equal(t, "compiled-ok", packs[0].ID)
	assert.Equal(t, "step-a", packs[0].Checks[0].ID)
	assert.Equal(t, map[string]any{"exit_code": 1, "stdout_contains": "ok"}, packs[0].Checks[0].Expected)
	assert.Equal(t, []string{"AC-1"}, packs[0].SourceRefs.AcceptanceRefs)
}

func TestIncludeCandidateFilters(t *testing.T) {
	t.Parallel()

	candidate := qacompile.Candidate{JourneyID: "j1", Adapter: "go-test"}

	assert.True(t, includeCandidate(candidate, Options{JourneyID: "j1", AdapterID: "go-test"}))
	assert.False(t, includeCandidate(candidate, Options{JourneyID: "other"}))
	assert.False(t, includeCandidate(candidate, Options{AdapterID: "pytest"}))
	assert.False(t, includeCandidate(qacompile.Candidate{JourneyID: "j1", ManualOrDeferred: true}, Options{}))
	assert.False(t, includeCandidate(qacompile.Candidate{Adapter: "go-test"}, Options{}))
}

func TestBuildPlanRejectsOutputOutsideProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := BuildPlan(Options{ProjectDir: dir, Output: filepath.Join(filepath.Dir(dir), "outside-runs")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "inside project root")
}

func TestBuildPlanRejectsNestedGeneratedSurfaceOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := BuildPlan(Options{ProjectDir: dir, Output: filepath.Join(dir, "sub", ".codex", "qa")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "generated surface .codex")
}

func TestBuildPlanRejectsMixedCaseGeneratedSurfaceOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
		want   string
	}{
		{name: "codex", output: filepath.Join(".CODEX", "qa"), want: ".codex"},
		{name: "claude", output: filepath.Join(".Claude", "qa"), want: ".claude"},
		{name: "gemini", output: filepath.Join(".Gemini", "qa"), want: ".gemini"},
		{name: "opencode", output: filepath.Join(".OpenCode", "qa"), want: ".opencode"},
		{name: "autopus plugins", output: filepath.Join(".Autopus", "Plugins", "qa"), want: ".autopus/plugins"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			_, err := BuildPlan(Options{ProjectDir: dir, Output: filepath.Join(dir, tt.output)})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestBuildPlanRejectsSymlinkedOutputOutsideProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "out-link")))

	_, err := BuildPlan(Options{ProjectDir: dir, Output: filepath.Join(dir, "out-link", "qa")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "inside project root")
}

func TestBuildPlanDefersQAMESH003JourneyPacks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := []byte(`id: mobile-smoke
title: Mobile smoke
surface: mobile
lanes: [fast]
adapter:
  id: maestro
checks:
  - id: mobile
    type: mobile_test
`)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "mobile.yaml"), body, 0o644))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "fast", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Contains(t, plan.ConfiguredJourneys, "mobile-smoke")
	assert.NotContains(t, plan.SelectedJourneys, "mobile-smoke")
	require.Len(t, plan.Deferred, 1)
	assert.Equal(t, "maestro", plan.Deferred[0].Adapter)
	assert.Contains(t, plan.Deferred[0].Reason, "SPEC-QAMESH-003")
}

func TestBuildPlanDefersAIAndProductionSessionJourneyPacks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := []byte(`id: replay-ai
title: Replay AI
surface: cli
source: production_session
pass_fail_authority: ai
lanes: [fast]
adapter:
  id: go-test
command:
  run: go test ./...
  cwd: .
  timeout: 60s
checks:
  - id: replay
    type: replay_test
`)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "replay-ai.yaml"), body, 0o644))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "fast", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Contains(t, plan.ConfiguredJourneys, "replay-ai")
	assert.NotContains(t, plan.SelectedJourneys, "replay-ai")
	require.Len(t, plan.Deferred, 1)
	assert.Equal(t, "go-test", plan.Deferred[0].Adapter)
	assert.Contains(t, plan.Deferred[0].Reason, "SPEC-QAMESH-003")
}
