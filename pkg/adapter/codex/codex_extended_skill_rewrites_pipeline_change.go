package codex

// This file holds the two routing contracts spliced after the gate
// applicability section: which authoring path a change class takes, and how a
// parallel fan-out closes with a single merged verification. Both are policy,
// not phase steps, so they live next to the receipt contract they read.

// codexChangeContractPath renders the `## Change Contract Path` section. It
// opens and closes with a newline so it splices between sections.
func codexChangeContractPath() string {
	return `
## Change Contract Path

Low-risk work takes a compact change contract by default. Only high-risk work authors the full four-document SPEC set.

` + "`auto spec change <SPEC-ID> --class <class> --ac <AC-ID,...> --surface <path,...> --verify \"<command>\" [--change-id <id>] [--new-contract] [--json]`" + ` writes one ` + "`change.md`" + ` referencing an existing SPEC and its acceptance-criteria ids. It refuses when the referenced SPEC or any acceptance-criteria id does not exist, and it restates no requirement: the SPEC stays their single source.

| Declared class | Risk | Authoring path |
|---|---|---|
| ` + "`test_only`" + `, ` + "`docs_only`" + `, ` + "`small_ui`" + `, ` + "`bugfix_existing_contract`" + ` | low | compact ` + "`change.md`" + `; ` + "`spec_authoring`" + ` and ` + "`risk_first_probe`" + ` are ` + "`not_applicable`" + ` |
| ` + "`feature`" + `, ` + "`multi_domain`" + `, ` + "`security_or_data`" + ` | high | full SPEC set plus the Phase 1.9 risk-first integration probe; both gates are ` + "`required`" + ` |

Risk is never decided by file count. A path under an auth, billing, data, migration, or security glob raises the class to ` + "`security_or_data`" + `. Production code spanning two module roots raises it to ` + "`multi_domain`" + `; documentation and test material never raise that signal alone. A new exported API or contract — declared with ` + "`--new-contract`" + `, or detected on an IDL file or a public API root — raises it to ` + "`feature`" + `.

When the declared class is contradicted by the intended surface — ` + "`test_only`" + ` including non-test source, ` + "`docs_only`" + ` including code, ` + "`small_ui`" + ` including non-UI source — the command reports ` + "`escalate_to_full_spec`" + ` with the reason, writes no ` + "`change.md`" + `, and exits non-zero. Escalation is a routing decision, not a warning to read past.

Every safety gate is retained on both paths: ` + "`security`" + `, ` + "`validation`" + `, ` + "`data_loss`" + `, and ` + "`deterministic_oracle`" + ` are never ` + "`not_applicable`" + `, ` + "`accessibility`" + ` and ` + "`ux_verification`" + ` stay ` + "`required`" + ` whenever the surface has UI paths, and race and coverage checks keep their thresholds. The compact path removes SPEC authoring, not verification.
`
}

// codexMergedFinalVerification renders the `## Merged Final Verification`
// section, including the review loop's terminal statuses.
func codexMergedFinalVerification() string {
	return `
## Merged Final Verification

Independent tasks with disjoint ownership run in parallel. Each worker verifies only its own surface. The full build, race, integration, coverage, security, and Phase 4 review then run once, after integration, over the union change set — never once per worker.

One merged run is not one verdict. The completion receipt records a verdict and an evidence ref per ` + "`spec_id`" + ` plus acceptance-criteria id. A Must criterion without its own evidence row is not closed by a sibling ` + "`PASS`" + `, and a failing slice is never offset by the number of passing ones.

The review loop terminates on ` + "`loop_status`" + `: ` + "`converged`" + ` when the verdict is ` + "`PASS`" + ` with no active findings, ` + "`awaiting_changes`" + ` when the reviewed input is unchanged from the previous revision, ` + "`revisions_exhausted`" + ` at the revision bound, and ` + "`provider_unavailable`" + ` when no usable provider review exists. ` + "`awaiting_changes`" + ` stops the loop before re-dispatching providers and returns the previous findings: the author must change something, because re-running the same input is not progress. The receipt names the blocking finding ids and the policy that blocked them.
`
}
