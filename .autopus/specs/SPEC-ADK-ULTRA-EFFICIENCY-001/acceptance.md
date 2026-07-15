# SPEC-ADK-ULTRA-EFFICIENCY-001 Acceptance: Token-Efficient Ultra Quality Allocation

**Version**: 0.2.1
**Status**: completed
**Updated**: 2026-07-15
**Lifecycle implementation**: `true` for the completed acceptance/SPEC lifecycle; the terminal experiment artifacts remain `implemented=false`

## Test Scenarios

### S1: OpenAI-style inclusive usage is normalized without subset double counting
Priority: Must

Given a provider receipt with input tokens 1000, cached input tokens 400, output tokens 300, reasoning tokens 100, and reasoning declared as a subset of output
When the receipt is normalized into a UsageEnvelope
Then expected JSON value `input_tokens_total` equals 1000
And expected JSON value `uncached_input_tokens` equals 600
And expected JSON value `cached_input_tokens` equals 400
And expected JSON value `output_tokens_total` equals 300
And expected JSON value `reasoning_tokens` equals 100
And expected JSON value `raw_total_tokens` equals 1300
And expected JSON value `usage_status` equals `actual`

Expected JSON excerpt:

```json
{
  "input_tokens_total": 1000,
  "uncached_input_tokens": 600,
  "cached_input_tokens": 400,
  "output_tokens_total": 300,
  "reasoning_tokens": 100,
  "reasoning_relation": "subset_of_output",
  "raw_total_tokens": 1300,
  "usage_status": "actual"
}
```

Adding cached and reasoning components again to produce 1800 fails this scenario.

### S2: Anthropic-style cache breakdown is converted to inclusive input
Priority: Must

Given a provider receipt with uncached input tokens 600, cache creation input tokens 100, cache read input tokens 300, and output tokens 200
When the receipt is normalized into a UsageEnvelope
Then expected JSON value `input_tokens_total` equals 1000
And expected JSON value `raw_total_tokens` equals 1200
And expected JSON value `cache_creation_input_tokens` equals 100
And expected JSON value `cache_read_input_tokens` equals 300

Expected JSON excerpt:

```json
{
  "input_tokens_total": 1000,
  "uncached_input_tokens": 600,
  "cache_creation_input_tokens": 100,
  "cache_read_input_tokens": 300,
  "output_tokens_total": 200,
  "raw_total_tokens": 1200
}
```

### S3: Cost-only, estimated, unavailable, and ambiguous usage remain distinct
Priority: Must

Given one cost-only receipt with actual cost 0.04, one prompt estimate of 1200 tokens, one provider output with no usage, and one receipt whose reasoning inclusion relation is unknown
When each receipt is normalized and serialized
Then the cost-only receipt has `usage_status=cost_only`, `actual_cost_usd=0.04`, and null actual token totals
And the estimate has `usage_status=estimated`, `estimated_total_tokens=1200`, and null `raw_total_tokens`
And the missing receipt has `usage_status=unavailable`, null actual fields, and `unavailable_reason=provider_usage_absent`
And the ambiguous receipt has null `raw_total_tokens` and `unavailable_reason=component_relation_unknown`
And none of the null values is serialized as zero

Expected JSON excerpts:

```json
{"usage_status":"cost_only","raw_total_tokens":null,"actual_cost_usd":0.04}
{"usage_status":"estimated","raw_total_tokens":null,"estimated_total_tokens":1200}
{"usage_status":"unavailable","raw_total_tokens":null,"actual_cost_usd":null,"unavailable_reason":"provider_usage_absent"}
```

### S4: One model call is counted once across event and result propagation
Priority: Must

Given run `r1` contains an event and result receipt for call `c1` with raw total 1300 and a receipt for distinct call `c2` with raw total 1200
When worker, phase, pipeline, and orchestra aggregation are applied
Then expected JSON value `unique_model_call_count` equals 2
And expected JSON value `raw_total_tokens` equals 2500
And the repeated `r1/c1` receipt is counted exactly once
And a retry with call identity `c3` is counted as a separate call
And transport aggregation performs no telemetry write by itself
And the owning supervisor invokes the validated telemetry-record bridge exactly once after final aggregation

Given the event and result for the same `r1/c1` identity disagree on an actual component
When aggregation is applied
Then expected JSON value `usage_status` equals `unavailable`
And expected JSON value `unavailable_reason` equals `duplicate_call_conflict`
And policy promotion is blocked

### S5: Failed and retry spend remains in the accepted-task denominator
Priority: Must

Given an arm has accepted task `t1` using 1000 raw tokens, rejected task `t2` using 900 raw tokens, and accepted task `t3` using 500 raw tokens
When accepted-task efficiency is calculated
Then expected JSON value `raw_tokens` equals 2400
And expected JSON value `accepted_tasks` equals 2
And expected JSON value `raw_total_tokens_per_accepted_task` equals 1200
And failed task spend equals 900

Given an arm has raw token spend but zero accepted tasks
When accepted-task efficiency is calculated
Then `raw_total_tokens_per_accepted_task` is null
And expected JSON value `unavailable_reason` equals `zero_accepted_tasks`

### S6: Cache benefit changes billable cost but not raw token reduction
Priority: Must

Given a cold call has input 1000, cached input 0, output 200, and actual cost 0.020
And a warm call has input 1000, cached input 400, output 200, and actual cost 0.014
When raw and billable comparison is calculated
Then expected JSON value `cold_raw_total_tokens` equals 1200
And expected JSON value `warm_raw_total_tokens` equals 1200
And expected JSON value `raw_token_reduction_pct` equals 0 within numeric tolerance 0.001
And expected JSON value `actual_cost_reduction_pct` equals 30 within numeric tolerance 0.001

Expected JSON excerpt:

```json
{
  "cold_raw_total_tokens": 1200,
  "warm_raw_total_tokens": 1200,
  "raw_token_reduction_pct": 0,
  "actual_cost_reduction_pct": 30
}
```

### S7: Root routers are thin and every route resolves one detailed contract
Priority: Must

Given full-mode Claude and Gemini surfaces are rendered into temporary roots
When the root auto skills and command routes are measured
Then each root router is at most 8192 bytes
And each supported route resolves exactly one existing detailed command contract
And the route inventory contains setup, status, goal, update, plan, go, fix, review, sync, idea, map, why, verify, secure, test, qa, dev, canary, and doctor
And common language, source-ownership, subagent, review-convergence, and generated-surface safety tokens remain present
And no root router embeds the complete detailed bodies for every route

The size result is a byte oracle and is never labeled provider-actual tokens.

### S8: Command context profiles exclude unrelated project documents
Priority: Must

Given project context contains workspace, product, structure, tech, scenarios, canary, signatures, and learnings documents
When the `plan`, `test`, and `canary` command profiles are resolved
Then the `plan` profile includes core workspace policy and relevant SPEC evidence but excludes scenarios and canary unless independently relevant
And the `test` profile includes scenarios and excludes canary
And the `canary` profile includes canary and excludes scenarios
And signatures and learnings appear only in profiles that declare them

Expected profile rows:

| command | scenarios | canary | signatures | learnings |
|---|---:|---:|---:|---:|
| plan | false | false | conditional | conditional |
| test | true | false | conditional | conditional |
| canary | false | true | false | conditional |

### S9: A delegated context receipt is bounded and complete
Priority: Must

Given a supervisor has selected a 1200-token receipt metadata-and-optional-recall budget and has an Outcome Lock, two constraints, three owned paths, two forbidden paths, four acceptance criteria, five relevant references, and one decision delta
When it invokes budgeted recall and builds a context receipt
Then expected JSON value `budget_tokens` equals 1200
And the final receipt metadata and optional-recall estimate is at most 1200 tokens
And it contains every Outcome Lock, constraint, ownership boundary, acceptance ID, required reference, decision delta, snapshot hash, and prompt-manifest hash
And `ContextResult` receives only the residual budget after mandatory receipt fields are estimated
And complete required-document bodies are outside this budget and follow S28
And omitted optional source bodies remain retrievable through stable source references
And an omitted optional recall row is reported by count rather than silently dropped

### S10: Dynamic task evidence does not invalidate stable prompt hashes
Priority: Must

Given two renders use identical policy, route detail, and tool schemas but different task text and context receipts
When prompt manifests are compared
Then stable entry hashes are byte-equal
And snapshot or ephemeral entry hashes differ where their content changed
And the manifest reports no stable invalidation reason

Given one stable command instruction changes
When prompt manifests are compared
Then the affected stable entry hash differs
And the invalidation reason identifies the stable source

### S11: Completed stale tool pairs are pruned before hard compaction
Priority: Must

Given a phase handoff below the 50 percent hard threshold contains four complete successful tool call/result pairs and the compressor keeps the two most recent pairs
When the soft pruning policy is applied
Then exactly two old complete pairs are replaced by digest and artifact-reference records
And exactly two recent complete pairs remain
And expected event value `pruned_pair_count` equals 2
And expected event value `incomplete_pair_count` equals 0
And expected event reason codes include `tool_pair_pruned` and exclude `threshold_exceeded`

### S12: Protected context survives pruning and incomplete pairs fail closed
Priority: Must

Given a handoff contains a failed command, open security finding, user correction, migration invariant, acceptance criterion, file reference, provider reasoning signature, one incomplete tool pair, and two stale successful complete pairs
When soft pruning is attempted
Then the failed command, finding, correction, invariant, acceptance criterion, file reference, and reasoning signature remain verbatim or through a lossless stable artifact reference
And the incomplete pair is not orphaned
And expected JSON value `incomplete_pair_count` equals 1
And the next phase is blocked with reason `incomplete_tool_pair` when integrity cannot be preserved
And raw secrets and privileged absolute paths do not appear in the compaction event

### S13: Production phase transitions emit soft and hard compaction evidence
Priority: Must

Given a live worker phase transition contains stale complete pairs below the hard threshold
When the production pipeline advances to the next phase
Then the default compressor is installed without test-only injection
And a soft-prune event is emitted with input estimate, output estimate, source references, and exact pruned-pair count

Given a phase transition exceeds the 50 percent hard threshold
When the production pipeline advances
Then hard structured summarization runs
And expected event reason codes include `threshold_exceeded`
And protected summary sections include Goal, Constraints, Progress, Decisions, Relevant Files, Next Steps, and Critical Context

### S14: Sensitive, high, critical, malformed, and unknown risk select full Ultra
Priority: Must

Given fixtures for documentation-only paths, `pkg/worker` paths, authentication paths, an empty classifier input, and malformed risk JSON
When workflow risk is resolved
Then documentation-only paths resolve to low
And `pkg/worker` paths resolve to high
And authentication paths resolve to critical
And empty or malformed evidence resolves to unknown rather than low

When review allocation is resolved for high, critical, or unknown risk
Then expected JSON value `review_votes` equals 3
And expected JSON value `security_review_required` equals true
And expected JSON value `synthesis` equals true
And expected JSON value `fan_out_cap` equals 5
And expected JSON value `effort_downshifted` equals false

### S15: Eligible low and medium risk binds the compact review shape before dispatch
Priority: Must

Given an Ultra task is deterministically eligible at low or medium risk
And the run is not selected for full-depth audit
When the route-team workflow binding is serialized and rendered
Then expected JSON value `review_votes` equals 1
And expected JSON value `security_review_required` equals true
And expected JSON value `synthesis` equals false
And expected JSON value `fan_out_cap` equals 5
And expected JSON value `effort_downshifted` equals false
And the rendered workflow contains exactly one reviewer loop iteration and one mandatory security call
And all deterministic build, test, acceptance, coverage, and release gates remain scheduled

### S16: Audit selection and binding uncertainty choose full review before dispatch
Priority: Must

Given an eligible low or medium task is selected by deterministic full-depth audit sampling
When the route-team workflow binding is serialized
Then expected JSON value `review_votes` equals 3
And expected JSON value `security_review_required` equals true
And expected JSON value `synthesis` equals true
And expected JSON value `selection_reason` equals `audit_sample`

Given changed-file discovery returns no trustworthy evidence, risk input is malformed, or the compact binding cannot be validated
When route-team dispatch resolves its safe binding
Then expected JSON value `review_votes` equals 3
And expected JSON value `security_review_required` equals true
And expected JSON value `synthesis` equals true
And the exact fallback reason is recorded
And dispatch does not use the compact review shape

### S17: Review allocation does not alter implementation fan-out or scheduling
Priority: Must

Given canonical Ultra bindings for eligible low, medium, high, critical, sensitive, unknown, and audit fixtures
When review-risk binding is applied to each fixture
Then expected JSON value `fan_out_cap` equals 5 for every fixture
And implementation model, effort, retry, ownership-grouping prompt, and scheduling source are byte-equal to the canonical Ultra baseline
And no review-risk branch lowers or adapts implementation fan-out

### S18: Canonical high-risk profiles and Balanced outputs remain unchanged
Priority: Must

Given existing canonical Balanced and Ultra profile fixtures from quality and workflow tests
When the new allocation policy is disabled or any low, medium, high, critical, sensitive, unknown, and audit fixture is resolved
Then serialized Balanced phase bindings are byte-equal to the baseline fixtures
And every Ultra phase model and effort value is byte-equal to its canonical baseline fixture
And custom or pinned provider configuration is preserved
And no new user-facing quality mode appears

### S19: A/A measurement gate passes at 95 percent and blocks below it
Priority: Must

Given an A/A arm has 20 eligible calls, identical fixture/provider/model/version/effort/risk/cache/config identities, and 19 actual-complete calls
When measurement eligibility is evaluated
Then expected JSON value `actual_usage_capture_pct` equals 95
And expected JSON value `measurement_gate` equals `PASS`

Given the same A/A arm has only 18 actual-complete calls
When measurement eligibility is evaluated
Then expected JSON value `actual_usage_capture_pct` equals 90
And expected JSON value `measurement_gate` equals `BLOCKED`
And expected JSON value `rollout_decision` equals `insufficient_measurement`

Given shadow instrumentation changes objective output, call policy, or acceptance
When A/A neutrality is evaluated
Then measurement eligibility is blocked regardless of capture percentage

### S20: Paired comparison uses only common compatible task identities
Priority: Must

Given baseline has `t1=1000`, `t2=2000`, and `t3=900` raw tokens and candidate has `t1=700` and `t2=1400` with matching provider/model/version/effort/risk/cache strata
And `t1` and `t2` are accepted in both arms
When paired comparison is calculated
Then expected JSON `paired_task_ids` equals `["t1","t2"]`
And expected JSON `unpaired_task_ids` equals `["t3"]`
And expected JSON value `paired_a_raw_tokens` equals 3000
And expected JSON value `paired_b_raw_tokens` equals 2100
And expected JSON value `paired_reduction_pct` equals 30 within numeric tolerance 0.001
And expected JSON value `provisional_25pct_target` equals `PASS`
And agent calls are not treated as independent task samples

### S21: Quality regression overrides savings and target misses remain explicit
Priority: Must

Given a candidate reduces paired raw tokens by 40 percent but changes one critical baseline objective PASS to FAIL
When promotion is evaluated
Then expected JSON value `high_critical_regressions` equals 1
And expected JSON value `rollout_decision` equals `ROLLBACK`

Given a candidate has zero high/critical regression, actual coverage at least 95 percent, and median paired raw reduction 26 percent
When promotion is evaluated
Then expected JSON value `provisional_25pct_target` equals `PASS`
And expected JSON value `rollout_decision` equals `ELIGIBLE_NEXT_CANARY`

Given a candidate has zero high/critical regression, actual coverage at least 95 percent, and median paired raw reduction 24 percent
When promotion is evaluated
Then expected JSON value `provisional_25pct_target` equals `NOT_MET`
And expected JSON value `rollout_decision` equals `BLOCKED`
And the report retains the measured 24 percent result without rounding it up or presenting a 25 percent claim

### S22: Generation, architecture, and repository hygiene preserve source ownership
Priority: Must

Given implementation source changes and an otherwise clean nested repository
When focused tests, generation check, architecture enforcement, build, vet, strict SPEC validation, and repository hygiene checks run
Then all commands return success
And every changed Go source file is at most 300 lines
And installed generated surfaces are byte-consistent with canonical source generation
And no generated/runtime file is staged
And `git diff --check` reports no whitespace error
And the root meta-workspace pre-existing user changes are not modified

### S23: The exact frozen corpus and GPT/Codex live cohort are complete
Priority: Must

Given frozen corpus hash `sha256:a3454f01b734d3f72060bc9b93972032b908f88940960e7f7b0953ab7356958a`
When deterministic target preflight is evaluated
Then expected JSON value `corpus_task_count` equals 12
And expected JSON value `deterministic_pass_count` equals 12
And expected JSON value `deterministic_fail_count` equals 0

Given the v0.2.0 live cohort is frozen
When cohort identity and pair order are evaluated
Then expected JSON value `live_task_ids` equals `["ute-corpus-v1-001","ute-corpus-v1-004","ute-corpus-v1-005","ute-corpus-v1-011","ute-corpus-v1-012","ute-corpus-v1-006","ute-corpus-v1-009"]`
And expected JSON value `pair_orders` equals `{"ute-corpus-v1-001":"AB","ute-corpus-v1-004":"BA","ute-corpus-v1-005":"AB","ute-corpus-v1-011":"BA","ute-corpus-v1-012":"AB","ute-corpus-v1-006":"BA","ute-corpus-v1-009":"AB"}`
And every low or medium corpus task is present
And task `ute-corpus-v1-006` is the high sentinel
And task `ute-corpus-v1-009` is the critical sentinel
And every excluded high task retains full-profile policy parity
And each exclusion records the call/token authorization reason

### S24: Baseline and candidate use the exact role tuples and 58-call arithmetic
Priority: Must

Given all seven baseline tasks use the full five-call tuple
And candidate tasks `001`, `004`, `011`, and `012` use the compact two-call tuple
And candidate audit task `005`, high task `006`, and critical task `009` use the full five-call tuple
When planned calls are counted
Then expected JSON value `baseline_calls` equals 35
And expected JSON value `candidate_calls` equals 23
And expected JSON value `primary_calls` equals 58
And expected JSON value `xhigh_calls` equals 44
And expected JSON value `max_calls` equals 14

Given a full tuple is rendered
When role bindings are inspected
Then reviewer 1, reviewer 2, reviewer 3, and consolidator use `gpt-5.6-sol` with effort `xhigh`
And security uses `gpt-5.6-sol` with effort `max`
And the compact tuple contains exactly one `xhigh` reviewer and one `max` security call
And no child call uses effort `ultra`
And supervisor/orchestra Ultra parity remains byte-equal to its static canonical fixture

### S25: Canonical evidence is safe, normalized, and fail-closed
Priority: Must

Given one admitted live review call
When it executes through `auto agent run`
Then the execution path reaches the worker Codex adapter
And the adapter returns provider actual usage to telemetry
And direct `codex exec` output is not accepted as canonical operational evidence
And execution is read-only and ephemeral
And user config and rules are ignored
And repository discovery is skipped
And tools and optional features are disabled
And the output conforms to the strict verdict schema
And only allowlisted numeric, hash, enum, identity, verdict, finding-count, and normalized usage fields persist
And raw prompt, raw response, provider JSONL, credential, and privileged path retention all equal false

Given a tool event, non-PASS verdict, raw retention, missing actual usage, or ambiguous provider, model, effort, config, identity, or schema field
When evidence admission is evaluated
Then expected JSON value `circuit_breaker` equals `OPEN`
And expected JSON value `retry_count` equals 0
And expected JSON value `promotion_eligible` equals false

### S26: Per-call admission cannot exceed the authorized call or token cap
Priority: Must

Given 44 `xhigh` calls have a 22000-token rollout budget and 14 `max` calls have a 26000-token rollout budget
When the primary run is admitted
Then expected JSON value `primary_worst_case_raw_tokens` equals 1332000
And expected JSON value `authorization_call_cap` equals 64
And expected JSON value `raw_token_cap` equals 1500000
And expected JSON value `concurrency` equals 1
And expected JSON value `retries` equals 0
And no call starts unless its complete per-call budget fits within the remaining cap

Given the full-profile applied rollback replay requires four `xhigh` calls and one `max` call
When the complete hard envelope is pre-admitted
Then expected JSON value `required_replay_calls` equals 5
And expected JSON value `required_replay_reserve_tokens` equals 114000
And expected JSON value `planned_total_calls` equals 63
And expected JSON value `planned_call_ceiling` equals 63
And expected JSON value `planned_worst_case_raw_tokens` equals 1446000
And expected JSON value `raw_token_safety_margin` equals 54000
And all five replay calls and 114000 raw tokens are reserved atomically before primary execution
And no 64th provider call is admitted
And replay runs after every prior gate passes and expected JSON value `circuit_breaker` equals `CLOSED`
And observed primary underspend is not required for replay admission

### S27: Complete quality evidence governs promotion and applied rollback
Priority: Must

Given the primary GPT/Codex cohort finishes without a circuit break
When the strict efficiency evaluator reads the evidence
Then expected JSON value `paired_trial_count` equals 14
And expected JSON value `quality_row_count` equals 7
And every quality row has an exact expected and observed patch hash
And every quality row has deterministic verification exit code 0
And every task has mandatory security PASS evidence
And audit task `005`, high task `006`, and critical task `009` have full-depth evidence in both arms
And audit membership resolves to exactly one matching low or medium quality row with the same rollout risk
And a GPT verdict cannot override a deterministic patch, test, or security failure
And expected JSON value `high_critical_regressions` equals 0
And expected JSON value `median_raw_reduction_pct` is at least 25
And expected JSON value `rollout_decision` equals `ELIGIBLE_NEXT_CANARY`

Given a policy-parity or critical-security fault is injected into isolated rollout state
When promotion and rollback are evaluated
Then expected JSON value `rollout_decision` equals `ROLLBACK`
And expected JSON value `active_profile` equals `full_ultra`
And atomic state readback equals the written `full_ultra` binding
And actual user config and repository activation remain unchanged
And an independent diff-only review passes before any implementation claim
And SPEC status remains `approved` until the complete live evidence also passes; after it passes, only the document lifecycle may become `completed` without implying policy promotion or activation

### S28: GPT/Codex required-document snapshots are complete and fail closed
Priority: Must

Given a GPT/Codex `go` dispatch and a SPEC-review dispatch whose selected provider set consists entirely of GPT/Codex identities each have a receipt metadata-and-optional-recall budget, available architecture documents, and a supervisor-held task-specific reference set
When their required-context snapshots and serialized manifests are built
Then the `go` snapshot contains full `spec.md`, `plan.md`, and `acceptance.md` bodies
And the `review` snapshot coherently contains full `spec.md`, `plan.md`, `research.md`, and `acceptance.md`
And review requirements, identity, and injected `spec.md` bytes come from that same frozen snapshot
And every required `spec.md` metadata ID equals its containing SPEC directory ID
And available architecture documents and every supervisor-declared task-specific reference are present
And raw secrets are absent from delivered content and prompt-injection directives are neutralized while surrounding evidence and safe tail content remain present
And every required reference has a canonical `source_hash` over original source bytes, a canonical `prompt_hash` over sanitized delivered bytes, redaction metadata, a positive token estimate, and `complete=true`
And the manifest reference set exactly matches the supervisor-held command, SPEC, conditional-profile, and task-specific reference inputs
And required bodies are outside the 800–2,000-token receipt metadata-and-optional-recall budget
And optional recall alone may consume the residual budget or be omitted
And no required body is trimmed, summarized, or dropped
And the serialized manifest contains references and hashes but no prompt, layer, content, or body field

Given the base worktree has dirty context that differs from the final assigned execution worktree
When a retained direct or pipeline Codex execution resolves required context
Then snapshot construction occurs after final worktree assignment
And the provider prompt contains execution-worktree context and excludes dirty base-only context

Given a retained direct Codex run, a planner, executor, tester, and reviewer pipeline, and concurrent all-GPT/Codex review fan-out use one verified snapshot and one original task
When the direct task prompt, each phase prompt, and concurrent review prompts are dispatched while phase outputs change
Then every applicable prompt contains the complete required-document bodies and the same frozen snapshot identity
And raw task or phase text augments rather than replaces the verified snapshot
And the original task appears exactly once in the first phase without duplication
And each later phase reattaches the original task exactly once

Given an actual all-GPT `auto spec review` declares supervisor-held `--required-document` and `--conditional-profile` inputs
When revision zero or a later revise iteration builds its provider prompt
Then that revision calls `BuildContextDelivery` and then `VerifyContextDeliveryForOptions` against those supervisor-held sets
And the verified prompt contains complete core, available architecture, selected conditional-profile, and extra-reference bodies plus full `spec.md`, `plan.md`, `research.md`, and `acceptance.md`
And every required source body and each of the four SPEC documents appears exactly once
And `BuildReviewPromptFromContextDeliveryChecked` consumes the verified in-memory delivery without reloading the four SPEC documents

Given one required document is missing, empty, unreadable, tampered, stale, for the wrong SPEC, incomplete, omitted from the receipt despite the supervisor-held set, replayed from a wrong set, reference-set-mismatched, hash-mismatched, or has a `spec.md` metadata ID different from its containing SPEC directory
When required context is built or verified for compact eligibility or provider dispatch
Then expected context status equals `context_integrity_failed`
And compact selection does not occur
And expected provider call count equals 0
And expected active profile equals `full_ultra`

Given a SPEC-review provider set is mixed Codex plus Claude, Claude-only, or Gemini-only
When its review prompt is built
Then complete four-document GPT/Codex admission is not enabled
And the existing legacy review prompt behavior remains unchanged

Given a complete verified prompt exceeds 128K estimated tokens
When admission is evaluated
Then the task or review is blocked or split before a provider call
And expected provider call count equals 0
And no required content is trimmed, summarized, or dropped to fit

Given native Codex delegation guidance is rendered
When the generated agent-pipeline contract is inspected
Then it uses `spawn_agent(task_name, fork_turns="all", message)`
And it requests `context_ack` with observed source references and hashes as diagnostic evidence
And the enforceable admission gate remains supervisor-held reference-set and hash verification
And it contains neither legacy `agent_type` nor legacy `fork_context`

## Historical v1 Operational Acceptance State

The first frozen GPT/Codex primary attempt is a terminal fail-closed result, not a completed canary. It stopped at call 39 of 58 after 38 `PASS` verdicts when high sentinel task `006`, arm `B`, full-profile reviewer 1 at `xhigh` returned `FAIL` with one finding.

This section and the task006 diagnosis/transport sections through v8 preserve time-local acceptance snapshots. Their `approved`, `implemented=false`, and open-debt statements are superseded for the current SPEC lifecycle only by the later full-evaluation v2 terminal acceptance state.

| Scenario | Current evidence state | Evidence |
|---|---|---|
| S23 | Preflight complete: frozen 12-task deterministic target oracle passed 12 of 12 and the seven-task cohort was frozen. | Static cohort/preflight receipts referenced by the terminal outcome. |
| S24 | Incomplete: planned tuple arithmetic was frozen, but the live primary stopped at 39 of 58 calls. | `gpt-primary-call-ledger-v1.partial-fail.json` |
| S25 | Pass for the fail-closed branch: circuit `OPEN`, 39 of 39 actual-usage receipts, 523,811 raw tokens, tools 0, retries 0, no raw retention, and promotion false. | Partial ledger plus terminal outcome. |
| S26 | Incomplete: observed calls stayed within their caps, but the five-call replay path was not eligible or executed. | Terminal outcome. |
| S27 | Incomplete: no complete 14-trial ledger, seven-row quality ledger, strict efficiency result, or applied rollback replay exists. | Terminal outcome. |

The deterministic task `006` patch and test recheck passed, but it cannot convert the supplementary GPT `FAIL` into promotion eligibility. The strict evaluator and rollback replay path each rejected the partial ledger; the replay rejection used zero provider calls. Candidate activation, user config mutation, and repository policy activation remain false, and `full_ultra` remains active.

The cumulative-authorization P1 is resolved through canonical output binding and an ignored atomic runtime claim. Reconciliation marks the current policy hash as `CONSUMED_ON_RECONCILIATION`; canonical and noncanonical actual reuse probes both failed admission with zero provider calls. The final independent diff-only review is `PASS`, with open P0/P1/P2 counts of 0/0/0. Independent review is therefore closed and is not Completion Debt.

### Terminal evidence index

| Evidence | SHA-256 |
|---|---|
| `evidence/gpt-primary-call-ledger-v1.partial-fail.json` | `sha256:f1b2fc2171af84464c6e5e7f39d5db62918480740d60abe4fabc93784987b582` |
| `evidence/gpt-primary-terminal-outcome-v1.json` | `sha256:29f0a73fe758e4b564870922269bce24297795e8ba3040f346fca610ef1007b8` |
| `evidence/gpt-authorization-closure-v1.json` | `sha256:bde1c49d7458f43a1cb9bfd478bd84991645b161d6b9f9e7794f042e2051bf42` |

The SPEC stays `approved` with `implemented=false`. A later live attempt requires a newly frozen policy hash and explicit authorization; the incomplete ledger and consumed authorization are not reusable.

### Diagnostic-only task006 acceptance state

The diagnostic parser extension is authorized only for diagnostic mode and accepts bounded `finding_code` and `scope_hash` fields. The primary strict parser is unchanged. The approved diagnostic protocol is non-promotional and freezes task `006` arms `A` and `B` at full5 for 10 planned calls, eight `xhigh` plus two `max`, with a 228,000-raw-token cap.

| Diagnostic check | Current evidence state |
|---|---|
| New authorization | `sha256:920e6370cebb84739872233cd4a0eeb88295bf816b19b6d43cfac99591a1dc20`; single-use and consumed. |
| New policy | `sha256:4a4b84f7087a5bf40aa0f5c3c2e883e29d235e80bf91c84c1186ec758248b12f`; no promotion. |
| Exact plan | task006 A/full5 plus B/full5; 10 calls, 8 `xhigh`, 2 `max`, at most 228,000 raw tokens. |
| Execution | Incomplete: call 1, arm A, reviewer 1, `xhigh` ended `process_nonzero`. |
| Usage and tools | attempted 1, observed 1, actual usage 0, raw tokens 0, tools 0, retries 0. |
| Diagnostic result | No bounded finding was admitted; finding count 0 and no diagnostic quality conclusion. |
| Deterministic authority | Patch hash and verification command PASS; this does not establish diagnostic quality. |
| P1 receipt hardening | A nonzero process bypasses telemetry and `build_diag_row`; only schedule-bound metadata and a sanitized failure stub may persist. The historical ledger is not retroactively validated. |
| Reuse safety | Same-authorization probe exit 1, sentinel invocations 0, provider calls 0, raw tokens 0, and runtime claim hash unchanged. |
| Rollout state | Promotion false, candidate inactive, primary completion unchanged, `full_ultra` active. |

#### Diagnostic evidence index

| Evidence | SHA-256 |
|---|---|
| `evidence/gpt-diagnostic-cohort-v1.json` | `sha256:d060fe3ae06e5ee063c66a57dc7b6a96bd9c929361e81ca2e26d48d333db0d9f` |
| `evidence/gpt-diagnostic-config-v1.json` | `sha256:bff64292fdf49343bde1a53d9d40ffbea0af5f338cd48385c2a2dc6d752e0565` |
| `evidence/gpt-diagnostic-policy-v1.json` | `sha256:4a4b84f7087a5bf40aa0f5c3c2e883e29d235e80bf91c84c1186ec758248b12f` |
| `evidence/gpt-diagnostic-verdict-schema-v1.json` | `sha256:d81f0205cbc02ac7af0d0897078041f765873ba287b8dfd2c0dd3e66f35ca605` |
| `evidence/gpt-diagnostic-preflight-v1.json` | `sha256:83d79efa72892605276746e114ce740113e97d5225dbf5b6ab4bd1519f4de552` |
| `evidence/gpt-diagnostic-call-ledger-v1.partial-fail.json` | `sha256:d0d3881e6fdad03c3289761a535f3f34f5603001c0d96af9c66acea840ac6ee0` |
| `evidence/gpt-diagnostic-terminal-outcome-v1.json` | `sha256:9869bbceea0a5ba2db05b48980f4cda44ede8dceda1e2325e678826155a13892` |
| `evidence/gpt-diagnostic-authorization-reuse-v1.json` | `sha256:340e59d8791403853d5d4281bb02b0cdb4fb2af5d1c01ecc5644f15719649ebd` |

The diagnostic protocol is blocked and incomplete. It does not close S24, S27, or any primary Completion Debt. A new live attempt requires transport-failure diagnosis, a new frozen policy hash, and explicit authorization.

### Transport-only smoke acceptance state

The transport smoke used a separate single-use authorization and did not perform semantic evaluation.

| Transport check | Current evidence state |
|---|---|
| Authorization | `sha256:7078b87735deb9026654c38ae04305ab8874099ad99c5b4ae37d9956e0232b27`; consumed and not resumable. |
| Frozen artifacts | Policy `sha256:38b6ae94b0edf4c9cf09a505a5ff1b4f8cec17a478306cde99b95d9aec2411a3`; config `sha256:9dd237bf913b7ac30d4002e733b8299c007f7cb1646a0c1020a9dbbdc1bc2e34`; schema `sha256:61006491dddaadb43822608d10af5c3baa2e166950973dec95655ffd28003ced`. |
| Runtime identity | `0.50.68-ute-transport-smoke-v1`; executable `sha256:b90c7445ca8365ccf20ea044a4793f7f8bd16a4cd0f7385915b605e6493518d5`. |
| Approved call | task006, reviewer, `xhigh`, one call, 22,000-token cap, retries 0. |
| Terminal result | `process_nonzero`; attempted 1 and actual-usage calls 0. |
| Usage | `unavailable`; observed raw tokens null and tool-call count null. Neither value is reported as zero. |
| Evaluation | Transport conclusion unavailable; semantic evaluation false; promotion false. |
| Rollout state | `implemented=false`; `full_ultra` remains active; S24 and S27 remain blocked. |

#### Transport-smoke evidence index

| Evidence | SHA-256 |
|---|---|
| `evidence/gpt-transport-smoke-policy-v1.json` | `sha256:38b6ae94b0edf4c9cf09a505a5ff1b4f8cec17a478306cde99b95d9aec2411a3` |
| `evidence/gpt-transport-smoke-config-v1.json` | `sha256:9dd237bf913b7ac30d4002e733b8299c007f7cb1646a0c1020a9dbbdc1bc2e34` |
| `evidence/gpt-transport-smoke-schema-v1.json` | `sha256:61006491dddaadb43822608d10af5c3baa2e166950973dec95655ffd28003ced` |
| `evidence/gpt-transport-smoke-preflight-v1.json` | `sha256:d92a3cd44da58a7daff27a60245b6a4c6197c6b5d3cc6a1596384784237c90a4` |
| `evidence/gpt-transport-smoke-ledger-v1.partial-fail.json` | `sha256:eca927f2802b734336d5c34bd3833fb1a4385c93eff6f97a825bf153cbbe37c6` |
| `evidence/gpt-transport-smoke-terminal-outcome-v1.json` | `sha256:b92db43b32a9880b8e7e6987ba6398a50ccaef323f709beef698b07f6438f268` |

The transport smoke is blocked and incomplete. It provides no semantic or promotion evidence and does not close any primary Completion Debt.

### Transport diagnosis v2/v3 acceptance state

The two later diagnosis attempts remained GPT/Codex-only and used separate single-use authorizations. Each admitted one task006 reviewer call at `xhigh`, a 22,000-raw-token cap, concurrency one, and zero retries.

| Acceptance check | v2 | v3 |
|---|---|---|
| Authorization | `sha256:345523b25569eee5f691d3960c2caca0709aebcd324c7db103cb3c2b0ecf013f`; consumed once and not resumable | `sha256:db4f738c9226c55e339d8e52e875ac7e7b3aefe4efdf0b0372066fcb325a1de9`; consumed once and not resumable |
| Frozen policy/config/schema | `sha256:e60cc741ea5a2abd0be0ba7e25d5d6c639fe9e2b0cd52c10dcb14ac1565b6cdc` / `sha256:bdec20a7c73616afdcd5c70fa069f88a392a8da928bb90c8f4e192af657f5c80` / `sha256:0f77892d0472dedf2e6cdee5a9064e7e0798ad29e43769c87a00aff26f159306` | `sha256:b5c627569f95733511d10dda2c67b034e25f645fd292fe7b90f7bd1913180d07` / `sha256:f05ca25b74cdd748d97919c31357cc3676cf15c1f41b53f830232fe070d56eb6` / `sha256:ab787a1b9581b76160ffc48b1c22659865caa250a5f20de4249fb0ec6b81ec8d` |
| Runtime | `0.50.68-ute-transport-diagnosis-v2`; executable `sha256:fc5bf47bb3db020876605d7450661e732817790ab9190895bd6f78726096360a` | `0.50.68-ute-transport-diagnosis-v3`; executable `sha256:9e118039a9e1027087b578935c410c43187d1e9ec583eb4258d6c223c9b770a5` |
| Execution | `process_nonzero`; attempted 1; actual-usage calls 0; usage unavailable; raw total null; retries 0 | `process_nonzero`; attempted 1; actual-usage calls 0; usage unavailable; raw total null; retries 0 |
| Diagnosis | Class `unknown`; fingerprint `sha256:e161c851c1a8c4fdea86c031ea524f1e0c7d39c7399eb950ea081fa8d90f0a42` | Class `unknown`; same fingerprint; provider detail unclassified; failure-source metadata unavailable |
| Decision | Transport conclusion unavailable; semantic evaluation false; promotion false | Transport conclusion unavailable; semantic evaluation false; promotion false |

Current raw-free `operational_error_stage` and `operational_error_signals` handling passes hermetic positive and invalid-value fail-closed tests. This is accepted as current implementation hardening only. The immutable v3 ledger did not retain either field, so it is not accepted as evidence of the v3 root cause.

The acceptance decision remains `approved`, not implemented: `implemented=false`, effective profile `full_ultra`, S24 and S27 unmet, and CD-02/CD-03 open. The full 58-call primary plus 5-call replay is prohibited and unapproved until Transport PASS. A new live attempt requires a newly frozen policy plus explicit authorization and remains limited to GPT through Codex; Claude/Gemini live scope is not added.

#### Transport-diagnosis v2/v3 evidence index

| Evidence | SHA-256 |
|---|---|
| `evidence/gpt-transport-smoke-ledger-v2.partial-fail.json` | `sha256:3f5e76621709e9a77e222a317d34ebad664bbc1735c1f1ae2cec1f9fb2a51c6f` |
| `evidence/gpt-transport-smoke-terminal-outcome-v2.json` | `sha256:1723f7906b5e544fd9c7c3b7418bf9fd3522254e4d434da6987a34776a9c7196` |
| `evidence/gpt-transport-smoke-ledger-v3.partial-fail.json` | `sha256:32b0144aa61e34bad7b277390ac341e57f9cd0a2c1cefb9d240b45cd743735d8` |
| `evidence/gpt-transport-smoke-terminal-outcome-v3.json` | `sha256:ac0e928e43d1da5a4ed609e255ea2d67ef9361cbec27fa6fc42a5a96c518b23e` |

### Transport diagnosis v4 live-terminal acceptance state

The frozen v4 package received explicit authorization for one GPT/Codex transport-only call. Authorization `sha256:11bc59a14df0ce2c77e44e28bf644b1a8e07d6d63825189b81988b1da51a1a03` was consumed once and is not resumable.

| Acceptance check | V4 terminal state |
|---|---|
| Evidence chain | Policy `sha256:a562141171b03e3ffc61aee968dc09a1760b1216448469d8db95788ec33654bf`; config `sha256:1dce4c89d3e52231bb1cdeb54c96f2a6bbf82004b1ac14e3fcabd7d2f1821711`; schema `sha256:a90b361e401497102d90b563082f38ff0a3600376dffdef9b4ebfe106ccb1421`; preflight `sha256:da363839a19dc63805e1f5574759a3eb1a1d6fcc73ea340e5a665cc6a19e78cb`; ledger `sha256:7fdd5f75b610e71fceabbb910b477b2d00ec838f02b048b0d073b3bb16d04383`; reservation `sha256:078042f1035b9b29051dff784bc7c456b5c534a39550b2e155c7411461ffd6e6`; terminal `sha256:27a2cae7b3470ab219153738dbf75d9d35a24c342ed88aae00e2b7bad2edad18`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v4`, executable `sha256:f7d4c12f354e3f8b106e68ada07286e658f888b32b3467f8a9b6137debd89bd9`; harness `sha256:9078eee0b022d5da750762942585d5189245d9b12eddc0dafb6b1e8ccadb5c42`; Codex `0.144.1`. |
| Execution | Task006 reviewer `xhigh`; attempted 1; retries 0; `process_nonzero`; actual-usage calls 0; observed raw tokens null; usage unavailable. |
| Failure source | Class `unknown`; fingerprint `sha256:e161c851c1a8c4fdea86c031ea524f1e0c7d39c7399eb950ea081fa8d90f0a42`; stage `process_wait`; signals `[provider_failure_event]`; stderr observed false. |
| Retention | Raw stderr, provider-error event, prompt, response, and absolute paths were not retained. |

The source signal is accepted only as evidence that a provider failure event was observed. It does not accept event detail, root cause, Transport PASS, semantic evaluation, or promotion. SPEC remains `approved`, `implemented=false`, effective profile `full_ultra`, S24/S27 unmet, and CD-02/CD-03 open. The 58+5 sequence remains prohibited. The next live attempt requires a new frozen policy and new explicit authorization, stays GPT/Codex-only, and cannot expand to Claude/Gemini live scope.

### Transport diagnosis v5 live-terminal acceptance state

The frozen v5 package received explicit authorization for one GPT/Codex transport-only call. Authorization `sha256:036f1f0534fddaf72897ea5062ebd5747c558a725a47fa0149fd8de033469a64` was consumed once and is not resumable.

| Acceptance check | V5 terminal state |
|---|---|
| Evidence chain | Policy `sha256:49b57c44cfef9105dd93d92441b3c17c1d678cfb6b04527da2fec81c78388f19`; config `sha256:42b57ec9d7619263dd78cabff60570cdc7051203270625558b798a036428b074`; schema `sha256:2c6da9ba42f09487b4c7a8d1704e08133cbfe2b05ea7001101fea92a23994d6c`; authorized preflight `sha256:a3548711c44c3dc3b776cc38ef7460417795b9485dac9f70f2b5c37d998cea16`; ledger `sha256:df2f065ba9e0f5e9548e64e69eb92f36497f0a975da150abd28327ebaa30cc3a`; reservation `sha256:f4593e9e203e7b32ef9e8985fae944967827b64794c15b6c2e45be3f2c13294e`; terminal `sha256:40a7abef2e1d2d9da5477352b09277892b138b4ec17ceee68240189b2ec389a0`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v5`, executable `sha256:b4d67aac8b946f067c2df3c3656422647947a7ab28757d8bab0e671f9b620f37`; harness `sha256:82d0b30338fbc467d80b507411cf2bdd5e8cc7d3dd1f60de973bd4e2c4bb4e14`; Codex `0.144.1`. |
| Execution | Task006 reviewer `xhigh`; attempted 1; retries 0; `process_nonzero`; actual-usage calls 0; observed raw tokens null; usage unavailable. |
| Failure source | Class `unknown`; stage `process_wait`; signal `[provider_failure_event]`; kind `error_and_turn_failed`; shape `[top_level_message, nested_error_object, nested_error_message]`. |
| Retention | Provider-event values and raw sources were not retained. |

The kind/shape receipt is accepted only as sanitized failure-source metadata. It does not accept a root cause, Transport PASS, semantic evaluation, or promotion. SPEC remains `approved`, `implemented=false`, effective profile `full_ultra`, S24/S27 unmet, and CD-02/CD-03 open. The 58+5 sequence remains prohibited. Any next live attempt requires a new frozen GPT/Codex policy and new explicit authorization, with no Claude/Gemini live expansion.

## Task006 Transport Diagnosis v6 Terminal Acceptance State

The exact v6 authorization was granted, reserved, and consumed by one non-resumable GPT/Codex transport-only attempt. The accepted result is a sanitized terminal operational failure, not a Transport PASS or semantic verdict.

| Acceptance check | V6 terminal result |
|---|---|
| Artifact binding | Policy `sha256:d2da17f57fa900281d350a186dd7ed0e2aa6dcda9957c03df00e200eeca33495`, config `sha256:d88c97544931b2ac9b409100656220bf20da75b134a7208f348fa3bf49345e78`, schema `sha256:4d7a5cb92e32f45897a3a21adea0c638dd75893b109b2cb20205ba307379b986`, authorized preflight `sha256:c0eb6253943680908eaa1a9027469cf5ba6d5a44b631bee380b35c8b92fbb1b2`, partial ledger `sha256:f583d987fa793b98c89553c33789f8e8c88b4429546a341c8d09022b671d709f`, terminal outcome `sha256:35f76e17743874c7878f26743b6eacc75e201dbe0fc8897f23ee7b1f2a1e4905`, and reservation `sha256:d0a5acbe560f0a49e498ca46a452daf20701bb594c4e8cc78aa6192d742f5dfe` are cross-bound. |
| Execution bounds | Call cap 1, attempted 1, retries 0, single-use claim consumed; completion false, usage unavailable, actual-usage calls 0, observed raw tokens `null`. |
| Sanitized receipt | `process_wait` / `provider_failure_event` / `schema_or_response`; canonical `error` and `turn_failed` receipts each retain only allowlisted shape and trait metadata with explicit empty status-family arrays. |
| Privacy | Raw stderr, provider-event values, provider-event raw hashes, prompt, response, request identifiers, and absolute paths are absent. |
| Regression and immutability | Producer/consumer review remains P0/P1/P2 `0/0/0`; all 106 pre-v6 evidence files remain byte-identical to the frozen baseline. |

Acceptance does not classify a root cause from `schema_or_response`, accept a stable undocumented failure-payload schema, or accept Transport PASS, semantic evaluation, promotion, S24/S27, CD-02/CD-03, or the 58+5 sequence. Status remains `approved`, `implemented=false`, and effective profile `full_ultra`; another live attempt requires a new frozen GPT/Codex identity and explicit authorization.

## Task006 Transport Diagnosis v7 Provider-Zero Frozen State

This subsection is the freeze-time historical snapshot from before the later v7 authorization and terminal run recorded below.

The accepted provider-zero reconstruction is that v6 copied `gpt-transport-smoke-schema-v6.json` to runtime basename `gpt-diagnostic-verdict-schema-v1.json`, and both arrays in that actual runtime schema omitted `items`. This conflicts with the Structured Outputs contract, but without a raw provider message the classification is `STRONG_HYPOTHESIS_NOT_CONFIRMED_ROOT_CAUSE`. The separate diagnostic schema v1 `allOf`/`if`/`then`/`else` content was not v6 runtime content and is not accepted as its cause.

| Acceptance check | V7 frozen result |
|---|---|
| Artifact binding | Policy `sha256:e03ab4a20c4c6cea4a82364d9f86b782556c800dcd4a78d875d3673095384d66`, config `sha256:d3c1a5747c3dc1626fad4e4636086eab32f94b54fa9ce2f479ed388965ee299d`, schema `sha256:ceedc01912682cbb2cf870a7e0cd00c7096f48449d9f8602e9e52d92449b94e4`, preflight `sha256:309fd921b8bee649b002cef7286c4f1dc576bdd6b07d7e165bdcafad2cb958bc`, and authorization identity `sha256:fa56811f74e755711208878b2eb7e41db6071bb83e2208f8fd95024cb6d8d72a` are frozen. |
| Runtime binding | Auto `0.50.68-ute-transport-diagnosis-v7` at `sha256:10099d5ceb8aad68920e1ac4fd1aa8204dd2ec9c29d23f7b09303bc5d6306d69`; harness `sha256:ba9330911d8f34d72e5e868c7baf2236ede22b9f1bd4c0a411bcaa2ce86a8f50`; Codex `0.144.1`. |
| Exact schema | Root object; all four fields required; `additionalProperties:false`; verdict enum `PASS`; finding-count enum `0`; both arrays define string `items`; `$schema`, `$id`, `const`, `maxItems`, `minItems`, `uniqueItems`, `pattern`, and composition absent. The local postvalidator requires PASS/count 0/empty arrays. |
| Fail-closed fixtures | Exact schema and actual runtime-copy hash verification precede claim creation. Missing `items`, `allOf`, missing required fields, and `additionalProperties:true` produce provider/Auto/claim `0/0/0`; approval-pending also produces `0/0/0`; fake success/failure pass; v1-v6 regressions and evidence immutability pass; v6 raw-free failure receipts remain enforced. |
| Freeze-time live boundary | `AWAITING_EXPLICIT_LIVE_AUTHORIZATION`; provider receipt false; 0 calls/0 raw tokens were preflight-only; provider execution was false and claim was absent. |

At freeze time, acceptance required separate authorization for the exact v7 identity. The later exact `실행승인` satisfied only that admission requirement; it did not accept root cause, Transport PASS, semantic evaluation, promotion, usage, or completion.

## Task006 Transport Diagnosis v7 Terminal State

The exact `실행승인` bound authorization identity `sha256:fa56811f74e755711208878b2eb7e41db6071bb83e2208f8fd95024cb6d8d72a` at `2026-07-15T11:42:07+09:00`. It was consumed by one GPT/Codex transport-only attempt and is non-resumable.

| Acceptance check | V7 terminal result |
|---|---|
| Terminal binding | Policy `sha256:e03ab4a20c4c6cea4a82364d9f86b782556c800dcd4a78d875d3673095384d66`, config `sha256:d3c1a5747c3dc1626fad4e4636086eab32f94b54fa9ce2f479ed388965ee299d`, schema `sha256:ceedc01912682cbb2cf870a7e0cd00c7096f48449d9f8602e9e52d92449b94e4`, authorized preflight `sha256:33b1a1a266d722a32fd40abe93d9878c656e022f58a6b40e5726676cf651c4ab`, reservation `sha256:3a3939b2373d3335c1612e048a847db12bae3c5436430f487f715c1c0698f4cb`, partial ledger `sha256:a614cf254c02bbbe178f3b818cf50eae34b8191d3251db7834ae85aac185b73f`, and terminal `sha256:9e6db74077d64d72b515497a640b21891e7d65cd3780a505d2ca1800cb19f0f6` are bound. |
| Execution | Planned/attempted `1/1`, retries 0, exit 1, failure `missing_or_invalid_result`, completed false. |
| Usage | Actual-usage calls 0, raw-token total `null`, usage unavailable; zero usage or cost is not accepted. |
| Operational metadata | Class/fingerprint/stage `null`; serialized signals/events empty; event/stderr observation and operational receipt classification unavailable; canonical result and usage receipts false. This proves neither provider-event/stderr absence nor a cause. |
| Decision | Transport conclusion unavailable; root cause unclassified; semantic false; promotion false. |

Acceptance records v7 only as a consumed terminal operational failure. It proves neither schema-fix success nor failure, and v6 missing `items` remains an unconfirmed strong hypothesis. Never retry or reuse v7. Any later live attempt requires a new policy/config/schema identity and separate explicit authorization. GPT/Codex-only scope, `approved`, `implemented=false`, `full_ultra`, open S24/S27 and CD-02/CD-03, the 58+5 prohibition, and no Claude/Gemini live expansion remain unchanged.

## Task006 Transport Diagnosis v8 Provider-Zero Frozen State

This subsection is the freeze-time historical snapshot from before the later v8 authorization and terminal PASS recorded below.

Provider-zero acceptance reproduces `missing_or_invalid_result` when Auto's YAML `omitempty` omits empty PASS `finding_codes` and `finding_scope_hashes` but the smoke consumer requires explicit `[]`. The historical v7 raw/result artifact was not retained, so the accepted evidence level is `LOCALLY_REPRODUCED_HISTORICAL_RESULT_NOT_RETAINED`; exact v7 causality is not accepted.

| Acceptance check | V8 frozen result |
|---|---|
| Artifact binding | Policy `sha256:a8452ea635e09298b71044a00dddef02b13ac7073f828e4cf902add6d8a6b845`, config `sha256:2d714d493ca29d9b1bcea87dfd96e8cef0e1278edd5e34bf15053e9430fc3f39`, schema `sha256:ceedc01912682cbb2cf870a7e0cd00c7096f48449d9f8602e9e52d92449b94e4`, preflight `sha256:66c05018a7edfec0d773ada618566dffb3f71dcb66ed6a6c64a9c114f0f64c7a`, and authorization identity `sha256:c524d53a9b12d84457ee2ad44c7149c31648c35227ef066f938467af8a74945a` are frozen. |
| Runtime binding | Auto `0.50.68-ute-transport-diagnosis-v8` at `sha256:5744f2c4190a1f557320bd6f8034dd809e88bec626b237df1dd8d403fdc148f8`; harness `sha256:3caf3539135f03c1b613b7bf0468674736520d8a841961822f5ae1131814c8c8`; Codex `0.144.1`. |
| Consumer contract | `((has("finding_codes") | not) or .finding_codes == [])` and the equivalent scope-hash check accept only missing keys or explicit empty arrays; explicit `null` and nonempty findings remain rejected as `missing_or_invalid_result`. |
| Verification | RED→GREEN, including RED null accepted → GREEN null rejected; explicit-empty/omitted-empty success PASS, null/nonempty rejection, sanitized failure PASS, generic/v6/v7 regression PASS, and all 130 v1-v7 evidence files immutable. |
| Freeze-time live boundary | `AWAITING_EXPLICIT_LIVE_AUTHORIZATION`; provider receipt false; provider/Auto/claim was `0/0/0`; observed raw-token 0 was preflight-only. |

At freeze time, consumed v7 authorization could not authorize v8 and exact v8 authorization remained pending. The later authorization satisfied only v8 transport admission; the resulting acceptance state follows.

## Task006 Transport Diagnosis v8 Terminal State

Exact identity `sha256:c524d53a9b12d84457ee2ad44c7149c31648c35227ef066f938467af8a74945a` was authorized at `2026-07-15T12:48:10+09:00`, consumed by one GPT/Codex transport-only call, and is non-resumable.

| Acceptance check | V8 terminal result |
|---|---|
| Terminal binding | Policy `sha256:a8452ea635e09298b71044a00dddef02b13ac7073f828e4cf902add6d8a6b845`, config `sha256:2d714d493ca29d9b1bcea87dfd96e8cef0e1278edd5e34bf15053e9430fc3f39`, schema `sha256:ceedc01912682cbb2cf870a7e0cd00c7096f48449d9f8602e9e52d92449b94e4`, authorized preflight `sha256:76fa7e15ea7114428d74ef9520a7055778c361ea4f702e69c8cabd2e89a76353`, ledger `sha256:f81df2cc4b083d2649bcd948328003a434411a394aea98a82aa095ee86e6593d`, reservation `sha256:b300d143c097ed6bcc4f3af61304ad39e78e611a33db189cbd8cf66c77824142`, and terminal `sha256:13f31960752167111bc202899721f67e05b3c3d6da5c71372f778bfe65776792` are bound. |
| Runtime binding | Auto `0.50.68-ute-transport-diagnosis-v8` at `sha256:5744f2c4190a1f557320bd6f8034dd809e88bec626b237df1dd8d403fdc148f8`; harness `sha256:3caf3539135f03c1b613b7bf0468674736520d8a841961822f5ae1131814c8c8`; Codex `0.144.1`. |
| Execution | `TRANSPORT_DIAGNOSIS_TERMINAL_PASS`; completed true; planned/attempted `1/1`; canonical actual usage 1; raw tokens 16,094 ≤ 22,000; unique model call 1; tool calls 0; retries 0; schema conformance PASS. |
| Retention | No partial ledger or raw runtime files remain; raw prompt/response, provider stdout/stderr, provider-event values, and absolute paths are not retained. |
| Acceptance boundary | Transport prerequisite is satisfied. Missing-key normalization was not retained/observed live; exact v7 cause stays unclassified; semantic evaluation, promotion, and implementation are false. |

Acceptance closes only the Transport prerequisite. V8 authorization cannot be reused. Full 58+5 remains unapproved and non-executable until a separate full-evaluation admission and budget are frozen and explicitly authorized. Status remains `approved`, `implemented=false`, and `full_ultra`; S24/S27 and CD-02/CD-03 remain open. Scope remains GPT/Codex-only with no Claude/Gemini live expansion.

## Full-Evaluation v2 Provider-Zero Acceptance State

Provider-zero acceptance passes for the separately frozen full-evaluation identity `sha256:129521ff443c4ec01bc71cbb621c1dd3d515d5f460130a16732bf639b52e4978`.

| Acceptance check | Result |
|---|---|
| Frozen identity and runtime | Full17 `sha256:9c449510ae7918f2d066119cfc2374739e4ae7e83100321645c0c79710afc626`, admission9 `sha256:45711d43d420414be3ec462155542d9eeb0d4dd5d36daec9dee1ae801bd616ab`, Auto `sha256:5744f2c4190a1f557320bd6f8034dd809e88bec626b237df1dd8d403fdc148f8`, and resolved Codex `sha256:134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477` match the frozen identity. |
| Static evidence | Eight JSON artifacts and eight sidecars verify; the prior manifest binds 142 artifacts without absolute paths; deterministic preflight is 12/12 PASS. |
| Budget oracle | Primary `58 / 1,332,000`, replay `5 / 114,000`, planned `63 / 1,446,000`, caps `64 / 1,500,000`, concurrency 1, retries 0, and no scheduled 64th call. |
| Admission oracle | Full17 and admission9 are checked before helper sourcing, claim, reservation, and provider execution, then full17 is rechecked at primary/replay phase and call boundaries. |
| Evaluator oracle | Exact14 security/quality/evaluator artifacts are hash-bound. Task005 provides the isolated 100% audit receipt while primary sampling remains 20%; high006 and critical009 use full-depth rate-0 experiment identity/result-digest binding and cannot masquerade as audit samples. |
| Replay/finalizer oracle | Applied rollback and evaluator summary bind the nested five-call reservation; replay ledger, terminal, and closure bind the primary reservation/ledger, exact summary, nested reservation, replay, and combined budget. |
| Fail-closed tests | v2 admission and evaluator/finalizer suites PASS, including task swap, sidecar replacement, phase-boundary tamper, nested reservation, summary/manifest, ledger, and finalizer negatives with provider zero. V1 canary/evidence regressions also PASS. |
| Independent review | Guardian reports P0/P1 `0/0`; the three prior P1 findings are CLOSED. |
| Freeze-time live boundary | Decision `AWAITING_EXPLICIT_FULL_EVALUATION_AUTHORIZATION`; provider calls, authorization, reservations, v2 claims, and dynamic v2 artifacts were all zero at this checkpoint. |

This subsection accepts the v2 admission package at freeze time, not live execution. At that checkpoint, S24 remained without a new 58-call primary, the remaining S26 replay path had not run, S27 had no live exact14 quality ledger, and CD-02/CD-03 remained open. The later exact-identity authorization admitted `primary 58 → provider-zero evaluator → gated replay 5 → provider-zero finalizer` and produced the terminal state below.

## Full-Evaluation v2 Terminal Acceptance State

Exact identity `sha256:129521ff443c4ec01bc71cbb621c1dd3d515d5f460130a16732bf639b52e4978` received explicit single-use authorization at `2026-07-15T17:27:52+09:00`. Sidecar verification and the independent Guardian review accept the complete ordered chain.

| Acceptance check | Terminal result |
|---|---|
| Exact admission | Authorization `sha256:875d17bf450b78fa656263f7f5676cd56b36d5ea33748e34d9f81efa861ac677`; primary reservation `sha256:bd733533ab255957154e2b31cd905c165c6f0c9719671ace7909116a620e648e`; concurrency 1; retries 0; hard caps 64 calls and 1,500,000 raw tokens. |
| S24 | PASS: primary 58/58, baseline/candidate `35/23`, efforts `44 xhigh / 14 max`, actual usage 58/58, and observed raw tokens 1,071,031. All verdicts PASS, including task006 at the historical v1 failure boundary. |
| S25 | PASS: tools 0, modified files 0, retries 0, circuit `CLOSED`, raw prompt/response/provider output and absolute-path retention false. Exact full-evaluation partial-ledger count is 0. |
| S26 | PASS: nested replay reservation `sha256:032eec05a180733e14a0fccf0f375869b74c490010e799d5e5ddba2bbfec4edb`; replay 5/5, efforts `4 xhigh / 1 max`, raw tokens 90,735; combined 63 calls and 1,161,766 raw tokens, 338,234 below the authorization cap, with no 64th call. |
| S27 | PASS: evaluator provider calls 0; paired trials 14; quality 7/7; security 14/14; task005 audit plus high006 and critical009 PASS; measurement/neutrality PASS; median paired reduction 59.918%; high/critical regressions 0; applied `ROLLBACK` and atomic `full_ultra` readback PASS. |
| Terminal closure | Terminal `sha256:982863631858aad2a8eafc8eec4aa23218cf66b83110aed340fceb590098f376` is success true and `ELIGIBLE_NEXT_CANARY`; closure `sha256:a2e7af9b2fee116318316864fcae4802da53ec2843f4b648ab15c8523d8ade7c` is consumed true and reusable false. |
| Independent gate | Guardian final P0/P1 `0/0`; S24/S26/S27 PASS; CD-02/CD-03 resolved; blockers 0. |

The terminal result accepts the remaining live completion evidence and permits SPEC/document lifecycle `completed` with lifecycle `implemented=true`. It does not accept policy promotion, default activation, or user/repository mutation. The terminal and closure artifacts deliberately retain `promotion_eligible=false`, `activation_eligible=false`, and `implemented=false`; effective profile remains `full_ultra`.

### Full-evaluation v2 terminal evidence index

| Evidence | SHA-256 |
|---|---|
| `evidence/gpt-full-evaluation-identity-v2.json` | `sha256:129521ff443c4ec01bc71cbb621c1dd3d515d5f460130a16732bf639b52e4978` |
| `evidence/gpt-full-evaluation-authorization-v2.json` | `sha256:875d17bf450b78fa656263f7f5676cd56b36d5ea33748e34d9f81efa861ac677` |
| `evidence/gpt-full-evaluation-reservation-v2.json` | `sha256:bd733533ab255957154e2b31cd905c165c6f0c9719671ace7909116a620e648e` |
| `evidence/gpt-primary-call-ledger-v2.json` | `sha256:a8ceed2ebceae72bb120c84ecc6ace10e6ee0c2f4678c3eaa340ab86a3602aab` |
| `evidence/gpt-primary-evaluation-summary-v2.json` | `sha256:7ee81e2bf2f71ebc5d9a09471aa8b1f4b1e5513c0b69d60dfc161aff9a064f4e` |
| `evidence/gpt-applied-rollback-v2.json` | `sha256:a00bed018f5c5f4482fcec2dacdd5a07c82f43e5e936ecf27e7042fe9d482def` |
| `evidence/gpt-rollback-reservation-v2.json` | `sha256:032eec05a180733e14a0fccf0f375869b74c490010e799d5e5ddba2bbfec4edb` |
| `evidence/gpt-rollback-call-ledger-v2.json` | `sha256:e3b3dd78cbffa28419b6f9eabd88a11799370959400083b5322e62979cfeeba5` |
| `evidence/gpt-full-evaluation-terminal-outcome-v2.json` | `sha256:982863631858aad2a8eafc8eec4aa23218cf66b83110aed340fceb590098f376` |
| `evidence/gpt-full-evaluation-authorization-closure-v2.json` | `sha256:a2e7af9b2fee116318316864fcae4802da53ec2843f4b648ab15c8523d8ade7c` |

## Edge Cases

### Edge Case 1: AC-023 — A provider emits usage after the final message
Priority: Must

Given a provider emits the final assistant message before a later completion usage event
When the full stream is consumed
Then the final output and the later usage receipt are both present in one task result
And model-call identity is counted once

### Edge Case 2: AC-024 — A provider reports separate reasoning tokens
Priority: Must

Given input total is 1000, output total is 200, reasoning tokens are 50, and reasoning is explicitly declared separate from output
When raw total is calculated
Then expected value `raw_total_tokens` equals 1250

Given the relation is not declared
When raw total is calculated
Then expected value `raw_total_tokens` is null
And expected reason equals `component_relation_unknown`

### Edge Case 3: AC-025 — A classifier command cannot inspect the repository
Priority: Must

Given changed-file discovery returns an error
When team workflow risk is resolved
Then risk equals unknown
And allocation equals the current full Ultra profile
And the error is recorded without exposing an absolute privileged path

## Oracle Acceptance Notes

The authoritative formulas are:

```text
raw_total_tokens =
  input_tokens_total
  + output_tokens_total
  + reasoning_tokens only when reasoning_relation=separate
  + tool tokens only when their relation=separate

raw_total_tokens_per_accepted_task =
  all actual-complete eligible attempt tokens in the arm
  / distinct final accepted task count
```

Rules:

- Cached input is a subset of inclusive input and is never added again.
- Reasoning marked `subset_of_output` is never added again.
- Unknown component inclusion makes the affected raw aggregate null.
- A retry uses a new call identity and remains in total spend.
- Duplicate propagation of one call identity is counted once; conflicting duplicates block the metric.
- Zero accepted tasks produces null, not zero.
- A cache hit can reduce actual or estimated billable cost but cannot reduce raw-token totals by itself.
- The task, not the agent call, is the paired statistical unit.
- The 25 percent value is a provisional promotion target and never overrides the zero-regression gate.

## Verification Mapping

| Scenario | Primary test surface |
|---|---|
| S1–S6 | `pkg/telemetry`, provider adapter fixtures, telemetry CLI JSON tests |
| S7–S10 | adapter-rendered temporary roots, promptlayer/memindex/context receipt tests |
| S11–S13 | `pkg/worker/compress`, worker pipeline integration tests |
| S14–S18 | `pkg/workflow`, workflow CLI, generated route-team parity/launch tests |
| S19–S21 | telemetry efficiency evaluator and deterministic promotion gate tests |
| S22 | build/vet/race/coverage/generation/architecture/hygiene commands |
| S23 | frozen corpus manifest, deterministic target receipts, cohort/order manifest |
| S24 | role/config manifest and exact call-arithmetic preflight |
| S25 | `auto agent run` Codex adapter integration, telemetry receipt, retention and circuit-breaker checks; the non-PASS fail-closed branch is also proven by the partial live ledger and terminal outcome |
| S26 | rollout-budget admission ledger and pre-admitted replay reserve oracle |
| S27 | strict paired/quality evaluator and isolated atomic rollback state/readback |
| S28 | `pkg/promptlayer/context_delivery_test.go`, `internal/cli/workflow_context_test.go`, `internal/cli/workflow_binding_context_test.go`, `pkg/spec/prompt_complete_documents_test.go`, `pkg/worker/required_context_delivery_test.go`, `pkg/worker/pipeline_required_context_delivery_test.go`, `pkg/pipeline/context_delivery_test.go`, and `pkg/adapter/codex/context_delivery_surface_test.go` |
| Edge Case 1 | REQ-UTE-USAGE-04, T2/T3, late completion-event propagation fixture |
| Edge Case 2 | REQ-UTE-USAGE-02 through REQ-UTE-USAGE-04, T1, component-relation fixture |
| Edge Case 3 | REQ-UTE-POLICY-01, T8, changed-file discovery failure fixture |

## Final Verification Closure

The v0.2.0 Guardian closure accepts T1–T13, S1–S27, and Edge Cases 1–3. Broad race/coverage, `go vet`, `go build`, `go test ./...`, frozen-Auto architecture enforcement, changed-Go formatting, strict SPEC validation, scratch template generation, compatibility, and hygiene gates all pass.

The v0.2.1 amendment adds completed T14/S28 evidence for complete required-document delivery, coherent review and available architecture snapshots, supervisor-held extra-reference-set and omission/replay rejection, final-worktree resolution, body-free hash manifests, fail-closed provider blocking, the 128K block-or-split boundary, original-task/direct/pipeline continuity, and current native Codex spawn plus diagnostic `context_ack` guidance. It does not alter the accepted v0.2.0 live evidence or activate the candidate policy.

The focused coverage gate is 85%+ or an explicit review-approved exception. Guardian approved three aggregate-package exceptions without waiving regression coverage for changed paths:

| Package | Coverage | Direct changed-path evidence |
|---|---:|---|
| `pkg/worker` | 78.7% | Usage identity/provenance, late propagation, exactly-once, failed-spend, phase aggregation, pruning, and blocker tests pass in the named loop/pipeline suites; race/full pass and adapter/compressor coverage exceeds 93%. |
| `pkg/worker/host` | 61.1% | `resolve_test.go` directly verifies the changed telemetry callback wiring and PASS/FAIL normalized actual-usage exactly-once persistence/readback; race/full pass. |
| `pkg/adapter/gemini` | 84.7% | Router budget, context profile, Ultra-efficiency, generation/update, and minimality tests directly cover changed ownership, collision, parity, ordering, and corrupt-manifest behavior; race/full/scratch generation pass. |

The final Codex matrix correction is test-only. Its oracle distinguishes root supervisor Sol/`ultra` from managed orchestra Sol/`max`; production code is unchanged, targeted RED→GREEN and the full matrix pass, and Guardian reports P0/P1/P2 `0/0/0`.

## Definition of Done

- [x] S1 through S27 and all edge cases pass.
- [x] S28 passes for GPT/Codex complete-document integrity, authoritative reference/hash verification, final-worktree and coherent-review snapshots, and non-duplicated task continuity without reopening the v0.2.0 terminal acceptance state.
- [x] Every Must requirement maps to a plan task and semantic invariant.
- [x] Changed Go packages meet focused 85%+ coverage or have an explicit review-approved exception; all three exceptions above retain direct changed-path regression evidence.
- [x] Strict authoring validation and the final independent diff-only review pass with zero open P0/P1/P2 findings; multi-provider consensus is not required for v0.2.0 completion.
- [x] Balanced, high/critical Ultra, custom/pinned config, and non-Codex hermetic regression oracles pass without requiring non-Codex live calls.
- [x] Completion Debt stayed open until exact GPT/Codex paired live usage, canary, audit, quality-ledger, applied rollback, and replay receipts existed; CD-02/CD-03 are now resolved.
- [x] No user config or repository policy is activated by the experiment.
- [x] Independent review is closed as PASS; it is not open Completion Debt.
- [x] Status remained `approved` until the exact 58-call, 14-trial, seven-row quality, strict evaluator, and applied rollback replay evidence passed; lifecycle is now `completed`/`implemented=true`, while terminal experiment artifacts remain `implemented=false`.
- [x] The first live attempt opened the circuit on the first non-PASS verdict with zero retry and blocked evaluator/replay admission; this closes only the S25 fail-closed branch.
- [x] A new frozen policy/config/schema identity and explicit authorization governed the v2 live attempt that completed S24, the remaining S26 replay path, and S27.
- [x] Historical task006 diagnostic evidence is preserved as non-authoritative for current completion; its first-call failure neither supplies nor restricts the later v2 quality and promotion-boundary conclusion.
- [x] Historical task006 transport-smoke evidence is preserved with unavailable usage and null observed raw tokens; it neither supplies nor restricts the later v8 Transport PASS and v2 completion evidence.
- [x] Transport diagnosis v2/v3 terminal records are sidecar-verified and preserve null/unknown fields without inventing a root cause.
- [x] Exact v7 identity `sha256:fa56811f74e755711208878b2eb7e41db6071bb83e2208f8fd95024cb6d8d72a` received exact `실행승인`, was consumed once, and is non-resumable; this closes only v7 admission and does not close Transport or completion gates.
- [x] Exact v8 identity `sha256:c524d53a9b12d84457ee2ad44c7149c31648c35227ef066f938467af8a74945a` received separate explicit authorization, was consumed once, and is non-resumable; this closes only v8 transport admission.
- [x] V8 records Transport PASS with schema conformance and one canonical actual-usage receipt; this satisfies only the transport prerequisite for future full evaluation.
- [x] A separate v2 full-evaluation admission and budget are frozen and provider-zero verified.
- [x] Exact v2 identity `sha256:129521ff443c4ec01bc71cbb621c1dd3d515d5f460130a16732bf639b52e4978` was explicitly authorized before the 58-call primary and reservation.
- [x] The ordered v2 chain completed primary 58, provider-zero evaluator, applied rollback/readback, replay 5, provider-zero finalizer, and consumed non-reusable closure with Guardian P0/P1 `0/0`.

## Revision History

| Version | Date | Status | Change |
|---|---|---|---|
| 0.1.0 | 2026-07-11 | approved | Initial S1–S22 implementation and compatibility acceptance package. |
| 0.2.0 | 2026-07-12 | approved | Adds S23–S27 for GPT/Codex-only operational scope, exact corpus/cohort and call arithmetic, evidence safety, budget admission, quality completeness, and applied rollback. Records S25 fail-closed proof, consumed authorization, resolved P1, and final review PASS while keeping S24, the remaining S26 path, and S27 open. |
| 0.2.0 | 2026-07-13 | approved | Records the diagnostic-only task006 failure, sanitized nonzero-process hardening, operational reuse rejection, and separate single-call transport-smoke failure without changing primary acceptance, promotion, or Completion Debt. |
| 0.2.0 | 2026-07-14 | approved | Accepts the v2/v3 GPT/Codex terminal records as fail-closed evidence only, not transport or root-cause proof; preserves `full_ultra` and blocks the 58+5 live sequence pending Transport PASS. |
| 0.2.0 | 2026-07-14 | approved | Records v4 artifact and preflight consistency without accepting live execution; authorization remains explicit-user-pending and unconsumed. |
| 0.2.0 | 2026-07-14 | approved | Accepts v4 only as a sanitized terminal operational-failure record; provider-failure-event source metadata does not satisfy transport, semantic, promotion, or Completion Debt gates. |
| 0.2.0 | 2026-07-14 | approved | Records v5 artifact consistency and provider-zero readiness without accepting or authorizing its bounded event-shape diagnosis call. |
| 0.2.0 | 2026-07-15 | approved | Accepts v5 only as a sanitized terminal operational-failure record; event kind/shape does not satisfy root-cause, transport, semantic, promotion, or Completion Debt gates. |
| 0.2.0 | 2026-07-15 | approved | Accepts v6 only as sidecar-verified provider-zero readiness for a raw-free per-event diagnosis; explicit live authorization and Transport PASS remain open gates. |
| 0.2.0 | 2026-07-15 | approved | Accepts v6 only as a consumed sanitized terminal operational failure with `schema_or_response` coarse traits; all completion and promotion gates remain open. |
| 0.2.0 | 2026-07-15 | approved | Accepts v7's freeze-time state only as a provider-zero conservative Structured Outputs preflight with exact runtime-copy and pre-claim gates; separate exact-identity authorization was still required then. |
| 0.2.0 | 2026-07-15 | approved | Accepts the exact-authorized v7 run only as a consumed `missing_or_invalid_result` terminal record with unavailable usage and no canonical receipts; Transport and all completion gates remain open. |
| 0.2.0 | 2026-07-15 | approved | Accepts v8 only as a provider-zero missing-to-empty consumer fix with RED→GREEN and immutable-regression evidence; exact v7 causality is unconfirmed and separate v8 authorization remains pending. |
| 0.2.0 | 2026-07-15 | approved | Accepts v8 terminal Transport PASS and canonical actual usage while leaving semantic, promotion, implementation, S24/S27, CD-02/CD-03, and separately authorized full-evaluation gates open. |
| 0.2.0 | 2026-07-15 | approved | Accepts the v2 provider-zero full-evaluation admission, exact source/runtime/budget bindings, task-bound exact14 evaluator, nested replay/finalizer closure, regressions, and P0/P1 `0/0` review while keeping exact authorization and live acceptance debt open. |
| 0.2.0 | 2026-07-15 | completed | Accepts the exact-authorized 58-call primary, provider-zero exact14 evaluator, atomic rollback/readback, five-call replay, terminal, and non-reusable closure. S24/S26/S27 and CD-02/CD-03 are closed with no promotion, activation, user-config mutation, or repository-policy mutation. |
| 0.2.0 | 2026-07-15 | completed | Accepts T1–T13, S1–S27, Edge Cases 1–3, and all DoD items after broad verification passes. Records three explicit aggregate-coverage exceptions and closes the Codex matrix test-only oracle debt with production unchanged and P0/P1/P2 `0/0/0`. |
| 0.2.1 | 2026-07-15 | completed | Adds Must scenario S28 for complete GPT/Codex context and the actual all-GPT `auto spec review` path: each revision build-verifies supervisor-held required-document/conditional-profile sets and injects core, architecture, extra, and four SPEC bodies once. Missing, tampered, wrong-set, stale, wrong-SPEC, and 128K failures make zero provider calls; mixed Codex+Claude, Claude, and Gemini remain legacy. The v0.2.0 terminal, promotion, activation, release, and effective `full_ultra` profile remain unchanged. |
