package releasereadiness

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/adapter"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/insajin/autopus-adk/pkg/qa/release"
)

// AC-QAMESH11-007/008: an approved fixture whose mobile surface is present and
// whose pack requires maestro, with maestro absent from PATH, yields a mobile
// lane setup_gap with reason_code surface_tool_unavailable discovered via
// exec.LookPath (no GNU timeout wrapper), not passed, and a non-passing verdict.
func TestDispatch_MissingMaestro_SetupGapNotPass(t *testing.T) {
	root := t.TempDir()
	mobileSignals(t, root)
	// PATH with no maestro binary so exec.LookPath("maestro") fails.
	withPATH(t, filepath.Join(root, "emptybin"))

	pack := journey.Pack{
		ID:      "mobile-scripted-maestro",
		Surface: "mobile",
		Lanes:   []string{"mobile-scripted"},
		Adapter: journey.AdapterRef{ID: "maestro-scripted"},
	}
	row := dispatchLane(Options{ProjectDir: root, Approve: true}, pack, []string{"mobile"}, realRun)

	if row.ReasonCode != adapter.ReasonSurfaceToolUnavailable {
		t.Fatalf("reason_code = %q, want surface_tool_unavailable", row.ReasonCode)
	}
	if row.Status != statusSetupGap {
		t.Fatalf("status = %q, want setup_gap", row.Status)
	}
	if row.Status == "passed" {
		t.Fatalf("setup-gap lane must not be passed")
	}
	verdict := aggregateVerdict([]LaneRow{row})
	if verdict.Status == string(release.GateStatusPassed) {
		t.Fatalf("verdict must not pass with a tool-unavailable mobile lane: %+v", verdict)
	}
}

// AC-QAMESH11-009: an approved fixture with a browser-staging lane pack that
// exits 0 and a desktop-native lane pack that exits 1 yields passed/failed
// respectively, a non-passing verdict, and deterministic_authority true. Runs
// end-to-end through the real qarun.Execute with deterministic argv.
func TestDispatch_ExitDerivedVerdict_Deterministic(t *testing.T) {
	root := t.TempDir()
	webSignals(t, root)
	desktopSignals(t, root)

	webPack := customPack("good-web", "frontend", "browser-staging", []string{"true"})
	desktopPack := customPack("bad-desktop", "desktop", "desktop-native", []string{"false"})
	writePack(t, root, webPack)
	writePack(t, root, desktopPack)

	rows := dispatchAccepted(
		Options{ProjectDir: root, Approve: true},
		[]journey.Pack{webPack, desktopPack},
		[]string{"web", "desktop"},
		realRun,
	)

	byLane := map[string]LaneRow{}
	for _, r := range rows {
		byLane[r.Lane] = r
	}
	if got := byLane["browser-staging"].Status; got != string(release.LaneStatusPassed) {
		t.Fatalf("browser-staging status = %q, want passed", got)
	}
	if got := byLane["desktop-native"].Status; got != string(release.LaneStatusFailed) {
		t.Fatalf("desktop-native status = %q, want failed", got)
	}
	verdict := aggregateVerdict(rows)
	if verdict.Status == string(release.GateStatusPassed) {
		t.Fatalf("verdict must not pass when desktop lane failed: %+v", verdict)
	}
	if !verdict.DeterministicAuthority {
		t.Fatalf("verdict.deterministic_authority must be true")
	}
}

// AC-QAMESH11-015: prove INV-Q11-008 at the dispatch layer. The mobile-scripted
// lane is in the dispatched lane set when mobile is present and the tool probe
// is satisfied; qarun Result.Status exit-0/exit-1 maps to passed/failed and
// aggregates deterministically; and release.ReleaseLanes() is byte-identical to
// the documented slice.
//
// Mechanism note: the real mobile readiness gate (mobile.Assess /
// applyMobileScriptedLane) blocks execution unless full device readiness config
// is present, and the device runner is unexported, so a full hermetic
// qarun.Execute(mobile-scripted) on-device path is not provable here. Per the
// SPEC's documented fallback, INV-Q11-008 is proven at the dispatch layer:
// lane-set inclusion, exit-derived mapping, deterministic aggregation, and
// ReleaseLanes() invariance.
func TestDispatch_MobileScripted_DispatchInclusionAndMapping(t *testing.T) {
	root := t.TempDir()
	mobileSignals(t, root)
	// Satisfy the maestro tool probe hermetically with a fake maestro on PATH.
	bin := filepath.Join(root, "bin")
	fakeBin(t, bin, "maestro", 0)
	withPATH(t, bin)

	pack := journey.Pack{
		ID:      "mobile-scripted-maestro",
		Surface: "mobile",
		Lanes:   []string{"mobile-scripted"},
		Adapter: journey.AdapterRef{ID: "maestro-scripted"},
	}

	// (a) lane-set inclusion: the dispatched lane is mobile-scripted.
	if lane := laneForPack(pack); lane != "mobile-scripted" {
		t.Fatalf("laneForPack = %q, want mobile-scripted", lane)
	}

	// (b) exit-derived mapping yields passed/failed for status 0/1 inputs and
	// aggregates deterministically. The maestro probe passes (fake maestro), so
	// dispatch reaches the run seam; we drive deterministic statuses there.
	passRow := dispatchLane(Options{ProjectDir: root, Approve: true}, pack, []string{"mobile"}, fakeRun("passed"))
	if passRow.Status != string(release.LaneStatusPassed) {
		t.Fatalf("exit-0 mobile lane = %q, want passed", passRow.Status)
	}
	failRow := dispatchLane(Options{ProjectDir: root, Approve: true}, pack, []string{"mobile"}, fakeRun("failed"))
	if failRow.Status != string(release.LaneStatusFailed) {
		t.Fatalf("exit-1 mobile lane = %q, want failed", failRow.Status)
	}
	if v := aggregateVerdict([]LaneRow{passRow}); v.Status != string(release.GateStatusPassed) || !v.DeterministicAuthority {
		t.Fatalf("passing mobile lane verdict = %+v, want passed/deterministic", v)
	}
	if v := aggregateVerdict([]LaneRow{failRow}); v.Status == string(release.GateStatusPassed) {
		t.Fatalf("failed mobile lane verdict must not pass: %+v", v)
	}

	// (c) ReleaseLanes() is byte-identical to the documented slice and contains
	// no mobile-scripted entry (release-readiness uses a separate local set).
	want := []string{"fast", "browser-staging", "desktop-native", "gui-explore", "mobile-readiness", "canary-explicit", "evidence-dashboard"}
	got := release.ReleaseLanes()
	if !equalStrings(got, want) {
		t.Fatalf("ReleaseLanes() = %v, want %v", got, want)
	}
	for _, lane := range got {
		if lane == "mobile-scripted" {
			t.Fatalf("ReleaseLanes() must not contain mobile-scripted")
		}
	}
}

// AC-QAMESH11-010 (evidence half): after approval the published evidence summary
// declares schema_version qamesh.evidence.v2, redacts a token-like secret and an
// absolute local path to placeholders, and passes AssertSafeText.
func TestEvidenceSummary_V2SchemaAndRedaction(t *testing.T) {
	payload := Payload{
		AnalyzedSurfaces: []string{"web"},
		Phase:            string(PhaseExecuted),
		FilesWritten:     1,
		LanesExecuted:    1,
		LaneRows: []LaneRow{{
			Lane:           "browser-staging",
			Status:         "failed",
			FailureSummary: "leaked sk-proj-abcdefghijklmnopqrstuvwxyz at /Users/alice/secret/log.txt",
		}},
		Verdict: Verdict{Status: "blocked", DeterministicAuthority: true},
	}
	summary, err := buildEvidenceSummary(payload)
	if err != nil {
		t.Fatalf("buildEvidenceSummary: %v", err)
	}
	if !strings.Contains(summary, "\"schema_version\":\"qamesh.evidence.v2\"") {
		t.Fatalf("summary missing v2 schema_version: %s", summary)
	}
	if strings.Contains(summary, "sk-proj-abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("raw secret leaked into evidence summary")
	}
	if strings.Contains(summary, "/Users/alice/secret") {
		t.Fatalf("raw absolute local path leaked into evidence summary")
	}
	if !strings.Contains(summary, "[REDACTED_SECRET]") || !strings.Contains(summary, "[REDACTED_USER]") {
		t.Fatalf("expected redaction placeholders in summary: %s", summary)
	}
}
