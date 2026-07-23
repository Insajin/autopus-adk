# SPEC-FABLE5-001: Claude Fable 5 및 세션 effort 지원

---
id: SPEC-FABLE5-001
title: Claude Fable 5 및 세션 effort 지원
version: 0.1.0
status: completed
priority: HIGH
---

## Purpose

Autopus-ADK의 Claude Code 실행 경로가 Claude Fable 5와 최신 effort 계약을 정확히 구분해 지원하도록 한다. Fable 5는 접근 권한이 있는 사용자가 명시적으로 선택하는 opt-in 모델로 추가하고, 모델 effort와 Claude Code 전용 세션 모드인 `ultracode`가 서로 다른 저장·전달 경계를 갖도록 한다.

## Background

2026-07-23 기준 Anthropic 공식 계약에서 Fable 5의 full ID는 `claude-fable-5`, Claude Code alias는 `fable`, capability-aware alias는 `best`이다. Fable 5는 Claude Code 2.1.170 이상에서 선택할 수 있지만 기본 모델이 아니고, 조직 entitlement와 ZDR 정책에 따라 사용할 수 없을 수 있다.

ADK는 이미 모델/API effort 다섯 값인 `low`, `medium`, `high`, `xhigh`, `max`를 보유한다. 그러나 Fable 모델 whitelist·가격·Ultra 매핑이 없고, Claude worker adapter는 `TaskConfig.Effort`를 실제 `--effort` argv로 전달하지 않는다. 구성 기반 orchestra와 fallback orchestra 또한 명시적 global `--effort`를 Claude provider에 적용하지 않으며, route-team binding은 문서상 explicit effort 우선순위와 달리 해당 값을 소비하지 않는다.

`ultracode`는 여섯 번째 모델/API effort가 아니다. Claude Code 2.1.203 이상에서 제공되는 session-only CLI 모드이며 실제 모델 effort는 `xhigh`이다. Agent/team 전달 안정성의 공식 버전 경계는 2.1.210 이상이다.

## Outcome Boundary

- Outcome Lock: 사용자는 기존 Opus 4.8/Sonnet 5 기본값을 유지하면서 Fable 5를 명시적으로 선택할 수 있고, Claude 실행 경로에서 다섯 모델 effort와 session-only `ultracode`가 각자의 올바른 경계로 전달된다.
- Mandatory requirements: Fable safe model·가격·Ultra 매핑, worker argv 전달, configured/fallback orchestra override, route-team effort binding, enum 경계 보존, 문서·회귀 검증.
- Explicit non-goals: Fable을 기본 모델로 승격, entitlement 탐지, Fable 안전 classifier fallback receipt 확장, third-party provider env 생성, Claude Code 전역 업그레이드, broader SDK integration.
- Completion evidence: hermetic argv·whitelist·pricing·resolver·binding 테스트, focused race test, `go vet`, `go build`, strict SPEC validation, 생성 템플릿 결정성, Guardian 승인.

## Requirements

### Ubiquitous

REQ-MODEL-001: 시스템은 SHALL generated workflow의 fail-closed 모델 집합에 `claude-fable-5`, `fable`, `best`를 추가하고 그 밖의 임의 문자열과 JavaScript breakout 입력을 계속 거부한다.

REQ-PRICE-001: 시스템은 SHALL `claude-fable-5`의 결정적 표준 단가를 input 1M token당 USD 10, output 1M token당 USD 50으로 제공하고 동적으로 해석되는 alias에는 가격 key를 만들지 않는다.

REQ-QUALITY-001: 시스템은 SHALL Ultra quality에서 `claude-fable-5`, `fable`, `best`를 `max` effort로 해석하면서 기존 Opus 4.8/Sonnet 5 quality·router·workflow 기본 모델을 보존한다.

REQ-BOUNDARY-001: 시스템은 SHALL 모델/API·workflow·frontmatter effort enum을 `low|medium|high|xhigh|max`로 유지하고 `ultracode`를 persisted settings 또는 환경 변수 값으로 직렬화하지 않는다.

REQ-DOC-001: 시스템은 SHALL 사용자 문서와 생성 템플릿에 Fable opt-in 방법, 가격, entitlement/ZDR 주의, Fable 최소 버전 2.1.170, `ultracode` main-session 최소 버전 2.1.203, agent/team 안정 버전 2.1.210을 명시한다.

### Event-Driven

REQ-WORKER-001: WHEN Claude worker task의 `Effort`가 `low|medium|high|xhigh|max|ultracode` 중 하나이면 THEN 시스템은 `--effort`와 해당 값을 분리된 argv 항목으로 정확히 한 번 전달한다.

REQ-RUNTIME-001: WHEN 사용자가 global `--effort`를 명시하고 configured Claude orchestra provider가 존재하면 THEN 시스템은 메모리 안의 `Args`와 `PaneArgs`에서 기존 `--effort` 값을 교체하거나 누락된 flag를 추가하면서 model 및 다른 사용자 argv를 보존한다.

REQ-FALLBACK-001: WHEN config를 사용할 수 없는 fallback provider 구성에서 global `--effort`가 명시되면 THEN 시스템은 Claude subprocess와 pane argv 모두에 같은 값을 적용하고 기존 `opus` model 선택을 보존한다.

REQ-BINDING-001: WHEN route-team binding에 explicit `low|medium|high|xhigh|max`가 전달되면 THEN 시스템은 모든 agent-driven phase에 해당 모델 effort를 적용하고 quality model·depth·risk 정책은 보존한다.

REQ-ULTRACODE-001: WHEN route-team binding에 explicit `ultracode`가 전달되면 THEN 시스템은 workflow enum에 `ultracode`를 넣지 않고 모든 agent-driven phase의 실제 model effort를 `xhigh`로 정규화한다.

### Unwanted

REQ-WORKER-002: IF Claude worker task의 `Effort`가 비어 있거나 허용된 CLI session 값이 아니면 THEN 시스템은 `--effort` argv를 생략한다.

REQ-BINDING-002: IF route-team binding의 explicit effort가 비어 있지 않으면서 다섯 모델 effort 또는 `ultracode`가 아니면 THEN 시스템은 해당 값을 generated workflow에 넣지 않고 canonical Full Ultra binding으로 fail-closed한다.

REQ-VERIFY-001: IF live Fable entitlement 또는 Claude Code 2.1.203 이상이 로컬에 없으면 THEN 시스템은 live API 호출을 완료 조건으로 요구하지 않고 hermetic argv와 설치 버전 증거로 지원 경계를 검증한다.

## Acceptance Criteria

`acceptance.md`의 S1-S16과 Edge Case 1-3을 이 SPEC의 유일한 authoritative acceptance ID 집합으로 사용한다. Traceability Matrix는 모든 mandatory requirement를 해당 시나리오에 직접 연결한다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-MODEL-001 | T1 | S1, S2 | INV-001 |
| REQ-PRICE-001 | T2 | S3 | INV-002 |
| REQ-QUALITY-001 | T3 | S4, S5 | INV-003 |
| REQ-WORKER-001 | T4 | S6 | INV-004 |
| REQ-WORKER-002 | T4 | S7 | INV-004 |
| REQ-RUNTIME-001 | T5 | S8, S9 | INV-005 |
| REQ-FALLBACK-001 | T5 | S10 | INV-005 |
| REQ-BINDING-001 | T6 | S11 | INV-006 |
| REQ-ULTRACODE-001 | T6 | S12 | INV-006 |
| REQ-BINDING-002 | T6 | S13 | INV-006 |
| REQ-BOUNDARY-001 | T4, T5, T6 | S7, S12, S14 | INV-007 |
| REQ-DOC-001 | T7 | S15 | INV-008 |
| REQ-VERIFY-001 | T8 | S16 | INV-009 |

## Out of Scope

- Fable 5 또는 `best`를 Ultra/Balanced/router/orchestra의 새 기본값으로 지정
- 계정 entitlement, usage credit, ZDR 가능 여부의 live 탐지
- Fable safety classifier의 Opus fallback을 requested/actual receipt로 확장
- Bedrock, Google Cloud, Microsoft Foundry용 `ANTHROPIC_DEFAULT_FABLE_MODEL*` 생성
- local/global Claude Code 설치를 자동으로 2.1.218로 업그레이드
- `ultracode`를 API, workflow schema, agent frontmatter, `CLAUDE_CODE_EFFORT_LEVEL`, persisted `effortLevel` enum에 추가
- Claude Agent SDK의 `ModelInfo` 기반 동적 catalog 도입

## Clarification Ledger

| Item | Resolution | Basis |
|------|------------|-------|
| Fable을 기본값으로 올릴지 | assumed: 아니오 | 공식 문서상 Fable은 default가 아니며 entitlement/ZDR 제한이 있음 |
| `best` alias를 허용할지 | assumed: safe opt-in model literal로 허용 | 공식 Claude Code alias이며 Fable 미지원 시 최신 Opus로 해석됨 |
| `ultracode`를 effort enum에 추가할지 | resolved: 추가하지 않음 | 공식 계약상 xhigh + dynamic workflows인 session-only 모드 |
| live Fable 호출을 gate로 둘지 | deferred: 운영 entitlement 검증 | CI와 개발 환경의 계정 상태에 따라 비결정적 |

## Traceability

| Requirement | Test Surface | Status |
|-------------|--------------|--------|
| REQ-MODEL-001, REQ-PRICE-001, REQ-QUALITY-001 | `pkg/workflow`, `pkg/cost`, `internal/cli` unit tests | verified |
| REQ-WORKER-001, REQ-WORKER-002 | `pkg/worker/adapter` argv tests | verified |
| REQ-RUNTIME-001, REQ-FALLBACK-001 | `internal/cli` provider override tests | verified |
| REQ-BINDING-001, REQ-ULTRACODE-001, REQ-BINDING-002 | `internal/cli` binding receipt tests | verified |
| REQ-BOUNDARY-001, REQ-DOC-001, REQ-VERIFY-001 | static grep, template generation, verification commands | verified |

## Completion

- Completed: 2026-07-23
- Acceptance: S1-S16 및 Edge Case 1-3 PASS
- Guardian: Validator PASS, Reviewer PASS, Security Auditor PASS
- Annotation: `@AX` Phase 2.5 완료
- Live smoke: N/A — 로컬 Claude Code `2.1.198`은 `ultracode` 최소 버전 `2.1.203` 미만이며 Fable entitlement는 환경 의존
- Completion Debt: `none`
