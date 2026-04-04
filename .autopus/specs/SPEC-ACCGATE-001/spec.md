# SPEC-ACCGATE-001: Acceptance Gate 및 테스트 품질 강화

**Status**: implemented
**Created**: 2026-04-04
**Domain**: ACCGATE

## 목적

현재 `/auto go` 파이프라인은 SPEC의 acceptance.md를 생성은 하지만, 파싱하지 않고, 에이전트에 전달하지 않으며, 검증하지도 않는다. 이 생명주기 단절로 인해 executor는 요구사항 요약만으로 구현하게 되어 stub 수준 코드가 나오고, validator는 acceptance 기준 없이 정적 분석만 수행한다.

이 SPEC은 acceptance.md의 전체 생명주기를 복원하여 "생성 → 파싱 → 전달 → 검증" 체인을 완성한다.

## 요구사항

### P0 (Must Have)

- **REQ-001** (Gherkin Parser): WHEN acceptance.md가 Given/When/Then 형식을 포함하면 THEN 시스템은 SHALL 각 시나리오를 Criterion 구조체로 파싱한다.
- **REQ-002** (Validation Escalation): WHEN SpecDocument의 AcceptanceCriteria가 비어있으면 THEN 시스템은 SHALL validation level을 "error"로 반환한다.
- **REQ-003** (Executor Prompt Injection): WHEN Phase 2 executor를 spawn하면 THEN 시스템은 SHALL spec.md와 acceptance.md 전문을 프롬프트에 주입한다.
- **REQ-004** (Tester Prompt Injection): WHEN Phase 1.5 tester를 spawn하면 THEN 시스템은 SHALL acceptance.md의 시나리오를 기반으로 행위 테스트 생성을 지시한다.

### P1 (Should Have)

- **REQ-005** (Criterion Priority): WHEN Criterion을 파싱하면 THEN 시스템은 SHALL Must/Should/Nice 우선순위 필드를 추출한다.
- **REQ-006** (Gate 2 Acceptance Check): WHEN Gate 2 validator를 spawn하면 THEN 시스템은 SHALL acceptance 기준별 충족 여부 검증을 지시한다.
- **REQ-007** (Scenario ID): WHEN Gherkin 시나리오를 파싱하면 THEN 시스템은 SHALL 각 시나리오에 고유 ID(AC-NNN)를 부여한다.

### P2 (Nice to Have)

- **REQ-008** (Gherkin Validation): WHERE acceptance.md가 존재하지만 Given/When/Then 형식이 아닌 경우 THEN 시스템은 SHALL 형식 오류 경고를 반환한다.

## 생성 파일 상세

### Go 코드 변경 (pkg/spec/)

| 파일 | 변경 | 설명 |
|------|------|------|
| `types.go` | 수정 | Criterion에 Priority 필드 추가, GherkinStep 타입 추가 |
| `parser.go` | 수정 | ParseGherkin 함수 추가 (Given/When/Then 파싱) |
| `gherkin_parser.go` | 신규 | Gherkin 전용 파서 (파일 분리 — 파일 크기 제한 준수) |
| `validator.go` | 수정 | AcceptanceCriteria 빈 경우 WARNING → ERROR 변경 |

### 에이전트 정의 변경 (.claude/agents/autopus/)

| 파일 | 변경 | 설명 |
|------|------|------|
| `tester.md` | 수정 | Phase 1.5 입력에 acceptance.md 참조 추가 |
| `executor.md` | 수정 | 입력 형식에 Acceptance Criteria 섹션 추가 |
| `validator.md` | 수정 | Acceptance Coverage 검증 항목 추가 |

### 스킬 정의 변경 (.claude/skills/autopus/)

| 파일 | 변경 | 설명 |
|------|------|------|
| `agent-pipeline.md` | 수정 | Phase 1.5, Phase 2, Gate 2 프롬프트에 acceptance 주입 |
