package cli

import (
	"encoding/json"

	"github.com/insajin/autopus-adk/pkg/cost"
	"github.com/insajin/autopus-adk/pkg/workflow"
)

// teamQualityEnvKey is the environment variable the generated route_team JS reads
// to load the per-phase quality binding (REQ-015, S20). The value is the JSON of
// the bare phase map so the JS can read RT.<phase>.model/effort/...
const teamQualityEnvKey = "AUTOPUS_WORKFLOW_QUALITY"

// teamPhaseRoles maps each agent-driven team phase to the agent role(s) it runs.
// Deterministic phases (gate_build_test, release_hygiene) carry no roles and get
// no binding entry, so OverlayPhases keeps their schema baseline.
var teamPhaseRoles = map[string][]string{
	"planning":       {"planner"},
	"test_scaffold":  {"test_scaffold"},
	"implementation": {"executor"},
	"annotation":     {"annotator"},
	"testing":        {"tester"},
	"review":         {"reviewer", "security_auditor"},
}

// resolveTeamQualityBinding computes the complete per-phase quality binding for
// the team route from a quality tier and optional complexity (T18, REQ-015, S20).
// The model comes from cost.ModelForAgent (no fork), and the effort comes from
// ResolveEffort (the canonical resolver, no fork). Depth fields are applied only
// to the phases that own them: implementation gets the fan-out cap, review gets
// the verify-vote count and synthesis toggle.
func resolveTeamQualityBinding(quality, complexity string) workflow.QualityBinding {
	depth := workflow.ResolveDepth(quality)
	phases := make(map[string]workflow.PhaseBinding, len(teamPhaseRoles))

	for phase, roles := range teamPhaseRoles {
		primaryRole := roles[0]
		model := cost.ModelForAgent(quality, primaryRole)
		effRes, _ := ResolveEffort(EffortResolveInput{
			FlagQuality:    quality,
			FlagComplexity: complexity,
			Model:          model,
		})

		pb := workflow.PhaseBinding{
			Model:  model,
			Effort: string(effRes.Effort),
		}
		switch phase {
		case "implementation":
			pb.FanOutCap = depth.FanOutCap
		case "review":
			pb.VerifyVotes = depth.VerifyVotes
			pb.Synthesis = depth.Synthesis
		}
		phases[phase] = pb
	}

	return workflow.QualityBinding{Phases: phases}
}

// serializeTeamQualityBinding marshals the BARE phase map (not the wrapper) so
// the generated JS reads RT.<phase>.<field> directly from the env JSON.
func serializeTeamQualityBinding(b workflow.QualityBinding) (string, error) {
	data, err := json.Marshal(b.Phases)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
