package workflow

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func canonicalSchema(t *testing.T) Schema {
	t.Helper()
	data := []byte(`{"phases":[
		{"id":"planning","retry":0,"budget":60000},
		{"id":"implementation","retry":0,"budget":120000},
		{"id":"gate_build_test","retry":0,"budget":40000,"verdict_source":"exit_code"},
		{"id":"release_hygiene","retry":0,"budget":30000}
	]}`)
	s, err := ParseSchema(data)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	return s
}

// S7: Render emits the four canonical phases in order with gate verdict_source
// "exit_code".
func TestRender_DeterministicPhaseOrder(t *testing.T) {
	s := canonicalSchema(t)
	report := Render(s, nil, "js", "content/workflows/route_a.md", "content/workflows/route_a.schema.json")

	want := []string{"planning", "implementation", "gate_build_test", "release_hygiene"}
	if len(report.PhaseOrder) != len(want) {
		t.Fatalf("phase order = %v, want %v", report.PhaseOrder, want)
	}
	for i, p := range want {
		if report.PhaseOrder[i] != p {
			t.Fatalf("phase[%d] = %q, want %q", i, report.PhaseOrder[i], p)
		}
	}
	if report.GateVerdictSource != "exit_code" {
		t.Fatalf("gate verdict source = %q, want exit_code", report.GateVerdictSource)
	}
}

// S9: Render exposes a per-phase model/effort/depth surface drawn from the
// schema baseline.
func TestRender_ExposesPerPhaseQuality(t *testing.T) {
	data := []byte(`{"phases":[
		{"id":"planning","model":"claude-opus-4-8","effort":"medium"},
		{"id":"implementation","model":"claude-sonnet-4-6","effort":"high","fan_out_cap":5},
		{"id":"review","verify_votes":3,"synthesis":true},
		{"id":"gate_build_test","verdict_source":"exit_code"}
	]}`)
	s, err := ParseSchema(data)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	report := Render(s, nil, "js", "m", "s")
	byID := make(map[string]RenderedPhase, len(report.Phases))
	for _, p := range report.Phases {
		byID[p.ID] = p
	}
	if p := byID["planning"]; p.Model != "claude-opus-4-8" || p.Effort != "medium" {
		t.Errorf("planning = %+v", p)
	}
	if p := byID["implementation"]; p.FanOutCap != 5 {
		t.Errorf("implementation fan_out_cap = %d, want 5", p.FanOutCap)
	}
	if p := byID["review"]; p.VerifyVotes != 3 || !p.Synthesis {
		t.Errorf("review = %+v", p)
	}
}

func stableLayer(id, content string) promptlayer.Layer {
	return promptlayer.Layer{
		ID:      id,
		Kind:    promptlayer.KindStable,
		Group:   promptlayer.GroupMethodologyTools,
		Content: content,
	}
}

func ephemeralLayer(id, content string) promptlayer.Layer {
	return promptlayer.Layer{
		ID:      id,
		Kind:    promptlayer.KindEphemeral,
		Group:   promptlayer.GroupTaskContext,
		Content: content,
	}
}

// S11: identical non-ephemeral context yields an equal hash; mutating a stable
// layer changes it; mutating only an ephemeral layer leaves it unchanged.
func TestPromptManifestHash_Determinism(t *testing.T) {
	base := []promptlayer.Layer{
		stableLayer("contract", "phase order"),
		ephemeralLayer("task", "ephemeral A"),
	}
	same := []promptlayer.Layer{
		stableLayer("contract", "phase order"),
		ephemeralLayer("task", "ephemeral A"),
	}

	h1 := PromptManifestHash(base)
	h2 := PromptManifestHash(same)
	if h1 == "" {
		t.Fatal("hash must be non-empty")
	}
	if h1 != h2 {
		t.Fatalf("identical context produced different hashes: %s vs %s", h1, h2)
	}

	mutatedStable := []promptlayer.Layer{
		stableLayer("contract", "phase order CHANGED"),
		ephemeralLayer("task", "ephemeral A"),
	}
	if PromptManifestHash(mutatedStable) == h1 {
		t.Fatal("mutating a stable layer must change the hash")
	}

	mutatedEphemeral := []promptlayer.Layer{
		stableLayer("contract", "phase order"),
		ephemeralLayer("task", "ephemeral B"),
	}
	if PromptManifestHash(mutatedEphemeral) != h1 {
		t.Fatal("mutating only an ephemeral layer must not change the hash")
	}
}
