package content

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

// teamPhaseRoles maps each route_team agent phase-id to the agent role spawned
// in its phase block. gate_build_test and release_hygiene are deterministic
// agent.exec bridges and have no role entry.
var teamPhaseRoles = map[string]string{
	"planning":       "planner",
	"test_scaffold":  "test_scaffold",
	"implementation": "executor",
	"annotation":     "annotator",
	"testing":        "tester",
	"review":         "reviewer",
}

// deriveTeamWorkflowJS produces the deterministic route_team JS workflow template
// from the schema. Phase order equals schema array order. The output is a pure
// function of the schema: no timestamps, no randomness, byte-identical across
// runs. Per-phase baseline model/effort/depth literals are read from the schema
// via ModelSet/EffortSet/DepthSet; the agent role per phase comes from the fixed
// teamPhaseRoles map.
func deriveTeamWorkflowJS(schema workflow.Schema) string {
	models := schema.ModelSet()
	efforts := schema.EffortSet()
	depths := schema.DepthSet()

	var sb strings.Builder

	sb.WriteString(generatedTeamJSHeader)
	sb.WriteString("\n//\n")
	sb.WriteString("// route-team deterministic workflow adapter. The manifest (route_team.md +\n")
	sb.WriteString("// route_team.schema.json) is the source of truth; this file is regenerated.\n\n")

	// meta literal with ordered phase titles.
	sb.WriteString("export const meta = {\n")
	sb.WriteString("  name: 'route-team',\n")
	sb.WriteString("  description: 'Deterministic opt-in team Workflow substrate (claude-code --team): full team phase orchestration.',\n")
	sb.WriteString("  phases: [\n")
	for _, p := range schema.Phases {
		fmt.Fprintf(&sb, "    {title:'%s'},\n", p.ID)
	}
	sb.WriteString("  ],\n")
	sb.WriteString("};\n\n")

	// Script body preamble: the RT quality override + per-agent opt resolution.
	sb.WriteString("// Workflow API globals: agent, phase, log, env.\n")
	sb.WriteString("// RT carries the per-run quality override injected by the dispatch layer via the\n")
	sb.WriteString("// AUTOPUS_WORKFLOW_QUALITY env. Each agent() opt reads the RT override first and\n")
	sb.WriteString("// falls back to the schema baseline literal.\n")
	sb.WriteString("export default async function run() {\n")
	sb.WriteString("  const RT = JSON.parse(env('AUTOPUS_WORKFLOW_QUALITY') || '{}');\n")
	for _, p := range schema.Phases {
		writeTeamPhaseBlock(&sb, p, models[p.ID], efforts[p.ID], depths[p.ID])
	}
	sb.WriteString("}\n")

	return sb.String()
}

// writeTeamPhaseBlock emits a single deterministic phase(...) block. The gate and
// release-hygiene phases are identical to route_a (deterministic agent.exec
// bridges); agent phases emit a baseline comment plus an RT-overridable agent
// call shaped by the phase depth (fan-out for implementation, verify votes +
// synthesis for review, single call otherwise).
func writeTeamPhaseBlock(sb *strings.Builder, p workflow.PhaseDef, model, effort string, depth workflow.DepthProfile) {
	fmt.Fprintf(sb, "  await phase('%s', { retry: %d, budget: %d }, async () => {\n", p.ID, p.Retry, p.Budget)
	switch {
	case p.ID == gateBuildTestID:
		fmt.Fprintf(sb, "    // Deterministic gate: verdict_source '%s'. Shell out to the\n", p.ResultType)
		sb.WriteString("    // `auto workflow gate` CLI (JS->Go bridge) and parse its JSON.\n")
		sb.WriteString("    const out = await agent.exec(['auto', 'workflow', 'gate']);\n")
		sb.WriteString("    const gate = JSON.parse(out.stdout);\n")
		sb.WriteString("    log('gate', gate.verdict, gate.verdict_source, gate.build_exit, gate.test_exit);\n")
		sb.WriteString("    if (gate.verdict !== 'pass') {\n")
		sb.WriteString("      throw new Error('gate failed: ' + gate.verdict_source);\n")
		sb.WriteString("    }\n")
	case p.ID == releaseHygieneID:
		sb.WriteString("    // Release hygiene: generated/runtime drift + staged source size gate.\n")
		sb.WriteString("    const out = await agent.exec(['auto', 'check', '--hygiene', '--arch', '--quiet', '--staged']);\n")
		sb.WriteString("    log('release_hygiene', 'pass', out.stdout);\n")
	case depth.FanOutCap > 0:
		writeTeamFanOutBlock(sb, p.ID, model, effort, depth.FanOutCap)
	case depth.VerifyVotes > 0:
		writeTeamReviewBlock(sb, p.ID, model, effort, depth)
	default:
		writeTeamBaselineComment(sb, p.ID, model, effort, "")
		fmt.Fprintf(sb, "    await agent('%s', %s);\n", teamPhaseRoles[p.ID], teamAgentOpt(p.ID, model, effort))
	}
	sb.WriteString("  });\n")
}

// writeTeamFanOutBlock emits the bounded executor fan-out loop for the
// implementation phase. The fan-out count is RT-overridable but falls back to
// the schema fan_out_cap.
func writeTeamFanOutBlock(sb *strings.Builder, id, model, effort string, fanOut int) {
	writeTeamBaselineComment(sb, id, model, effort, fmt.Sprintf("fan_out_cap=%d", fanOut))
	fmt.Fprintf(sb, "    const FANOUT_%s = (RT.%s && RT.%s.fan_out_cap) || %d;\n", id, id, id, fanOut)
	fmt.Fprintf(sb, "    for (let i = 0; i < FANOUT_%s; i++) {\n", id)
	fmt.Fprintf(sb, "      await agent('%s', %s);\n", teamPhaseRoles[id], teamAgentOpt(id, model, effort))
	sb.WriteString("    }\n")
}

// writeTeamReviewBlock emits the reviewer verify-vote loop followed by the
// security_auditor pass and the optional synthesis pass. Vote count and
// synthesis are RT-overridable but fall back to the schema baseline.
func writeTeamReviewBlock(sb *strings.Builder, id, model, effort string, depth workflow.DepthProfile) {
	writeTeamBaselineComment(sb, id, model, effort,
		fmt.Sprintf("verify_votes=%d synthesis=%t", depth.VerifyVotes, depth.Synthesis))
	opt := teamAgentOpt(id, model, effort)
	fmt.Fprintf(sb, "    const VOTES_%s = (RT.%s && RT.%s.verify_votes) || %d;\n", id, id, id, depth.VerifyVotes)
	fmt.Fprintf(sb, "    const SYNTH_%s = (RT.%s && RT.%s.synthesis) || %t;\n", id, id, id, depth.Synthesis)
	fmt.Fprintf(sb, "    for (let v = 0; v < VOTES_%s; v++) {\n", id)
	fmt.Fprintf(sb, "      await agent('%s', %s);\n", teamPhaseRoles[id], opt)
	sb.WriteString("    }\n")
	fmt.Fprintf(sb, "    await agent('security_auditor', %s);\n", opt)
	fmt.Fprintf(sb, "    if (SYNTH_%s) {\n", id)
	fmt.Fprintf(sb, "      await agent('%s', %s);\n", teamPhaseRoles[id], opt)
	sb.WriteString("    }\n")
}

// writeTeamBaselineComment emits the REQUIRED baseline comment line. The parity
// gate reads the model=/effort=/depth tokens from this line, so its format is a
// contract: "// route_team baseline <id>: model=<m> effort=<e> [extra]".
func writeTeamBaselineComment(sb *strings.Builder, id, model, effort, extra string) {
	line := fmt.Sprintf("    // route_team baseline %s: model=%s effort=%s", id, model, effort)
	if extra != "" {
		line += " " + extra
	}
	sb.WriteString(line)
	sb.WriteString("\n")
}

// teamAgentOpt builds the RT-overridable agent() options literal for a phase:
// each field reads the RT override first and falls back to the schema baseline.
func teamAgentOpt(id, model, effort string) string {
	return fmt.Sprintf("{ model: (RT.%s && RT.%s.model) || '%s', effort: (RT.%s && RT.%s.effort) || '%s' }",
		id, id, model, id, id, effort)
}
