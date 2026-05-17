package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPlanEmitsReleaseContractAndRedactsCommandPreview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"), 0o644))
	writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{
		"go", "test", "./...", "--password", "hunter2",
		"--report", "https://user:pass@example.test/out",
		"--config", "/Users/alice/private.env", "API_TOKEN=s3cr3t",
	})
	writeReleaseJourney(t, dir, "staging-login", "browser-staging", "node-script", []string{"npm", "test"})
	writeReleaseJourney(t, dir, "local-gui", "gui-explore", "gui-explore", []string{"npx", "playwright", "test"})

	plan, err := BuildPlan(Options{
		ProjectDir: dir,
		Profile:    "prelaunch",
		DryRun:     true,
		Command:    "auto qa release --profile prelaunch --dry-run --format json",
	})
	require.NoError(t, err)

	assert.Equal(t, PlanSchemaVersion, plan.SchemaVersion)
	assert.Equal(t, "auto qa release --profile prelaunch --dry-run --format json", plan.Command)
	assert.True(t, plan.DryRun)
	assert.Equal(t, "prelaunch", plan.Profile)
	assert.Equal(t, ReleaseLanes(), plan.SelectedLanes)
	assert.Equal(t, RedactionRedacted, plan.RedactionStatus)
	assert.Empty(t, plan.SideEffects)
	assert.Contains(t, plan.RedactionRules, "token-flag")
	assert.Contains(t, plan.RedactionRules, "env-secret")
	assert.Contains(t, plan.RedactionRules, "credential-url")
	assert.Contains(t, plan.RedactionRules, "private-path")

	fast := findJourneyPackRow(t, plan.JourneyPacks, "fast")
	assert.Equal(t, "unit", fast.JourneyID)
	assert.Equal(t, "go-test", fast.Adapter)
	assert.True(t, fast.CommandDeclared)
	assert.True(t, fast.Executable)
	assert.Equal(t, "SPEC-QAMESH-002", fast.SourceSpec)
	assert.False(t, fast.InventedCommand)
	assert.True(t, fast.CommandPreviewRedacted)
	assert.NotNil(t, fast.AcceptanceRefs)
	assert.NotContains(t, fast.CommandPreview, "s3cr3t")
	assert.NotContains(t, fast.CommandPreview, "hunter2")
	assert.NotContains(t, fast.CommandPreview, "user:pass@")
	assert.NotContains(t, fast.CommandPreview, "/Users/alice")
	assert.Contains(t, fast.CommandPreview, "<redacted>")
	assert.Contains(t, fast.CommandPreview, "https://<redacted>@example.test/out")

	assertReleaseGap(t, plan.SetupGaps, "desktop-native", SetupGapMissingJourneyPack, true, SeverityHigh)
	assertReleaseGap(t, plan.SetupGaps, "canary-explicit", SetupGapCanaryTemplate, false, SeverityLow)
	assertReleaseGap(t, plan.SetupGaps, "mobile-readiness", SetupGapSiblingSpecPending, false, SeverityLow)
	assert.Equal(t, []string{"fast", "browser-staging", "desktop-native", "gui-explore"}, plan.BlockerRules.MustLanes)
	assert.Equal(t, []string{"canary-explicit"}, plan.BlockerRules.OptionalLanes)
	assert.Equal(t, []string{"mobile-readiness", "evidence-dashboard"}, plan.BlockerRules.DeferredLanes)
	assert.Equal(t, BlockingMatrixVersion, plan.BlockerRules.MatrixVersion)
	assert.Contains(t, plan.OutputPaths.ReleaseIndexPreviewPath, "release-index.json")
	assert.Contains(t, siblingSpecIDs(plan.SiblingSpecs), "SPEC-QAMESH-006")
}

func TestBuildPlanDemotesInapplicableSurfaceLanes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{"go", "test", "./..."})
	writeReleaseJourney(t, dir, "canary", "canary-explicit", "canary-template", []string{"auto", "canary"})

	plan, err := BuildPlan(Options{ProjectDir: dir, Profile: "release-candidate", DryRun: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"fast", "canary-explicit"}, plan.BlockerRules.MustLanes)
	assert.NotContains(t, plan.BlockerRules.MustLanes, "browser-staging")
	assert.NotContains(t, plan.BlockerRules.MustLanes, "desktop-native")
	assertReleaseGap(t, plan.SetupGaps, "browser-staging", SetupGapMissingJourneyPack, false, SeverityMedium)
	assertReleaseGap(t, plan.SetupGaps, "desktop-native", SetupGapMissingJourneyPack, false, SeverityMedium)
}

func TestBuildPlanSerializesEmptyAcceptanceRefsAsArray(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReleaseJourneyNoRefs(t, dir, "unit", "fast", "go-test", []string{"go", "test", "./..."})

	plan, err := BuildPlan(Options{ProjectDir: dir, Profile: "prelaunch", DryRun: true})
	require.NoError(t, err)
	body, err := json.Marshal(plan)
	require.NoError(t, err)

	fast := findJourneyPackRow(t, plan.JourneyPacks, "fast")
	assert.NotNil(t, fast.AcceptanceRefs)
	assert.Contains(t, string(body), `"acceptance_refs":[]`)
}

func TestRoadmapReportsCanonicalLaneOwnership(t *testing.T) {
	t.Parallel()

	roadmap := Roadmap()

	assert.Equal(t, RoadmapSchemaVersion, roadmap.SchemaVersion)
	assert.Equal(t, ReleaseLanes(), roadmapLaneIDs(roadmap.Lanes))
	assert.Equal(t, "SPEC-QAMESH-005", findRoadmapLane(t, roadmap.Lanes, "browser-staging").OwnerSpec)
	assert.Equal(t, "SPEC-QAMESH-003", findRoadmapLane(t, roadmap.Lanes, "gui-explore").OwnerSpec)
	assert.Equal(t, "autopus-adk", findRoadmapLane(t, roadmap.Lanes, "mobile-readiness").OwnerRepo)
	assert.Equal(t, "SPEC-QAMESH-006", findRoadmapLane(t, roadmap.Lanes, "mobile-readiness").SiblingDependency)
	assert.Equal(t, LanePolicyMust, findRoadmapLane(t, roadmap.Lanes, "canary-explicit").LaunchBlockingPolicy["release-candidate"])
	assert.Contains(t, roadmap.Profiles["release-candidate"].MustLanes, "canary-explicit")
}

func TestBuildPlanRedactsSensitiveCommandRunAsWholePreview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReleaseJourneyRun(t, dir, "unit", "fast", "go-test", "go test ./... --password 'hunter two' --config /Users/alice/private file.env")

	plan, err := BuildPlan(Options{ProjectDir: dir, Profile: "prelaunch", DryRun: true})
	require.NoError(t, err)

	fast := findJourneyPackRow(t, plan.JourneyPacks, "fast")
	assert.Equal(t, "<redacted>", fast.CommandPreview)
	assert.True(t, fast.CommandPreviewRedacted)
	assert.NotContains(t, fast.CommandPreview, "hunter")
	assert.NotContains(t, fast.CommandPreview, "two")
	assert.NotContains(t, fast.CommandPreview, "/Users/alice")
}

func writeReleaseJourney(t *testing.T, dir, id, lane, adapter string, argv []string) {
	t.Helper()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := "id: " + id + "\n" +
		"title: " + id + "\n" +
		"surface: cli\n" +
		"lanes: [" + lane + "]\n" +
		"adapter:\n  id: " + adapter + "\n" +
		"command:\n  argv:\n"
	for _, arg := range argv {
		body += "    - " + arg + "\n"
	}
	body += "  cwd: .\n  timeout: 60s\n" +
		"checks:\n  - id: smoke\n    type: unit_test\n" +
		"source_refs:\n  source_spec: SPEC-QAMESH-002\n  acceptance_refs: [AC-QAMESH2-005]\n"
	if adapter == "gui-explore" {
		body += "gui:\n" +
			"  allowed_origins: [http://127.0.0.1:4173]\n" +
			"  forbidden_actions: [mutation, payment, email_send]\n" +
			"  selector_strategy: role-first\n" +
			"  network_policy:\n    mode: summary-only\n" +
			"  artifact_retention:\n    publish_raw: false\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, id+".yaml"), []byte(body), 0o644))
}

func writeReleaseJourneyRun(t *testing.T, dir, id, lane, adapter, run string) {
	t.Helper()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := "id: " + id + "\n" +
		"title: " + id + "\n" +
		"surface: cli\n" +
		"lanes: [" + lane + "]\n" +
		"adapter:\n  id: " + adapter + "\n" +
		"command:\n  run: " + run + "\n  cwd: .\n  timeout: 60s\n" +
		"checks:\n  - id: smoke\n    type: unit_test\n" +
		"source_refs:\n  source_spec: SPEC-QAMESH-002\n  acceptance_refs: [AC-QAMESH2-005]\n"
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, id+".yaml"), []byte(body), 0o644))
}

func writeReleaseJourneyNoRefs(t *testing.T, dir, id, lane, adapter string, argv []string) {
	t.Helper()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := "id: " + id + "\n" +
		"title: " + id + "\n" +
		"surface: cli\n" +
		"lanes: [" + lane + "]\n" +
		"adapter:\n  id: " + adapter + "\n" +
		"command:\n  argv:\n"
	for _, arg := range argv {
		body += "    - " + arg + "\n"
	}
	body += "  cwd: .\n  timeout: 60s\n" +
		"checks:\n  - id: smoke\n    type: unit_test\n"
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, id+".yaml"), []byte(body), 0o644))
}
