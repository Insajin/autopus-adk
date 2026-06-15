package releasereadiness

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/insajin/autopus-adk/pkg/qa/release"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

// AC-QAMESH11-005/006: the public Orchestrate entry point (not the orchestrateWith
// seam) runs over a hermetic web fixture without --approve and presents the
// redacted diff with no write and no execution. This exercises the real
// qarun.Execute wiring through the only entry point.
func TestOrchestrate_PublicEntry_NoApproveNoSideEffects(t *testing.T) {
	root := t.TempDir()
	webSignals(t, root)
	// An empty journeys dir makes the synthesized web pack an "added" entry, so
	// the diff is non-trivial while nothing is persisted before approval.
	writeSignal(t, root, ".autopus/qa/journeys/.keep", "")

	payload, err := Orchestrate(Options{ProjectDir: root, Approve: false})
	if err != nil {
		t.Fatalf("Orchestrate: %v", err)
	}
	if payload.Phase != string(PhaseDiffPresented) {
		t.Fatalf("phase = %q, want diff_presented", payload.Phase)
	}
	if payload.FilesWritten != 0 {
		t.Fatalf("files_written = %d, want 0", payload.FilesWritten)
	}
	if payload.LanesExecuted != 0 {
		t.Fatalf("lanes_executed = %d, want 0", payload.LanesExecuted)
	}
	if payload.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q, want %q", payload.SchemaVersion, SchemaVersion)
	}
	if !equalStrings(payload.AnalyzedSurfaces, []string{"web"}) {
		t.Fatalf("analyzed_surfaces = %v, want [web]", payload.AnalyzedSurfaces)
	}
}

// mapRunStatusToLane covers every branch: the explicit warning->warn mapping, the
// passed/failed/blocked passthrough, and the fail-closed default for the empty
// string and any setup-gap-like token.
func TestMapRunStatusToLane_AllBranches(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want release.LaneStatus
	}{
		{"warning maps to warn", "warning", release.LaneStatusWarn},
		{"passed passthrough", "passed", release.LaneStatusPassed},
		{"failed passthrough", "failed", release.LaneStatusFailed},
		{"blocked passthrough", "blocked", release.LaneStatusBlocked},
		{"warn alias fails closed", "warn", release.LaneStatusBlocked},
		{"setup-gap fails closed", "setup-gap", release.LaneStatusBlocked},
		{"empty fails closed", "", release.LaneStatusBlocked},
		{"unknown fails closed", "mystery", release.LaneStatusBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapRunStatusToLane(tc.in); got != tc.want {
				t.Fatalf("mapRunStatusToLane(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// laneForPack covers the surface-derived fallback for every canonical surface
// plus the unknown-surface passthrough, exercised by packs that declare no lane.
func TestLaneForPack_SurfaceFallback(t *testing.T) {
	cases := []struct {
		name    string
		surface string
		want    string
	}{
		{"frontend -> browser-staging", "frontend", "browser-staging"},
		{"web -> browser-staging", "web", "browser-staging"},
		{"desktop -> desktop-native", "desktop", "desktop-native"},
		{"mobile -> mobile-scripted", "mobile", "mobile-scripted"},
		{"unknown passes through surface token", "embedded", "embedded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pack := journey.Pack{ID: "p", Surface: tc.surface}
			if got := laneForPack(pack); got != tc.want {
				t.Fatalf("laneForPack(surface=%q) = %q, want %q", tc.surface, got, tc.want)
			}
		})
	}
}

// laneForPack honors a declared lane over the surface fallback.
func TestLaneForPack_DeclaredLaneWins(t *testing.T) {
	pack := journey.Pack{ID: "p", Surface: "mobile", Lanes: []string{"canary-explicit"}}
	if got := laneForPack(pack); got != "canary-explicit" {
		t.Fatalf("laneForPack = %q, want declared canary-explicit", got)
	}
	// An empty declared lane falls through to the surface fallback.
	blank := journey.Pack{ID: "p", Surface: "desktop", Lanes: []string{""}}
	if got := laneForPack(blank); got != "desktop-native" {
		t.Fatalf("laneForPack(empty lane) = %q, want desktop-native fallback", got)
	}
}

// normalizeSurface covers each alias and the unknown passthrough.
func TestNormalizeSurface_Aliases(t *testing.T) {
	cases := map[string]string{
		"frontend": surfaceWeb,
		"web":      surfaceWeb,
		"desktop":  surfaceDesktop,
		"mobile":   surfaceMobile,
		"unknown":  "unknown",
	}
	for in, want := range cases {
		if got := normalizeSurface(in); got != want {
			t.Fatalf("normalizeSurface(%q) = %q, want %q", in, got, want)
		}
	}
}

// missingBinary returns no-missing for an unknown adapter (not in the registry)
// and for an adapter that declares no required binaries.
func TestMissingBinary_UnknownAndNoBinaries(t *testing.T) {
	if bin, missing := missingBinary("does-not-exist"); missing || bin != "" {
		t.Fatalf("unknown adapter: got (%q,%v), want (\"\",false)", bin, missing)
	}
	// custom-command declares no required binaries.
	if bin, missing := missingBinary("custom-command"); missing || bin != "" {
		t.Fatalf("custom-command: got (%q,%v), want (\"\",false)", bin, missing)
	}
}

// laneRowFromRun: a runner error with a non-failing status fails closed to
// blocked so a run failure never reads as a pass.
func TestLaneRowFromRun_RunnerErrorBlocks(t *testing.T) {
	row := LaneRow{Lane: "browser-staging", DeterministicAuthority: true}
	result := qarun.Result{Status: "passed"}
	got := laneRowFromRun(row, result, errStub("boom"))
	if got.Status != string(release.LaneStatusBlocked) {
		t.Fatalf("status = %q, want blocked when runner errored", got.Status)
	}

	// An error alongside an already-failed status keeps failed (no double-demote).
	failed := laneRowFromRun(row, qarun.Result{Status: "failed"}, errStub("boom"))
	if failed.Status != string(release.LaneStatusFailed) {
		t.Fatalf("status = %q, want failed preserved", failed.Status)
	}

	// A clean pass with no error stays passed and carries no failure summary.
	pass := laneRowFromRun(row, qarun.Result{Status: "passed"}, nil)
	if pass.Status != string(release.LaneStatusPassed) || pass.FailureSummary != "" {
		t.Fatalf("clean pass = %+v, want passed with empty summary", pass)
	}

	// A failed run surfaces the first adapter failure summary.
	withSummary := laneRowFromRun(row, qarun.Result{
		Status:         "failed",
		AdapterResults: []qarun.AdapterResult{{FailureSummary: "assertion failed"}},
	}, nil)
	if withSummary.FailureSummary != "assertion failed" {
		t.Fatalf("failure_summary = %q, want assertion failed", withSummary.FailureSummary)
	}
}

// buildEvidenceSummary over a minimal approved payload with no lane rows still
// declares the v2 schema and passes the safe-text gate (no-rows branch).
func TestBuildEvidenceSummary_NoLaneRows(t *testing.T) {
	payload := Payload{
		AnalyzedSurfaces: []string{"web"},
		Phase:            string(PhaseExecuted),
		FilesWritten:     0,
		LanesExecuted:    0,
		LaneRows:         []LaneRow{},
		Verdict:          Verdict{Status: "passed", DeterministicAuthority: true},
	}
	summary, err := buildEvidenceSummary(payload)
	if err != nil {
		t.Fatalf("buildEvidenceSummary: %v", err)
	}
	if summary == "" {
		t.Fatalf("expected non-empty summary")
	}
	if want := "\"schema_version\":\"qamesh.evidence.v2\""; !strings.Contains(summary, want) {
		t.Fatalf("summary missing v2 schema_version: %s", summary)
	}
	if !strings.Contains(summary, "\"lane_rows\":[]") {
		t.Fatalf("expected empty lane_rows array: %s", summary)
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }
