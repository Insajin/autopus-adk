# PRD: Token-Efficient Ultra Quality Allocation

> Product Requirements Document — Standard mode.

- **SPEC-ID**: SPEC-ADK-ULTRA-EFFICIENCY-001
- **Source idea**: BS-052
- **Author**: Autopus planning workflow
- **Status**: Draft
- **Date**: 2026-07-11

---

## Discovery Q&A Checklist

- [x] **Problem**: Ultra applies broad context, high effort, fan-out, and review depth too uniformly while actual provider token use is not observable end to end.
- [x] **Target Users**: Autopus maintainers and teams that use Ultra for high-confidence implementation and review.
- [x] **Success Metrics**: Provider-actual usage completeness, raw tokens per accepted task, quality non-inferiority, high/critical regression count, retries, and interventions.
- [x] **Constraints**: Preserve premium capability and fail-closed quality/security gates; source changes remain in `autopus-adk`; generated workspace surfaces are not edited directly.
- [x] **Prior Art**: Existing prompt layers, context compression, telemetry, review-risk classification, quality depth, workflow gates, and budgeted memory context are reused.
- [x] **Scope Boundary**: No provider price table redesign, no public 25% claim without paired evidence, no mandatory prompt cache integration, no adaptive implementation fan-out, no within-run reviewer expansion, and no model/effort downshift.

## 1. Problem & Context

### Current Situation

Ultra currently expresses quality mainly by allocating more computation everywhere:

- Codex supervisor/orchestra uses the highest managed profile, strategic workers use `max`, and other workers use `xhigh`.
- Team workflow review uses three reviewer votes, one mandatory security audit, and one synthesis call.
- Claude and Gemini root routers embed broad workflow guidance and require a large context set before the selected subcommand is known.
- Worker compression is primarily an overflow response after the estimated context reaches 50% of the provider window.
- Telemetry stores one `estimated_tokens` value and cannot distinguish actual input, output, cached input, reasoning, or unavailable usage.

### Problem Statement

Ultra users need the highest-confidence outcome, but they cannot tell which tokens contributed to an accepted result and which came from repeated instructions, unrelated context, stale tool output, redundant review votes, or excessive effort. The mode therefore charges an uncertainty tax even for bounded low- and medium-risk work.

### Impact

- Maintainers cannot prove an efficiency improvement because provider-actual usage does not survive worker and orchestra aggregation.
- A monolithic router and unconditional context loading create a large fixed input before task-specific work begins.
- Low- and medium-risk team runs execute the same five-call review shape as high/critical work.
- Cache metadata exists without cache-hit evidence, so billable savings can be mistaken for raw-token savings.

### Change Motivation

BS-052 combined local code analysis, official provider guidance, and a degraded but convergent multi-provider debate. The evidence supports a staged change: measure first, remove lossless fixed waste second, and apply quality-sensitive review reduction only through conservative pre-dispatch binding with a full-depth fallback.

## 2. Goals & Success Metrics

| Goal | Success Metric | Target | Timeline |
|---|---|---:|---|
| Make token usage observable | Calls with `usage_status=actual` in the eligible paired corpus | at least 95% | Before adaptive policy promotion |
| Remove fixed prompt waste | Rendered root router size | at most 8,192 bytes per supported root router | First implementation slice |
| Preserve quality | New high/critical objective or security regressions | exactly 0 | Every canary stage |
| Improve accepted-task efficiency | Median raw-token reduction over paired accepted tasks | provisional target at least 25% | Pilot, not product promise |
| Preserve operational reliability | Retry, timeout, context overflow, and human-intervention regression | no material increase beyond registered tolerance | Every canary stage |
| Keep reports semantically correct | Cached and reasoning subsets double-counted in raw totals | exactly 0 occurrences | Unit and integration gates |

The 25% value is a promotion target inherited as an assumption from BS-052. It is not a guaranteed outcome and cannot appear as a public claim until the paired quality and usage gates pass.

### Anti-Goals

- Do not make Ultra equivalent to Balanced.
- Do not lower model tier as the first optimization.
- Do not count cache hits as raw-token reduction.
- Do not reduce security, build, test, release, data-loss, or generated-surface hygiene gates.
- Do not optimize failed-task token totals while hiding lower acceptance.

## 3. Target Users

| User Group | Role | Usage Frequency | Key Expectation |
|---|---|---|---|
| Ultra operators | Maintainer/team lead | Daily or per release | Highest-confidence results with defensible token and cost evidence |
| Workflow implementers | ADK contributor | Per feature | Stable normalized usage schema and deterministic fallback rules |
| Review and security owners | Reviewer/auditor | Per change | No high/critical downshift and no hidden gate removal |
| Benchmark owners | Performance/eval engineer | Per release candidate | Paired accepted-task metrics with explicit missing-usage treatment |

**Primary User**: Ultra operators who need quality evidence and resource accountability from the same run.

## 4. User Stories / Job Stories

### Story 1: Explain where Ultra spent tokens

When an Ultra task finishes, I want provider-actual input, output, cache, reasoning, tool, model-call, and status evidence so I can distinguish measured usage from estimates and unavailable data.

Acceptance intent:

- Given a provider usage event, when it is aggregated into telemetry, then actual component fields and provenance remain intact.
- Given a provider that exposes no token usage, when the run is reported, then unavailable fields are `null`, not zero.
- Given cached and reasoning components, when raw totals are calculated, then subset fields are not added twice.

### Story 2: Pay only the relevant fixed-context cost

When I invoke an Autopus subcommand, I want a thin root router to load one detailed command contract and a bounded task-relevant context receipt so unrelated command and project material does not consume the initial attention budget.

Acceptance intent:

- Given a rendered platform root router, when its size is measured, then it is no larger than 8,192 bytes.
- Given `plan`, `test`, and `canary` invocations, when context is selected, then each receives its declared context profile and not the union of all profiles.
- Given a dynamic task change, when prompt layers are rendered, then the stable layer hash remains unchanged.

### Story 3: Select minimum sufficient review depth before dispatch

When a change has trustworthy pre-dispatch risk evidence, I want Ultra to avoid redundant votes and synthesis for eligible low/medium work while using the current full-depth path for high, critical, sensitive, unknown, audited, or malformed cases.

Acceptance intent:

- Given high, critical, or unknown risk, when review starts, then three votes, security, and synthesis run.
- Given eligible low/medium risk, when the binding is created, then one reviewer and one mandatory security call run without synthesis.
- Given deterministic audit selection or binding uncertainty, when the binding is created, then the current full-depth path is selected before dispatch.

### Story 4: Promote only proven efficiency

When an optimization is canaried, I want a paired accepted-task report and deterministic quality gate so token savings never outrank correctness or security.

Acceptance intent:

- Given incomplete actual usage, when promotion is evaluated, then the decision is blocked.
- Given one new high/critical regression, when promotion is evaluated, then the decision is blocked regardless of token savings.
- Given sufficient actual usage, zero high/critical regression, and the target median reduction, when promotion is evaluated, then the candidate is eligible for the next canary stage.

## 5. Functional Requirements

### P0 — Must Have

| ID | Requirement | Notes |
|---|---|---|
| FR-01 | Normalize actual, estimated, cost-only, and unavailable provider usage without double counting. | Backward compatible with existing telemetry JSONL. |
| FR-02 | Propagate usage through worker adapters, phase aggregation, pipeline telemetry, and orchestra responses. | Raw provider usage fragment is sanitized and versioned. |
| FR-03 | Report raw-token and billable-cost views separately and use accepted tasks as the efficiency denominator. | Failed runs remain visible but do not masquerade as efficiency wins. |
| FR-04 | Replace monolithic root routers with route-only surfaces that lazy-load one detailed command contract. | Root router size budget is a deterministic oracle. |
| FR-05 | Select bounded context by command profile and materialize a manifest-backed context receipt. | Reuse `auto mem context`; no new retrieval service. |
| FR-06 | Proactively prune completed stale tool pairs while preserving failures, constraints, decisions, references, and tool-pair integrity. | Existing hard compaction remains a fallback. |
| FR-07 | Reuse the existing risk-tier classifier in a live route-team binding command and fail open to current full Ultra. | Missing discovery and invalid binding are full depth. |
| FR-08 | Keep the security auditor and deterministic quality gates mandatory in every adaptive path. | No security skip. |
| FR-09 | Gate canary promotion on usage completeness, accepted-task savings, and zero high/critical regression. | 25% remains configurable experiment evidence. |
| FR-10 | Keep generated/platform surfaces derived from ADK source and verify rendered parity. | Root generated copies remain edit-forbidden. |

### P1 — Should Have

| ID | Requirement | Notes |
|---|---|---|
| FR-11 | Attribute fixed prompt cost to manifest layers and report invalidation reasons. | Diagnostic, not a provider cache claim. |
| FR-12 | Preserve a deterministic full-depth audit sample for low/medium adaptive runs. | Default sample percentage belongs to internal rollout configuration. |
| FR-13 | Emit human and JSON comparison reports with the same formulas and reason codes. | JSON contract is authoritative for automation. |

### P2 — Could Have

| ID | Requirement | Notes |
|---|---|---|
| FR-20 | Connect explicit provider prompt caching and TTL-aware warmup. | Separate billable-cost experiment. |
| FR-21 | Downshift routine Ultra effort after role-specific evaluations prove non-inferiority. | Not required for the first Outcome Lock. |
| FR-22 | Add lazy provider tool-schema discovery where the provider supports it. | Capability-gated enhancement. |
| FR-23 | Add structured within-run review escalation after reviewer-output schemas and deterministic result consumption exist. | Separate quality-sensitive experiment. |
| FR-24 | Evaluate adaptive implementation fan-out after the current cap is changed to batching without task loss. | Not safe in the current prefix-limited generator. |

## 6. Non-Functional Requirements

| Category | Requirement | Target |
|---|---|---|
| Correctness | Normalized totals preserve provider semantics | No subset double count; unavailable is never zero-filled |
| Compatibility | Existing telemetry events remain readable | 100% of legacy fixtures parse |
| Security | Raw prompts, secrets, credentials, and privileged absolute paths are not retained in usage records | Zero leaks in fixtures and security review |
| Reliability | Uncertain risk, missing discovery, invalid binding, or missing usage fails open | Current full Ultra or blocked promotion |
| Performance | Root router fixed input is bounded | at most 8,192 rendered bytes |
| Quality | High/critical paths retain current depth and premium mappings | Zero downshift cases |
| Testability | New formulas and state transitions have concrete hermetic oracles | 85%+ coverage for changed Go packages |
| Portability | Shell and generated-surface checks remain macOS/Linux compatible | No GNU-only timeout or shell dependency |

## 7. Technical Constraints

### Technology Stack Constraints

- Brownfield Go module; preserve the current `go.mod` major versions.
- Reuse `pkg/telemetry`, `pkg/promptlayer`, `pkg/worker/compress`, `pkg/workflow`, `pkg/experiment`, `pkg/evalregression`, and `internal/cli/review_risk_tier.go` patterns before introducing a new package.
- Do not add an external tokenizer or statistics dependency for this scope.
- Generated `.claude/**`, `.codex/**`, `.gemini/**`, `.opencode/**`, and root runtime artifacts are verification outputs, not direct source edits.

### Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|---|---|---|---|---|---|
| brownfield | Existing Go module and embedded template/content filesystems | Current repository manifests | `go.mod`, `pkg/telemetry`, `pkg/content`, `templates` | 2026-07-11 | New tokenizer/statistics service: unnecessary dependency and broader deployment boundary |

### External Dependencies

| Dependency | Version / SLA | Risk if Unavailable |
|---|---|---|
| Claude/Codex/Gemini CLI usage events | Provider and CLI dependent | Mark usage unavailable and block quantitative promotion; do not infer zero |
| Live paired task corpus | Operational asset | Hermetic implementation can pass, but Completion Debt remains until live evidence is produced |

### Compatibility Requirements

- Existing `estimated_tokens` JSONL records remain readable.
- Existing Balanced depth, non-Claude platform routing, and pinned/custom model policies remain unchanged.
- Current high/critical Ultra tuple and review depth remain unchanged.
- Existing `auto telemetry summary`, `cost`, and `compare` users retain their fields while receiving additive usage fields.

### Infrastructure Constraints

- No new managed service.
- No hard dependency on the absent sibling benchmark repository.
- Live provider tests are opt-in operational evidence and are not run inside ordinary unit tests.

## 8. Out of Scope

- Provider price-table modernization beyond separating actual and estimated cost.
- Public or marketing claim of 25% savings.
- Replacing Ultra with Balanced or adding another user-facing quality mode.
- Skipping security audit, build/test/coverage, release hygiene, or data-safety checks.
- Automatic downshift for auth, IAM, billing, payment, migration, data-loss, release, production, compliance, encryption, or public-API work.
- Direct edits to installed/generated workspace surfaces.
- A new external benchmark repository or a new hosted telemetry backend.

### Deferred to Future Iterations

- Explicit prompt-cache activation and pre-warming after cache semantics are proven per provider.
- Routine-role effort reduction after role-specific evals.
- Structured within-run reviewer expansion after workflow results are consumed deterministically.
- Adaptive implementation fan-out after scheduling batches every planned task without loss.
- Lazy tool catalogs and provider-native compaction.
- Delta-only SPEC re-review after stable finding IDs are available across the target path.

## 9. Risks & Open Questions

### Risks

| Risk | Severity | Probability | Mitigation Strategy |
|---|---|---|---|
| Provider usage schemas differ or change | High | High | Preserve source/schema metadata, use nullable fields, and test fixtures independently. |
| A low/medium classifier misses hidden critical scope | High | Medium | Critical path allowlist, unknown-to-full fallback, mandatory security, and audit sampling. |
| Thin-router extraction loses policy parity | High | Medium | Rendered-surface parity tests, route coverage, and source-size oracles. |
| Pruning removes evidence needed later | High | Medium | Preserve required sections and full artifact references; incomplete pairs fail closed. |
| Cached tokens are credited as raw savings | Medium | Medium | Separate raw and billable metrics in types, formulas, reports, and tests. |
| 25% target is not achieved | Medium | Medium | Keep it a promotion target; accept smaller measured savings without false claim and reassess levers. |
| Live corpus remains unavailable | High | Medium | Record Completion Debt and block quantitative promotion while allowing hermetic implementation verification. |

### Open Questions

| # | Question | Owner | Due Date | Status |
|---|---|---|---|---|
| Q1 | Which current benchmark checkout owns the paired 45-task corpus? | Maintainer | Before canary promotion | Open, non-blocking for implementation |
| Q2 | Which CLI versions expose complete actual usage for each provider? | Provider adapter owner | During T2 implementation | Open, handled by capability fixtures |
| Q3 | What full-depth audit sample rate balances detection and cost? | Quality owner | Before default enablement | Assumed 10% for planning; configurable |
| Q4 | Should a lower measured reduction pass if quality improves materially? | Product/quality owner | After pilot | Deferred; no automatic exception |

## 10. Pre-mortem

| # | Failure Scenario | Probability | Impact | Preventive Action |
|---|---|---|---|---|
| 1 | Reports show impressive savings because cached or reasoning subsets were counted incorrectly. | Medium | High | Inclusive-total contract, arithmetic fixtures, and raw/billable split. |
| 2 | Router becomes small but command behavior silently disappears. | Medium | High | Every route must resolve one detailed source and rendered parity must assert mandatory tokens. |
| 3 | Compact review skips a vote that would have found a critical defect. | Low/Medium | Critical | High/critical exclusions, security always, unknown full depth, deterministic audit sampling, and immediate rollout rollback. |
| 4 | Provider usage is mostly unavailable, yet the team promotes the policy from estimates. | Medium | High | `actual_coverage >= 0.95` is a hard promotion gate. |
| 5 | The implementation expands into provider caching, pricing, and benchmark infrastructure and never converges. | Medium | Medium | Keep those items advisory unless required by the Outcome Lock. |

Connection to risks: scenarios 1–4 map directly to usage semantics, router parity, risk classification, and live-corpus risks. Scenario 5 is controlled by the Minimality Decision Matrix and explicit scope boundary.

## 11. Practitioner Q&A

**Q1: Is prompt caching part of the raw-token target?**  
A: No. Cache reads remain processed input. They are reported under billable cost and latency, not raw-token reduction.

**Q2: What is an accepted task?**  
A: A run whose deterministic acceptance and final pipeline status are PASS. Failed or unavailable-quality runs remain visible but cannot improve the efficiency denominator.

**Q3: What happens when usage is missing?**  
A: Token fields remain `null`, status is `cost_only`, `estimated`, or `unavailable`, and the run does not satisfy the actual-coverage promotion gate.

**Q4: Does adaptive review remove the security auditor?**  
A: No. Every path runs the security auditor. Only redundant reviewer votes and synthesis are conditional for eligible low/medium work.

**Q5: Does this SPEC add review calls dynamically after model output?**  
A: No. Review depth is bound before dispatch. Dynamic escalation needs structured result consumption and is a separate experiment; compact-path failures still cannot satisfy deterministic acceptance.

**Q6: Why not lower model tiers first?**  
A: Router/context cleanup and bounded history remove structural waste with lower quality risk. Model/effort reductions require separate role-specific evidence.

**Q7: How is rollback decided?**  
A: One new high/critical regression, unsafe context handling, actual-coverage failure, or registered reliability regression blocks promotion and restores full Ultra behavior.

**Q8: Is a sibling SPEC required?**  
A: No. Usage, context, and risk-bound review depth are tightly coupled parts of one Ultra resource-policy outcome. Optional provider integrations remain Evolution Ideas rather than sibling work.
