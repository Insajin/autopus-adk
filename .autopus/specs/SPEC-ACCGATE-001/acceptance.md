# SPEC-ACCGATE-001 수락 기준

## 시나리오

### S1: Gherkin 시나리오 파싱 (REQ-001)
- Given: acceptance.md에 Given/When/Then 형식의 시나리오가 3개 존재
- When: ParseGherkin 함수를 호출
- Then: 3개의 Criterion 구조체가 반환되고, 각각 ID, Description, 그리고 GherkinStep(Given/When/Then)이 올바르게 채워져 있다

### S2: 빈 AcceptanceCriteria 검증 에러 (REQ-002)
- Given: SpecDocument에 Requirements는 있으나 AcceptanceCriteria가 비어있음
- When: ValidateSpec을 호출
- Then: Field="acceptance_criteria", Level="error"인 ValidationError가 반환된다 (기존 "warning"이 아님)

### S3: Executor 프롬프트에 acceptance 주입 (REQ-003)
- Given: SPEC-ACCGATE-001의 spec.md와 acceptance.md가 존재
- When: Phase 2에서 executor agent를 spawn
- Then: executor 프롬프트에 "## Acceptance Criteria" 섹션이 포함되어 있고, acceptance.md의 시나리오 전문이 포함된다

### S4: Tester 프롬프트에 acceptance 주입 (REQ-004)
- Given: SPEC-ACCGATE-001의 acceptance.md가 존재
- When: Phase 1.5에서 tester agent를 spawn
- Then: tester 프롬프트에 acceptance.md 시나리오가 포함되고, "각 시나리오에 대한 행위 테스트 생성" 지시가 포함된다

### S5: Criterion Priority 필드 (REQ-005)
- Given: acceptance.md의 시나리오에 "Priority: Must" 태그가 있음
- When: ParseGherkin 함수를 호출
- Then: 해당 Criterion의 Priority 필드가 "Must"로 설정된다

### S6: Gate 2 Acceptance 검증 (REQ-006)
- Given: SPEC에 acceptance 기준 5개가 정의됨
- When: Gate 2 validator를 spawn
- Then: validator 프롬프트에 "각 acceptance 기준별 충족 여부를 검증하라"는 지시와 5개 기준 목록이 포함된다

### S7: 시나리오 ID 자동 부여 (REQ-007)
- Given: acceptance.md에 ID 없는 3개 시나리오 존재
- When: ParseGherkin 함수를 호출
- Then: 각 시나리오에 AC-001, AC-002, AC-003 ID가 순서대로 부여된다

### S8: Gherkin 형식 검증 경고 (REQ-008)
- Given: acceptance.md가 존재하지만 Given/When/Then 패턴이 없는 자유 형식 텍스트
- When: ParseGherkin 함수를 호출
- Then: 파싱 결과는 빈 Criterion 배열이며, 형식 경고가 반환된다
