# PRD: OMP 워크플로 실행 파리티 및 설정 안전성

> Product Requirements Document — Standard (11-section format).

- **SPEC-ID**: SPEC-OMP-002
- **Author**: Autopus-ADK
- **Status**: Implemented
- **Date**: 2026-08-02

## Discovery Q&A Checklist

- [x] **Problem**: OMP가 커맨드 20종을 발견해도 상세 `auto-*` 스킬을 생성하지 않아 실행 계약이 중간에 끊긴다.
- [x] **Target Users**: OMP-only 워크스페이스에서 Autopus 워크플로를 실행하는 엔지니어와 하네스 운영자다.
- [x] **Success Metrics**: `workflowSpecs`에서 파생한 command/skill 20/20, thin router 1개와 detailed skill 19개 파리티, content-gated 대표 RPC workflow smoke 완료, 설정 내용·mode·remove transaction 오라클 전건 통과, OMP 변경 패키지 커버리지 85% 이상이다.
- [x] **Constraints**: 기존 OMP 설정 마커는 `skills.customDirectories`만 관리하며 모델 라우팅과 실 인증 모델 호출은 제외한다.
- [x] **Prior Art**: `SPEC-OMP-001`, OpenCode workflow skill emitter, cross-adapter context engineering oracle을 재사용한다.
- [x] **Scope Boundary**: OMP를 orchestra provider로 등록하지 않고 root meta workspace의 generated surface를 커밋하지 않는다.

## 1. Problem & Context

**Current Situation**

`SPEC-OMP-001`은 OMP 규칙 14종, 에이전트 16종, 커맨드 20종과 최소 스킬 표면을 제공했다. 현재 `pkg/adapter/omp/omp_skills.go`는 얇은 `auto` 스킬 하나만 생성하지만 `omp_commands.go`는 matching `auto-*` 스킬을 로드하라고 지시한다. 기존 delivery receipt도 실 모델 turn의 스킬 호출은 Not-tested로 남겼다.

**Problem Statement**

OMP 사용자는 보이는 커맨드를 선택할 수 있지만 상세 실행 계약과 context profile이 생성물에 없으므로 동일한 Autopus 명령이 다른 플랫폼보다 약하게 실행되거나 런타임에서 중단될 수 있다. 동시에 `Clean`은 사용자 내용이 남은 `.omp/config.yml`을 `0644`로 다시 써 기존 `0600` 권한을 넓히고, `Validate`는 세 경로 존재만 확인해 잘못된 값과 실행 불능을 healthy로 판정한다.

**Impact**

해결하지 않으면 OMP 지원은 discovery badge와 실제 실행 사이가 분리되고, 사용자 provider 설정이 든 파일의 기밀성이 platform remove 시 약화될 수 있다. 모델 라우팅 최적화를 시작해도 신뢰할 실행·진단 기준선이 없다.

**Change Motivation**

OMP 멀티프로바이더 최적화 전에 workflow parity, context receipt, config safety, value-based readiness를 하나의 기준선으로 닫아야 한다.

## 2. Goals & Success Metrics

| Goal | Success Metric | Target | Timeline |
|------|---------------|--------|----------|
| OMP workflow parity | `workflowSpecs`와 workflow skill set 일치 | command/skill 20/20, router 1 + detail 19, 누락·고아 0 | SPEC sync 전 |
| 실행 증명 | 실제 OMP RPC + auth-free fake provider 대표 turn | ordered skill load와 tool execution 1/1 PASS | SPEC sync 전 |
| 설정 안전성 | Generate/Update/Clean byte·mode oracle | 0600 fixture 포함 전건 PASS | SPEC sync 전 |
| readiness 정확성 | 구조만 정상이고 값이 잘못된 fixture 탐지 | rules/agents/workflows/compiled skills의 source-derived set 및 YAML/model/version 오류 100% 탐지 | SPEC sync 전 |
| 회귀 방지 | OMP 변경 패키지 statement coverage | 85% 이상 | SPEC sync 전 |

**Anti-Goals**

- 특정 LLM/provider를 역할에 배정하거나 `.omp/config.yml`에 `modelRoles`를 생성하지 않는다.
- 인증된 상용 모델의 응답 품질을 이 SPEC의 PASS 조건으로 삼지 않는다.
- OMP process ancestry를 provider·권한·judge 선택의 authority로 승격하지 않는다.

## 3. Target Users

| User Group | Role | Usage Frequency | Key Expectation |
|------------|------|-----------------|-----------------|
| OMP 기반 엔지니어 | 워크플로 실행자 | Daily | `/auto-*`가 다른 하네스와 같은 상세 계약으로 실제 실행된다. |
| 하네스 운영자 | 설치·진단 담당 | Weekly / release | doctor가 누락뿐 아니라 잘못된 값과 버전 불일치를 설명한다. |
| Autopus 개발자 | 어댑터 유지보수자 | Per change | context·ownership·권한 파리티가 결정적 테스트로 잠긴다. |

**Primary User**: OMP-only 워크스페이스에서 `/auto plan`, `/auto go`, `/auto review`를 실행하는 엔지니어다.

## 4. User Stories / Job Stories

### Story 1: 상세 워크플로 실행

**When** OMP-only 프로젝트에서 Autopus slash command를 선택할 때,
**I want to** router와 matching 상세 스킬이 동일한 payload와 context profile을 사용하게 하고,
**so I can** discovery가 아니라 실제 완료 turn까지 일관된 하네스를 실행한다.

Acceptance Criteria: Given `/auto plan`과 `/auto-plan` 생성 surface가 있을 때, when 본문 계약을 파싱하면, then 둘 다 `workflowSpecs`의 `auto-plan` exact target과 model/variant placeholder를 한 번씩 방출한다. 실제 대표 wiring은 content-gated RPC로 별도 검증한다.

### Story 2: 안전한 설정 수명주기

**When** 내 `.omp/config.yml`에 사용자 설정과 제한된 file mode가 있을 때,
**I want to** Generate, Update, Clean이 `skills.customDirectories` 한 owned YAML path만 구조적으로 관리하게 하고,
**so I can** platform lifecycle 후에도 사용자 내용과 기밀성 경계를 유지한다.

Acceptance Criteria: Given mode `0600`, sibling `skills` key, managed path 밖 사용자 bytes가 있을 때, when 세 lifecycle을 수행하면, then 남는 파일의 mode와 사용자 bytes/node는 동일하고 owned-path 충돌은 fail-closed한다.

### Story 3: 실행 readiness 진단

**When** OMP 표면이 디스크에는 있지만 값이나 설치 버전이 맞지 않을 때,
**I want to** doctor가 expected/got 값과 capability 원인을 보여주게 하고,
**so I can** 실제 모델 요청 전에 고칠 수 있다.

Acceptance Criteria: Given 19개 커맨드와 잘못된 `customDirectories`가 있을 때, when doctor를 실행하면, then directory existence가 모두 참이어도 두 value finding이 분리되어 보고된다.

## 5. Functional Requirements

### P0 — Must Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-01 | OMP-only 생성물이 thin `auto` router 1종과 detailed `auto-*` skill 19종을 만들고 command 20종과 1:1 대응한다. | router와 direct alias body oracle 분리 |
| FR-02 | plan/test/canary/go context matrix와 worker receipt 5필드를 cross-adapter oracle로 검증한다. | OMP를 기존 surface map에 추가 |
| FR-03 | Generate/Update/Clean은 두 marker 표현으로 `skills.customDirectories`만 구조적으로 소유하고 bytes/node/mode를 보존하며 remove를 preflight→Clean→save로 처리한다. | 신규 config 0600, conflict fail-closed, repair receipt |
| FR-04 | Validate/doctor는 source-derived surface set, actual `omp models --json`, selector, exact 10개 OMP identity/capability를 값으로 검증한다. | credential·catalog·protocol 원인 분리 |
| FR-05 | 실제 OMP RPC에서 content-gated auth-free fake provider가 `/auto plan`과 `/auto-plan` 대표 wiring, paired tool event, `message_end`를 완료한다. | 일반 parser·품질 증거 아님 |
| FR-06 | bare `omp` ancestry는 authority 신호로 사용되지 않는다. | F-010 경계 고정 |
| FR-07 | user-owned `.omp/`, manifest ownership, mixed-platform 양보, root meta workspace 비커밋 정책을 보존한다. | generated surface는 scratch에서만 검증 |

### P1 — Should Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-10 | doctor text/JSON finding은 안정적인 ID와 expected/got/version detail을 제공한다. | 자동 복구는 범위 밖 |

### P2 — Could Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-20 | 없음 | optional 확장은 research의 Evolution Ideas에만 둔다. |

## 6. Non-Functional Requirements

| Category | Requirement | Target |
|----------|-------------|--------|
| Correctness | command-skill-context 추적 | 20/20 set equality, 대표 RPC 1/1 |
| Security | config mode와 secret 비노출 | 기존 mode 보존, 신규 0600, credential bytes/log 0 |
| Compatibility | OMP main 문서와 설치 버전 차이 | hard-coded main 가정 0, version/capability probe 필수 |
| Reliability | fake-provider smoke | 외부 인증 0, bounded timeout, deterministic ordered trace |
| Testability | 변경 패키지 coverage | 85% 이상 |
| Maintainability | 소스 파일 크기 | 변경·신규 Go 파일 각 300줄 이하 |

## 7. Technical Constraints

**Technology Stack Constraints**

- Brownfield Go module과 기존 `gopkg.in/yaml.v3`, `testify`, 표준 `os/exec`, `net/http`, `encoding/json`을 재사용한다.
- OMP CLI는 빌드 의존성이 아니라 live integration dependency다.
- `PlatformAdapter` 경계와 OMP manifest ownership을 유지하고 remove의 mutation 순서를 preflight→Clean→config save로 고정한다.
- OMP config marker 관리 대상은 `skills.customDirectories`로 고정한다.

**Technology Stack Decision**

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go + existing adapter/test stack | `go 1.26`, toolchain `go1.26.5` | repository `go.mod` | 2026-08-02 | 새 런타임·dependency 불필요 |
| brownfield integration | OMP capability-probed CLI | installed `omp/17.1.8`; main docs는 이동 가능 | `omp --version`, official settings/models/providers docs | 2026-08-02 | main 문서 동작의 무조건 하드코딩 거부 |

**External Dependencies**

| Dependency | Version / SLA | Risk if Unavailable |
|------------|---------------|---------------------|
| OMP CLI | capability probe가 승인한 버전; 기준 설치본 17.1.8 | live RPC gate는 blocked, unit/fixture gate와 혼동하지 않음 |
| Local fake provider | test-owned loopback endpoint | 시작 실패 시 smoke fail-closed |

**Compatibility Requirements**

- `SPEC-OMP-001`의 현재 rules 14, agents 16, commands 20 증거를 회귀시키지 않되 expected set은 content/catalog source에서 파생한다.
- 기존 Claude, Codex, OpenCode, Gemini, Antigravity context matrix의 expected set을 바꾸지 않는다.

**Infrastructure Constraints**

- credential·OAuth·외부 network 없이 RPC smoke를 실행한다.
- root meta workspace에는 생성물을 쓰지 않고 임시 workspace만 사용한다.

## 8. Out of Scope

- OMP 역할별 멀티프로바이더 모델 라우팅과 `modelRoles`/fallback 생성 — `[NEW] SPEC-OMP-003` 책임.
- OMP compaction/memory 기반 context-native 최적화 — `[NEW] SPEC-OMP-004` 책임.
- OMP orchestra provider, worker `ProviderAdapter`, interactive pane 구현.
- 실 인증 모델 호출, 모델 품질 벤치마크, provider 가격 최적화.
- config marker에 `enabledModels`, `disabledProviders`, `tools`, `task`, `memory` 키 추가.

**Deferred to Future Iterations**

없음. Outcome Lock의 필수 범위는 이 Primary SPEC 안에서 닫고, 위 항목은 독립 outcome의 related Primary SPEC이다.

## 9. Risks & Open Questions

### Risks

| Risk | Severity | Probability | Mitigation Strategy |
|------|----------|-------------|---------------------|
| OMP main과 17.1.8 RPC/schema 차이 | High | Medium | version identity 후 capability probe, unsupported면 구체 finding으로 fail-closed |
| fake provider가 discovery나 scripted sequence만 증명 | High | Medium | actual request body/hash content gate, paired OMP tool event, `message_end`를 고정하고 일반 parser 증명과 분리 |
| shared `.agents/` 소유권 침범 | High | Low | mixed-platform manifest set과 byte-identity 회귀 테스트 |
| config YAML 충돌·Clean 권한 확대·부분 제거 재발 | High | Medium | 두 marker 표현, pre/post 0600, preflight→Clean→save, late repair receipt oracle |
| 상세 template의 foreign-platform 문구 잔존 | Medium | Medium | OMP normalization과 금지 토큰 set 검사 |

### Open Questions

| # | Question | Owner | Due Date | Status |
|---|----------|-------|----------|--------|
| Q1 | live smoke가 지원할 RPC frame 모양은 무엇인가? | executor | 구현 시작 시 | Resolved: `available_commands_update`, paired `tool_execution_start/end`, `message_end` probe |
| Q2 | 인증 없는 환경에서 모델 availability는 어떻게 판정하는가? | executor | 구현 시작 시 | Resolved: syntax/catalog는 error, credential 부재는 별도 warning, request 없음 |

## 10. Pre-mortem

| # | Failure Scenario | Probability | Impact | Preventive Action |
|---|-----------------|-------------|--------|-------------------|
| 1 | 20개 스킬이 생성됐지만 direct command가 다른 이름을 로드한다. | Medium | High | workflowSpecs 기반 양방향 set·mapping oracle |
| 2 | RPC smoke가 command 목록이나 provider-script만으로 PASS한다. | Medium | High | actual request content gate → paired tool event → `message_end` 요구 |
| 3 | platform remove가 YAML sibling을 잃거나 config를 0644로 쓰고 부분 상태를 숨긴다. | Medium | High | 두 marker 표현, 0600 fixture, preflight→Clean→save와 repair receipt |
| 4 | 새 OMP 버전에서 RPC 명령이 바뀌어 doctor가 오판한다. | Medium | Medium | version/capability receipt와 unsupported finding 분리 |
| 5 | process명이 `omp`인 무관한 바이너리를 trusted runtime으로 쓴다. | Low | High | ancestry는 hint-only, version/provenance 없이 authority 금지 |

**Connection to Risks (Section 9)**: 실패 1·2는 실행 parity, 3은 설정 안전성, 4는 version drift, 5는 F-010 identity risk와 직접 연결된다.

## 11. Practitioner Q&A

**Q1: 상세 workflow와 pipeline의 source of truth는 무엇인가?**
A: workflow는 `workflowSpecs`와 기존 templates, pipeline은 `content/skills/agent-pipeline.md`와 compiled skill catalog이며 OMP 생성물은 파생 surface다.

**Q2: 모든 20개 워크플로를 live 실행하는가?**
A: 모든 20개는 set·content parity를 검증하고, 대표 `auto-plan`은 실제 RPC fake-provider turn으로 실행한다.

**Q3: fake provider가 parser나 품질을 증명하는가?**
A: 아니다. actual request content gate를 통과한 대표 command expansion, skill load, context 전달, tool execution, completion protocol만 증명한다.

**Q4: model selector 검증이 모델 라우팅인가?**
A: 아니다. 기존 agent frontmatter selector의 syntax/catalog readiness만 확인하며 정책 생성은 SPEC-OMP-003에 남긴다.

**Q5: credential 부재는 doctor 실패인가?**
A: selector/catalog 불능과 credential 부재를 분리한다. 전자는 error, 후자는 명시적 readiness warning이며 모델 호출은 하지 않는다.

**Q6: rollback은 어떻게 하는가?**
A: preflight 후 OMP manifest 파일과 marker만 제거하고 Clean 성공 뒤 config를 저장한다. late save 실패는 changed paths와 `repair_required`를 남긴다.

**Q7: F-010은 어떻게 닫는가?**
A: `AgentRuntimeOMP` process ancestry를 telemetry/hint로만 제한하고 adapter/doctor는 resolved executable의 `omp/` version 및 capability evidence를 사용한다.

**Q8: root generated surface는 갱신하는가?**
A: 구현 검증은 scratch workspace에서 수행하며 root meta workspace generated surface는 커밋하지 않는다.
