package codex

// This file holds the two deterministic runtimes the Codex agent-pipeline skill
// consumes around its phases: the gate applicability / evidence receipts written
// by `auto spec gates`, and the lead-time records written by `auto telemetry
// record`. Both are body-free receipt contracts, so they live next to each other
// rather than inside any single phase section.

// codexGateApplicabilityContract renders the `## Gate Applicability` section. It
// is spliced immediately before the Phase 1.9 probe gate, which is the first
// phase that reads the receipt, so it opens and closes with a newline.
func codexGateApplicabilityContract() string {
	return `
## Gate Applicability

Every phase gate reports ` + "`gate: applicability — reason`" + ` from ` + "`required | reusable | not_applicable | blocked`" + `. The value is a deterministic classifier decision, never a worker judgement.

- Before the Phase 2 fan-out, the main session runs ` + "`auto spec gates <SPEC-ID> --base <ref>`" + ` (or ` + "`--changed p1,p2,...`" + `), which writes ` + "`{SPEC_DIR}/gate-applicability.json`" + ` over the closed gate set ` + "`risk_first_probe, build, unit_tests, integration, security, validation, data_loss, deterministic_oracle, accessibility, ux_verification, annotation, provider_review, doc_sync`" + `, and carries those decisions into every worker prompt.
- ` + "`reusable`" + ` comes only from that receipt: a prior ` + "`{SPEC_DIR}/gates/evidence-<gate>.json`" + ` whose ` + "`input_closure_sha256`" + ` still matches the current tree, with ` + "`status: pass`" + `, ` + "`complete: true`" + `, and ` + "`observed_at`" + ` inside ` + "`--max-age`" + ` (default 168h). Its ` + "`input_globs`" + ` are re-expanded, so an added file invalidates evidence exactly like an edit.
- Any dependency change, ` + "`fail`" + `, ` + "`partial`" + `, missing input, or stale receipt returns ` + "`required`" + ` with the failed condition named. Workers never self-assign ` + "`reusable`" + `.
- After each real build/test/UX execution, record its evidence with ` + "`auto spec gates record <SPEC-ID> --gate <id> --status pass|fail|partial --inputs <glob,...> [--dynamic-deps <path,...>] [--command \"<text>\"]`" + ` so the next run reuses exact-input evidence instead of repeating the work.
- ` + "`security`" + `, ` + "`validation`" + `, ` + "`data_loss`" + `, and ` + "`deterministic_oracle`" + ` are never ` + "`not_applicable`" + `. ` + "`accessibility`" + ` and ` + "`ux_verification`" + ` are ` + "`required`" + ` when the change set has UI paths and ` + "`not_applicable`" + ` reasoned ` + "`no UI surface in change set`" + ` otherwise; only the classifier decides that.
- An auxiliary step reports ` + "`blocked`" + ` with a named fallback instead of stalling. @AX annotation with a missing reference source is ` + "`blocked`" + `; having nothing to annotate is ` + "`not_applicable`" + `.
`
}

// codexLeadTimeTelemetryContract renders the `## Lead-Time Telemetry` section.
// It is spliced before the completion contract, whose summary consumes it.
func codexLeadTimeTelemetryContract() string {
	return `
## Lead-Time Telemetry

Record each event with ` + "`auto telemetry record --spec-id <SPEC-ID> --action <kind>`" + ` as it happens; a summary reconstructed at the end is not lead-time evidence.

- ` + "`--action estimate --min 30m --max 2h`" + ` at planning.
- ` + "`--action milestone --name first_vertical_slice`" + ` at the first Phase 1.9 probe ` + "`PASS`" + `, or at the first real integration execution when every probe row is honestly ` + "`not-run`" + `.
- ` + "`--action action --kind reread|rerun --target <path|cmd> --reason <text>`" + ` whenever an already-verified input is re-read or a passing check is re-run.
- ` + "`--action defect --id <id> --discovered-phase <phase> [--fixed-phase <phase>] [--files N] [--escaped] [--repeat]`" + ` per defect.
- ` + "`--action gate --gate <id> --applicability required|reusable|not_applicable|blocked [--resolved]`" + ` for every gate decision in the applicability receipt.
- ` + "`--depends-on <phase,...>`" + ` with ` + "`--action start|agent --phase <phase>`" + `, so the phase DAG is real. The critical path is the longest wall-clock path through that DAG, not the sum of phase durations.

The completion summary runs ` + "`auto telemetry leadtime [--run <SPEC-ID>] [--baseline <SPEC-ID|dir>] [--json]`" + ` — using ` + "`--baseline`" + ` whenever a prior run exists — and reports ` + "`time_to_first_slice`" + `, completion lead time, ` + "`critical_path`" + `, reread/rerun counts by reason, ` + "`defects_by_discovery_phase`" + `, ` + "`repeat_finding_rate`" + `, ` + "`estimate_vs_actual`" + `, and that ` + "`escaped_defects`" + ` and ` + "`unresolved_safety_gates`" + ` did not increase. ` + "`regression: true`" + ` with a non-zero exit is a completion blocker, not a note.
`
}
