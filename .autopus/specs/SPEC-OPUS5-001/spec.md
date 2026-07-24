# SPEC-OPUS5-001: Claude Opus 5 기본 경로 승격

---
id: SPEC-OPUS5-001
title: Claude Opus 5 기본 경로 승격
version: 0.1.0
status: completed
priority: HIGH
---

## Purpose

Autopus-ADK의 Claude Code 프리미엄 경로를 고정 ID `claude-opus-5`로 승격하고, 가격·effort·라우팅·workflow·문서·생성 템플릿을 하나의 결정적 계약으로 동기화한다.

## Background

2026-07-25 기준 Claude Opus 5는 Opus 4.8의 공식 drop-in 후속 모델이다. 고정 ID는 `claude-opus-5`, 가격은 input USD 5/MTok과 output USD 25/MTok, context window는 1M, max output은 128k이다. Adaptive thinking은 기본 활성화되고 effort는 `low|medium|high|xhigh|max`, 기본값은 `high`이다.

Claude Code의 `opus` alias는 v2.1.219 이상에서 Anthropic API, Claude Platform on AWS, Amazon Bedrock, Google Cloud Agent Platform 사용 시 Opus 5로 해석된다. Microsoft Foundry에서는 현재 Opus 4.6이고 v2.1.219 직전의 지원 버전 범위에서는 Opus 4.8이었다. 따라서 ADK의 deterministic config와 workflow에는 alias가 아니라 고정 ID를 사용한다.

## Outcome Boundary

- Outcome Lock: Ultra 전체, Balanced strategic, complex routing, premium tier, canonical workflow의 Claude 프리미엄 모델이 `claude-opus-5`로 일치한다.
- Mandatory requirements: safe whitelist, deterministic pricing, Ultra max effort, 고정 ID 기본 매핑, Claude Code v2.1.219 route-team gate와 route_a v2.1.154 호환성, 버전·provider alias 문서, Opus 4.8/Fable 5/Sonnet 5 호환성, source/generated parity, 회귀 테스트.
- Explicit non-goals: 글로벌 Claude Code 자동 업그레이드, live paid API call, Microsoft Foundry의 Opus 5 강제 지정, 역사적 Opus 4.8 telemetry·pinned argv fixture·과거 SPEC 재작성.
- Completion evidence: 값 기반 unit/oracle tests, focused packages, template 결정성, strict SPEC validation, review finding closure.

## Requirements

### Ubiquitous

REQ-CATALOG-001: THE SYSTEM SHALL generated workflow의 closed model whitelist에서 `claude-opus-5`와 기존 `claude-opus-4-8`을 모두 허용하고 임의 문자열 및 JavaScript breakout 입력을 계속 거부한다.

REQ-PRICE-001: THE SYSTEM SHALL `claude-opus-5`의 표준 단가를 input USD `5.0`/MTok, output USD `25.0`/MTok으로 제공하고 동적 `opus` alias에는 가격 key를 만들지 않는다.

REQ-EFFORT-001: THE SYSTEM SHALL Ultra quality에서 `claude-opus-5`를 `max` effort로 해석하면서 공식 effort 집합을 `low|medium|high|xhigh|max`로 유지한다.

REQ-DEFAULT-001: THE SYSTEM SHALL Ultra의 모든 Claude agent와 Balanced의 strategic agent를 `claude-opus-5`로 지정하고, Balanced execution은 `claude-sonnet-5`, Fable 5는 capability opt-in으로 유지한다.

REQ-DOC-001: THE SYSTEM SHALL source 문서에 Opus 5 고정 ID, 가격, context/output, adaptive thinking, effort 기본값, v2.1.219 alias matrix, drop-in migration, cybersecurity fallback을 기록한다.

REQ-COMPAT-001: THE SYSTEM SHALL `claude-opus-4-8`을 selectable legacy 및 Opus 5 cybersecurity fallback으로 보존하고, 역사적 Opus 4.8 fixture와 완료된 SPEC를 현재 기본값으로 일괄 치환하지 않는다.

### Event-Driven

REQ-ROUTE-001: WHEN premium, strategic, complex, Ultra 또는 canonical planning 경로가 Claude 프리미엄 모델을 선택하면 THE SYSTEM SHALL 관측 가능한 config·routing·workflow output에 `claude-opus-5`를 기록한다.

REQ-VERSION-001: WHEN `auto workflow doctor`가 Claude Code 버전을 평가하면 THE SYSTEM SHALL model-free `route_a`에는 기존 v2.1.154 최소 버전을 적용하고, Opus 5를 고정한 `route_team`에는 v2.1.218 이하를 fail-closed하며 v2.1.219 이상만 통과시킨다.

REQ-SYNC-001: WHEN source skill·workflow 계약이 갱신되면 THE SYSTEM SHALL 생성기를 통해 Codex/Gemini/Claude 관련 template surface를 동일 값으로 동기화하고 두 번째 생성에서 추가 diff를 만들지 않는다.

### Unwanted

REQ-VERIFY-001: IF 로컬 Claude Code가 v2.1.219 미만이거나 paid entitlement가 확인되지 않으면 THE SYSTEM SHALL live Opus 5 smoke를 `N/A`로 기록하고 hermetic tests와 설치 버전으로 호환성 경계를 검증한다.

REQ-MIGRATION-001: IF Opus 5에 thinking-disabled 상태로 `xhigh` 또는 `max`를 전달할 가능성이 있으면 THE SYSTEM SHALL 해당 조합을 생성하지 않거나 명시적으로 거부하며, 기존 ADK 경로에서는 thinking을 비활성화하지 않는다.

## Generated Files Detail

| Surface | Responsibility |
|---------|----------------|
| `pkg/workflow`, `pkg/cost`, `internal/cli` | whitelist, pricing, effort, canonical workflow, route-aware v2.1.154/v2.1.219 doctor gate |
| `pkg/config`, `pkg/worker/routing`, `autopus.yaml`, `configs/autopus.yaml` | premium·strategic·complex defaults |
| `content/skills`, `content/workflows` | source-of-truth guidance and schema |
| `templates/**` | generated platform parity |
| focused `*_test.go` files | value-based regression oracles |

## Related SPECs

- `SPEC-FABLE5-001`: Fable 5 opt-in과 Claude effort 경계를 제공하며 이 SPEC에서 재작성하지 않는다.
- Sibling SPEC: None.

## Acceptance Criteria

`acceptance.md`의 S1-S13이 authoritative acceptance ID 집합이다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-CATALOG-001 | T1 | S1, S11 | INV-001, INV-006 |
| REQ-PRICE-001 | T2 | S2 | INV-002 |
| REQ-EFFORT-001 | T2 | S3, S8 | INV-003, INV-006 |
| REQ-DEFAULT-001 | T3 | S4, S5 | INV-004 |
| REQ-ROUTE-001 | T4 | S6 | INV-004 |
| REQ-VERSION-001 | T4, T8 | S12 | INV-008 |
| REQ-DOC-001 | T5 | S7, S8, S9, S11 | INV-005, INV-006 |
| REQ-SYNC-001 | T5, T7 | S10, S13 | INV-007 |
| REQ-COMPAT-001 | T6 | S5, S11 | INV-004, INV-006, INV-007 |
| REQ-VERIFY-001 | T7, T8 | S12, S13 | INV-008 |
| REQ-MIGRATION-001 | T5, T6 | S8, S11 | INV-006 |

## Out of Scope

- 로컬 또는 글로벌 Claude Code를 v2.1.219 이상으로 자동 업그레이드
- 비용이 발생하는 live Opus 5 API/Claude Code 호출
- provider entitlement나 safety classifier 결과의 사전 탐지
- Microsoft Foundry alias를 Opus 5로 덮어쓰기
- Fable 5를 기본 모델로 승격하거나 Balanced execution의 Sonnet 5를 교체
- historical telemetry, pinned argv fixtures, completed SPEC의 과거 증거 재작성

## Completion

- Completed: 2026-07-25
- Acceptance: S1-S13 PASS
- Guardian: Reviewer APPROVE, Security Auditor PASS, Validator PASS
- Live smoke: N/A — 로컬 Claude Code `2.1.218`은 Opus 5 최소 버전 `2.1.219` 미만이며 유료 호출은 0회
- Route gate: `route_a` PASS at v2.1.218; `route_team` expected FAIL at v2.1.218
- Completion Debt: `none`
