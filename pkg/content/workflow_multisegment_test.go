// S13 oracle for SPEC-HARNESS-WORKFLOW-STABILITY-001: the route_team generator
// must interpose a deterministic dispatcher barrier after gate_build_test, after
// the coverage-gated testing phase, and after the review phase, yielding >= 4
// segment guards where testing, review, and release_hygiene land in DIFFERENT
// guards.
package content

import (
	"strings"
	"testing"
)

// assertTeamMultiSegment validates S13: the route_team generator interposes a
// deterministic dispatcher barrier after gate_build_test, after the coverage-
// gated testing phase, and after the review phase. This yields >= 4 segment
// guards (A..D) where testing, review, and release_hygiene land in DIFFERENT
// guards so the dispatcher can run the coverage gate after testing and the
// review barrier after review.
func assertTeamMultiSegment(t *testing.T, js, name string) {
	t.Helper()

	// args normalization + SEGMENT preamble (same runtime contract as route_a).
	if !strings.Contains(js, "const ARGV = (typeof args === 'string')") || !strings.Contains(js, "JSON.parse(args)") {
		t.Errorf("[%s] missing ARGV string-args normalization preamble", name)
	}
	if !strings.Contains(js, "const SEGMENT = (ARGV && ARGV.segment) || 'A'") {
		t.Errorf("[%s] missing SEGMENT preamble", name)
	}

	// At least 4 segment guards (A,B,C,D) for the 8-phase schema.
	for _, label := range []string{"A", "B", "C", "D"} {
		guard := "if (SEGMENT === '" + label + "')"
		if count := strings.Count(js, guard); count != 1 {
			t.Errorf("[%s] expected exactly 1 segment-%s guard, got %d", name, label, count)
		}
	}

	// Segment guard ordering: A < B < C < D in source order.
	idxA := strings.Index(js, "if (SEGMENT === 'A')")
	idxB := strings.Index(js, "if (SEGMENT === 'B')")
	idxC := strings.Index(js, "if (SEGMENT === 'C')")
	idxD := strings.Index(js, "if (SEGMENT === 'D')")
	if !(idxA >= 0 && idxA < idxB && idxB < idxC && idxC < idxD) {
		t.Errorf("[%s] segment guards not in A<B<C<D order: A=%d B=%d C=%d D=%d", name, idxA, idxB, idxC, idxD)
		return
	}

	// review lives in segment C, release_hygiene in segment D — they are NOT in
	// the same guard, so the review barrier can interpose between them. Concrete
	// oracle: SEGMENT === 'C' precedes phase('review'), and SEGMENT === 'D'
	// precedes phase('release_hygiene'), with review preceding release_hygiene.
	idxReview := strings.Index(js, "phase('review')")
	idxRelease := strings.Index(js, "phase('release_hygiene')")
	idxTesting := strings.Index(js, "phase('testing')")
	if idxReview < 0 || idxRelease < 0 || idxTesting < 0 {
		t.Errorf("[%s] missing testing/review/release_hygiene phase markers", name)
		return
	}
	if !(idxC < idxReview && idxReview < idxD) {
		t.Errorf("[%s] phase('review') must sit inside segment C (after C guard, before D guard): C=%d review=%d D=%d", name, idxC, idxReview, idxD)
	}
	if !(idxD < idxRelease) {
		t.Errorf("[%s] phase('release_hygiene') must sit inside segment D (after D guard): D=%d release=%d", name, idxD, idxRelease)
	}
	// testing must be in segment B (after B guard, before C guard) so the
	// coverage gate interposes between testing and review.
	if !(idxB < idxTesting && idxTesting < idxC) {
		t.Errorf("[%s] phase('testing') must sit inside segment B (after B guard, before C guard): B=%d testing=%d C=%d", name, idxB, idxTesting, idxC)
	}
}
