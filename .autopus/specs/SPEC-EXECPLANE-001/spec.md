# SPEC-EXECPLANE-001: 실행 평면 분리와 티어·계정·카탈로그 정합

**Status**: draft
**Created**: 2026-08-10
**Domain**: EXECPLANE

---
id: SPEC-EXECPLANE-001
title: 실행 평면 분리와 티어·계정·카탈로그 정합
version: 0.1.0
status: draft
priority: HIGH
---

## Purpose

autopus-adk·omp·orca 셋을 동시에 쓰되 서로의 어휘를 복제하지 않도록 **세 평면 경계를 고정**하고, 그 경계에서 끊겨 있는 접합면 하나 — 티어 약속과 실제 실행 계정 — 를 잇는 계약을 정의한다.

이 SPEC은 설계 고정 문서다. 구현은 포함하지 않는다.

## Background

세 시스템은 각자 독점적인 강점을 갖는다.

| 평면 | 시스템 | 독점 표면 |
| --- | --- | --- |
| 정책 | autopus-adk | SPEC, 16 canonical agent, quality tier, 게이트, 합의 전략, 비용 |
| 모델 | omp | role -> `provider/model:thinking`, provider 횡단 카탈로그, RPC 제어 |
| 프로세스 | orca | worktree 격리, provider 계정 소유, 감독 워커, durability, 스케줄 |

접합면은 둘이다. **J1(티어 -> 모델)은 PR #154로 닫혔다** — `quality.default`가 내장 role-model 프로파일을 거쳐 omp `modelRoles`에 도달한다. **J2(티어 -> 실행 계정)는 끊겨 있다.**

끊김의 형태는 research.md F1·F2에 실측으로 기록되어 있다. 이 워크스테이션에서 orca의 활성 Codex 계정은 `bitgapnam@gmail.com`인데 로컬 `codex` CLI 로그인은 `jroad1049@gmail.com`이다. autopus의 카탈로그 프로브는 PATH 바이너리를 실행하므로(`pkg/codexruntime/probe.go:39`) **로컬 로그인 계정의 카탈로그로 검증하고, orca는 다른 계정으로 실행한다**. `codex debug models`는 계정별 카탈로그이므로 두 집합은 다를 수 있다.

이 실패 양식은 가설이 아니다. PR #151에서 실제로 발생해 티어 승격이 세대 강등으로 뒤집혔고, 조용했기 때문에 근거와 결과가 어긋난 채 결론까지 갔다(research.md F3).

## Outcome Boundary

- **Outcome Lock**: 세 평면의 소유 표면이 문서로 배타적으로 고정되고, autopus가 결정한 모델 티어는 그 워크로드를 실제로 실행할 계정의 카탈로그로 검증된 뒤에만 실행 단계로 넘어간다. 검증에 실패하면 조용히 강등되지 않고 원한 티어·실행 계정·실제 제공 모델 셋을 모두 지목하는 영수증을 남긴 뒤 멈추거나 명시적으로 강등한다.
- **Mandatory requirements**: 평면 배타성 선언(REQ-001), orca 티어 어휘 무증식(REQ-002), 실행 계정 확인(REQ-003), 실행 계정 기준 카탈로그 검증(REQ-004), 정합 영수증 방출(REQ-005), 불일치 시 fail-loud(REQ-006), 부작용 이전 배치(REQ-007), handoff 경계 배치(REQ-008), 검증 불가 provider의 명시적 표기(REQ-009).
- **Explicit non-goals**: 정합 게이트의 구현(별도 SPEC), `--execution-owner orca` handoff 스텁을 실제 Run으로 대체하는 일(별도 SPEC), 여러 계정 사이의 자동 라우팅, 티어 강등 시 대안 provider 폴백, 실행 원장 통합, J1 재설계(PR #154 결과 불변), omp `modelRoles` 어휘 변경, orca CLI 표면 변경.
- **Completion evidence**: 4개 SPEC 문서가 `auto spec validate`를 통과하고, research.md의 Completion Debt 3건이 해소 또는 후속 SPEC으로 이관되며, 세 평면 경계에 대한 리뷰 합의가 기록된다. 구현 증거는 요구하지 않는다.

## Requirements

### REQ-001 — 세 평면의 소유 표면은 배타적으로 선언된다
THE SYSTEM SHALL declare exactly one owning plane for each execution surface — policy for SPEC, canonical agent roles, quality tiers, gates, consensus, and cost; model for role-to-model routing, thinking levels, and cross-provider catalogs; process for worktree isolation, provider accounts, supervision, durability, and scheduling — so that adding a capability to one plane never duplicates its vocabulary in another.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 세 시스템 중 하나에 모델 선택·실행 배치 관련 표면을 추가할 때.
- Observability: research.md Feature Coverage Map의 각 표면에 소유자가 정확히 하나 지정되어 있고, 같은 어휘가 둘 이상의 평면에 나타나지 않음을 S1로 확인한다.

### REQ-002 — 프로세스 평면에 티어 어휘를 도입하지 않는다
THE SYSTEM SHALL keep the process plane free of tier vocabulary, passing only opaque provider model identifiers and effort levels across the policy-to-process boundary, so the tier ladder stays single-sourced in the policy plane.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 정책 평면이 프로세스 평면에 실행을 위임할 때.
- Observability: 경계를 넘는 값에 `balanced`/`ultra`/`opus`/`sonnet`/`haiku` 토큰이 없고 provider 고유 model id와 effort만 있음을 S2로 확인한다.

### REQ-003 — 실행 전에 실제 실행 계정을 확인한다
WHEN the policy plane prepares a workload for execution, THEN THE SYSTEM SHALL determine the provider account that will actually run it from the process plane rather than assuming the account the local CLI is logged in as.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 파이프라인·워크플로가 provider 프로파일을 확정하는 시점.
- Observability: 정합 영수증에 실행 계정 식별자가 기록되고, 그 값이 로컬 CLI 로그인 계정과 다를 수 있음을 S3의 분기 계정 픽스처로 확인한다.

### REQ-004 — 카탈로그 검증은 실행 계정 기준으로 수행한다
THE SYSTEM SHALL validate a requested model tier against the model catalog of the execution account identified in REQ-003, and SHALL NOT treat a catalog probed under a different account as evidence for that tier.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 요청 티어를 provider 모델로 해석할 때.
- Observability: 검증에 사용한 카탈로그의 출처 계정이 영수증의 실행 계정과 일치함을 S3로 확인한다. 두 값이 다르면 검증은 성립하지 않은 것으로 처리된다.

### REQ-005 — 정합 결과를 영수증으로 방출한다
THE SYSTEM SHALL emit an integrity receipt carrying the requested tier, the resolved provider model, the execution account identifier, the catalog source, the resolution reason, and the verification status, so a later reader can reconstruct why a workload ran at the tier it ran at.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 정합 점검이 끝난 시점.
- Observability: 영수증이 위 6개 필드를 모두 담고 스키마 버전을 갖는지 S4로 확인한다.

### REQ-006 — 티어 불일치는 조용히 강등되지 않는다
IF the execution account cannot serve the requested tier, THEN THE SYSTEM SHALL fail closed or record an explicit downgrade that names both the requested tier and the model actually available, and SHALL NOT substitute a lower model with no record.
- EARS type: Unwanted
- Priority: Must
- Trigger/Condition: 실행 계정 카탈로그에 요청 모델이 없을 때.
- Observability: 강등 경로에서 영수증의 `reason`이 비어 있지 않고 요청 값과 실제 값이 모두 존재함을 S5로 확인한다. 두 값 중 하나라도 없으면 위반이다.

### REQ-007 — 정합 점검은 실행 부작용 이전에 끝난다
WHEN the integrity check runs, THEN THE SYSTEM SHALL complete it before creating any worktree, Run, worker, or provider session, so a failed check leaves no resource behind.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 워크로드 준비 단계.
- Observability: 점검 실패 시나리오에서 생성된 워크트리·Run·세션이 0건임을 S6로 확인한다.

### REQ-008 — 소유자 handoff 직전에 배치한다
WHERE the execution owner is the process plane, THE SYSTEM SHALL run the integrity check before emitting the handoff result, so the receiving side starts from an already-verified tier contract instead of rediscovering the mismatch mid-run.
- EARS type: State-driven
- Priority: Must
- Trigger/Condition: `--execution-owner orca` 경로가 handoff 결과를 반환하기 직전.
- Observability: handoff 결과에 정합 영수증 참조가 포함되고, 점검 실패 시 handoff 결과 대신 정합 실패가 반환됨을 S7로 확인한다.

### REQ-009 — 검증 수단이 없는 provider는 검증됨으로 표기하지 않는다
IF a provider exposes no account-scoped catalog probe, THEN THE SYSTEM SHALL mark that provider's tier as unverified in the receipt rather than reporting it as verified.
- EARS type: Unwanted
- Priority: Must
- Trigger/Condition: provider가 카탈로그 조회 수단을 제공하지 않을 때.
- Observability: 해당 provider의 영수증 항목이 `unverified` 상태와 사유를 갖고, `verified`로 표기되지 않음을 S8로 확인한다.

## Acceptance Criteria

- [ ] 세 평면의 소유 표면이 배타적으로 열거되고 중복 어휘가 없다
- [ ] 프로세스 평면 경계를 넘는 값에 티어 어휘가 없다
- [ ] 정합 점검이 로컬 로그인 계정이 아니라 실행 계정을 사용한다
- [ ] 정합 영수증이 6개 필드를 모두 담는다
- [ ] 티어 불일치가 조용히 강등되지 않는다
- [ ] 점검 실패가 리소스를 남기지 않는다
- [ ] handoff 결과가 정합 영수증을 참조한다
- [ ] 검증 불가 provider가 `unverified`로 표기된다

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1 | S1 | INV-001 |
| REQ-002 | T1 | S2 | INV-001 |
| REQ-003 | T2 | S3 | INV-003 |
| REQ-004 | T2 | S3 | INV-003 |
| REQ-005 | T3 | S4 | INV-004 |
| REQ-006 | T3 | S5 | INV-004 |
| REQ-007 | T4 | S6 | INV-005 |
| REQ-008 | T4 | S7 | INV-002, INV-005 |
| REQ-009 | T3 | S8 | INV-004 |

## Out of Scope

- 정합 게이트의 구현 코드. 이 SPEC은 계약만 고정한다.
- `--execution-owner orca`의 handoff 스텁을 실제 orca Run 생성으로 대체하는 작업.
- 요청 티어를 제공 가능한 계정을 자동 선택하는 다계정 라우팅.
- 티어 강등이 불가피할 때 다른 provider로 넘기는 폴백 정책.
- autopus·omp·orca 관측 데이터를 한 영수증으로 합치는 실행 원장.
- J1(티어 -> omp `modelRoles`) 재설계. PR #154 결과를 불변으로 둔다.
- orca CLI 표면 변경. orca는 소비자로만 취급한다.

## Traceability

| Requirement | Test | Status |
|-------------|------|--------|
| REQ-001 | S1 | pending |
| REQ-002 | S2 | pending |
| REQ-003 | S3 | pending |
| REQ-004 | S3 | pending |
| REQ-005 | S4 | pending |
| REQ-006 | S5 | pending |
| REQ-007 | S6 | pending |
| REQ-008 | S7 | pending |
| REQ-009 | S8 | pending |
