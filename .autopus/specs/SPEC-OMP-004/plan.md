# SPEC-OMP-004 구현 계획

## Implementation Strategy

기존 `autopus.context_delivery.v1` full-document authority와 `autopus.context_plan.v2` shadow-only selector를 변경하지 않고, 그 위에 provider-neutral OMP session binding, supervisor-held transient ephemeral channel, body-free receipt를 추가한다. Native selective pinning을 가정하지 않고 post-compaction canonical full delivery를 재구성·재주입하며 token credit만 eligible history에 귀속한다. 구현은 TDD로 Must oracle을 먼저 고정한다.

## Visual Planning Brief

```mermaid
sequenceDiagram
  participant S as Autopus Supervisor
  participant P as promptlayer v1
  participant B as OMP Context Bridge
  participant O as OMP Runtime
  S->>P: Build + Verify(same ContextDeliveryOptions)
  P-->>S: full receipt + exact refs/hashes
  S->>B: bind task/phase + retain transient required bodies
  B->>O: shadow profile / capability-gated history-only overlay
  O->>B: pre-compaction event
  B->>S: checkpoint hashes + task/finding/ownership schema
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
- [x] **T2: opt-in config와 capability receipt를 구현한다.** `[NEW] history_mode(off|shadow|active)`와 독립된 `memory_mode(off|shadow)`, target, TTL/namespace, fallback을 validate한다. version 문자열 외에 pre/post event, post-compaction canonical injection/admission block, `--no-session` 또는 isolated session root, cleanup readback, memory interception을 각각 probe한다. 하나라도 없으면 active admission을 막는다.
- [x] **T3: provider-neutral history classifier와 session binding을 구현한다.** v1 `RequiredDocuments` 전체를 `[NEW] full_document_refs`로, ephemeral hashes와 eligible history rows를 별도 set으로 만든다. native selective retention을 가정하지 않고 v2 verified candidate는 shadow로 복사하며 unavailable status/reason은 빈 candidate set으로 보존한다. OMP platform을 worker/provider 이름과 연결하지 않는다.
- [x] **T4: transient checkpoint·rehydration·receipt engine을 구현한다.** pre-compaction에 full task/decision/frozen findings/ownership/result schema를 supervisor-only process memory에 보관하고 body-free receipt에는 hashes/IDs만 쓴다. post-compaction에 같은 options로 v1 delivery를 재검증하고 transient bodies를 실제 재주입해 body hash를 대조한다. state 부재·불일치 시 provider call 0회 또는 independently rebuilt canonical full fallback을 보장한다.
- [ ] **T5: OMP native bridge와 artifact lifecycle을 capability-gated로 투영한다.** Go promptlayer/CLI가 hashes, canonical re-injection, admission, runtime-root ownership과 cleanup을 소유한다. `[NEW]` TypeScript adapter는 proved pre/post event만 전달한다. Managed run은 proved no-session 또는 task-owned 0700/0600 isolated root만 쓰고 symlink/path escape와 user root 접근을 거부하며 모든 terminal path에서 root deletion을 검증한다. Hash-only native forwarding과 injected driver는 구현됐지만 native hook return/ACK 계약상 active end-to-end admission은 아직 fail-closed다.
- [x] **T6: OMP memory shadow safety를 구현한다.** workspace/SPEC/role/ref/hash/checked_at/TTL/namespace provenance, current-source 재검증, secret redaction, injection neutralization, stale/tampered rejection, deterministic omission reason을 적용한다. memory body/document selection은 active prompt에 주입하지 않고 `.autopus/learnings/pipeline.jsonl`과 canonical docs는 계속 authority다.
- [x] **T7: shadow→active promotion과 rollback transaction을 구현한다.** gate set을 모두 만족할 때만 active overlay를 만들고, regression 시 shadow/off로 원자적 전환한 뒤 effective config/mode hash를 다시 읽어 receipt에 기록한다. user-owned config와 tracked files는 건드리지 않는다.
- [x] **T8: paired AB/BA canary와 metrics를 구현한다.** 최소 20개 동일 `task_id`를 full-history/history-compacted로 한 번씩 실행하고 AB/BA order를 균형화한다. pair completeness, full document/ephemeral equality, task oracle, security/integrity, baseline/optimized tokens, formula, fallback/rollback을 집계한다. fake runtime이 기본이며 live provider는 explicit opt-in, bounded cohort다.
- [ ] **T9: doctor·문서·전체 회귀 gate를 완료한다.** body-free mode/capability/checkpoint/fallback/cleanup을 노출한다. `[NEW] omp_context_coverage_test.go`가 T1 base diff에서 네 owning directory 아래 이 SPEC이 수정한 non-test Go files를 exact path set으로 동결해 aggregate 85%+와 package baseline no-regression을 검증한다. capability-approved installed OMP로 pre/post/reinject/cleanup canary를 통과시킨다. Doctor와 actual installed injected-driver canary는 완료됐고 integrated coverage/full/strict/git hygiene는 parent pending이다.

## Implementation Evidence

| Slice | Current evidence | State |
|---|---|---|
| T1~T4 | Must oracle, opt-in policy/capabilities, provider-neutral binding, process-private checkpoint/rehydration과 body-free receipt | implemented |
| T5 native bridge | Ambient generated extension discovery와 exact hash-only pre/post notify, bridge smoke provider requests `0`; supervisor core와 injected driver 존재 | partial, unchecked |
| T6~T8 | memory shadow provenance/security, balanced AB/BA metric, promotion/rollback readback | implemented |
| Installed canary | `omp/17.1.8`, threshold `7000`, loopback turns/requests `2/2`, external `0`, native start/end `1/1`, early admission `0`, optimized dispatch `1`, cleanup `0`, sandbox `true` | observed with injected Go driver |
| Native active admission | `session_before_compact` throw/timeout is swallowed to `undefined`; `pi.sendMessage()` is void; generated bridge must pre-notify then return `{cancel:true}` until verified long-lived supervisor ACK | blocking debt |
| T9 convergence | doctor and installed canary complete; integrated coverage/full/strict/git hygiene not yet run by parent | pending, unchecked |

## 파일 영향 분석

| Slice | Existing owners to change | `[NEW]` planned additions | Must not change |
|---|---|---|---|
| authority/classification | `pkg/promptlayer/context_profile.go`, `context_delivery*.go` only if additive metadata is unavoidable | `pkg/promptlayer/omp_context_*.go` | v1 schema/body completeness/hash semantics |
| OMP projection | `pkg/adapter/omp/omp*.go` | `pkg/adapter/omp/omp_context_*.go`, `templates/omp/extensions/autopus-context.ts.tmpl`, generated bridge | generic hook contract, user-owned config |
| config/CLI | `pkg/config/schema.go`, `pkg/config/defaults.go`, `internal/cli/workflow_context_plan.go`, doctor projections | `[NEW] pkg/config/schema_omp_context.go`, context runtime command/receipt helpers | global OMP config, orchestra provider map |
| verification | existing focused test packages | fake runtime/canary/adversarial tests | live provider by default |

Exact filenames may converge during implementation, but every not-yet-existing path/symbol above remains `[NEW]` until created.

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

## Verification Commands

Implementation phase shall adapt package names only when the `[NEW]` paths converge:

```bash
go test ./pkg/promptlayer ./pkg/adapter/omp ./pkg/config ./internal/cli -count=1
go test -race ./pkg/promptlayer ./pkg/adapter/omp ./internal/cli -count=1
go test -coverprofile=/tmp/spec-omp-004.cover ./pkg/promptlayer ./pkg/adapter/omp ./pkg/config ./internal/cli
AUTOPUS_COVER_BASE=<T1_BASE_SHA> AUTOPUS_COVER_PROFILE=/tmp/spec-omp-004.cover AUTOPUS_COVER_BASELINE=/tmp/spec-omp-004-baseline.json go test ./pkg/adapter/omp -run TestOMPContextChangedFileCoverage -count=1
go vet ./pkg/promptlayer ./pkg/adapter/omp ./pkg/config ./internal/cli
go test ./... -count=1
go run ./cmd/auto spec validate .autopus/specs/SPEC-OMP-004 --strict
```

Hermetic unit canary uses a temporary workspace and fake OMP event stream. Completion additionally requires any installed OMP that passes the exact lifecycle probes to cross a real compaction boundary using a loopback fake provider, one-shot overlay, and task-owned runtime root, then prove cleanup count zero. A version string alone cannot pass. External live-provider execution remains separate opt-in.

## Feature Completion Scope

This Primary SPEC closes canonical re-admission, eligible-history credit, transient checkpoint/rehydration, managed runtime artifact cleanup, memory shadow safety, paired promotion, and rollback. `SPEC-OMP-003` remains optional integration.

## Completion Debt

| Item | Blocks | Required resolution |
|---|---|---|
| Generated bridge에서 verified long-lived `Supervisor.Run` ACK, authoritative `CanonicalSource`, real dispatch admission wiring이 없음 | user-facing active optimization과 lifecycle completion | Native pre-compaction hook에서 supervisor ACK를 동기적으로 검증할 수 있는 계약을 구현하고, ACK 전에는 pre notify 후 `{cancel:true}`를 반환하며 end-to-end active provider admission을 재검증 |

## Current Pending Gates

- T9 integrated exact-set coverage, package baseline, full test/vet/build, strict validation과 git hygiene는 parent 실행 전이다.
- Actual installed canary는 installed native events + injected Go driver 증거이며 generated bridge end-to-end active proof로 승격하지 않는다.
