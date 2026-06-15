package regen

import (
	"encoding/json"
	"strings"
	"testing"
)

// BuildResult over a web+mobile fixture synthesizes one accepted pack per
// surface and reports both as added (no existing packs). Exercises the full
// Unit 1 pipeline plus AcceptedPacks.
func TestBuildResult_WebAndMobile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "playwright.config.ts", "export default {}")
	writeFile(t, dir, "android/app/src/main/AndroidManifest.xml", "<manifest/>")

	result, err := BuildResult(dir)
	if err != nil {
		t.Fatalf("BuildResult: %v", err)
	}
	if got := result.Surfaces; len(got) != 2 || got[0] != SurfaceWeb || got[1] != SurfaceMobile {
		t.Fatalf("surfaces = %v, want [web mobile]", got)
	}
	accepted := result.AcceptedPacks()
	if len(accepted) != 2 {
		t.Fatalf("accepted packs = %d, want 2", len(accepted))
	}
	if result.Diff.AddedCount != 2 {
		t.Fatalf("AddedCount = %d, want 2", result.Diff.AddedCount)
	}
	ids := map[string]bool{}
	for _, e := range result.Diff.Added {
		ids[e.JourneyID] = true
	}
	if !ids[webStarterID] || !ids[mobileStarterID] {
		t.Fatalf("added ids = %v, want %s and %s", ids, webStarterID, mobileStarterID)
	}
}

// AnalyzeProject redacts untrusted Cobra Short/Use text before it enters the
// returned flows. We embed a token-shaped secret in a leaf command's Short.
func TestAnalyzeProject_RedactsExtractedFlowText(t *testing.T) {
	dir := t.TempDir()
	cobraSrc := `package main

import "github.com/spf13/cobra"

var leafCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy with token sk-proj-ABCDEFGHIJKLMNOP1234567890",
	RunE:  func(cmd *cobra.Command, args []string) error { return nil },
}
`
	writeFile(t, dir, "main.go", cobraSrc)

	analysis, err := AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	body, _ := json.Marshal(analysis.CLIFlows)
	if strings.Contains(string(body), "sk-proj-ABCDEFGHIJKLMNOP1234567890") {
		t.Fatalf("raw secret leaked into extracted flows: %s", body)
	}
}

// Synthesize over a desktop-only fixture produces a single accepted desktop pack.
func TestSynthesize_DesktopSurface(t *testing.T) {
	dir := t.TempDir()
	packs := Synthesize(dir, []string{SurfaceDesktop})
	if len(packs) != 1 {
		t.Fatalf("synthesized = %d, want 1", len(packs))
	}
	if packs[0].Excluded {
		t.Fatalf("desktop starter pack unexpectedly excluded: %s", packs[0].Reason)
	}
	if packs[0].Pack.ID != desktopStarterID {
		t.Fatalf("pack id = %q, want %q", packs[0].Pack.ID, desktopStarterID)
	}
}

// AssertDiffSafe surfaces an error when an unredacted secret remains in the diff.
func TestAssertDiffSafe_DetectsUnsafe(t *testing.T) {
	diff := Diff{
		Added: []DiffEntry{{
			JourneyID: "x",
			Category:  "added",
			ChangedFields: []FieldChange{
				{Field: "command.argv", Before: "", After: "[run --token=sk-proj-ABCDEFGHIJKLMNOP1234567890]"},
			},
		}},
	}
	if err := AssertDiffSafe(diff); err == nil {
		t.Fatal("AssertDiffSafe must reject an unredacted secret")
	}
}
