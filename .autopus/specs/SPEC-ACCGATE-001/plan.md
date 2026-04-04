# SPEC-ACCGATE-001 구현 계획

## 태스크 목록

### Go 코드 (pkg/spec/)

- [ ] T1: `types.go` — Criterion에 Priority 필드 추가, GherkinStep 타입 정의
- [ ] T2: `gherkin_parser.go` — Gherkin 파서 신규 작성 (Given/When/Then → Criterion 변환)
- [ ] T3: `validator.go` — AcceptanceCriteria 빈 경우 WARNING → ERROR 변경
- [ ] T4: `gherkin_parser_test.go` — Gherkin 파서 단위 테스트
- [ ] T5: `validator_test.go` — 변경된 validation level 테스트 업데이트

### 에이전트/스킬 정의 (.md)

- [ ] T6: `agent-pipeline.md` — Phase 1.5 tester 프롬프트에 acceptance.md 전문 주입
- [ ] T7: `agent-pipeline.md` — Phase 2 executor 프롬프트에 acceptance.md 전문 주입
- [ ] T8: `agent-pipeline.md` — Gate 2 validator 프롬프트에 acceptance coverage 검증 추가
- [ ] T9: `tester.md` — Phase 1.5 입력 형식에 Acceptance Criteria 섹션 추가
- [ ] T10: `executor.md` — 입력 형식에 Acceptance Criteria 섹션 추가
- [ ] T11: `validator.md` — Acceptance Coverage 검증 항목 추가

## 구현 전략

### Phase 2 병렬화

| 태스크 그룹 | 모드 | 이유 |
|-------------|------|------|
| T1 | 선행 | T2, T3의 타입 의존성 |
| T2, T3 | 병렬 | 파일 소유권 독립 (gherkin_parser.go vs validator.go) |
| T4, T5 | 병렬 | 테스트 파일, T2/T3 완료 후 |
| T6-T11 | 병렬 | 모두 .md 파일, 서로 독립 |

### 기존 코드 활용

1. **Criterion 구조체 재사용**: `types.go`에 이미 정의된 Criterion 타입에 Priority 필드만 추가
2. **ParseEARS 패턴 참조**: `parser.go`의 정규식 기반 파싱 패턴을 Gherkin 파서에도 동일하게 적용
3. **ValidateSpec 확장**: 기존 validation 흐름에 자연스럽게 통합 (42행 수정)

### 변경 범위

- Go 소스 코드: 4개 파일 (신규 1, 수정 2, 테스트 2)
- 에이전트/스킬 .md: 4개 파일 수정
- 총 예상 변경: ~150줄 Go 코드, ~80줄 .md 변경
