package releasereadiness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/adapter"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// AC-QAMESH11-001/005: analyze present surfaces and assert no write/execution
// before approval; the journeys dir is byte-identical and phase is diff_presented.
func TestOrchestrate_NoApprove_PresentsDiffWithoutSideEffects(t *testing.T) {
	root := t.TempDir()
	webSignals(t, root)
	desktopSignals(t, root)
	mobileSignals(t, root)
	// Seed an existing journeys dir with one pack to make "added" non-trivial.
	writePack(t, root, customPack("existing-web", "frontend", "browser-staging", []string{"true"}))
	journeysDir := filepath.Join(root, ".autopus", "qa", "journeys")
	before := snapshotDir(t, journeysDir)

	payload, err := orchestrateWith(Options{ProjectDir: root}, fakeRun("passed"))
	if err != nil {
		t.Fatalf("orchestrate: %v", err)
	}

	wantSurfaces := []string{"web", "desktop", "mobile"}
	if got := payload.AnalyzedSurfaces; !equalStrings(got, wantSurfaces) {
		t.Fatalf("analyzed_surfaces = %v, want %v", got, wantSurfaces)
	}
	if payload.Phase != string(PhaseDiffPresented) {
		t.Fatalf("phase = %q, want diff_presented", payload.Phase)
	}
	if payload.FilesWritten != 0 || payload.LanesExecuted != 0 {
		t.Fatalf("files_written=%d lanes_executed=%d, want 0/0", payload.FilesWritten, payload.LanesExecuted)
	}
	if after := snapshotDir(t, journeysDir); after != before {
		t.Fatalf("journeys dir changed before approval:\nbefore=%q\nafter=%q", before, after)
	}
}

// AC-QAMESH11-011 (Should): declined approval reports phase declined with no
// side effects.
func TestOrchestrate_Decline_PhaseDeclinedNoSideEffects(t *testing.T) {
	root := t.TempDir()
	webSignals(t, root)
	journeysDir := filepath.Join(root, ".autopus", "qa", "journeys")
	_ = os.MkdirAll(journeysDir, 0o755)
	before := snapshotDir(t, journeysDir)

	payload, err := orchestrateWith(Options{ProjectDir: root, Approve: true, Decline: true}, fakeRun("passed"))
	if err != nil {
		t.Fatalf("orchestrate: %v", err)
	}
	if payload.Phase != string(PhaseDeclined) {
		t.Fatalf("phase = %q, want declined", payload.Phase)
	}
	if payload.FilesWritten != 0 || payload.LanesExecuted != 0 {
		t.Fatalf("decline produced side effects: written=%d executed=%d", payload.FilesWritten, payload.LanesExecuted)
	}
	if after := snapshotDir(t, journeysDir); after != before {
		t.Fatalf("declined run mutated journeys dir")
	}
}

// AC-QAMESH11-013: no surfaces yields empty analyzed_surfaces, zero diff counts,
// no execution, and no regenerated claim (phase analyzed).
func TestOrchestrate_NoSurfaces_EmptyDiffNoExecution(t *testing.T) {
	root := t.TempDir()

	payload, err := orchestrateWith(Options{ProjectDir: root}, fakeRun("passed"))
	if err != nil {
		t.Fatalf("orchestrate: %v", err)
	}
	if len(payload.AnalyzedSurfaces) != 0 {
		t.Fatalf("analyzed_surfaces = %v, want empty", payload.AnalyzedSurfaces)
	}
	if payload.Diff.AddedCount != 0 || payload.Diff.ChangedCount != 0 || payload.Diff.RemovedCount != 0 {
		t.Fatalf("diff counts non-zero: %+v", payload.Diff)
	}
	if payload.FilesWritten != 0 || payload.LanesExecuted != 0 {
		t.Fatalf("execution happened with no surfaces")
	}
	if payload.Phase != string(PhaseAnalyzed) {
		t.Fatalf("phase = %q, want analyzed", payload.Phase)
	}
}

// AC-QAMESH11-016: an approved pack declares a mobile surface but no Android/iOS
// signal files exist, so the mobile lane carries reason_code surface_absent, is
// not passed, and the verdict is not a false pass.
func TestOrchestrate_SurfaceAbsent_NotFalsePass(t *testing.T) {
	root := t.TempDir()
	// Only web present; mobile pack declares an absent surface.
	webSignals(t, root)
	rows := dispatchAccepted(
		Options{ProjectDir: root},
		[]journey.Pack{customPack("mobile-pack", "mobile", "mobile-scripted", []string{"true"})},
		[]string{"web"},
		fakeRun("passed"),
	)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.ReasonCode != adapter.ReasonSurfaceAbsent {
		t.Fatalf("reason_code = %q, want surface_absent", row.ReasonCode)
	}
	if row.Status == "passed" {
		t.Fatalf("mobile lane must not be passed when surface absent")
	}
	verdict := aggregateVerdict(rows)
	if verdict.Status == "passed" {
		t.Fatalf("verdict must not be a false pass: %+v", verdict)
	}
}

func snapshotDir(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read dir: %v", err)
	}
	out := ""
	for _, e := range entries {
		body, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			t.Fatalf("read %s: %v", e.Name(), rerr)
		}
		out += e.Name() + ":" + string(body) + "\n"
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
