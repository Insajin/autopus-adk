package spec

import "fmt"

// generateResearchMd는 리서치 섹션을 포함한 research.md 내용을 생성한다.
func generateResearchMd(specID, title string) string {
	return fmt.Sprintf(`# %s Research: %s

## Codebase Analysis

대상 코드 영역의 구조, 의존성, 패턴을 분석합니다.

### Target Files

| 파일 | 역할 | 변경 필요 |
|------|------|-----------|

### Dependencies

기존 코드와의 의존 관계를 매핑합니다.

## Lore Decisions

`+"`auto lore context`"+`로 조회한 과거 의사결정 기록입니다.

## Architecture Compliance

`+"`auto arch enforce`"+`로 확인한 아키텍처 정합성 결과입니다.

## Outcome Lock

- User-visible outcome: [완료해야 하는 결과]
- Mandatory requirements: [Primary SPEC 요구사항]
- Explicit non-goals: [이번에 하지 않을 것]
- Completion evidence: [sync 완료 판정 증거]

## Visual Planning Brief

Mermaid flowchart:
  flowchart TD
    A[Current state] --> B[Planned change]
    B --> C[Outcome Lock]

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | [sanitized user request evidence] | [ordering / parser / formula / state transition] | [stdout/API field/file content] | S1 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| [slice] | [Primary SPEC or approved sibling SPEC] | covered / approved-sibling / completion-debt |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| None | Outcome Lock is limited to this SPEC | User explicitly requests it |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC closes Outcome Lock | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| [path or symbol] | existing / [NEW] planned addition | existing refs verified with rg/read; [NEW] excluded from existing-reference checks |

## Reviewer Brief

- Intended scope: [이 SPEC가 닫는 결과]
- Explicit non-goals: [리뷰어가 새 scope로 확장하지 말아야 할 항목]
- Self-verified: Traceability Matrix, Semantic Invariant Inventory, oracle acceptance, existing/[NEW] reference discipline
- Reviewer should focus on: correctness, convergence safety, regression risk

## Key Findings

리서치 과정에서 발견된 주요 사항을 정리합니다.

## Recommendations

구현 시 참고할 권고사항을 나열합니다.

## Self-Verify Summary

- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: existing/[NEW] reference discipline recorded
- Q-COMP-05 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: semantic invariants mapped to oracle acceptance
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Reviewer Brief and Traceability Matrix present
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt and Evolution Ideas separated
`, specID, title)
}
