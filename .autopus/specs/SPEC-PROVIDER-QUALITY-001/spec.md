# SPEC-PROVIDER-QUALITY-001: Provider별 Quality Mode

---
id: SPEC-PROVIDER-QUALITY-001
title: Provider별 Quality Mode
version: 0.1.0
status: completed
priority: HIGH
---

## Purpose

Autopus-ADK가 Claude와 Codex에 서로 다른 persisted quality mode를 지정하면서도 기존 전역 `quality.default`와 per-run `--quality` 계약을 완전히 보존하도록 한다.

## Background

현재 `QualityConf.Default`는 Claude workflow와 Codex root·agents·orchestra가 공유하는 단일 quality selector다. Codex와 Claude의 비용·capability 요구가 달라도 한 provider만 Ultra로 올리거나 내릴 수 없고, `auto quality <preset> --apply`는 구성된 모든 플랫폼을 갱신한다.

신규 `quality.providers` map은 canonical key `claude`, `codex`만 저장한다. CLI 입력 `claude-code`는 `claude`로 정규화한다. 기존 `quality.default`는 provider override가 없는 모든 구성의 authoritative fallback으로 남는다.

## Outcome Boundary

- Outcome Lock: 사용자는 Claude와 Codex quality를 독립적으로 저장·적용하고 각 runtime consumer에서 동일한 effective mode를 관측한다.
- Mandatory requirements: central resolution, global override precedence, provider CLI, scoped apply, Codex/Claude consumer wiring, raw YAML/atomic safety, docs/templates parity, legacy compatibility.
- Explicit non-goals: provider별 신규 per-run runtime flag, Ultra/Balanced model·effort mapping 변경, Gemini/OpenCode provider override, 메타 workspace generated surface 수정.
- Completion evidence: 값 기반 precedence·CLI·apply·consumer·byte-preservation oracle, focused/race tests, generator 결정성, strict SPEC와 Guardian closure.

## Requirements

### Ubiquitous

REQ-SCHEMA-001: THE SYSTEM SHALL `QualityConf`에 canonical `claude|codex` key와 configured preset value만 허용하는 `quality.providers` map을 제공한다.

REQ-RESOLVE-001: THE SYSTEM SHALL `QualityConf.EffectiveMode(provider)`와 `ForProvider(provider)`에서 persisted provider override, persisted default, `balanced` safety fallback 순으로 mode를 중앙 해석한다.

REQ-COMPAT-001: THE SYSTEM SHALL `quality.providers`가 없는 기존 config에서 모든 provider의 effective mode를 기존 `quality.default`와 동일하게 유지한다.

REQ-MAPPING-001: THE SYSTEM SHALL 기존 Ultra/Balanced Claude model·effort 및 Codex Sol/Terra/Luna·effort mapping을 변경하지 않는다.

REQ-DOC-001: THE SYSTEM SHALL source 문서와 generated templates에 provider map, CLI, precedence, scoped/global apply 차이를 동일하게 설명한다.

### Event-Driven

REQ-OVERRIDE-001: WHEN 명시적 per-run global `--quality`가 전달되면 THE SYSTEM SHALL persisted provider override보다 먼저 그 값을 모든 provider의 in-memory effective mode로 사용하고 persisted YAML은 변경하지 않는다.

REQ-CLI-001: WHEN `auto quality provider <claude|claude-code|codex> <preset|inherit>`를 실행하면 THE SYSTEM SHALL provider alias를 canonical key로 정규화해 preset을 저장하거나 `inherit`에서 해당 override를 삭제한다.

REQ-APPLY-001: WHEN provider command에 `--apply`가 있으면 THE SYSTEM SHALL 해당 provider의 설치 플랫폼만 갱신하고, global quality command의 `--apply`는 기존처럼 구성된 전체 플랫폼을 갱신한다.

REQ-CODEX-001: WHEN Codex root, managed agents 또는 orchestra profile을 계산하면 THE SYSTEM SHALL Codex effective mode를 사용한다.

REQ-CLAUDE-001: WHEN Claude route-team workflow dispatcher가 quality binding을 계산하면 THE SYSTEM SHALL Claude effective mode를 사용한다.

REQ-WRITE-001: WHEN provider override를 저장하거나 삭제하면 THE SYSTEM SHALL raw YAML comment, env placeholder, 알 수 없는 future field, 기존 file mode를 보존하고 동일 디렉터리 atomic rename으로 교체한다.

### Unwanted

REQ-VALIDATE-001: IF provider key 또는 preset이 허용되지 않으면 THE SYSTEM SHALL config/CLI를 fail-closed하고 YAML과 platform surface를 변경하지 않는다.

REQ-ATOMIC-001: IF temp write, sync 또는 rename이 실패하면 THE SYSTEM SHALL 원본 `autopus.yaml` bytes를 보존하고 apply를 실행하지 않는다.

## Generated Files Detail

| Surface | Responsibility |
|---------|----------------|
| `pkg/config` | schema, validation, effective resolution, provider-specific profiles |
| `internal/cli/quality*`, runtime overrides | CLI persistence, precedence, scoped/global apply |
| `internal/cli/workflow*`, Codex adapters | Claude/Codex effective-mode consumers |
| `content/skills`, README, CHANGELOG | source operator guidance |
| `templates/**` | generated platform parity |
| focused `*_test.go` files | precedence, byte, apply, profile and workflow oracles |

## Related SPECs

None. One Primary SPEC closes the provider-quality Outcome Lock.

## Acceptance Criteria

`acceptance.md`의 S1-S14가 authoritative acceptance ID 집합이다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-SCHEMA-001 | T1 | S2, S3 | INV-001 |
| REQ-RESOLVE-001 | T1 | S1, S2 | INV-001 |
| REQ-COMPAT-001 | T1, T9 | S1, S14 | INV-002 |
| REQ-OVERRIDE-001 | T2 | S4 | INV-003 |
| REQ-CLI-001 | T3 | S3, S5, S6 | INV-004 |
| REQ-APPLY-001 | T4 | S7, S8 | INV-005 |
| REQ-CODEX-001 | T5 | S9 | INV-006 |
| REQ-CLAUDE-001 | T6 | S10 | INV-007 |
| REQ-MAPPING-001 | T5, T6, T9 | S9, S10, S14 | INV-006, INV-007 |
| REQ-WRITE-001 | T3, T7 | S5, S6, S11 | INV-004, INV-008 |
| REQ-VALIDATE-001 | T1, T3 | S3 | INV-001, INV-004 |
| REQ-ATOMIC-001 | T7 | S12 | INV-008 |
| REQ-DOC-001 | T8 | S13 | INV-009 |

## Out of Scope

- `--claude-quality`, `--codex-quality` 같은 신규 provider별 runtime flag
- Gemini 또는 OpenCode를 `quality.providers` canonical key로 추가
- Ultra/Balanced preset의 기존 model, role, effort 값 변경
- provider-specific preset 정의나 새로운 quality tier 생성
- 메타 workspace `autopus-co`의 generated `.codex`, `.claude`, `.agents`, `.opencode` 수정
- platform update 실패 시 이미 성공한 다른 플랫폼 surface의 transaction rollback
