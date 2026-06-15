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
