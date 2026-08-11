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

끊김의 형태는 research.md F1·F2·F8·F9에 실측으로 기록되어 있다. 이 워크스테이션에는 provider 계정이 셋 있다 — orca 관리 Codex 활성 계정 `bitgapnam@gmail.com`, 로컬 codex CLI 계정 `gnkong@alipeople.kr`(orca가 `codex.systemDefault`로 같은 값을 보고한다), 로컬 claude CLI 계정 `jroad1049@gmail.com`. orca는 계정별 `CODEX_HOME`을 실제로 갈아끼우고(`codex-accounts/<accountId>/home/.orca-managed-home`), autopus의 카탈로그 프로브는 PATH 바이너리를 실행한다(`pkg/codexruntime/probe.go:39`). 즉 **`gnkong` 계정의 카탈로그로 검증하고 `bitgapnam` 계정으로 실행한다**.

이 위험은 자격에 종속된 카탈로그에서 나온다. `codex debug models`를 인증 없이 프로브하면 슬러그 집합이 달라진다 — 인증 시에만 `gpt-5.6-sol-wm`과 `gpt-5.3-codex-spark`가 나오고 미인증에서는 `gpt-5.2`가 대신 나온다(research.md F9). 서버가 호출자의 자격을 보고 목록을 바꾼다.

다만 **위험 조건은 계정 신원 차이가 아니라 자격 등급 차이다.** 위 두 Codex 계정은 조직이 다르지만 자격 등급이 같아(`chatgpt_plan_type: "pro"`) 판정에 쓰이는 필드가 하나도 다르지 않았다 — 슬러그 집합과 `supported_reasoning_levels`가 동일하고, 갈리는 것은 `ResolveCodexProfile`이 읽지 않는 `base_instructions`·`model_messages`뿐이다. 따라서 게이트의 기본 경로는 카탈로그 재프로브가 아니라 **자격 등급 비교**다.

PR #151의 세대 강등은 이 SPEC이 다루는 실패가 **아니다**. 그것은 한 계정 안에서 카탈로그 폴백 로직이 만든 버그였고 PR #152가 고쳤다(research.md F3). 계정 불일치로 인한 손해는 아직 관측된 적이 없다. #151이 이 SPEC에 주는 것은 사례가 아니라 비용의 크기다 — 티어 판정이 틀리면 조용히 틀리고, 조용하면 근거와 결과가 어긋난 채로 결론까지 간다.

## Outcome Boundary

- **Outcome Lock**: 세 평면의 소유 표면이 문서로 배타적으로 고정되고, autopus가 결정한 모델 티어는 그 워크로드를 실제로 실행할 계정과 **같은 자격으로** 검증된 뒤에만 실행 단계로 넘어간다. 자격 등급이 같으면 기존 카탈로그 판정을 신뢰하고, 다르면 실행 계정 기준으로 다시 판정하거나 검증 불가로 표기한다. 어느 경우에도 조용히 강등되지 않고 원한 티어·실행 계정·실제 제공 모델을 지목하는 영수증을 남긴다.
- **Mandatory requirements**: 평면 배타성 선언(REQ-001), orca 티어 어휘 무증식(REQ-002), 실행 계정 확인과 해석 규칙(REQ-003), 자격 등급 동일성 검증과 불일치 시 재판정(REQ-004), 정합 영수증 방출(REQ-005), 불일치 시 fail-loud(REQ-006), 부작용 이전 배치(REQ-007), handoff 경계 배치(REQ-008), 검증 불가 항목의 명시적 표기(REQ-009).
- **Explicit non-goals**: 정합 게이트의 구현(별도 SPEC), `--execution-owner orca` handoff 스텁을 실제 Run으로 대체하는 일(별도 SPEC), 여러 계정 사이의 자동 라우팅, 티어 강등 시 대안 provider 폴백, 실행 원장 통합, J1 재설계(PR #154 결과 불변), omp `modelRoles` 어휘 변경, orca CLI 표면 변경.
- **Completion evidence**: 4개 SPEC 문서가 `auto spec validate`를 통과하고, 착수 시점의 Completion Debt 3건이 해소되며(research.md F8의 실측과 결정 표 — 현재 `none remaining`, 후속 이관 0건), 세 평면 경계에 대한 리뷰 합의가 기록된다. 구현 증거는 요구하지 않는다.

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
WHEN the policy plane prepares a workload for execution, THEN THE SYSTEM SHALL determine the provider account that will actually run it from the process plane rather than assuming the account the local CLI is logged in as, resolving it as the process plane's active account for that provider, or — when no active account is selected — the single registered account when exactly one exists, and otherwise marking the account as indeterminate.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 파이프라인·워크플로가 provider 프로파일을 확정하는 시점.
- Observability: 정합 영수증에 실행 계정 식별자가 기록되고, 그 값이 로컬 CLI 로그인 계정과 다를 수 있음을 S3의 분기 계정 픽스처로 확인한다. 활성 계정이 없고 등록 계정이 0개이거나 2개 이상이면 영수증이 실행 계정을 특정하지 않고 REQ-009의 unverified 경로로 분기함을 S9로 확인한다.
- 해소된 설계 결정: 조회 소스는 `orca account list --json`이다. 실행 계정은 `activeAccountId`, 비교 대상인 프로브 계정은 `systemDefault`(codex) 또는 provider CLI 신원이며, 한 번의 호출로 두 값을 모두 얻는다(research.md F1, F8). 활성 계정이 없을 때 추측하지 않는 이유는 잘못 특정한 계정이 조용한 강등을 만들기 때문이다.

### REQ-004 — 티어는 실행 계정과 같은 자격으로 얻은 카탈로그로만 검증된다
THE SYSTEM SHALL treat a probed catalog as evidence for a requested tier only when the probing account and the execution account identified in REQ-003 carry the same entitlement grade, SHALL re-probe under the execution account when the grades differ, and SHALL fall back to REQ-009 when neither is possible.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 요청 티어를 provider 모델로 해석할 때.
- Observability: 영수증의 카탈로그 출처가 프로브 계정과 그 자격 등급을 함께 담고, 그 등급이 실행 계정의 등급과 같음을 S3로 확인한다. 등급이 다른데 재프로브 없이 통과한 판정은 위반이다. 카탈로그가 자격에 종속된다는 전제 자체는 S10으로 고정한다 — 자격과 무관하게 카탈로그를 캐시하거나 재사용하는 구현은 등급 비교를 무의미하게 만든다.
- 해소된 설계 결정 1 — 기본 경로는 자격 비교다. 조직만 다르고 자격 등급이 같은 두 Codex 계정은 판정 필드(슬러그 집합, `supported_reasoning_levels`)가 완전히 동일했다(research.md F9). 반면 인증 자체를 제거하면 슬러그 집합이 달라진다. 따라서 판정을 무효화하는 것은 신원 차이가 아니라 자격 차이이며, Codex의 자격 등급은 `auth.json` id_token의 `https://api.openai.com/auth.chatgpt_plan_type` 클레임에서 **네트워크 호출 없이** 읽힌다. 등급이 같으면 카탈로그를 다시 받지 않는다.
- 해소된 설계 결정 2 — 이 비교는 자격에서 모델을 유추하지 않는다. 두 등급이 같은지만 본다. 그래서 하드코딩 매핑 표가 필요 없고, Claude `subscriptionType` 게이트를 기각한 근거(플랜 -> 모델 매핑은 추정이다)와 충돌하지 않는다.
- 해소된 설계 결정 3 — provider별 깊이는 그대로다. Codex는 자격 비교 후 필요 시 카탈로그 재프로브까지 가능하다. Claude는 계정별 카탈로그 프로브가 없어 `claude auth status --json`의 `orgId`를 프로세스 평면의 `organizationUuid`와 대조하는 신원 검증까지만 하고, 모델 가용성은 REQ-009의 unverified로 남긴다.

### REQ-005 — 정합 결과를 영수증으로 방출한다
THE SYSTEM SHALL emit an integrity receipt carrying the requested tier, the resolved provider model, the execution account identifier, the catalog source with the entitlement grade it was probed under, the resolution reason, and the verification status, so a later reader can reconstruct why a workload ran at the tier it ran at.
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
- Observability: handoff 결과에 정합 영수증 참조와 verification status·reason이 모두 포함됨을 S7로 확인한다. `unverified` 판정은 handoff를 막지 않는다 — 검증 수단이 없는 provider가 존재하는 것은 REQ-009가 인정한 정상 상태이므로, 그것을 치명적으로 다루면 게이트가 기존 경로를 근거 없이 차단한다. fail-closed는 REQ-006의 더 강한 조건(실행 계정이 요청 티어를 제공할 수 없음)에 속하지 "검증하지 못했음"에 속하지 않는다.

### REQ-009 — 검증할 수 없는 것은 검증됨으로 표기하지 않는다
IF the execution account cannot be determined, or the account that will run the workload exposes no account-scoped catalog probe, THEN THE SYSTEM SHALL mark that provider's tier as unverified in the receipt with a reason rather than reporting it as verified.
- EARS type: Unwanted
- Priority: Must
- Trigger/Condition: 계정을 특정할 수 없거나 그 계정의 카탈로그를 조회할 수단이 없을 때.
- Observability: 해당 provider의 영수증 항목이 `unverified` 상태와 비어 있지 않은 사유를 갖고, `verified`로 표기되지 않음을 S8로 확인한다. 실행 계정을 특정할 수 없는 경우는 S9로 확인한다.
- 이 요구가 흡수하는 세 인스턴스: (1) Claude 모델 가용성 — 계정별 카탈로그 프로브가 없다, (2) 원격 orca 환경 — `orca account list`가 `--environment`를 구조적으로 거부하므로 원격 호스트의 계정을 조회할 수 없다, (3) 활성 계정 부재 + 등록 계정이 1개가 아닌 경우(REQ-003). 셋 다 별도 요구를 만들지 않고 같은 fail-loud 규칙으로 다룬다.

## Acceptance Criteria

- [ ] 세 평면의 소유 표면이 배타적으로 열거되고 중복 어휘가 없다
- [ ] 프로세스 평면 경계를 넘는 값에 티어 어휘가 없다
- [ ] 정합 점검이 로컬 로그인 계정이 아니라 실행 계정을 사용한다
- [ ] 정합 영수증이 6개 필드를 모두 담는다
- [ ] 티어 불일치가 조용히 강등되지 않는다
- [ ] 점검 실패가 리소스를 남기지 않는다
- [ ] handoff 결과가 정합 영수증을 참조한다
- [ ] 검증할 수 없는 항목이 `unverified`로 표기된다
- [ ] 실행 계정을 특정할 수 없으면 추측하지 않고 unverified로 분기한다

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1 | S1 | INV-001 |
| REQ-002 | T1 | S2 | INV-001 |
| REQ-003 | T2 | S3, S9 | INV-003 |
| REQ-004 | T2 | S3, S10 | INV-003 |
| REQ-005 | T3 | S4 | INV-004 |
| REQ-006 | T3 | S5 | INV-004 |
| REQ-007 | T4 | S6 | INV-005 |
| REQ-008 | T4 | S7 | INV-002, INV-005 |
| REQ-009 | T3 | S8, S9 | INV-004 |

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
| REQ-003 | S3, S9 | pending |
| REQ-004 | S3, S10 | pending |
| REQ-005 | S4 | pending |
| REQ-006 | S5 | pending |
| REQ-007 | S6 | pending |
| REQ-008 | S7 | pending |
| REQ-009 | S8, S9 | pending |
