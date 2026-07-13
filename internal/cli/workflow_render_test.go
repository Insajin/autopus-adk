package cli_test

import (
	"strings"
	"testing"
)

// renderLines runs `workflow render ...` with the standard hermetic harness and
// returns stdout split into lines (panicking via t.Fatalf on Execute error).
func renderLines(t *testing.T, args ...string) []string {
	t.Helper()
	out, err := runWorkflow(nil, nil, append([]string{"workflow", "render"}, args...)...)
	if err != nil {
		t.Fatalf("render %v failed: %v\n%s", args, err, out)
	}
	return strings.Split(out, "\n")
}

// findLine returns the first line that has the given prefix, or "" if absent.
func findLine(lines []string, prefix string) string {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

// S18: --route team lists the 8 team phases in order; no --route lists the 4
// route_a phases in order.
func TestWorkflowRender_RouteSelectsPhaseOrder(t *testing.T) {
	t.Parallel()

	teamOrder := findLine(renderLines(t, "--route", "team", "--dry-run"), "phase order:")
	wantTeam := []string{
		"planning", "test_scaffold", "implementation", "gate_build_test",
		"annotation", "testing", "review", "release_hygiene",
	}
	prev := -1
	for _, ph := range wantTeam {
		idx := strings.Index(teamOrder, ph)
		if idx < 0 {
			t.Fatalf("team phase order missing %q: %q", ph, teamOrder)
		}
		if idx <= prev {
			t.Fatalf("team phase %q out of order in %q", ph, teamOrder)
		}
		prev = idx
	}

	aOrder := findLine(renderLines(t, "--dry-run"), "phase order:")
	wantA := []string{"planning", "implementation", "gate_build_test", "release_hygiene"}
	prev = -1
	for _, ph := range wantA {
		idx := strings.Index(aOrder, ph)
		if idx < 0 {
			t.Fatalf("route_a phase order missing %q: %q", ph, aOrder)
		}
		if idx <= prev {
			t.Fatalf("route_a phase %q out of order in %q", ph, aOrder)
		}
		prev = idx
	}
	// route_a must NOT contain team-only phases.
	for _, ph := range []string{"test_scaffold", "annotation", "testing", "review"} {
		if strings.Contains(aOrder, ph) {
			t.Fatalf("route_a phase order unexpectedly contains %q: %q", ph, aOrder)
		}
	}
}

// S9: team baseline (no --quality) renders schema baseline model/effort/depth.
func TestWorkflowRender_TeamBaselinePhases(t *testing.T) {
	t.Parallel()

	lines := renderLines(t, "--route", "team", "--dry-run")

	planning := findLine(lines, "phase planning:")
	if !strings.Contains(planning, "model=claude-opus-4-8") || !strings.Contains(planning, "effort=medium") {
		t.Fatalf("planning baseline = %q, want opus-4-8 + medium", planning)
	}

	impl := findLine(lines, "phase implementation:")
	if !strings.Contains(impl, "fan_out_cap=5") {
		t.Fatalf("implementation baseline = %q, want fan_out_cap=5", impl)
	}

	review := findLine(lines, "phase review:")
	if !strings.Contains(review, "verify_votes=1") || !strings.Contains(review, "synthesis=false") {
		t.Fatalf("review baseline = %q, want verify_votes=1 synthesis=false", review)
	}
}

// S16: --quality overlays per-phase model/effort/depth deterministically.
func TestWorkflowRender_TeamQualityOverlay(t *testing.T) {
	t.Parallel()

	ultra := renderLines(t, "--route", "team", "--quality", "ultra")
	implU := findLine(ultra, "phase implementation:")
	if !strings.Contains(implU, "model=claude-opus-4-8") || !strings.Contains(implU, "effort=max") {
		t.Fatalf("ultra implementation = %q, want opus-4-8 + max", implU)
	}
	reviewU := findLine(ultra, "phase review:")
	if !strings.Contains(reviewU, "verify_votes=3") || !strings.Contains(reviewU, "synthesis=true") {
		t.Fatalf("ultra review = %q, want verify_votes=3 synthesis=true", reviewU)
	}

	balanced := renderLines(t, "--route", "team", "--quality", "balanced")
	implB := findLine(balanced, "phase implementation:")
	if !strings.Contains(implB, "model=claude-sonnet-5") || !strings.Contains(implB, "effort=medium") {
		t.Fatalf("balanced implementation = %q, want sonnet-5 + medium", implB)
	}
	reviewB := findLine(balanced, "phase review:")
	if !strings.Contains(reviewB, "verify_votes=1") || !strings.Contains(reviewB, "synthesis=false") {
		t.Fatalf("balanced review = %q, want verify_votes=1 synthesis=false", reviewB)
	}
}

// S15: quality is ephemeral and excluded from the prompt-manifest hash, so the
// hash is identical across quality tiers for the same route.
func TestWorkflowRender_QualityExcludedFromHash(t *testing.T) {
	t.Parallel()

	hashOf := func(lines []string) string {
		l := findLine(lines, "prompt-manifest hash:")
		return strings.TrimSpace(strings.TrimPrefix(l, "prompt-manifest hash:"))
	}

	ultra := hashOf(renderLines(t, "--route", "team", "--quality", "ultra"))
	balanced := hashOf(renderLines(t, "--route", "team", "--quality", "balanced"))
	if ultra == "" {
		t.Fatal("missing prompt-manifest hash line")
	}
	if ultra != balanced {
		t.Fatalf("prompt-manifest hash changed with quality: ultra=%q balanced=%q", ultra, balanced)
	}
}
