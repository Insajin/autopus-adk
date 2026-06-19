package workflow

import "testing"

// S9: every derived failure input maps to exactly one of the four classes and
// no known failure is left unclassified.
func TestClassify_TotalityOverKnownKinds(t *testing.T) {
	want := map[FailureKind]FallbackClass{
		FailureNonClaudePlatform: FallbackFailFast,
		FailureDoctorFail:        FallbackFailFast,
		FailureParityDrift:       FallbackFailClosed,
		FailureExecutionAbort:    FallbackResumable,
		FailureAPIUnavailable:    FallbackExplicit,
	}

	valid := map[FallbackClass]bool{
		FallbackFailFast:   true,
		FallbackFailClosed: true,
		FallbackResumable:  true,
		FallbackExplicit:   true,
	}

	kinds := KnownFailureKinds()
	if len(kinds) != len(want) {
		t.Fatalf("KnownFailureKinds count = %d, want %d", len(kinds), len(want))
	}

	for _, k := range kinds {
		class, ok := Classify(k)
		if !ok {
			t.Fatalf("known failure %q is unclassified (silent opt-out forbidden)", k)
		}
		if !valid[class] {
			t.Fatalf("failure %q mapped to invalid class %q", k, class)
		}
		if class != want[k] {
			t.Fatalf("failure %q -> %q, want %q", k, class, want[k])
		}
	}
}

// S9: an unknown failure kind is reported as unclassified (ok=false) so callers
// can never silently swallow it.
func TestClassify_UnknownKindIsNotSilent(t *testing.T) {
	if _, ok := Classify(FailureKind("totally_unknown")); ok {
		t.Fatal("unknown failure kind must return ok=false")
	}
}
