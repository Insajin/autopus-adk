package workflow

import "testing"

func bindingSchema(t *testing.T) Schema {
	t.Helper()
	data := []byte(`{"phases":[
		{"id":"planning","model":"claude-sonnet-4-6","effort":"medium"},
		{"id":"implementation","model":"claude-sonnet-4-6","effort":"high","fan_out_cap":3},
		{"id":"review","verify_votes":1},
		{"id":"gate_build_test","verdict_source":"exit_code"}
	]}`)
	s, err := ParseSchema(data)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	return s
}

// TestOverlayPhases_BaselineNilBinding asserts a nil binding yields the schema
// baseline in order.
func TestOverlayPhases_BaselineNilBinding(t *testing.T) {
	t.Parallel()
	s := bindingSchema(t)
	got := OverlayPhases(s, nil)
	want := []RenderedPhase{
		{ID: "planning", Model: "claude-sonnet-4-6", Effort: "medium"},
		{ID: "implementation", Model: "claude-sonnet-4-6", Effort: "high", FanOutCap: 3},
		{ID: "review", VerifyVotes: 1},
		{ID: "gate_build_test"},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("phase[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestOverlayPhases_OverridesBoundPhases asserts bound phases are replaced
// wholesale while unbound phases keep their baseline.
func TestOverlayPhases_OverridesBoundPhases(t *testing.T) {
	t.Parallel()
	s := bindingSchema(t)
	b := &QualityBinding{Phases: map[string]PhaseBinding{
		"implementation": {Model: "claude-opus-4-8", Effort: "max", FanOutCap: 5},
		"review":         {VerifyVotes: 3, Synthesis: true},
	}}
	got := OverlayPhases(s, b)
	byID := make(map[string]RenderedPhase, len(got))
	for _, p := range got {
		byID[p.ID] = p
	}

	impl := byID["implementation"]
	if impl.Model != "claude-opus-4-8" || impl.Effort != "max" || impl.FanOutCap != 5 {
		t.Errorf("implementation overlay = %+v", impl)
	}
	rev := byID["review"]
	if rev.VerifyVotes != 3 || !rev.Synthesis {
		t.Errorf("review overlay = %+v", rev)
	}
	// Unbound phases keep their baseline.
	plan := byID["planning"]
	if plan.Model != "claude-sonnet-4-6" || plan.Effort != "medium" {
		t.Errorf("planning baseline changed: %+v", plan)
	}
	if byID["gate_build_test"] != (RenderedPhase{ID: "gate_build_test"}) {
		t.Errorf("gate phase should stay empty: %+v", byID["gate_build_test"])
	}
}
