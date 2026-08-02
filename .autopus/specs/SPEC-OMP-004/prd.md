# PRD: OMP 네이티브 컨텍스트 최적화

- **SPEC-ID**: SPEC-OMP-004
- **Author**: Autopus-ADK Agent System
- **Status**: Implemented
- **Created**: 2026-08-02
- **Mode**: Standard (11 sections)

---

## Discovery Q&A Checklist

- [x] **Problem**: OMP의 compaction·read pruning·memory를 그대로 켜면 Autopus full-delivery와 ephemeral phase 증거가 native history 변환 과정에서 약화될 수 있다.
- [x] **Target Users**: OMP에서 Autopus 워크플로를 장시간 실행하는 엔지니어와 하네스 운영자다.
- [x] **Success Metrics**: required ref/hash 불일치 0건, managed runtime artifact 잔존 0건, 보안·품질 열화 0건, paired canary median token reduction 20% 이상이다.
- [x] **Constraints**: 기존 `autopus.context_delivery.v1`과 `autopus.context_plan.v2`를 보존하고, OMP 기능은 capability probe와 명시적 opt-in 뒤에만 사용한다.
- [x] **Prior Art**: `SPEC-CONTEXT-ENGINEERING-001`의 full required delivery와 `SPEC-CONTEXT-ENGINEERING-EVOLUTION-001`의 shadow-only JIT plan을 재사용한다.
- [x] **Scope Boundary**: provider-native transcript mutation, 새 canonical memory store, 모델 라우팅, OMP workflow parity는 포함하지 않는다.

## Implementation Status

- T1~T4와 T6~T8의 canonical binding, transient checkpoint/rehydration, memory shadow safety, canary/promotion/rollback 구현이 source tree에 반영됐다.
- Installed `omp/17.1.8` lifecycle canary는 threshold `7000`, loopback turns/requests `2/2`, external requests `0`, native start/end `1/1`, early admission `0`, optimized dispatch `1`, cleanup `0`, sandbox `true`를 증명했다.
- 이 canary는 installed native events를 injected Go driver에 공급한 증거다. Native `session_before_compact` hook의 throw/timeout은 `undefined`로 흡수되고 `pi.sendMessage()`는 void이므로 generated bridge가 long-lived `Supervisor.Run` ACK 없이 active provider admission을 여는 것은 아직 증명되지 않았다.
- Generated bridge는 verified ACK가 생길 때까지 pre notify 후 `{cancel:true}`로 fail-closed하며, T5와 T9 integrated gates가 남아 있어 lifecycle은 completed가 아니다.

## 1. Problem & Context

**Current Situation**

Autopus는 `pkg/promptlayer`에서 command profile별 required document를 완전하게 읽고 source/prompt/snapshot/manifest hash를 검증한다. OMP는 자체 compaction, tool-result pruning, memory backend를 제공하지만, 현재 OMP 어댑터는 이 기능을 Autopus phase와 연결하지 않으며 설치 버전별 capability도 증명하지 않는다.

**Problem Statement**

OMP 세션의 토큰을 줄이면서도 full delivered documents, 원 태스크, ownership와 열린 finding body가 손실되지 않았음을 결정적으로 증명할 경계가 없다. OMP memory가 현재 저장소보다 오래된 내용을 주입하거나 compaction이 ephemeral phase state를 제거하면 잘못된 실행을 정상으로 오인할 수 있다.

**Impact**

최적화를 끄면 장시간 세션 비용과 overflow 위험이 커지고, 검증 없이 켜면 required-context 누락, stale decision 재사용, secret retention, prompt injection 재주입이 발생할 수 있다.

**Change Motivation**

OMP가 one-shot overlay와 compaction lifecycle을 제공하므로 native selective pinning을 가정하지 않고 매 compaction 뒤 canonical full context를 재구성·재주입하며, completed-history token credit만 shadow에서 active로 승격할 수 있다.

## 2. Goals & Success Metrics

| Goal | Success Metric | Target | Timeline |
|---|---|---|---|
| required integrity 보존 | pre/post required ref, source hash, prompt hash, snapshot hash, manifest hash 일치 | 100%; mismatch 0 | 구현 릴리스 전 |
| 토큰 절감 | 20개 이상 complete AB/BA pair의 `100*(baseline-optimized)/baseline` median | 20% 이상 | active 승격 전 |
| 품질·보안 동등성 | 동일 task oracle pass delta, integrity/security finding | 열화 0; finding 0 | 각 승격 gate |
| 복구 가능성 | fail-closed/full fallback 및 rollback readback 성공률 | 100% | 각 canary |

**Anti-Goals**

- required body를 줄였다는 이유만으로 성공으로 판정하지 않는다.
- OMP memory를 `.autopus/learnings/pipeline.jsonl`이나 canonical 문서의 대체 저장소로 만들지 않는다.
- 특정 OMP `main` 문서의 설정을 설치된 모든 버전에서 지원한다고 가정하지 않는다.

## 3. Target Users

| User Group | Role | Usage Frequency | Key Expectation |
|---|---|---|---|
| 하네스 사용자 | OMP에서 Autopus task 실행 | 매일 | 긴 세션에서도 task/SPEC 무결성 유지 |
| 플랫폼 운영자 | profile 승격·rollback 담당 | 릴리스별 | capability와 canary receipt로 결정 |
| 리뷰어·보안 담당 | 품질 및 data-loss gate 검토 | 변경별 | body-free 결정적 증거와 재현 가능한 oracle |

**Primary User**: OMP에서 `auto` 워크플로를 실행하는 하네스 사용자다.

## 4. User Stories / Job Stories

### Story 1: 안전한 OMP 컨텍스트 절감

**When** OMP 세션이 phase boundary에서 compaction을 수행할 때,
**I want to** delivered documents와 transient phase state는 그대로 재검증하고 completed tool/transcript history만 최적화하며,
**so I can** 품질 저하 없이 긴 작업을 이어갈 수 있다.

**Acceptance Criteria**

- Given verified full delivery가 있을 때, when pre-compaction checkpoint를 만들면, then exact required ref/hash 집합과 phase state가 기록된다.
- Given compaction이 끝났을 때, when 같은 supervisor-held options로 rehydrate하면, then snapshot/manifest와 모든 required document가 일치한다.
- Given 불일치가 있을 때, when 다음 provider call을 준비하면, then call이 차단되거나 canonical full delivery로 전환된다.
- Given completed history가 있을 때, when 최적화하면, then eligible history와 omission reason만 receipt에 남고 document omission은 0이다.

### Story 2: 비권위 OMP memory

**When** OMP memory가 이전 세션 지식을 제안할 때,
**I want to** provenance, namespace, TTL, hash를 현재 저장소와 대조하되 shadow evidence로만 다루고,
**so I can** stale·tampered·injected memory를 권위 있는 사실로 사용하지 않는다.

**Acceptance Criteria**

- Given provenance가 완전한 fresh cache entry가 있을 때, when 검증하면, then shadow candidate로만 기록되고 active injection은 0이다.
- Given TTL 만료, namespace mismatch 또는 hash mismatch가 있을 때, when 검증하면, then entry가 결정적으로 생략된다.
- Given secret나 prompt injection 문자열이 있을 때, when sanitization하면, then secret는 유출되지 않고 injection은 instruction 권한을 얻지 못한다.
- Given canonical 문서와 memory가 충돌할 때, when context를 구성하면, then canonical 문서가 이긴다.

### Story 3: 증거 기반 승격과 rollback

**When** 운영자가 OMP 최적화를 active로 바꾸려 할 때,
**I want to** paired AB/BA 결과와 capability receipt를 gate로 사용하고,
**so I can** 열화가 생기면 즉시 shadow/full로 되돌릴 수 있다.

**Acceptance Criteria**

- Given 동일 task cohort가 있을 때, when full과 optimized를 AB/BA로 실행하면, then `task_id`별 두 결과가 정확히 pair된다.
- Given 승격 기준이 하나라도 실패할 때, when promotion을 평가하면, then active 전환이 거부된다.
- Given active profile에서 회귀가 감지될 때, when rollback하면, then effective profile readback이 `shadow` 또는 `off`다.
- Given live provider canary가 승인되지 않았을 때, when 기본 검증을 실행하면, then hermetic fake runtime만 사용한다.

## 5. Functional Requirements

### P0 — Must Have

| ID | Requirement | Notes |
|---|---|---|
| FR-01 | v1-delivered document와 ephemeral phase body를 never summarize/truncate/drop로 분류한다. | core/SPEC/conditional docs/task/ownership/finding 포함 |
| FR-02 | supervisor transient bundle을 실제 재주입하고 pre/post body/hash/ref equality를 검증한다. | bundle loss/mismatch는 fail-closed/full restart |
| FR-03 | native compaction 뒤 canonical full docs/ephemeral을 재주입하고 token credit은 completed history에만 귀속한다. | selective pinning 가정 0 |
| FR-04 | OMP memory를 provenance가 있는 shadow-only evidence로 검증한다. | active injection 0, TTL/namespace/redaction/injection guard |
| FR-05 | version/capability probe로 지원 기능을 해석한다. | 문서나 버전 문자열 단독 추정 금지 |
| FR-06 | 20+ complete AB/BA pair 뒤 history shadow→active promotion과 rollback receipt를 제공한다. | memory mode 독립, effective readback 필수 |
| FR-07 | 최소 20개 complete pair의 balanced AB/BA canary를 수행한다. | 19개 이하는 승격 거부, hermetic 기본 |
| FR-08 | managed OMP run은 no-session 또는 task-owned isolated root를 쓰고 종료 시 native transcript/compaction artifact를 제거한다. | user root 접근 0, cleanup 잔존 0 |

### P1 — Should Have

| ID | Requirement | Notes |
|---|---|---|
| FR-10 | doctor가 mode, capability, last checkpoint, fallback reason을 body-free로 표시한다. | secret/raw memory 제외 |
| FR-11 | `context_plan.v2` selection metrics를 OMP shadow evidence에 재사용한다. | active authority로 승격하지 않음 |

### P2 — Could Have

| ID | Requirement | Notes |
|---|---|---|
| FR-20 | 승인된 representative cohort별 장기 비용 추세를 비교한다. | Outcome Lock 밖의 운영 개선 |

## 6. Non-Functional Requirements

| Category | Requirement | Target |
|---|---|---|
| Integrity | required ref/hash equality | 100%, mismatch 0 |
| Security | secret redaction, injection neutralization, path containment | 회귀 0 |
| Quality | baseline 대비 task oracle pass delta | 0 이상 |
| Performance | paired median input token reduction | 20% 이상 |
| Recovery | fallback/rollback 성공 | 100% |
| Testability | 네 owning directory의 SPEC diff-owned non-test Go file coverage + package baseline | 85% 이상 + 감소 0 |
| Privacy | receipt에 raw prompt, body, secret, 절대 경로 저장 금지 | 0건 |

## 7. Technical Constraints

**Technology Stack Constraints**

- brownfield Go module과 기존 YAML/JSON receipt 패턴을 유지한다.
- 새 canonical database나 provider transcript writer를 추가하지 않는다.
- `BuildContextDelivery`의 complete `RequiredDocuments`와 `VerifyContextDeliveryForOptions`를 무결성 authority로 재사용한다.
- managed session은 proved `--no-session` 또는 task-owned 0700/0600 isolated root만 사용하고 user session root를 건드리지 않는다.
- `workerUsesGPTContextDelivery`의 provider-name gate를 OMP session projection으로 재사용하지 않는다.

**Technology Stack Decision**

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|---|---|---|---|---|---|
| brownfield | Go module + OMP capability-negotiated runtime | existing `go.mod`; local `omp/17.1.8` probe | `go.mod`, OMP official settings/compaction/memory docs, local `omp config list --json` | 2026-08-02 | OMP main 설정 고정, 새 vector DB, provider-native transcript mutation |

**External Dependencies**

| Dependency | Version / SLA | Risk if Unavailable |
|---|---|---|
| OMP CLI/runtime | capability-probed; local evidence `17.1.8`은 비권위 snapshot | optimization unavailable; canonical full fallback |
| OMP native compaction/memory hooks | capability receipt에 선언된 기능만 | shadow 유지 또는 기능별 disable |

**Compatibility Requirements**

- `autopus.context_delivery.v1` wire와 기존 full delivery 결과가 바뀌지 않아야 한다.
- `autopus.context_plan.v2`는 shadow evidence이며 OMP active dispatch의 authority가 아니다.
- OMP config object deep-merge와 array replacement를 고려하고 user-owned key를 덮어쓰지 않아야 한다.

**Infrastructure Constraints**

- 기본 canary는 fake runtime과 임시 workspace에서 수행한다.
- 실제 provider 호출은 별도 명시 opt-in과 자격증명 경계를 요구한다.

## 8. Out of Scope

The following are out of scope for this release:

- OMP advisor/model routing과 provider별 역할 선택 (`SPEC-OMP-003` 책임)
- OMP workflow/skill 실행 parity (`SPEC-OMP-002` 선행 책임)
- OMP를 orchestra provider로 자동 등록
- provider-native transcript 직접 수정 또는 OMP 내부 저장 포맷 소유
- 새 canonical context/memory storage
- v1-delivered document의 요약·절단·누락과 `context_plan.v2` active JIT 승격
- OMP memory content를 repo tracked source로 생성
- OMP memory body의 active prompt injection 또는 history active에 따른 implicit memory enable
- user global/project OMP memory·compaction default 변경

**Deferred to Future Iterations**

- live provider 대규모 비용 벤치마크는 opt-in canary 증거가 축적된 뒤 검토한다.

## 9. Risks & Open Questions

### Risks

| Risk | Severity | Probability | Mitigation Strategy |
|---|---|---|---|
| OMP main과 설치 버전의 hook/settings 차이 | High | High | runtime capability probe; unsupported면 shadow/full |
| compaction 후 required task state 손실 | High | Medium | pre-checkpoint, same-options rebuild, provider-call gate |
| native transcript/compaction artifact에 body 잔존 | High | Medium | no-session/isolated root, 0700/0600, cleanup count zero |
| stale/injected memory 재주입 | High | Medium | provenance+TTL+namespace+hash, redaction, omission receipt |
| project config array replacement | High | Medium | one-shot overlay와 conflict/readback oracle |
| token 절감이 품질 열화를 가림 | High | Medium | paired AB/BA와 quality/security equivalence 우선 |

### Open Questions

| # | Question | Owner | Due Date | Status |
|---|---|---|---|---|
| Q1 | 없음. 구현 세부 event surface는 capability probe 결과에 따라 결정한다. | executor | 2026-08-02 | Resolved by fail-closed design |

## 10. Pre-mortem

| # | Failure Scenario | Probability | Impact | Preventive Action |
|---|---|---|---|---|
| 1 | summary가 acceptance/open finding을 잃는다. | Medium | High | never-optimize classifier와 exact rehydration oracle |
| 2 | OMP 업그레이드가 hook payload를 바꾼다. | High | High | version+capability receipt, active admission 차단 |
| 3 | memory가 secret 또는 injected instruction을 보존한다. | Medium | High | pre-persist redaction, neutralization, adversarial fixture |
| 4 | 20% 절감만 보고 품질 열화를 승격한다. | Medium | High | integrity/security/quality gate를 token metric보다 선행 |
| 5 | rollback 표시와 실제 설정이 다르다. | Low | High | effective overlay/config readback hash 검증 |

**Connection to Risks (Section 9)**

각 failure는 version drift, data loss, memory trust, metric gaming, config ownership 위험을 직접 검증하는 Must scenario와 연결된다.

## 11. Practitioner Q&A

**Q1: context authority는 무엇인가?**
A: supervisor-held options로 다시 만든 `autopus.context_delivery.v1`과 그 source/prompt/snapshot/manifest hash다.

**Q2: 무엇을 절대 요약하지 않는가?**
A: v1이 선택한 core/relevant SPEC/plan/acceptance/research/conditional refs 전체와 원 태스크, ownership/forbidden paths, open findings, worker receipt schema다.

**Q3: OMP memory의 지위는 무엇인가?**
A: 현재 저장소 증거로 재검증하는 shadow-only candidate다. active prompt에 주입되지 않고 canonical docs나 pipeline learning을 대체하지 않는다.

**Q4: active 승격은 언제 가능한가?**
A: capability가 증명되고, 최소 20개 paired task에서 무결성·보안·품질 열화 0과 median token reduction 20% 이상, rollback 100%가 확인될 때다.

**Q5: hook이 설치 버전에서 없으면 어떻게 되는가?**
A: active를 거부하고 shadow/full delivery를 유지한다.

**Q6: rollback은 무엇을 증명해야 하는가?**
A: 요청 상태뿐 아니라 OMP에 적용되는 effective profile이 `shadow` 또는 `off`임을 readback hash로 증명해야 한다.

**Q7: `workerUsesGPTContextDelivery`를 확장하는가?**
A: 아니다. worker backend 이름과 OMP platform session은 다른 경계다. provider-neutral OMP binding을 별도로 둔다.

**Q8: 실제 provider canary가 완료 조건인가?**
A: 외부 live provider는 아니다. version과 무관하게 exact lifecycle capability를 통과한 installed OMP를 loopback fake provider와 task-owned runtime root로 구동한 canary는 필수이며, 외부 live canary는 명시 opt-in 증거다.
