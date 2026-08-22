# SPEC-OMP-004 구현 계획

## Implementation Strategy

기존 `autopus.context_delivery.v1` full-document authority와 `autopus.context_plan.v2` shadow-only selector를 변경하지 않고, 그 위에 provider-neutral OMP session binding, supervisor-held transient ephemeral channel, body-free receipt를 추가한다. Native selective pinning을 가정하지 않고 post-compaction canonical full delivery를 재구성·재주입하며 token credit만 eligible history에 귀속한다. 구현은 TDD로 Must oracle을 먼저 고정한다.

## Visual Planning Brief

```mermaid
sequenceDiagram
  participant A as /auto OMP Session Caller
  participant S as Autopus Supervisor
  participant P as promptlayer v1
  participant B as OMP Context Bridge
  participant O as OMP Runtime
  A-->>S: assemble request + CanonicalSource + managed driver
  Note over A,S: canonical-full route wired; active-history authority blocked
  S->>P: Build + Verify(same ContextDeliveryOptions)
  P-->>S: full receipt + exact refs/hashes
  S->>B: bind task/phase + retain transient required bodies
  B->>O: shadow profile / capability-gated history-only overlay
  O->>B: pre-compaction event
  B->>S: correlated ui.confirm ACK + authority hashes
  O->>O: native compaction; selective pinning not assumed
  O->>B: post-compaction event
  B->>P: rebuild + verify(same options)
  alt exact match
    B-->>O: canonical full docs + ephemeral reinjected; provider call admitted
  else mismatch or unsupported
    B-->>O: cancel/block or canonical full fallback
    B-->>S: fallback/rollback receipt
  end
```

## Tasks

- [x] **T1: Must oracle과 baseline을 RED로 고정한다.** `pkg/promptlayer`, `pkg/adapter/omp`, `internal/cli`에 exact ref/body-hash, fake lifecycle, transient loss, runtime artifact cleanup, memory attack, AB/BA, rollback oracle을 추가한다. exact base SHA와 세 package pre-change coverage를 `/tmp/spec-omp-004-baseline.json`에 기록하고 v1/v2 golden을 보존한다.
- [x] **T2: opt-in config와 capability receipt를 구현한다.** `history_mode(off|shadow|active)`와 독립된 `memory_mode(off|shadow)`, target, TTL/namespace, fallback을 validate한다. version 문자열 외에 pre/post event, post-compaction canonical injection/admission block, `--no-session` 또는 isolated session root, cleanup readback, memory interception을 각각 probe한다. 하나라도 없으면 active admission을 막는다.
- [x] **T3: provider-neutral history classifier와 session binding을 구현한다.** v1 `RequiredDocuments` 전체를 `full_document_refs`로, ephemeral hashes와 eligible history rows를 별도 set으로 만든다. native selective retention을 가정하지 않고 v2 verified candidate는 shadow로 복사하며 unavailable status/reason은 빈 candidate set으로 보존한다. OMP platform을 worker/provider 이름과 연결하지 않는다.
- [x] **T4: transient checkpoint·rehydration·receipt engine을 구현한다.** pre-compaction에 full task/decision/frozen findings/ownership/result schema를 supervisor-only process memory에 보관하고 body-free receipt에는 hashes/IDs만 쓴다. post-compaction에 같은 options로 v1 delivery를 재검증하고 transient bodies를 실제 재주입해 body hash를 대조한다. state 부재·불일치 시 provider call 0회 또는 independently rebuilt canonical full fallback을 보장한다.
- [ ] **T5: OMP native bridge와 artifact lifecycle을 product entrypoint까지 capability-gated로 투영한다.** Generated `/auto go SPEC-ID` route, one-shot `auto pipeline run --platform omp`, sealed five-phase OMP RPC backend, strict receipt/readback와 private runtime을 구현했고 canonical backend의 retry/compaction은 껐다. Local producer는 pinned candidate/credential을 root-owned sandbox에 격리하고 양 variant를 동일한 10/10 segment로 `nobody`에서 순차 실행한다. 각 segment의 fresh process/session, sequence reset, unique receipt와 stable provider authority를 검증해 K2-signed orphan evidence로만 승격한다. Corrective explicit-live diagnostic은 통과했지만 immutable release-source evidence와 active promotion 전이므로 T5는 partial이다.
- [x] **T6: OMP memory shadow safety를 구현한다.** workspace/SPEC/role/ref/hash/checked_at/TTL/namespace provenance, current-source 재검증, secret redaction, injection neutralization, stale/tampered rejection, deterministic omission reason을 적용한다. memory body/document selection은 active prompt에 주입하지 않고 `.autopus/learnings/pipeline.jsonl`과 canonical docs는 계속 authority다.
- [x] **T7: shadow→active promotion과 rollback transaction을 구현한다.** gate set을 모두 만족할 때만 active overlay를 만들고, regression 시 shadow/off로 원자적 전환한 뒤 effective config/mode hash를 다시 읽어 receipt에 기록한다. user-owned config와 tracked files는 건드리지 않는다.
- [x] **T8: paired AB/BA canary와 metrics를 구현한다.** 최소 20개 동일 `task_id`를 full-history/history-compacted로 한 번씩 실행하고 AB/BA order를 균형화한다. pair completeness, full document/ephemeral equality, task oracle, security/integrity, baseline/optimized tokens, formula, fallback/rollback을 집계한다. fake runtime이 기본이며 live provider는 explicit opt-in, bounded cohort다.
- [x] **T9: doctor·문서·전체 회귀 gate를 완료한다.** Installed managed/product live canary, changed-production exact aggregate `85.02%`, package coverage `88.86/87.99/93.19/79.94%`와 CLI baseline `79.3%` 이상, exact coverage gate, four-package race, vet/build, serial full suite, gofmt, diff check, production Go max 286 lines, strict SPEC와 lore가 모두 PASS했다.

## Implementation Evidence

| Slice | Current evidence | State |
|---|---|---|
| T1~T4 | Must oracle, opt-in policy/capabilities, provider-neutral binding, process-private checkpoint/rehydration과 body-free receipt | implemented |
| T5 canonical route | Generated `/auto` route → one canonical pipeline run → sealed five-phase long-lived OMP RPC, strict model receipt/workspace binding, private executable/session runtime | production command path implemented; compaction off |
| T5 active evidence | Deterministic 40-call plan, local root-owned/nobody sandbox, symmetric 10/10 paired-session producer, four unique session receipts, K2 signer, active/historical verifier와 anonymous orphan evidence publication | corrective live cohort 14/14 gates passed; immutable release evidence pending |
| T6~T8 | memory shadow provenance/security, balanced AB/BA metric, promotion/rollback readback | implemented |
| Installed managed canary | `omp/17.1.8`, provider requests `3`, pre/post ACK `1/1`, native start/end `1/1`, same PID/session, provider #3 exact body, cleanup root `0`, sandbox `true` | real managed driver primitive observed |
| User active admission | Corrective provider-bound cohort에서 reusable multi-compaction admission은 관측했지만 K2-signed immutable release evidence와 active readback은 아직 없음 | active-history debt; fail-closed |
| T9 convergence | installed managed/product live canary; exact coverage `85.02%`; package baseline; four-package race; vet/build/full; gofmt/diff/line cap; strict/lore | verified complete |

## 파일 영향 분석

| Slice | Existing owners to change | `[NEW]` planned additions | Must not change |
|---|---|---|---|
| authority/classification | `pkg/promptlayer/context_*.go`, `pkg/promptlayer/omp_context_*.go` | None | v1 schema/body completeness/hash semantics |
| OMP projection | `pkg/adapter/omp/omp_context_*.go`, generated bridge | None | generic hook contract, user-owned config |
| config/CLI | OMP context config, native route, `internal/cli/pipeline_backend_omp*.go`, `workflow_context_runtime*.go`, local production evidence producer | None | global OMP config, orchestra provider map |
| verification | existing focused fake-runtime/canary/adversarial tests, local release transaction gates | production provider observation `[NEW evidence]` | external live provider by default |

The process-private product entrypoint, native `/auto` canonical-full pipeline, and explicit local production evidence producer are implemented. Provider-bound v0.50.109 observation and reusable-session active admission remain runtime evidence debt; a persisted body-free fixture or isolated RPC test is not active-history production evidence.

## Architecture Alignment

1. **Authority plane**: `pkg/promptlayer` owns full document build/verify and v2 shadow selection; supervisor transient state owns active-phase ephemeral bodies.
2. **Policy plane**: Autopus config owns opt-in mode, eligible classes, promotion and fallback rules.
3. **Projection plane**: `pkg/adapter/omp` translates proved policy to the current OMP runtime.
4. **Evidence plane**: body-free receipts and hermetic canary prove admission, omission, fallback and rollback.
5. **Non-authority cache**: OMP memory may produce shadow suggestions but cannot inject active bodies, select active documents, or update canonical docs/learning storage.

## Dependencies and Ordering

- The hard dependency chain is `SPEC-OMP-001` → `SPEC-OMP-002` → `SPEC-OMP-004`: the minimal surface exists first, then OMP workflow/session parity must execute and report evidence before native context lifecycle can be trusted.
- `SPEC-OMP-003` is independent and optional: model resolution may feed OMP runtime selection, but context integrity works with any admitted model.
- `SPEC-CONTEXT-ENGINEERING-001` and `SPEC-CONTEXT-ENGINEERING-EVOLUTION-001` are completed contracts reused without weakening.
- OMP current-main docs are research evidence only; runtime capability probe decides execution for installed versions such as `omp/17.1.8`.

## Risk Controls

| Risk | Control | Verification |
|---|---|---|
| required data loss | disjoint classifier, same-options rehydrate | S1-S4 |
| runtime capability drift | version+schema+event probe | S8 |
| memory trust escalation | provenance/TTL/namespace/hash and neutralization | S6-S7 |
| metric gaming/order effect | identical task pair and balanced AB/BA | S10-S11 |
| false rollback | effective mode/config readback | S12 |
| config ownership collision | overlay-first, conflict fail-closed | S8, S12 |
| cleaned driver reuse or preflight leak | managed fallback block, shadow rollback, cleanup lease/readback | S4, S13 |

## Verification Commands

Implementation phase shall adapt package names only when the `[NEW]` paths converge:

```bash
go test ./pkg/promptlayer ./pkg/adapter/omp ./pkg/config ./internal/cli -count=1
go test -race ./pkg/promptlayer ./pkg/adapter/omp ./pkg/config ./internal/cli -count=1
go test -coverprofile=/tmp/spec-omp-004.cover ./pkg/promptlayer ./pkg/adapter/omp ./pkg/config ./internal/cli
AUTOPUS_COVER_BASE=<T1_BASE_SHA> AUTOPUS_COVER_PROFILE=/tmp/spec-omp-004.cover AUTOPUS_COVER_BASELINE=/tmp/spec-omp-004-baseline.json go test ./pkg/adapter/omp -run TestOMPContextChangedFileCoverage -count=1
AUTOPUS_OMP_CONTEXT_LIVE=1 go test ./internal/cli -run TestWorkflowContextRuntime_InstalledOMPCompactionLifecycleCanary_AdmitsExactBodyOnACKedLiveSession -count=1 -v
go vet ./pkg/promptlayer ./pkg/adapter/omp ./pkg/config ./internal/cli
go test ./... -count=1
go run ./cmd/auto spec validate .autopus/specs/SPEC-OMP-004 --strict
```

Hermetic unit canary uses a temporary workspace and fake OMP event stream. Completion additionally requires any installed OMP that passes the exact lifecycle probes to cross a real compaction boundary using a loopback fake provider, one-shot overlay, and task-owned runtime root, then prove cleanup count zero. A version string alone cannot pass. External live-provider execution remains separate opt-in.

## Feature Completion Scope

This Primary SPEC owns canonical re-admission, eligible-history credit, transient checkpoint/rehydration, managed runtime artifact cleanup, memory shadow safety, paired promotion, rollback, and user-facing `/auto` OMP reachability. T9 evidence is complete and the internal managed primitive is verified, but the Primary SPEC does not close the Outcome Lock until T5 product wiring completes. `SPEC-OMP-003` remains optional integration.

## Completion Debt

| Item | Blocks | Required resolution |
|---|---|---|
| K2-signed immutable release-source v0.50.109 40-call evidence와 active readback이 아직 없음 | user-facing active optimization과 lifecycle completion | local release producer로 balanced AB/BA 20 pairs를 동일한 10/10 full/optimized segment에서 관측·K2 서명·공개 검증하고, 각 segment 내부의 반복 compaction마다 canonical re-admission과 segment 간 receipt 격리를 provider boundary에서 검증한 뒤에만 active를 허용 |

## Final T9 Verification Evidence

- Coverage: changed production exact aggregate `85.02%` (`3075/3617`); `promptlayer=88.86%`, `adapter/omp=87.99%`, `config=93.19%`, `internal/cli=79.94%` with CLI baseline `79.3%` 이상; exact coverage gate PASS.
- Current delta: exact aggregate `85.04%` (`3321/3905`), `promptlayer=88.15%`, `adapter/omp=87.70%`, `config=90.99%`, `internal/cli=79.82%`; installed loopback canaries, package race, vet/build, strict SPEC, gofmt/diff/300-line gate PASS.
- Runtime/quality: historical four-package race PASS (`promptlayer=9.838s`, `adapter/omp=216.317s`, `config=1.465s`, `internal/cli=506.403s`); current code-identical serial suite PASS (`go test -p 1 -count=1 -timeout=20m ./...`, `1089.145s`, failures/timeouts `0`), followed only by this evidence-line update and strict SPEC revalidation.
- 기존 installed managed canary는 bounded primitive의 real driver ACK/admission 증거다. 이번 native `/auto` canonical-full delta는 active-history 또는 installed provider-bound multi-compaction 증거로 승격하지 않는다.
