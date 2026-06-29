package content

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

func TestWorkflowParityStability_S11(t *testing.T) {
	t.Parallel()

	// Schema with coverage_threshold = 90
	schemaJSON := `{"phases":[
		{"id":"testing","retry":0,"budget":60000,"model":"claude-sonnet-4-6","effort":"medium","coverage_threshold":90}
	]}`

	// derivedJS block containing coverage_threshold = 85 (mismatched)
	derivedJS := `const ctx = args || {};
const RT = (args && args.quality) || {};

await phase('testing');
// route_team baseline testing: model=claude-sonnet-4-6 effort=medium retry: 0 budget: 60000 coverage_threshold=85
await agent("Run and verify tests", { model: 'claude-sonnet-4-6', effort: 'medium' });
`

	md := "# Route Team\n\n### testing\ntests.\n"

	schema, err := workflow.ParseSchema([]byte(schemaJSON))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	err = checkWorkflowParity(parityArtifacts{
		schema:     schema,
		derivedJS:  derivedJS,
		markdownMD: md,
	})
	if err == nil {
		t.Fatal("expected parity error due to coverage_threshold drift, got nil")
	}

	// diverging element (testing + coverage_threshold) must be named in the error message
	if !strings.Contains(err.Error(), "testing") || !strings.Contains(err.Error(), "coverage_threshold") {
		t.Errorf("error must name testing and coverage_threshold, got: %v", err)
	}

	// Aligned case: coverage_threshold = 90 in derivedJS
	alignedJS := `const ctx = args || {};
const RT = (args && args.quality) || {};

await phase('testing');
// route_team baseline testing: model=claude-sonnet-4-6 effort=medium retry: 0 budget: 60000 coverage_threshold=90
await agent("Run and verify tests", { model: 'claude-sonnet-4-6', effort: 'medium' });
`

	err = checkWorkflowParity(parityArtifacts{
		schema:     schema,
		derivedJS:  alignedJS,
		markdownMD: md,
	})
	if err != nil {
		t.Errorf("expected no parity error for aligned coverage_threshold, got: %v", err)
	}
}

func TestWorkflowParityStability_S12(t *testing.T) {
	t.Parallel()

	// Route A schema has no coverage_threshold and no gate retry.
	// Test that route_a is unaffected and works as before.
	routeASchemaJSON := `{"phases":[
		{"id":"planning","retry":0,"budget":60000},
		{"id":"implementation","retry":0,"budget":120000},
		{"id":"gate_build_test","retry":0,"budget":40000,"verdict_source":"exit_code"},
		{"id":"release_hygiene","retry":0,"budget":30000}
	]}`

	routeAMD := "# Route A\n\n### planning\n### implementation\n### gate_build_test\n### release_hygiene\n"

	routeAJS := `const ctx = args || {};
const RT = (args && args.quality) || {};

await phase('planning');
// route_a baseline planning: retry: 0 budget: 60000
await phase('implementation');
// route_a baseline implementation: retry: 0 budget: 120000
await phase('gate_build_test');
// Deterministic gate: verdict_source 'exit_code'. retry: 0 budget: 40000
await phase('release_hygiene');
// Release hygiene: retry: 0 budget: 30000
`

	schema, err := workflow.ParseSchema([]byte(routeASchemaJSON))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	err = checkWorkflowParity(parityArtifacts{
		schema:     schema,
		derivedJS:  routeAJS,
		markdownMD: routeAMD,
	})
	if err != nil {
		t.Errorf("expected route_a to pass parity check, got: %v", err)
	}
}
