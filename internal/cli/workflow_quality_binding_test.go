package cli

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/cost"
)

// S20: resolveTeamQualityBinding + serializeTeamQualityBinding produce the bare
// phase map the generated JS reads, with the ultra implementation/review values.
func TestResolveTeamQualityBinding_SerializesBarePhaseMap(t *testing.T) {
	t.Parallel()

	if teamQualityArgsKey != "quality" {
		t.Fatalf("teamQualityArgsKey = %q, want quality", teamQualityArgsKey)
	}

	b := resolveTeamQualityBinding("ultra", "")
	s, err := serializeTeamQualityBinding(b)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	var phases map[string]map[string]any
	if err := json.Unmarshal([]byte(s), &phases); err != nil {
		t.Fatalf("unmarshal bare phase map %q: %v", s, err)
	}

	impl, ok := phases["implementation"]
	if !ok {
		t.Fatalf("missing implementation entry in %q", s)
	}
	if impl["model"] != "claude-opus-4-8" {
		t.Fatalf("implementation model = %v, want claude-opus-4-8", impl["model"])
	}
	if impl["effort"] != "max" {
		t.Fatalf("implementation effort = %v, want max", impl["effort"])
	}

	review, ok := phases["review"]
	if !ok {
		t.Fatalf("missing review entry in %q", s)
	}
	if vv, _ := review["verify_votes"].(float64); vv != 3 {
		t.Fatalf("review verify_votes = %v, want 3", review["verify_votes"])
	}
	if review["synthesis"] != true {
		t.Fatalf("review synthesis = %v, want true", review["synthesis"])
	}
}

// S16/T13: the binding reuses the canonical resolvers (no fork) — model comes
// from cost.ModelForAgent and effort from ResolveEffort.
func TestResolveTeamQualityBinding_ReusesCanonicalResolvers(t *testing.T) {
	t.Parallel()

	ultra := resolveTeamQualityBinding("ultra", "")
	wantModel := cost.ModelForAgent("ultra", "executor")
	impl := ultra.Phases["implementation"]
	if impl.Model != wantModel {
		t.Fatalf("implementation model = %q, want %q (cost.ModelForAgent)", impl.Model, wantModel)
	}
	effRes, err := ResolveEffort(EffortResolveInput{FlagQuality: "ultra", Model: wantModel})
	if err != nil {
		t.Fatalf("ResolveEffort: %v", err)
	}
	if impl.Effort != string(effRes.Effort) {
		t.Fatalf("implementation effort = %q, want %q (ResolveEffort)", impl.Effort, string(effRes.Effort))
	}

	balanced := resolveTeamQualityBinding("balanced", "")
	bi := balanced.Phases["implementation"]
	if bi.Model != "claude-sonnet-5" || bi.Effort != "medium" {
		t.Fatalf("balanced implementation = %+v, want sonnet-5 + medium", bi)
	}
	br := balanced.Phases["review"]
	if br.VerifyVotes != 1 || br.Synthesis {
		t.Fatalf("balanced review = %+v, want verify_votes=1 synthesis=false", br)
	}
}
