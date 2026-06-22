package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

// teamDriftSchemaJSON declares planning model=claude-opus-4-8 while the drifted
// JS below emits model=claude-sonnet-4-6 in the planning block.
const teamDriftSchemaJSON = `{"phases":[
	{"id":"planning","retry":0,"budget":60000,"model":"claude-opus-4-8","effort":"medium"}
]}`

const teamDriftMD = "# Route Team\n\n### planning\nplan.\n"

// teamDriftJS keeps the schema phase-id/retry/budget tokens aligned (so the
// per-phase quality check is reached) but diverges on the planning model token.
const teamDriftJS = `const ctx = args || {};
const RT = (args && args.quality) || {};

await phase('planning');
// route_team baseline planning: model=claude-sonnet-4-6 effort=medium retry: 0 budget: 60000
await agent(` + "`" + `Execute planner agent for spec ${ctx.spec || ''}` + "`" + `, { model: (RT.planning && RT.planning.model) || 'claude-sonnet-4-6', effort: (RT.planning && RT.planning.effort) || 'medium' });
`

// TestS5_ParityPerPhaseModelDrift verifies the per-phase quality parity check
// fails closed and names the diverging element as <phase>.<field> when the
// derived JS planning block carries a model token that disagrees with the schema
// baseline.
func TestS5_ParityPerPhaseModelDrift(t *testing.T) {
	t.Parallel()
	schema, err := workflow.ParseSchema([]byte(teamDriftSchemaJSON))
	if err != nil {
		t.Fatalf("parse drift schema: %v", err)
	}
	err = checkWorkflowParity(parityArtifacts{
		schema:     schema,
		derivedJS:  teamDriftJS,
		markdownMD: teamDriftMD,
	})
	if err == nil {
		t.Fatal("expected per-phase model drift error, got nil")
	}
	if !strings.Contains(err.Error(), "planning.model") {
		t.Errorf("error must name diverging element planning.model, got: %v", err)
	}
}

// TestS5_ParityPerPhaseAlignedTeam verifies the positive case: the real
// route_team manifest and its derived JS pass the extended parity gate.
func TestS5_ParityPerPhaseAlignedTeam(t *testing.T) {
	t.Parallel()
	contentDir := repoContentDir(t)
	schema, err := workflow.LoadSchema(filepath.Join(contentDir, "workflows", "route_team.schema.json"))
	if err != nil {
		t.Fatalf("load team schema: %v", err)
	}
	mdBytes, err := os.ReadFile(filepath.Join(contentDir, "workflows", "route_team.md"))
	if err != nil {
		t.Fatalf("read team md: %v", err)
	}
	derived := deriveTeamWorkflowJS(schema)
	if err := checkWorkflowParity(parityArtifacts{
		schema:     schema,
		derivedJS:  derived,
		markdownMD: string(mdBytes),
	}); err != nil {
		t.Errorf("aligned team manifest must pass parity: %v", err)
	}
}
