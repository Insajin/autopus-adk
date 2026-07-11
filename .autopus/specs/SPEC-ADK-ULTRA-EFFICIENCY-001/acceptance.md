# SPEC-ADK-ULTRA-EFFICIENCY-001 Acceptance: Token-Efficient Ultra Quality Allocation

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

Given a supervisor has selected a 1200-token total receipt budget and has an Outcome Lock, two constraints, three owned paths, two forbidden paths, four acceptance criteria, five relevant references, and one decision delta
When it invokes budgeted recall and builds a context receipt
Then expected JSON value `budget_tokens` equals 1200
And the final receipt estimate is at most 1200 tokens
And it contains every Outcome Lock, constraint, ownership boundary, acceptance ID, required reference, decision delta, snapshot hash, and prompt-manifest hash
And `ContextResult` receives only the residual budget after mandatory receipt fields are estimated
And omitted source bodies remain retrievable through stable source references
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
| Edge Case 1 | REQ-UTE-USAGE-04, T2/T3, late completion-event propagation fixture |
| Edge Case 2 | REQ-UTE-USAGE-02 through REQ-UTE-USAGE-04, T1, component-relation fixture |
| Edge Case 3 | REQ-UTE-POLICY-01, T8, changed-file discovery failure fixture |

## Definition of Done

- [ ] S1 through S22 and all edge cases pass.
- [ ] Every Must requirement maps to a plan task and semantic invariant.
- [ ] Changed Go packages meet the focused 85%+ coverage gate.
- [ ] Strict authoring validation and multi-provider SPEC review pass.
- [ ] Balanced, high/critical Ultra, custom/pinned config, and non-Claude regression oracles pass.
- [ ] Completion Debt stays open until paired live usage, canary, audit, and rollback receipts exist.
