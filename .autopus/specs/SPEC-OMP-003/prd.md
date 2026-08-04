# PRD: SPEC-OMP-003 - OMP 역할 기반 멀티프로바이더 모델 라우팅

**Status**: Implemented
**Created**: 2026-08-02
**Domain**: OMP
**Mode**: standard

## Discovery Q&A Checklist

- [x] **Problem**: OMP 에이전트가 `sonnet`/`opus`를 그대로 받아 실제 provider/model 선택과 품질 의도가 연결되지 않는다.
- [x] **Target Users**: OMP에서 Autopus 하네스를 사용하는 엔지니어와 하네스 운영자다.
- [x] **Success Metrics**: hermetic 16-agent 역할 행렬 100%, fallback/receipt/ignore oracle 100%, routing-owned changed Go file 85%+ coverage와 package baseline no-regression이다.
- [x] **Constraints**: OMP는 platform/runtime이며 orchestra provider가 아니다. 사용자 설정과 credential을 보존한다.
- [x] **Prior Art**: Codex catalog resolver, OMP adapter marker/manifest, quality preset 계층을 재사용한다.
- [x] **Scope Boundary**: OMP subprocess gateway, worker `ProviderAdapter`, pane, context 압축은 제외한다.

## Implementation Status

- T2~T11의 정책, catalog, resolver, projection, activation, agent generation, receipt/doctor, security와 integration implementation이 source tree에 반영됐다.
- Installed catalog의 semantic metadata gap은 `catalog_metadata_insufficient`로 fail-closed하고, explicit profile 선언과 actual available catalog의 교집합만 enrichment한다. 이 catalog 증거의 provider request는 0회다.
- T1 technical predecessor gate는 green이지만 SPEC-OMP-002 lifecycle commit/status 승격은 deferred이며, T12 integrated gates는 아직 parent pending이다.
- Paid live S19는 `not-run: explicit approval required`이고 S1~S18 completion 판단의 blocker가 아니다.

## 1. Problem & Context

**Current Situation**

`pkg/adapter/omp.prepareAgentMappings`는 `HarnessConfig`를 받지 않고 source agent를 변환한다.
`TransformAgentForOMP`는 source `model` 값을 그대로 frontmatter에 기록하며 `.omp/config.yml`의
Autopus 관리 영역은 `skills.customDirectories`뿐이다. 반면 OMP는 역할 alias, thinking selector,
fallback chain, 여러 provider의 auth-enabled model catalog를 제공한다.

**Problem Statement**

Autopus에는 역할·위험·품질 의도가 있지만 이를 OMP의 실제 provider/model로 안전하게 해석하는
공통 계약이 없다. 단순히 `quality.providers.omp`를 추가하면 platform과 LLM transport provider를
혼동하고, 사용자 설정 충돌·미인증 selector·검토자 독립성 상실을 탐지하지 못한다.

**Impact**

역할별 모델 최적화가 작동했는지 증명할 수 없고, fallback이 독립 검토로 오인되며, project array가
global allow/deny 목록을 대체해 예상 밖 provider를 다시 활성화할 수 있다.

**Change Motivation**

SPEC-OMP-001이 최소 플랫폼 표면을 완성했고 SPEC-OMP-002가 실행 parity와 설정 안전성을 선행
정리한다. 이제 OMP 17.1.8과 version-gated upstream capability를 활용할 수 있는 정책 계층이 필요하다.

## 2. Goals & Success Metrics

| Goal | Success Metric | Target | Timeline |
|------|----------------|--------|----------|
| 역할 의도를 실제 모델로 결정적 해석 | 동일 입력 resolution byte equality | 100% | 구현 iteration |
| availability와 독립 검토를 구분 | fallback/diversity oracle 통과 | 100% | 구현 iteration |
| 사용자 설정·secret 보존 | adversarial preservation fixture | 100% | 구현 iteration |
| 운영 증거 제공 | doctor/receipt 필수 필드 | 누락 0개 | 구현 iteration |
| 회귀 방지 | routing-owned changed-file coverage + package baseline | 85% 이상 + 감소 0 | sync 전 |

**Anti-Goals**

- 특정 vendor 모델을 Autopus 기본 source에 영구 고정하지 않는다.
- fallback을 provider quorum 또는 독립 리뷰 증거로 취급하지 않는다.
- OMP를 `orchestra.providers.omp`에 등록하지 않는다.

## 3. Target Users

| User Group | Role | Usage Frequency | Key Expectation |
|------------|------|-----------------|-----------------|
| OMP 엔지니어 | 구현·검토 작업 수행 | 매일 | 역할에 맞는 모델과 안전한 fallback |
| 하네스 운영자 | profile/catalog 관리 | 수시 | 설정 소유권과 effective 결과 증거 |
| 보안·품질 담당자 | reviewer/security 결과 감사 | 변경마다 | provider-family 독립성과 secret-free receipt |

**Primary User**: OMP에서 Autopus agent를 실행하는 엔지니어다.

## 4. User Stories / Job Stories

### Story 1: 역할 기반 실행

**When** OMP runtime profile을 opt-in하고 Autopus agent를 호출할 때,
**I want to** 역할과 위험도에 맞는 capability가 auth-enabled 모델로 해석되기를 원한다,
**so I can** vendor 이름을 agent source에 박지 않고 품질과 비용 의도를 유지할 수 있다.

### Story 2: 독립 검토 유지

**As a** reviewer/security 담당자,
**I want** executor와 다른 provider family가 선택됐는지 확인하고 싶다,
**so that** 단순 availability fallback을 독립 검토로 오인하지 않는다.

### Story 3: 운영 가능한 증거

**When** catalog, auth 또는 OMP 버전이 바뀔 때,
**I want to** doctor와 resolution receipt에서 요청·실효 선택·degraded 사유를 확인하고 싶다,
**so I can** 재현 가능한 상태로 문제를 진단할 수 있다.

## 5. Functional Requirements

### P0 — Must Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-01 | opt-in `RoleModelPolicy` profile을 capability route로 해석한다. | legacy tier 호환 포함 |
| FR-02 | capability canonical role과 현재 source agent 16개 전량을 OMP native role 또는 selector로 컴파일한다. | 미매핑 future agent fail-closed |
| FR-03 | 설치 버전과 auth-enabled catalog를 probe한다. | bounded, version-gated |
| FR-04 | exact selector·thinking·fallback 순서를 검증한다. | unresolved는 fail/degraded |
| FR-05 | reviewer/security의 provider-family 다양성을 검사한다. | fallback과 별도 증거 |
| FR-06 | overlay 또는 명시적 managed-key 소유권으로 OMP 설정을 투영하고 managed invocation의 effective readback을 검증한다. | sidecar-only PASS 금지, 충돌 fail-closed |
| FR-07 | approval/isolation safety profile을 명시적 opt-in으로 적용한다. | user state를 universal default로 단정 금지 |
| FR-08 | secret-free resolution receipt와 doctor 결과를 생성하고 receipt를 exact ignored/untracked runtime artifact로 유지한다. | catalog fingerprint 포함 |
| FR-09 | `prepareAgentMappings`가 config와 compiled policy를 소비한다. | raw tier passthrough 제거 |
| FR-10 | OMP platform/orchestra provider 경계를 보존한다. | `PlatformToProvider("omp")==""` |

### P1 — Should Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-11 | 같은 profile/catalog의 산출물을 canonical ordering으로 고정한다. | diff noise 방지 |
| FR-12 | 명시 승인된 1-call live canary로 실제 선택을 확인한다. | 기본 검증은 hermetic |

### P2 — Could Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-20 | 비용·latency 관측치를 다음 profile 추천에 사용한다. | 이번 Outcome Lock 밖 |

## 6. Non-Functional Requirements

| Category | Requirement | Target |
|----------|-------------|--------|
| Determinism | 같은 policy/version/catalog/config는 같은 projection/receipt를 낸다. | byte-identical |
| Security | secret, token, auth material, 민감 env 값은 생성물·receipt에 없다. | 노출 0건 |
| Compatibility | legacy `opus/sonnet/haiku` 입력과 비-OMP 플랫폼 동작을 보존한다. | 회귀 0건 |
| Reliability | probe timeout, malformed/oversized catalog를 bounded 처리한다. | hang 0건 |
| Testability | fake catalog와 fake OMP runner로 network/auth 없이 검증한다. | Must oracle 100% |
| Coverage | routing-owned changed production file coverage와 pre-change package baseline을 유지한다. | 85% 이상, baseline 감소 0 |

## 7. Technical Constraints

**Technology Stack Decision**

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | 기존 Go module + `gopkg.in/yaml.v3` + OMP CLI | `go.mod` 유지, installed OMP 17.1.8 | repo manifest, OMP official main docs | 2026-08-02 | 신규 YAML/SDK dependency 불필요 |

**External Dependencies**

| Dependency | Version / SLA | Risk if Unavailable |
|------------|---------------|---------------------|
| OMP CLI | installed 17.1.8, future version probe | profile projection을 fail/degraded 처리 |
| OMP model catalog/auth state | runtime-local | 모델 선택 불가, credential 자체는 읽거나 기록하지 않음 |

**Compatibility Requirements**

- SPEC-OMP-002 완료 후 구현한다.
- SPEC-OMP-001 REQ-018과 `PlatformToProvider("omp") == ""`를 보존한다.
- OMP official `main` 문서는 가능성 근거일 뿐 설치 17.1.8 지원을 대신하지 않는다.
- project array는 상위 layer array를 통째로 대체하므로 암묵적으로 쓰지 않는다.

## 8. Out of Scope

- OMP gateway, orchestra subprocess, worker `ProviderAdapter`, interactive pane.
- provider quorum·토론·judge를 OMP fallback/advisor로 대체하는 일.
- API key, OAuth token, auth broker secret 생성·복사·영구 기록.
- 기존 `pkg/promptlayer` authority를 소비하는 SPEC-OMP-004의 OMP lifecycle bridge, compaction, memory 최적화.
- 모델 benchmark 기반 자동 자기수정 profile.

## 9. Risks & Open Questions

### Risks

| Risk | Severity | Probability | Mitigation Strategy |
|------|----------|-------------|---------------------|
| OMP main과 설치 버전 capability drift | High | Medium | version probe와 supported-key gate |
| selector가 fuzzy하게 다른 모델로 해석 | High | Medium | exact canonical selector oracle |
| array 대체가 provider policy를 완화 | High | Medium | full-array ownership 없으면 fail-closed |
| gateway provider가 실제 model family를 숨김 | Medium | Medium | catalog profile의 명시적 family metadata 요구 |
| receipt가 auth 상태를 과도하게 노출 | High | Low | boolean availability와 fingerprint만 기록 |

### Open Questions

| # | Question | Owner | Due Date | Status |
|---|----------|-------|----------|--------|
| - | Outcome Lock을 막는 미해결 질문 없음 | - | - | Resolved |

## 10. Pre-mortem

| # | Failure Scenario | Probability | Impact | Preventive Action |
|---|------------------|-------------|--------|-------------------|
| 1 | agent source에 vendor 모델이 다시 하드코딩됨 | Medium | High | semantic capability golden test |
| 2 | fallback을 독립 검토로 보고함 | Medium | High | diversity receipt를 별도 필드로 검증 |
| 3 | 사용자 `.omp/config.yml`이 재포맷·권한 완화됨 | Low | High | overlay 기본, byte/mode oracle |
| 4 | 구형 OMP가 미지원 key를 무시함 | Medium | High | capability gate와 degraded reason |
| 5 | live test가 비용·외부 상태로 flaky해짐 | Medium | Medium | fake catalog가 release gate, live는 1-call 승인 canary |

## 11. Practitioner Q&A

**Q1: 왜 `quality.providers.omp`를 추가하지 않는가?**
A: 그 map은 LLM transport 품질 모드용이고 OMP는 여러 model provider를 호스팅하는 platform/runtime다.

**Q2: 실제 모델 이름은 어디에 있는가?**
A: 사용자가 선택한 role-model profile/catalog에만 있으며 기본 agent source에는 없다.

**Q3: fallback은 멀티프로바이더 검토인가?**
A: 아니다. availability 복구일 뿐이며 provider quorum은 기존 orchestra 계약이 소유한다.

**Q4: 사용자 OMP config는 누가 소유하는가?**
A: overlay가 기본이다. project-managed key는 명시적 opt-in과 이전 fingerprint가 일치할 때만 Autopus가 소유한다.

**Q5: catalog가 없으면 무엇을 하는가?**
A: 필수 역할은 fail-closed한다. 명시적으로 허용된 degraded mode만 role alias/runtime default를 사용하고 receipt에 사유를 남긴다.

**Q6: 실제 provider 인증을 테스트에 필요한가?**
A: S1~S18 release gate는 catalog와 config readback까지 hermetic fake OMP다. actual readback과 live 1-call canary는 별도 명시 승인이 있을 때만 S19에서 실행한다.
