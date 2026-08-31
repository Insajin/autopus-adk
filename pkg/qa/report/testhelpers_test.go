package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/evidence"
	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

const (
	fixtureRunID     = "qa-fixture-1"
	fixtureReleaseID = "rel-fixture-1"
)

type fixture struct {
	root         string
	runIndexPath string
	runDir       string
}

func (f fixture) options() Options {
	return Options{ProjectDir: f.root, Now: fixedNow}
}

func fixedNow() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

// newFixture writes a project containing one passing CLI journey and one failing
// frontend journey, mirroring the layout `auto qa run` produces.
func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	runDir := filepath.Join(root, ".autopus", "qa", "runs", fixtureRunID)

	writeManifest(t, root, runDir, manifestSpec{
		journey:   "cli-fast",
		surface:   "cli",
		lane:      "fast",
		adapter:   "go-test",
		status:    "passed",
		startedAt: "2026-01-02T03:00:00Z",
		endedAt:   "2026-01-02T03:00:04Z",
		check:     evidence.CheckResult{ID: "unit", Type: "unit", Status: "passed", Expected: "tests pass", Actual: "tests passed"},
		artifacts: []artifactSpec{{kind: "stdout", name: "stdout.log", body: "ok 12 tests\n", publishable: true, redaction: "text_redacted_and_scanned"}},
	})
	writeManifest(t, root, runDir, manifestSpec{
		journey:   "gui-checkout",
		surface:   "frontend",
		lane:      "gui-explore",
		adapter:   "gui-explore",
		status:    "failed",
		startedAt: "2026-01-02T03:00:05Z",
		endedAt:   "2026-01-02T03:00:20Z",
		check: evidence.CheckResult{
			ID: "checkout-redirects", Type: "browser", Status: "failed",
			Expected: "expired session redirected", Actual: "stayed on checkout",
			FailureSummary: "expired sessions are not redirected",
		},
		artifacts: []artifactSpec{
			{kind: "console_summary", name: "console.json", body: `{"errors":1,"token":"sk-live-abcdefghijklmnopqrst"}`, publishable: true, redaction: "text_redacted_and_scanned"},
			{kind: "screenshot_quarantine_ref", name: "shot.json", body: `{"ref":"local-only"}`, publishable: false, redaction: "local_only_quarantine_ref"},
			{kind: "video_quarantine_ref", name: "clip.webm", body: "binary", publishable: true, redaction: "local_only_quarantine_ref"},
		},
	})

	runIndexPath := filepath.Join(runDir, RunIndexFile)
	writeJSON(t, runIndexPath, qarun.Index{
		SchemaVersion: qarun.RunIndexSchemaVersion,
		RunID:         fixtureRunID,
		Workspace:     qarun.WorkspaceRef{WorkspaceID: "fixture", RepoID: "fixture-repo", RepoRoot: "."},
		Status:        "failed",
		StartedAt:     "2026-01-02T03:00:00Z",
		EndedAt:       "2026-01-02T03:00:20Z",
		Profile:       "local",
		Lane:          "gui-explore",
		ManifestPaths: []string{
			manifestRef("cli-fast"),
			manifestRef("gui-checkout"),
		},
		Checks: []qarun.IndexCheck{
			{ID: "unit", JourneyID: "cli-fast", Adapter: "go-test", Status: "passed"},
			{ID: "checkout-redirects", JourneyID: "gui-checkout", Adapter: "gui-explore", Status: "failed", FailureSummary: "expired sessions are not redirected"},
		},
		AdapterResults: []qarun.AdapterResult{
			{Adapter: "go-test", JourneyID: "cli-fast", Status: "passed", QAMESHManifestPath: manifestRef("cli-fast")},
			{Adapter: "gui-explore", JourneyID: "gui-checkout", Status: "failed", QAMESHManifestPath: manifestRef("gui-checkout"), RepairPromptAvailable: true, FailureSummary: "expired sessions are not redirected"},
		},
		SetupGaps:           []qarun.SetupGap{{Adapter: "playwright", JourneyID: "mobile-smoke", Reason: "journey pack missing"}},
		FeedbackBundlePaths: []string{filepath.ToSlash(filepath.Join(".autopus", "qa", "runs", fixtureRunID, "feedback", "bundle.json"))},
		RedactionStatus:     qarun.RedactionStatus{Status: "passed"},
		SourceRefs:          []string{"qamesh://source/fixture-repo/specs/SPEC-QAMESH-003"},
	})
	return fixture{root: root, runIndexPath: runIndexPath, runDir: runDir}
}

func manifestRef(journey string) string {
	return filepath.ToSlash(filepath.Join(".autopus", "qa", "runs", fixtureRunID, journey, "manifest.json"))
}

// writeReleaseIndex adds a release index so the gate matrix section is exercised.
func writeReleaseIndex(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, ".autopus", "qa", "releases", fixtureReleaseID, ReleaseIndexFile)
	writeJSON(t, path, qarelease.Index{
		SchemaVersion:   qarelease.IndexSchemaVersion,
		ReleaseID:       fixtureReleaseID,
		Profile:         "prelaunch",
		StartedAt:       "2026-01-02T03:00:00Z",
		EndedAt:         "2026-01-02T03:05:00Z",
		Status:          qarelease.GateStatusBlocked,
		SelectedLanes:   []string{"fast", "gui-explore"},
		RedactionStatus: qarelease.RedactionClean,
		LaneRows: []qarelease.LaneRow{
			{Lane: "fast", LanePolicy: qarelease.LanePolicyMust, Status: qarelease.LaneStatusPassed, LaneVerdict: qarelease.LaneVerdictPass, Severity: qarelease.SeverityNone, SetupGapClass: qarelease.SetupGapNone, OwnerSpec: "SPEC-QAMESH-002"},
			{Lane: "gui-explore", LanePolicy: qarelease.LanePolicyMust, Status: qarelease.LaneStatusFailed, LaneVerdict: qarelease.LaneVerdictBlock, Severity: qarelease.SeverityHigh, SetupGapClass: qarelease.SetupGapNone, OwnerSpec: "SPEC-QAMESH-003", FailedJourneyID: "gui-checkout", FailureSummary: "expired sessions are not redirected"},
		},
		SetupGaps: []qarelease.SetupGapRow{{Lane: "canary-explicit", SetupGapClass: qarelease.SetupGapMissingJourneyPack, Reason: "no explicit canary journey pack"}},
		Blockers:  []qarelease.Blocker{{Lane: "gui-explore", Reason: "must lane failed"}},
	})
	return path
}

type artifactSpec struct {
	kind        string
	name        string
	body        string
	publishable bool
	redaction   string
}

type manifestSpec struct {
	journey   string
	surface   string
	lane      string
	adapter   string
	status    string
	startedAt string
	endedAt   string
	// retentionClass defaults to local-redacted; a capture journey that kept raw
	// media locally declares its own class instead.
	retentionClass string
	check          evidence.CheckResult
	artifacts      []artifactSpec
}

func writeManifest(t *testing.T, root, runDir string, spec manifestSpec) {
	t.Helper()
	dir := filepath.Join(runDir, spec.journey)
	refs := make([]evidence.ArtifactRef, 0, len(spec.artifacts))
	for _, artifact := range spec.artifacts {
		rel := filepath.ToSlash(filepath.Join("artifacts", artifact.kind, artifact.name))
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), artifact.body)
		refs = append(refs, evidence.ArtifactRef{Kind: artifact.kind, Path: rel, Publishable: artifact.publishable, Redaction: artifact.redaction})
	}
	writeJSON(t, filepath.Join(dir, "manifest.json"), evidence.Manifest{
		SchemaVersion:   evidence.SchemaVersionV2,
		QAResultID:      "qa-" + spec.journey,
		Surface:         spec.surface,
		Lane:            spec.lane,
		ScenarioRef:     spec.journey,
		Runner:          evidence.Runner{Name: spec.adapter, Command: "npm exec playwright test"},
		Status:          spec.status,
		StartedAt:       spec.startedAt,
		EndedAt:         spec.endedAt,
		DurationMS:      spanMS(spec.startedAt, spec.endedAt),
		Artifacts:       refs,
		OracleResults:   evidence.OracleResults{Checks: []evidence.CheckResult{spec.check}},
		RedactionStatus: evidence.RedactionStatus{Status: "passed"},
		SourceRefs: evidence.SourceRefs{
			SourceSpec:     "SPEC-QAMESH-003",
			AcceptanceRefs: []string{"AC-001"},
			JourneyID:      spec.journey,
			StepID:         "step-1",
			Adapter:        spec.adapter,
		},
		RetentionClass:      firstNonEmpty(spec.retentionClass, "local-redacted"),
		ReproductionCommand: "auto qa run --journey " + spec.journey,
	})
	_ = root
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	require.NoError(t, err)
	writeFile(t, path, string(body))
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}
