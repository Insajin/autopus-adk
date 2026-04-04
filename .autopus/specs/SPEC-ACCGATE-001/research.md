# SPEC-ACCGATE-001 리서치

## 기존 코드 분석

### pkg/spec/types.go (76줄)

현재 Criterion 구조체:
```go
type Criterion struct {
    ID          string // 기준 ID
    Description string // 기준 설명
    TracesTo    string // 추적 대상
}
```

- Priority 필드가 없음 → P0/P1/P2 구분 불가
- GherkinStep(Given/When/Then) 구조가 없음 → 시나리오 세부 정보 손실

### pkg/spec/parser.go (70줄)

- `ParseEARS()` 함수만 존재 — EARS 패턴(WHEN/WHERE/IF/SHALL) 정규식 기반 파싱
- Gherkin(Given/When/Then) 파싱 로직 없음
- 정규식 패턴 기반 접근이 입증됨 → 동일 패턴으로 Gherkin 파서 구현 가능
- `detectEARSType()` 함수의 우선순위 매칭 패턴을 Gherkin 파서에도 적용

### pkg/spec/validator.go (67줄)

- 42-48행: AcceptanceCriteria 빈 경우 `Level: "warning"` 반환
- 이것이 acceptance 생명주기 단절의 핵심 원인 중 하나: WARNING은 파이프라인을 멈추지 않음
- ERROR로 변경하면 Gate 2에서 acceptance 없는 SPEC이 통과 불가

### .claude/agents/autopus/tester.md (212줄)

- Phase 1.5 입력 형식에 `Requirements: [P0/P1 requirements list]`만 존재
- acceptance.md 참조 없음 → tester는 EARS 요구사항만으로 테스트 생성
- 행위(behavior) 테스트가 아닌 기능(function) 테스트만 나옴

### .claude/agents/autopus/executor.md (178줄)

- 입력 형식: `## Requirements` 섹션만 존재
- acceptance criteria 섹션 없음 → executor는 요구사항 요약만으로 구현
- 수락 기준 없이 구현하므로 "무엇이 완성인지" 기준이 모호

### .claude/agents/autopus/validator.md (124줄)

- 검증 항목: 컴파일, 테스트, 린트, 커버리지, 구조
- acceptance coverage 검증 없음 → 정적 분석만 통과하면 PASS
- Gate Verdict에 acceptance 관련 실패 원인이 없음

### .claude/skills/autopus/agent-pipeline.md (535줄)

- Phase 1.5 (Line 140-158): `SPEC: .autopus/specs/SPEC-{SPEC_ID}/spec.md`만 참조
- Phase 2 (Line 204-254): `{task_description}`에 요구사항 요약만 전달
- Gate 2 (Line 284-294): 일반적 validation만 지시, acceptance 기반 검증 없음

## 설계 결정

### Gherkin 파서를 별도 파일로 분리하는 이유

1. **파일 크기 제한**: parser.go(70줄)에 Gherkin 파서를 추가하면 ~150줄 예상 → 200줄 경고 근접
2. **관심사 분리**: EARS 파싱과 Gherkin 파싱은 독립적 도메인
3. **테스트 독립성**: gherkin_parser_test.go로 단위 테스트 분리 가능

### WARNING → ERROR 변경의 영향 분석

- 기존 테스트 `TestValidateSpec_ValidDocument`는 AcceptanceCriteria가 있으므로 영향 없음
- `TestValidateSpec_EmptyRequirements`는 AcceptanceCriteria도 비어있으므로 error 하나 추가됨
- validator_test.go에 acceptance empty = error 검증 테스트 추가 필요

### 프롬프트 주입 방식: 전문 vs 요약

- **결정: 전문 주입** (acceptance.md 전체를 프롬프트에 포함)
- **이유**: acceptance.md는 보통 50-100줄 이내로 토큰 비용 낮음. 요약하면 정보 손실 발생
- **대안 검토**: 요약 주입 → 시나리오 세부 조건(Given/Then의 구체적 값)이 누락될 위험. BS-022에서 "모든 단계가 동일한 immutable acceptance 계약 참조"를 ICE 최고점으로 평가한 것과 일치

### Priority 필드 설계

- MoSCoW 기반: Must / Should / Nice (Could → Nice로 단순화)
- acceptance.md에서 `Priority: Must` 형식의 태그로 표기
- 태그 없으면 기본값 "Must" (모든 기준은 기본적으로 필수)

## 위험 요소

1. **하위 호환성**: WARNING → ERROR 변경 시 기존에 acceptance 없는 SPEC이 validation 실패할 수 있음
   - 완화: `--auto` 플래그로 실행 시 draft SPEC은 자동 리뷰 후 진행하도록 이미 정책화됨 (feedback_auto_draft_guard)
2. **프롬프트 크기 증가**: acceptance.md 전문 주입으로 Phase 2/1.5 프롬프트 길이 증가
   - 완화: acceptance.md는 보통 50-100줄 → ~500 토큰 수준으로 Context7 토큰 예산(10K)에 비해 미미
