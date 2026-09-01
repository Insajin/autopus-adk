package releasereadiness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/insajin/autopus-adk/pkg/qa/regen"
	"github.com/insajin/autopus-adk/pkg/qa/release"
)

// AC-QAMESH11-009/011 (approved half): an approved run over a web+desktop
// fixture persists the accepted synthesized packs, dispatches one lane per pack,
// reports phase executed with files_written equal to the persisted pack count,
// and aggregates a deterministic verdict. Tool probes for the synthesized npm
// packs are satisfied hermetically with fake node/npm binaries so the lanes run.
func TestOrchestrate_Approve_ExecutesAndCountsFiles(t *testing.T) {
	root := t.TempDir()
	webSignals(t, root)
	desktopSignals(t, root)
	// Fake node/npm so the synthesized playwright/node-script packs pass the
	// tool probe; the commands themselves run hermetically (exit 0 fake).
	bin := filepath.Join(root, "bin")
	fakeBin(t, bin, "node", 0)
	fakeBin(t, bin, "npm", 0)
	withPATH(t, bin)

	payload, err := orchestrateWith(Options{ProjectDir: root, Approve: true}, fakeRun("passed"))
	if err != nil {
		t.Fatalf("orchestrate approve: %v", err)
	}
	if payload.Phase != string(PhaseExecuted) {
		t.Fatalf("phase = %q, want executed", payload.Phase)
	}
	// Two surfaces present -> two synthesized valid packs persisted and dispatched.
	if payload.FilesWritten != 2 {
		t.Fatalf("files_written = %d, want 2", payload.FilesWritten)
	}
	if payload.LanesExecuted != 2 {
		t.Fatalf("lanes_executed = %d, want 2", payload.LanesExecuted)
	}
	if !payload.Verdict.DeterministicAuthority {
		t.Fatalf("verdict.deterministic_authority must be true")
	}
	// All lanes passed via fakeRun -> verdict passes.
	if payload.Verdict.Status != string(release.GateStatusPassed) {
		t.Fatalf("verdict = %q, want passed", payload.Verdict.Status)
	}
	// Evidence summary attached and v2.
	if payload.EvidenceSummary == "" {
		t.Fatalf("expected evidence summary on approved run")
	}
	// Persisted files exist on disk.
	journeysDir := filepath.Join(root, ".autopus", "qa", "journeys")
	if got := snapshotDir(t, journeysDir); got == "" {
		t.Fatalf("no packs persisted under %s", journeysDir)
	}
}

// D6/D25 regression: a Go/CLI project has packs on disk but no analyzable
// surface. Approving must not report an executed, deterministically
// authoritative pass over zero lanes, must not propose discarding the packs
// qa init created, and must leave every one of them byte-identical on disk.
func TestOrchestrate_Approve_NoSurfaces_NoFalseExecutionAndNoPackLoss(t *testing.T) {
	root := t.TempDir()
	writePack(t, root, customPack("go-fast", "frontend", "browser-staging", []string{"true"}))
	writePack(t, root, customPack("canary-explicit", "frontend", "canary-explicit", []string{"true"}))
	journeysDir := filepath.Join(root, ".autopus", "qa", "journeys")
	before := snapshotDir(t, journeysDir)

	payload, err := orchestrateWith(Options{ProjectDir: root, Approve: true}, fakeRun("passed"))
	if err != nil {
		t.Fatalf("orchestrate approve: %v", err)
	}
	if payload.Phase == string(PhaseExecuted) {
		t.Fatalf("phase = executed with lanes_executed=%d", payload.LanesExecuted)
	}
	if payload.LanesExecuted != 0 || payload.FilesWritten != 0 {
		t.Fatalf("lanes_executed=%d files_written=%d, want 0/0", payload.LanesExecuted, payload.FilesWritten)
	}
	if payload.Verdict.Status != VerdictNotEvaluated || payload.Verdict.DeterministicAuthority {
		t.Fatalf("verdict = %+v, want not_evaluated without deterministic authority", payload.Verdict)
	}
	if payload.Diff.UnmatchedCount != 0 {
		t.Fatalf("unmatched = %d, want 0: an empty analysis accounts for no pack", payload.Diff.UnmatchedCount)
	}
	if payload.EvidenceSummary != "" {
		t.Fatalf("evidence summary attached to a run that produced no evidence")
	}
	if after := snapshotDir(t, journeysDir); after != before {
		t.Fatalf("approval mutated the journeys dir:\nbefore=%q\nafter=%q", before, after)
	}
}

// The diff must describe what approval applies. Every id the preview reports as
// added or changed is written; nothing else on disk is touched, so an unmatched
// row can never be read as a pending deletion.
func TestOrchestrate_Approve_DiffMatchesWhatApprovalApplies(t *testing.T) {
	root := t.TempDir()
	webSignals(t, root)
	writePack(t, root, customPack("go-fast", "frontend", "browser-staging", []string{"true"}))
	bin := filepath.Join(root, "bin")
	fakeBin(t, bin, "node", 0)
	fakeBin(t, bin, "npm", 0)
	withPATH(t, bin)

	preview, err := orchestrateWith(Options{ProjectDir: root}, fakeRun("passed"))
	if err != nil {
		t.Fatalf("orchestrate preview: %v", err)
	}
	if preview.Diff.UnmatchedCount != 1 || preview.Diff.Unmatched[0].JourneyID != "go-fast" {
		t.Fatalf("unmatched = %+v, want exactly go-fast", preview.Diff.Unmatched)
	}
	wantWritten := preview.Diff.AddedCount + preview.Diff.ChangedCount

	approved, err := orchestrateWith(Options{ProjectDir: root, Approve: true}, fakeRun("passed"))
	if err != nil {
		t.Fatalf("orchestrate approve: %v", err)
	}
	if approved.FilesWritten != wantWritten {
		t.Fatalf("files_written = %d, want added+changed = %d", approved.FilesWritten, wantWritten)
	}
	if _, err := os.Stat(filepath.Join(root, ".autopus", "qa", "journeys", "go-fast.yaml")); err != nil {
		t.Fatalf("unmatched pack go-fast was not left on disk: %v", err)
	}
	if approved.ApprovalDeletesPacks {
		t.Fatalf("approval_deletes_packs must always be false")
	}

	// Re-approving an already-current project proposes nothing and writes
	// nothing, so files_written keeps agreeing with the diff instead of
	// counting byte-identical rewrites.
	rerun, err := orchestrateWith(Options{ProjectDir: root, Approve: true}, fakeRun("passed"))
	if err != nil {
		t.Fatalf("orchestrate re-approve: %v", err)
	}
	if rerun.Diff.AddedCount != 0 || rerun.Diff.ChangedCount != 0 {
		t.Fatalf("second preview still proposes changes: %+v", rerun.Diff)
	}
	if rerun.FilesWritten != 0 {
		t.Fatalf("files_written = %d on a no-op approval, want 0", rerun.FilesWritten)
	}
	if rerun.LanesExecuted != 1 {
		t.Fatalf("lanes_executed = %d, want the web lane still dispatched", rerun.LanesExecuted)
	}
}

// AC-QAMESH11-012: regeneration where one accepted pack is valid (good-web) and
// one is invalid (bad-mobile) -> ApplyPacks (the function orchestrate feeds with
// AcceptedPacks) persists only the valid pack and excludes the invalid one with
// no partial file. This proves the partial-failure contract at the apply seam
// that orchestrate relies on.
func TestApply_InvalidPackExcluded_NoPartialFile(t *testing.T) {
	root := t.TempDir()
	good := customPack("good-web", "frontend", "browser-staging", []string{"true"})
	// bad-mobile: mobile surface but missing required mobile policy fields ->
	// journey.Validate fails -> excluded by ApplyPacks.
	bad := journey.Pack{
		ID:      "bad-mobile",
		Surface: "mobile",
		Lanes:   []string{"mobile-scripted"},
		Adapter: journey.AdapterRef{ID: "maestro-scripted"},
		Command: journey.Command{Argv: []string{"maestro", "test", "missing.yaml"}, CWD: ".", Timeout: "60s"},
		Checks: []journey.Check{{
			ID: "bad-check", Type: "deterministic", Expected: map[string]any{"exit_code": 0},
		}},
	}

	result, err := regen.ApplyPacks(root, []journey.Pack{good, bad})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(result.Written) != 1 || result.Written[0].JourneyID != "good-web" {
		t.Fatalf("written = %+v, want only good-web", result.Written)
	}
	if len(result.Excluded) != 1 || result.Excluded[0].JourneyID != "bad-mobile" {
		t.Fatalf("excluded = %+v, want only bad-mobile", result.Excluded)
	}
	// No file written for bad-mobile.
	badPath := filepath.Join(root, ".autopus", "qa", "journeys", "bad-mobile.yaml")
	if _, statErr := os.Stat(badPath); statErr == nil {
		t.Fatalf("partial file written for bad-mobile")
	}
}
