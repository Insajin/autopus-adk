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

func TestBuildPlanReportsHarnessContractAndProjectLocalGUIGap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test:visual:macos":"node scripts/visual/run-macos-wkwebview-suite.mjs"},"dependencies":{"@tauri-apps/api":"latest"}}`), 0o644))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Equal(t, "harness", plan.HarnessContract.Role)
	assert.Equal(t, "project-local", plan.HarnessContract.JourneyPackOwnership)
	assert.Contains(t, plan.HarnessContract.JourneyPackRoot, ".autopus/qa/journeys")
	require.NotEmpty(t, plan.SetupGaps)
	assert.Equal(t, "gui-explore", plan.SetupGaps[0].Adapter)
	assert.Contains(t, plan.SetupGaps[0].Reason, "ADK is a harness")
	assert.Contains(t, plan.SetupGaps[0].Reason, ".autopus/qa/journeys")
}

func TestBuildPlanReportsDesktopGUIHintWithoutFastLaneSetupGap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"), 0o644))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "fast", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	require.NotEmpty(t, plan.ProjectHints)
	assert.Equal(t, "gui-explore", plan.ProjectHints[0].Adapter)
	assert.Contains(t, plan.ProjectHints[0].Reason, "desktop GUI tooling detected")
	for _, gap := range plan.SetupGaps {
		assert.NotEqual(t, "project-local-gui-explore", gap.JourneyID)
	}
}

func TestBuildPlanDoesNotUseDetectedFallbackForGUIExploreLane(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"node test.js"},"devDependencies":{"@playwright/test":"latest","vitest":"latest"}}`), 0o644))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Contains(t, plan.DetectedAdapters, "playwright")
	assert.Empty(t, plan.SelectedJourneys)
	assert.Empty(t, plan.SelectedAdapters)
	require.NotEmpty(t, plan.SetupGaps)
	assert.Equal(t, "project-local-gui-explore", plan.SetupGaps[0].JourneyID)
}

func TestBuildPlanDoesNotReportGUIGapWhenProjectLocalJourneyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"), 0o644))
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := []byte(`id: desktop-gui
title: Desktop GUI
surface: desktop
lanes: [gui-explore]
adapter:
  id: gui-explore
command:
  run: npm exec playwright test
  cwd: .
  timeout: 60s
checks:
  - id: desktop-gui
    type: gui_exploration
gui:
  allowed_origins: ["http://127.0.0.1:1420"]
  forbidden_actions: ["mutation", "payment", "email_send"]
  selector_strategy: role-first
  network_policy:
    mode: local-only
  artifact_retention:
    publish_raw: false
`)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "desktop-gui.yaml"), body, 0o644))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Contains(t, plan.SelectedJourneys, "desktop-gui")
	for _, gap := range plan.SetupGaps {
		assert.NotEqual(t, "project-local-gui-explore", gap.JourneyID)
	}
}
