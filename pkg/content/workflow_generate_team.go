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
	"test_scaffold":  "tester",
	"implementation": "executor",
	"annotation":     "annotator",
	"testing":        "tester",
	"review":         "reviewer",
}

// PLAN_SCHEMA literal text to be emitted into the template.
const planSchemaJS = `const PLAN_SCHEMA = {
  type: 'object',
  properties: {
    tasks: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id:          { type: 'string' },
          description: { type: 'string' },
          files:       { type: 'array', items: { type: 'string' } }
        },
        required: ['id', 'description', 'files']
      }
    }
  },
  required: ['tasks']
};

`

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
	sb.WriteString(planSchemaJS)

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
	case p.ID == "planning":
		writeTeamPlanningBlock(sb, p.ID, model, effort, extra)
	case depth.FanOutCap > 0:
		writeTeamFanOutBlock(sb, p.ID, model, effort, depth.FanOutCap, extra)
	case depth.VerifyVotes > 0:
		writeTeamReviewBlock(sb, p.ID, model, effort, depth, extra)
	default:
		writeTeamBaselineComment(sb, p.ID, model, effort, extra)
		role := teamPhaseRoles[p.ID]
		var prompt string
		switch p.ID {
		case "test_scaffold":
			prompt = "Scaffold tests for SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''}"
		case "annotation":
			prompt = "Scan and apply @AX annotations for SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''}"
		case "testing":
			prompt = "Run and verify tests for SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''}"
		default:
			prompt = fmt.Sprintf("Execute %s agent for SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''}", role)
		}
		fmt.Fprintf(sb, "await agent(`%s`, %s);\n",
			prompt, teamAgentOpt(p.ID, model, effort, role))
	}
	sb.WriteString("\n")
}

// writeTeamPlanningBlock emits the structured-output planner call.
func writeTeamPlanningBlock(sb *strings.Builder, id, model, effort string, extra string) {
	writeTeamBaselineComment(sb, id, model, effort, extra)
	prompt := "Plan SPEC ${ctx.spec || ''} at ${ctx.workingDir || ''}, produce the task assignment table (id, description, file ownership)"
	opt := fmt.Sprintf("{ agentType: 'planner', schema: PLAN_SCHEMA, model: (RT.%s && RT.%s.model) || '%s', effort: (RT.%s && RT.%s.effort) || '%s' }",
		id, id, model, id, id, effort)
	fmt.Fprintf(sb, "const plan = await agent(`%s`, %s);\n", prompt, opt)
}

// writeTeamFanOutBlock emits the bounded executor fan-out loop for the
// implementation phase. The fan-out count is RT-overridable but falls back to
// the schema fan_out_cap.
//
// The real Workflow runtime contract is parallel(thunks: Array<() => Promise>):
// it takes a single array of deferred call thunks, not spread already-invoked
// promises. So the loop pushes `() => agent(...)` thunks and calls
// `parallel(executors)` with the array. A degenerate floor guarantees at least
// one executor when the planner produces no tasks (FIDELITY-001 F2).
func writeTeamFanOutBlock(sb *strings.Builder, id, model, effort string, fanOut int, extra string) {
	writeTeamBaselineComment(sb, id, model, effort, fmt.Sprintf("fan_out_cap=%d %s", fanOut, extra))
	opt := fmt.Sprintf("{ agentType: 'executor', isolation: 'worktree', model: (RT.%s && RT.%s.model) || '%s', effort: (RT.%s && RT.%s.effort) || '%s' }",
		id, id, model, id, id, effort)
	fmt.Fprintf(sb, "const FANOUT_%s = (RT.%s && RT.%s.fan_out_cap) || %d;\n", id, id, id, fanOut)
	fmt.Fprintf(sb, "const tasks_%s = (plan && plan.tasks) || [];\n", id)
	fmt.Fprintf(sb, "const limit_%s = Math.min(tasks_%s.length, FANOUT_%s);\n", id, id, id)
	fmt.Fprintf(sb, "const executors_%s = [];\n", id)
	fmt.Fprintf(sb, "for (let i = 0; i < limit_%s; i++) {\n", id)
	fmt.Fprintf(sb, "  const task = tasks_%s[i];\n", id)
	fmt.Fprintf(sb, "  const taskPrompt = `Implement task ${task.id}: ${task.description}, files: ${task.files ? task.files.join(', ') : ''} for SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''}`;\n")
	fmt.Fprintf(sb, "  executors_%s.push(() => agent(taskPrompt, %s));\n", id, opt)
	sb.WriteString("}\n")
	// Degenerate floor: an empty/failed plan.tasks would otherwise leave zero
	// executors and silently no-op the implementation phase (worse than Route A).
	fmt.Fprintf(sb, "if (executors_%s.length === 0) {\n", id)
	fmt.Fprintf(sb, "  executors_%s.push(() => agent(`Implement SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''} (no planner task breakdown; implement the full SPEC)`, %s));\n", id, opt)
	sb.WriteString("}\n")
	fmt.Fprintf(sb, "await parallel(executors_%s);\n", id)
}

// writeTeamReviewBlock emits the reviewer verify-vote loop followed by the
// security_auditor pass and the optional synthesis pass. Vote count and
// synthesis are RT-overridable but fall back to the schema baseline.
func writeTeamReviewBlock(sb *strings.Builder, id, model, effort string, depth workflow.DepthProfile, extra string) {
	writeTeamBaselineComment(sb, id, model, effort,
		fmt.Sprintf("verify_votes=%d synthesis=%t %s", depth.VerifyVotes, depth.Synthesis, extra))
	fmt.Fprintf(sb, "const VOTES_%s = (RT.%s && RT.%s.verify_votes) || %d;\n", id, id, id, depth.VerifyVotes)
	fmt.Fprintf(sb, "const SYNTH_%s = (RT.%s && RT.%s.synthesis) || %t;\n", id, id, id, depth.Synthesis)

	reviewerOpt := teamAgentOpt(id, model, effort, "reviewer")
	secAuditorOpt := teamAgentOpt(id, model, effort, "security-auditor")

	fmt.Fprintf(sb, "for (let v = 0; v < VOTES_%s; v++) {\n", id)
	fmt.Fprintf(sb, "  await agent(`Review changes for SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''} (vote ${v})`, %s);\n", reviewerOpt)
	sb.WriteString("}\n")

	fmt.Fprintf(sb, "await agent(`Perform OWASP security audit for SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''}`, %s);\n", secAuditorOpt)

	fmt.Fprintf(sb, "if (SYNTH_%s) {\n", id)
	fmt.Fprintf(sb, "  await agent(`Synthesize review results for SPEC ${ctx.spec || ''} in ${ctx.workingDir || ''}`, %s);\n", reviewerOpt)
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
func teamAgentOpt(id, model, effort, agentType string) string {
	return fmt.Sprintf("{ agentType: '%s', model: (RT.%s && RT.%s.model) || '%s', effort: (RT.%s && RT.%s.effort) || '%s' }",
		agentType, id, id, model, id, id, effort)
}
