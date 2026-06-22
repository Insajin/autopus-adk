package workflow

import "testing"

// fakeProber reports configured availability per primitive and a fixed version.
type fakeProber struct {
	unavailable map[string]bool
	version     string
}

func (f fakeProber) Probe(primitive string) bool {
	return !f.unavailable[primitive]
}

func (f fakeProber) Version() string {
	return f.version
}

func findPrimitive(t *testing.T, r CapabilityReport, name string) PrimitiveStatus {
	t.Helper()
	for _, p := range r.Primitives {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("primitive %q not in report", name)
	return PrimitiveStatus{}
}

// S4: a required primitive (schema) probed unavailable fails the gate and is
// reported unavailable+gating.
func TestEvaluate_RequiredUnavailableFailsGate(t *testing.T) {
	r := EvaluateCapabilities(fakeProber{
		unavailable: map[string]bool{"schema": true},
		version:     "2.1.154",
	})

	schema := findPrimitive(t, r, "schema")
	if schema.Status != StatusUnavailable {
		t.Fatalf("schema status = %q, want unavailable", schema.Status)
	}
	if !schema.Gating {
		t.Fatal("schema must be marked gating (required)")
	}
	if r.Overall != OverallFail {
		t.Fatalf("overall = %q, want fail", r.Overall)
	}
}

// S12: a version below the pin fails the gate.
func TestEvaluate_BelowMinVersionFailsGate(t *testing.T) {
	r := EvaluateCapabilities(fakeProber{version: "2.1.140"})

	if r.VersionOK {
		t.Fatal("VersionOK must be false for 2.1.140 < 2.1.154")
	}
	if r.Overall != OverallFail {
		t.Fatalf("overall = %q, want fail", r.Overall)
	}
}

// S14: an advisory primitive (budget) unavailable does NOT fail the gate when
// all required primitives are available and the version is ok.
func TestEvaluate_AdvisoryUnavailableDoesNotFailGate(t *testing.T) {
	r := EvaluateCapabilities(fakeProber{
		unavailable: map[string]bool{"budget": true},
		version:     "2.1.154",
	})

	b := findPrimitive(t, r, "budget")
	if b.Status != StatusUnavailable {
		t.Fatalf("budget status = %q, want unavailable", b.Status)
	}
	if b.Gating {
		t.Fatal("budget must be non-gating (advisory)")
	}
	if r.Overall != OverallPass {
		t.Fatalf("overall = %q, want pass", r.Overall)
	}
}

// FIDELITY-001 F1: parallel and isolation are required (gating) primitives because
// the generated route_team JS hard-depends on parallel(...) + isolation:'worktree'.
// An unavailable one must fail the gate and be reported gating, so a runtime that
// lacks them fails fast instead of crashing mid-launch.
func TestEvaluate_ParallelIsolationAreRequiredGating(t *testing.T) {
	for _, name := range []string{"parallel", "isolation"} {
		r := EvaluateCapabilities(fakeProber{
			unavailable: map[string]bool{name: true},
			version:     "2.1.154",
		})
		p := findPrimitive(t, r, name)
		if p.Status != StatusUnavailable {
			t.Fatalf("%s status = %q, want unavailable", name, p.Status)
		}
		if !p.Gating {
			t.Fatalf("%s must be marked gating (required)", name)
		}
		if r.Overall != OverallFail {
			t.Fatalf("overall with %s unavailable = %q, want fail", name, r.Overall)
		}
	}
}

// Higher-than-pin versions also pass the version check.
func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		got, min string
		want     bool
	}{
		{"2.1.154", "2.1.154", true},
		{"2.1.155", "2.1.154", true},
		{"2.2.0", "2.1.154", true},
		{"2.1.140", "2.1.154", false},
		{"", "2.1.154", false},
		{"2.1", "2.1.154", false},
	}
	for _, c := range cases {
		if got := versionAtLeast(c.got, c.min); got != c.want {
			t.Errorf("versionAtLeast(%q,%q)=%v want %v", c.got, c.min, got, c.want)
		}
	}
}
