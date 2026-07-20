# SPEC-ORCH-024 Acceptance Criteria

모든 시나리오는 concrete expected output과 예상 값을 판정하며 file exists, heading, exit code, non-empty output만으로 통과하지 않는다.

## Test Scenarios

## Pipeline

### S1: nil backend fail-closed
Given non-dry-run `SubprocessEngine` with nil backend.
When it runs.
Then result is nil, error is `pipeline: backend is required unless dry-run`, and dispatch count is 0.

### S2: nonexistent SPEC
Given no `.autopus/specs/SPEC-DOES-NOT-EXIST/spec.md`.
When `auto pipeline run SPEC-DOES-NOT-EXIST` runs.
Then error contains `SPEC not found: SPEC-DOES-NOT-EXIST`, stdout lacks `Pipeline complete`, calls are 0.

### S3: observed dispatch receipt
Given five passing phases and a recording backend.
When the pipeline completes.
Then backend calls, receipt dispatch count and completed phase count are each exactly 5 and terminal is `completed`.

### S4: stop after failed gate
Given validate returns an exact FAIL verdict.
When the pipeline runs.
Then review calls are 0, validate is failed, review pending, terminal is `blocked`.

### S5: checkpoint/dashboard parity
Given a successful run for SPEC-PIPE-001.
When dashboard renders that SPEC.
Then the canonical per-SPEC checkpoint is used, five phases are done, pending count is 0, no missing warning appears.

### S6: conflicting verdict
Given `VERDICT: PASS` and `VERDICT: FAIL` in one validation response, or APPROVE and REQUEST_CHANGES.
When the gate evaluates.
Then verdict is fail in both cases.

### S7: malformed or unknown verdict
Given unknown gate, BYPASS, free-form approval, or negated approval.
When evaluated.
Then each verdict is fail; one exact typed PASS/APPROVE is the only passing form.

## Orchestra

### S8: requested/effective strategy
Given debate and consensus requests.
When subprocess orchestration runs.
Then result and receipt requested/effective strategy equal the request; unsupported strategy makes 0 calls and errors.

### S9: strategy role counts
Given three providers.
When consensus runs, R1=3, R2=0, judge=0. When standard debate runs, R1=3, R2=3, judge=1.

### S10: fallback modes
Given pane pre-commit failure and a working subprocess backend.
When mode is subprocess, skip, or abort.
Then subprocess executes exactly once, zero times, or zero times respectively; receipt reason and terminal match mode.

### S11: judge failure visibility
Given successful participants and judge error, timeout, or empty output.
When debate finishes.
Then participant results remain, one failed provider has role judge, degraded is true, terminal is blocked, summary claims no judge success.

### S12: provider sets and quorum
Given requested/configured providers 3 and resolved/usable providers 1.
When SPEC review computes integrity.
Then quorum required is 2, met is false, all six provider sets retain exact membership, status remains draft.

### S13: degraded promotion override
Given degraded PASS.
When no override exists, gate is blocked and status unchanged. With override, promotion may occur but degraded and audit reason remain.

### S14: dissent preservation
Given one provider says rotate a leaked key and two say no action.
When threshold .67 merges results.
Then both claims remain, minority count is 1/3, and dissent section is present.

### S15: stable finding identity and Critical veto
Given the same finding at different list positions and a unique unresolved Critical security finding.
When merged.
Then same identity forms one cluster, unrelated same-index items remain separate, Critical is open dissent with veto and blocked gate.

## Platform contracts

### S16: review/idea/team semantic parity
Given generated Codex and Claude surfaces.
When their `orchestration-contract.v1` semantic blocks are parsed.
Then review authority/degraded/retry, idea strategy/judge/dissent, and team ownership/receipt/teardown fields are deeply equal.

### S17: native tool bindings
Given Codex surfaces.
Then Claude primitive count is 0 and native spawn evidence exists. Given Claude surfaces, Codex primitive count is 0 and native team/agent evidence exists.

### S18: promotion and forwarding parity
Given Codex and Claude auto-go/auto-review/auto-idea/SPEC-review routes.
Then each uses risk-tiered review, forwards requested strategy/providers, preserves degraded PASS protection, and requires the five worker receipt fields.

## Review convergence additions

### S19: frozen pipeline prompt context
Given a resolved SPEC with required `spec.md`, `plan.md`, and `acceptance.md` plus unsafe prior output.
When real and dry-run phase prompts are built.
Then both use one verified frozen snapshot and manifest-backed layer rendering, include all required documents, and fence or redact prior output without promoting it to instructions.

### S20: resume identity and dependency closure
Given an empty-identity, route/snapshot-mismatched, or non-prefix checkpoint such as `validate=pending, review=done`.
When `--continue` runs.
Then dispatch count is 0, terminal is blocked, the canonical checkpoint bytes remain unchanged, and a later retry cannot launder stale done state.

### S21: receipt/checkpoint/dashboard failure parity
Given restored done phases, a persistence failure, or malformed checkpoint YAML.
When the engine or dashboard runs.
Then restored phases remain `done` with a restored transition reason, every non-nil result has exactly one terminal, persistence errors are joined with the blocker, and dashboard returns non-zero except for `os.ErrNotExist`.

### S22: selected-backend fallback policy
Given `InteractivePaneBackend` fails before pane commit.
When fallback mode is subprocess, skip, or abort.
Then the selected backend calls subprocess exactly 1, 0, or 0 times and preserves the corresponding degraded reason and terminal receipt.

### S23: dispatch-complete receipts and partial failure
Given debate R1/R2 success and failure combinations.
When any dispatch occurs.
Then provider receipt count equals dispatch count, each receipt contains actual role, attempt, backend, outcome and artifact, and post-dispatch failure returns a non-nil blocked partial result.

### S24: provider recovery quorum
Given one configured provider fails in R1 and succeeds in R2 with usable output.
When the run finalizes.
Then its historical failure remains auditable, it is also in the final usable set, and quorum uses the final attempt-aware outcome.

### S25: evidence-complete Critical veto and judge validation
Given same-identity findings with different severity/status/suggestions and malformed or same-family judge output.
When consensus or debate finalizes.
Then clustering is provider-order independent, preserves every remediation, the highest unresolved Critical sets veto, and required judge output/family separation fails closed while participant results remain.

### S26: typed CLI receipt consumption
Given synchronous, no-detach, and detached orchestra paths.
When generated review or idea workflows await completion.
Then each obtains `orchestration_run_receipt.v1` JSON or its artifact path and gates on current `critical_veto`, `gate_status`, `degraded_reasons`, judge and terminal fields rather than merged prose.

### S27: authoritative promotion receipt
Given SPEC review PASS, degraded PASS, override, and unchanged-status cases.
When Codex or Claude auto-go consumes the result.
Then runtime emits and the workflow verifies exact `status_changed`, `degraded_reasons`, and `override_applied` fields; no prompt directly edits SPEC status.

### S28: operational surface and unsupported-platform oracle
Given generated Codex, Claude, Gemini, and OpenCode surfaces.
When concrete argv and operational worker prompts are inspected.
Then debate/non-debate flags are valid, explicit provider values reach review, five worker fields plus disjoint ownership and dispatch evidence occur outside semantic-only JSON, Claude handles are valid, and Gemini/OpenCode foreign team primitive and dangling team-skill reference counts are 0.

## Quality Gates

- Must acceptance: S1~S28 all green.
- Existing affected package tests green; unrelated known eval-regression sibling oracle is reported separately if still present.
- `go build ./...`, `go vet ./...`, strict SPEC validation and architecture enforcement green.
- Modified/new source files below 300 lines; SPEC Markdown exempt.
- No generated/runtime root artifacts staged.

## Oracle Acceptance Notes

- S1, S2, S3, S4, S5, S6, S7 assert exact error strings, backend call counts, phase status counts, and pass/fail enums.
- S8, S9, S10, S11, S12, S13, S14, S15 assert exact role counts, provider membership, quorum value, degraded reasons, cluster counts, and veto state.
- S16, S17, S18 parse generated semantic blocks and compare exact fields while requiring foreign primitive count `0`.
- S19~S25 assert prompt manifest/source hashes, unchanged checkpoint bytes, exact receipt/dispatch parity, recovery membership, evidence retention, and typed judge/family failure.
- S26~S28 execute or parse concrete generated branches and receipt fields; semantic-block string presence alone cannot satisfy them.
- Structural signals such as file existence, heading presence, command exit success, or non-empty output are supplementary and cannot pass a Must scenario alone.

## Completion Evidence

- S1~S28: GREEN through focused runtime, CLI, generated-surface, and adapter behavioral tests.
- Frozen findings: P-01~P-06, O-01~O-06, X-01~X-07 all CLOSED by diff-only verification (`19/19`, open `0`).
- Concurrency: `go test -race ./pkg/pipeline ./pkg/orchestra -count=1` passed.
- Repository: `go test -p 1 ./... -skip TestEvalRegressionAutopusGateFetchSelectionReviewOracle -count=1` passed.
- Static gates: `go build ./...`, `go vet ./...`, strict SPEC validation, architecture enforcement, template generation, diff hygiene, and the modified-source 300-line gate passed.
- Existing unrelated oracle: the unchanged eval-regression workflow still lacks `actions/checkout@v4`; the isolated failing test is recorded and excluded from this SPEC's completion verdict.
