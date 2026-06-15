package regen

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// AC-QAMESH11-002: one new web flow with no existing pack, plus an unrelated
// existing pack that still matches a synthesized pack of the same id, yields
// exactly one added entry and nothing else.
func TestComputeDiff_AddedOnly(t *testing.T) {
	newWeb := validWebPack("brand-new-web")
	existingMatch := validDesktopPack("existing-web")

	synthesized := []SynthesizedPack{
		accepted(SurfaceWeb, newWeb),
		accepted(SurfaceDesktop, existingMatch), // identical to existing -> no change
	}
	existing := map[string]journey.Pack{
		"existing-web": existingMatch,
	}

	diff := ComputeDiff(synthesized, existing)

	if diff.AddedCount != 1 {
		t.Fatalf("AddedCount = %d, want 1", diff.AddedCount)
	}
	if diff.Added[0].JourneyID != "brand-new-web" {
		t.Fatalf("added id = %q, want brand-new-web", diff.Added[0].JourneyID)
	}
	if diff.ChangedCount != 0 {
		t.Fatalf("ChangedCount = %d, want 0", diff.ChangedCount)
	}
	if diff.RemovedCount != 0 {
		t.Fatalf("RemovedCount = %d, want 0", diff.RemovedCount)
	}
	for _, e := range append(append([]DiffEntry{}, diff.Added...), append(diff.Changed, diff.Removed...)...) {
		if e.JourneyID == "existing-web" {
			t.Fatalf("existing-web must appear in no category, found in %s", e.Category)
		}
	}
}

// AC-QAMESH11-003: changed argv on browser-staging-playwright plus one added and
// one removed; exact counts, the changed field oracle, and byte-identical
// json.Marshal across two runs.
func TestComputeDiff_AddedChangedRemoved(t *testing.T) {
	existingPlaywright := webStarterPack()
	existingPlaywright.Command.Argv = []string{"npm", "run", "e2e:legacy"}

	synthPlaywright := webStarterPack()
	synthPlaywright.Command.Argv = []string{"npm", "run", "test"}

	synthesized := []SynthesizedPack{
		accepted(SurfaceWeb, synthPlaywright),
		accepted(SurfaceWeb, validWebPack("zeta-new")),
	}
	existing := map[string]journey.Pack{
		"browser-staging-playwright": existingPlaywright,
		"omega-stale":                validDesktopPack("omega-stale"),
	}

	diff := ComputeDiff(synthesized, existing)

	if diff.AddedCount != 1 || diff.ChangedCount != 1 || diff.RemovedCount != 1 {
		t.Fatalf("counts = (%d,%d,%d), want (1,1,1)", diff.AddedCount, diff.ChangedCount, diff.RemovedCount)
	}
	if diff.Added[0].JourneyID != "zeta-new" {
		t.Fatalf("added id = %q, want zeta-new", diff.Added[0].JourneyID)
	}
	if diff.Removed[0].JourneyID != "omega-stale" {
		t.Fatalf("removed id = %q, want omega-stale", diff.Removed[0].JourneyID)
	}
	changed := diff.Changed[0]
	if changed.JourneyID != "browser-staging-playwright" {
		t.Fatalf("changed id = %q, want browser-staging-playwright", changed.JourneyID)
	}
	var argvChange *FieldChange
	for i := range changed.ChangedFields {
		if changed.ChangedFields[i].Field == "command.argv" {
			argvChange = &changed.ChangedFields[i]
		}
	}
	if argvChange == nil {
		t.Fatalf("expected command.argv change, got fields %+v", changed.ChangedFields)
	}
	if argvChange.Before != "[npm run e2e:legacy]" {
		t.Fatalf("before = %q, want [npm run e2e:legacy]", argvChange.Before)
	}
	if argvChange.After != "[npm run test]" {
		t.Fatalf("after = %q, want [npm run test]", argvChange.After)
	}

	first, err := json.Marshal(ComputeDiff(synthesized, existing))
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(ComputeDiff(synthesized, existing))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("diff json not byte-identical across runs:\n%s\n%s", first, second)
	}
}

// AC-QAMESH11-013: a project with no surface signals yields empty surfaces and a
// zero-count diff over no synthesized packs.
func TestComputeDiff_NoSurfaces(t *testing.T) {
	dir := t.TempDir()
	surfaces := PresentSurfaces(dir)
	if len(surfaces) != 0 {
		t.Fatalf("surfaces = %v, want empty", surfaces)
	}
	diff := ComputeDiff(nil, map[string]journey.Pack{})
	if diff.AddedCount != 0 || diff.ChangedCount != 0 || diff.RemovedCount != 0 {
		t.Fatalf("counts = (%d,%d,%d), want all 0", diff.AddedCount, diff.ChangedCount, diff.RemovedCount)
	}
}
