package codex

// This file holds the phase sequence of the Codex agent-pipeline skill: the
// phase-flow overview block and the Phase 1.9 risk-first probe gate that runs
// between doc fetch and implementation fan-out. Keeping them together means the
// flow table and the phase it advertises cannot drift apart.

// codexAgentPipelinePhaseFlow renders the phase-flow overview block. The
// returned value carries no trailing newline; the caller supplies it.
func codexAgentPipelinePhaseFlow() string {
	return "```text" + `
Step 0:    Triage          -> main session
Phase 0.5: Change Class    -> main session  (compact change contract or escalate to full SPEC)
Phase 0.7: Authenticity    -> main session  (verify real worker dispatch plan)
Phase 1:   Planning        -> planner
Phase 1.5: Test Scaffold   -> tester        (optional)
Gate 1:    Approval        -> main session  (skip with --auto)
Phase 1.8: Doc Fetch       -> main session  (fetch current docs if needed)
Phase 1.9: Probe Gate      -> main session  (risk-first probe; not_applicable for low-risk classes)
Phase 2:   Implementation  -> executor x N  (parallel only with disjoint ownership)
Phase 2.1: Integration     -> main session  (inspect and integrate worker returns)
Gate 2:    Validation      -> validator
Phase 2.5: Annotation      -> annotator
Phase 3:   Testing         -> tester
Phase 3.5: UX Verify       -> frontend-specialist (optional)
Gate 3:    Acceptance      -> main session  (coverage of MUST acceptance)
Phase 4A:  Review          -> reviewer + security-auditor (+ optional multi-provider discovery)
Phase 4B:  Verify Fixes    -> reviewer/security follow-up, diff-only
` + "```"
}

// codexRiskFirstProbeGate renders the Phase 1.9 section. It is spliced between
// the Phase 1.8 and Phase 2 sections, so it opens and closes with a newline.
func codexRiskFirstProbeGate() string {
	return `
### Phase 1.9: Risk-First Probe Gate

This phase stays in the main session and runs before any implementation worker is spawned. It applies to the default prompt-driven pipeline; the deterministic Route A / Route Team contracts are unchanged.

Probe gate rules:

- read the ` + "`## Risk-First Integration Probe`" + ` table in ` + "`plan.md`" + `: 1-3 rows of ` + "`assumption_id`" + `, ` + "`class`" + `, ` + "`risk`" + `, ` + "`boundary`" + `, ` + "`input`" + `, ` + "`oracle`" + `, ` + "`isolation`" + `, ` + "`status`" + `, ` + "`reason`" + `, ` + "`evidence`" + `
- execute every ` + "`not-run`" + ` row that is executable now: a real boundary, an isolated fixture, and no unapproved external effect
- record ` + "`PASS`" + ` or ` + "`FAIL`" + ` with an evidence ref from that execution; a row without execution evidence stays ` + "`not-run`" + `
- a row that stays ` + "`not-run`" + ` keeps its reason and travels into the handoff as an explicit limitation, never as ` + "`PASS`" + `
- on ` + "`FAIL`" + ` for a high or critical assumption, return to planning and revise ` + "`plan.md`" + `; this is bounded to one re-plan, and a second ` + "`FAIL`" + ` surfaces to the user
- treat an implementer-introduced constraint broader than the requirement (new ACL, compatibility limit, security limit) as a scope expansion and probe it against the existing runtime here

Gate applicability comes from ` + "`auto spec gates`" + `, run here so the whole ` + "`gate-applicability.json`" + ` receipt rides into the fan-out. For ` + "`risk_first_probe`" + ` it is ` + "`required`" + ` whenever a high-risk change touches an integration boundary; ` + "`not_applicable`" + ` for a doc-only change set or a low-risk declared class (` + "`test_only`" + `, ` + "`docs_only`" + `, ` + "`small_ui`" + `, ` + "`bugfix_existing_contract`" + `), which still keeps one ` + "`not-run`" + ` row with the reason ` + "`no integration boundary`" + `; ` + "`blocked`" + ` when the fixture or environment is missing.

At the first row that returns ` + "`PASS`" + ` — or at the first real integration execution when every row stays ` + "`not-run`" + ` — record ` + "`auto telemetry record --spec-id <SPEC-ID> --action milestone --name first_vertical_slice`" + `. Recording it after the fact makes ` + "`time_to_first_slice`" + ` unmeasurable.
`
}
