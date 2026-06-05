package spec

import (
	"fmt"
	"os"
	"path/filepath"
)

// Scaffold는 SPEC 디렉터리와 기본 파일들을 생성한다.
func Scaffold(baseDir, id, title string) error {
	specID := fmt.Sprintf("SPEC-%s", id)
	specDir := filepath.Join(baseDir, ".autopus", "specs", specID)

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return fmt.Errorf("SPEC 디렉터리 생성 실패: %w", err)
	}

	files := map[string]string{
		"spec.md":       generateSpecMd(specID, title),
		"plan.md":       generatePlanMd(specID, title),
		"acceptance.md": generateAcceptanceMd(specID, title),
		"research.md":   generateResearchMd(specID, title),
	}

	for name, content := range files {
		path := filepath.Join(specDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("%s 파일 생성 실패: %w", name, err)
		}
	}

	return nil
}

// Load는 SPEC 디렉터리에서 SpecDocument를 로드한다.
func Load(specDir string) (*SpecDocument, error) {
	if _, err := os.Stat(specDir); err != nil {
		return nil, fmt.Errorf("SPEC 디렉터리 접근 실패: %w", err)
	}

	specFile := filepath.Join(specDir, "spec.md")
	content, err := os.ReadFile(specFile)
	if err != nil {
		return nil, fmt.Errorf("spec.md 읽기 실패: %w", err)
	}

	doc, err := parseSpecMd(string(content))
	if err != nil {
		return nil, err
	}

	criteria, err := loadAcceptanceCriteria(specDir)
	if err != nil {
		return nil, err
	}
	doc.AcceptanceCriteria = criteria
	return doc, nil
}

// parseSpecMd는 spec.md 내용을 SpecDocument로 파싱한다.
func parseSpecMd(content string) (*SpecDocument, error) {
	doc := ParseSpecMetadata(content)

	if doc.ID == "" {
		return nil, fmt.Errorf("spec.md에서 SPEC ID를 찾을 수 없습니다")
	}

	// 요구사항 파싱
	reqs, _ := ParseEARS(content)
	doc.Requirements = reqs
	doc.RawContent = content

	return &doc, nil
}

func loadAcceptanceCriteria(specDir string) ([]Criterion, error) {
	path := filepath.Join(specDir, "acceptance.md")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("acceptance.md 읽기 실패: %w", err)
	}

	criteria, _ := ParseGherkin(string(content))
	return criteria, nil
}

// generateSpecMd는 구조화된 섹션을 포함한 spec.md 내용을 생성한다.
func generateSpecMd(specID, title string) string {
	return fmt.Sprintf(`# %s: %s

---
id: %s
title: %s
version: 0.1.0
status: draft
priority: MEDIUM
---

## Purpose

이 기능의 목적과 해결하려는 문제를 설명합니다.

## Background

현재 상태와 변경이 필요한 배경을 설명합니다.

## Outcome Boundary

- Outcome Lock: [사용자가 기대한 최종 동작]
- Mandatory requirements: [반드시 충족할 요구]
- Explicit non-goals: [이번 SPEC에서 하지 않을 일]
- Completion evidence: [sync 완료 판정에 사용할 증거]

## Requirements

### Ubiquitous
시스템은 SHALL [동작]을 제공합니다.

### Event-Driven
WHEN [트리거] THEN 시스템은 [동작]합니다.

### Unwanted
IF [비정상 상태] THEN 시스템은 [대응]합니다.

## Acceptance Criteria

- [ ] 인수 기준 1
- [ ] 인수 기준 2

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|

## Out of Scope

이 SPEC의 범위 밖 항목을 나열합니다.

## Traceability

| Requirement | Test | Status |
|-------------|------|--------|
`, specID, title, specID, title)
}

// generatePlanMd는 구조화된 섹션을 포함한 plan.md 내용을 생성한다.
func generatePlanMd(specID, title string) string {
	return fmt.Sprintf(`# %s Plan: %s

## Implementation Strategy

구현 전략을 설명합니다.

## File Impact Analysis

| 파일 | 작업 (생성/수정/삭제) | 설명 |
|------|---------------------|------|

## Architecture Considerations

레이어 규칙, 의존성 방향, 기존 패턴과의 정합성을 설명합니다.

## Visual Planning Brief

Mermaid flowchart:
  flowchart TD
    A[Current state] --> B[Planned change]
    B --> C[Outcome Lock]

## Feature Completion Scope

Primary SPEC가 Outcome Lock을 닫는 방법, 승인된 sibling 의존성, 남은 Completion Debt 여부를 설명합니다.

## Tasks

- [ ] 태스크 1
- [ ] 태스크 2

## Risks & Mitigations

| 리스크 | 영향도 | 대응 |
|--------|--------|------|

## Dependencies

외부 라이브러리, 내부 패키지 의존성을 나열합니다.

## Exit Criteria

- [ ] 모든 Requirements 구현 완료
- [ ] 테스트 통과
- [ ] 커버리지 85%%+
`, specID, title)
}

// generateAcceptanceMd는 Gherkin 형식의 구조화된 acceptance.md 내용을 생성한다.
func generateAcceptanceMd(specID, title string) string {
	return fmt.Sprintf(`# %s Acceptance: %s

## Test Scenarios

### Scenario 1: [시나리오 제목]

Given [초기 상태]
When [동작]
Then [예상 결과]
And [구체 기대 출력/상태/값]

### Scenario 2: [시나리오 제목]

Given [초기 상태]
When [동작]
Then [예상 결과]

## Edge Cases

### Edge Case 1: [에지 케이스]

Given [비정상 상태]
When [동작]
Then [에러 처리]

## Oracle Acceptance Notes

- Must 시나리오는 파일 존재, heading, exit code, non-empty output만으로 닫지 않고 concrete expected output 또는 explicit tolerance를 포함합니다.

## Definition of Done

- [ ] 모든 Scenario 통과
- [ ] Edge Case 처리 완료
- [ ] 코드 리뷰 완료
`, specID, title)
}
