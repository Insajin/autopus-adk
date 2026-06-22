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
//
// Segmented dispatch: phases are partitioned into two guard blocks by the
// gate_build_test boundary. The dispatcher launches segment A, runs
// "auto workflow gate" as a hard exit-code barrier, then launches segment B
// only when the verdict is pass. The SEGMENT constant is injected via args.
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

	// Script body preamble: ctx, RT quality override, and SEGMENT selector.
	// RT carries the per-run quality override injected by the dispatch layer via
	// args.quality. Each agent() opt reads the RT override first and falls back to
	// the schema baseline literal. SEGMENT is delivered via args.segment.
	sb.WriteString("// Workflow API globals: agent, phase, log, args.\n")
	sb.WriteString("const ctx = args || {};\n")
	sb.WriteString("const RT = (args && args.quality) || {};\n")
	sb.WriteString("const SEGMENT = (args && args.segment) || 'A';\n\n")

	// Split phases: segment A includes everything up to and including gate_build_test;
	// segment B includes everything after gate_build_test.
	var segA, segB []workflow.PhaseDef
	foundGate := false
	for _, p := range schema.Phases {
		if foundGate {
			segB = append(segB, p)
		} else {
			segA = append(segA, p)
			if p.ID == gateBuildTestID {
				foundGate = true
			}
		}
	}

	sb.WriteString("if (SEGMENT === 'A') {\n")
	for _, p := range segA {
		writeTeamPhaseBlock(&sb, p, models[p.ID], efforts[p.ID], depths[p.ID])
	}
	sb.WriteString("}\n\n")

	sb.WriteString("if (SEGMENT === 'B') {\n")
	for _, p := range segB {
		writeTeamPhaseBlock(&sb, p, models[p.ID], efforts[p.ID], depths[p.ID])
	}
	sb.WriteString("}\n")

	return sb.String()
}

// writeTeamPhaseBlock emits a single deterministic phase(...) block. The gate and
// release-hygiene phases are identical to route_a; agent phases emit a baseline
// comment plus an RT-overridable agent call shaped by the phase depth.
func writeTeamPhaseBlock(sb *strings.Builder, p workflow.PhaseDef, model, effort string, depth workflow.DepthProfile) {
	fmt.Fprintf(sb, "await phase('%s');\n", p.ID)
	extra := fmt.Sprintf("retry: %d budget: %d", p.Retry, p.Budget)
	switch {
	case p.ID == gateBuildTestID:
		fmt.Fprintf(sb, "// Deterministic gate: verdict_source '%s'. %s\n", p.ResultType, extra)
		sb.WriteString("// This gate is executed outside the JS by the Go runtime.\n")
		fmt.Fprintf(sb, "log('gate', '%s');\n", p.ID)
	case p.ID == releaseHygieneID:
		fmt.Fprintf(sb, "// Release hygiene: generated/runtime drift + staged source size gate. %s\n", extra)
		sb.WriteString("// This gate is executed outside the JS by the Go runtime.\n")
		fmt.Fprintf(sb, "log('release_hygiene', '%s');\n", p.ID)
	case depth.FanOutCap > 0:
		writeTeamFanOutBlock(sb, p.ID, model, effort, depth.FanOutCap, extra)
	case depth.VerifyVotes > 0:
		writeTeamReviewBlock(sb, p.ID, model, effort, depth, extra)
	default:
		writeTeamBaselineComment(sb, p.ID, model, effort, extra)
		role := teamPhaseRoles[p.ID]
		fmt.Fprintf(sb, "await agent(`Execute %s agent for spec ${ctx.spec || ''} in ${ctx.workingDir || ''}`, %s);\n",
			role, teamAgentOpt(p.ID, model, effort))
	}
	sb.WriteString("\n")
}

// writeTeamFanOutBlock emits the bounded executor fan-out loop for the
// implementation phase. The fan-out count is RT-overridable but falls back to
// the schema fan_out_cap.
func writeTeamFanOutBlock(sb *strings.Builder, id, model, effort string, fanOut int, extra string) {
	writeTeamBaselineComment(sb, id, model, effort, fmt.Sprintf("fan_out_cap=%d %s", fanOut, extra))
	fmt.Fprintf(sb, "const FANOUT_%s = (RT.%s && RT.%s.fan_out_cap) || %d;\n", id, id, id, fanOut)
	fmt.Fprintf(sb, "for (let i = 0; i < FANOUT_%s; i++) {\n", id)
	role := teamPhaseRoles[id]
	fmt.Fprintf(sb, "  await agent(`Execute %s agent (fan-out ${i}) for spec ${ctx.spec || ''} in ${ctx.workingDir || ''}`, %s);\n",
		role, teamAgentOpt(id, model, effort))
	sb.WriteString("}\n")
}

// writeTeamReviewBlock emits the reviewer verify-vote loop followed by the
// security_auditor pass and the optional synthesis pass. Vote count and
// synthesis are RT-overridable but fall back to the schema baseline.
func writeTeamReviewBlock(sb *strings.Builder, id, model, effort string, depth workflow.DepthProfile, extra string) {
	writeTeamBaselineComment(sb, id, model, effort,
		fmt.Sprintf("verify_votes=%d synthesis=%t %s", depth.VerifyVotes, depth.Synthesis, extra))
	opt := teamAgentOpt(id, model, effort)
	fmt.Fprintf(sb, "const VOTES_%s = (RT.%s && RT.%s.verify_votes) || %d;\n", id, id, id, depth.VerifyVotes)
	fmt.Fprintf(sb, "const SYNTH_%s = (RT.%s && RT.%s.synthesis) || %t;\n", id, id, id, depth.Synthesis)
	role := teamPhaseRoles[id]
	fmt.Fprintf(sb, "for (let v = 0; v < VOTES_%s; v++) {\n", id)
	fmt.Fprintf(sb, "  await agent(`Execute %s agent (vote ${v}) for spec ${ctx.spec || ''} in ${ctx.workingDir || ''}`, %s);\n",
		role, opt)
	sb.WriteString("}\n")
	fmt.Fprintf(sb, "await agent(`Execute security auditor for spec ${ctx.spec || ''} in ${ctx.workingDir || ''}`, %s);\n", opt)
	fmt.Fprintf(sb, "if (SYNTH_%s) {\n", id)
	fmt.Fprintf(sb, "  await agent(`Execute %s agent (synthesis) for spec ${ctx.spec || ''} in ${ctx.workingDir || ''}`, %s);\n",
		role, opt)
	sb.WriteString("}\n")
}

// writeTeamBaselineComment emits the REQUIRED baseline comment line. The parity
// gate reads the model=/effort=/depth tokens from this line, so its format is a
// contract: "// route_team baseline <id>: model=<m> effort=<e> [extra]".
func writeTeamBaselineComment(sb *strings.Builder, id, model, effort, extra string) {
	line := fmt.Sprintf("// route_team baseline %s: model=%s effort=%s", id, model, effort)
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
