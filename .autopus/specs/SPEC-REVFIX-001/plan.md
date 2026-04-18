# SPEC-REVFIX-001 구현 계획

## Phase 구성

TDD 원칙에 따라 각 태스크는 **테스트 먼저 → 구현 → 통과 확인** 순서로 진행한다.
REVCONV-001 기존 테스트(`findings_test.go`, `reviewer_test.go`, `convergence_test.go`, `coverage_merge_test.go`, `coverage_gap_test.go`)는 regression 방지 기반으로 전 구간 통과를 유지한다.

## Phase 0: Config 기반 정비

### T1: 신규 config key 정의 + 기본값 설정
- **파일**: `pkg/config/spec.go` (또는 review_gate가 정의된 파일), 대응 test
- **내용**:
  - `VerdictThreshold float64` 필드 추가 (기본 0.67)
  - `PassCriteria string` 필드 추가 (빈 문자열이면 code-level default 사용)
  - `DocContextMaxLines int` 필드 추가 (기본 200)
  - YAML 태그, `omitempty` 적용
- **테스트**: `config_test.go`에 기본값 검증 case 추가, 기존 autopus.yaml load가 regression 없이 통과
- **Agent**: executor-1 (단독)
- **File Ownership**: `pkg/config/*`

## Phase 1: 코어 라이브러리 개편 (병렬 가능)

### T2: `MergeVerdicts` supermajority 로직 구현
- **파일**: `pkg/spec/reviewer.go`, `pkg/spec/reviewer_test.go`
- **TDD 순서**:
  1. failing test 작성: 입력 [PASS, PASS, REVISE] + threshold=0.67 → PASS 기대
  2. failing test 작성: 입력 [PASS, REVISE, REVISE] + threshold=0.67 → REVISE 기대
  3. failing test 작성: 입력 [PASS, PASS, REJECT] → REJECT (security gate 유지)
  4. failing test 작성: threshold=1.0 일 때 unanimous 강제 (후방 호환)
  5. 시그니처 변경: `MergeVerdicts(results []ReviewResult, threshold float64, totalProviders int) ReviewVerdict`
  6. 호출자(`spec_review.go`) 업데이트
- **Agent**: executor-2
- **File Ownership**: `pkg/spec/reviewer.go` (MergeVerdicts 섹션만), 해당 test
- **선행조건**: T1 완료

### T3: `parseDiscoverFindings` ID 발급 제거 (REQ-07)
- **파일**: `pkg/spec/reviewer.go`, `pkg/spec/reviewer_test.go`
- **TDD 순서**:
  1. failing test 작성: parseDiscoverFindings 결과의 모든 finding.ID가 `""` 임을 검증
  2. failing test 작성: `DeduplicateFindings`가 빈 ID를 받아 `F-001, F-002, ...` 전역 sequential ID를 재발급함을 검증
  3. `seq := 1` 블록 제거, ID 필드는 할당하지 않음 (line 54, 59, 69 수정)
  4. `parseVerifyFindings`는 prior findings의 기존 ID를 유지하므로 변경 없음(line 130의 `seq := len(priorFindings) + 1`은 escape hatch 신규 finding 용, 현행 유지 검토)
- **Agent**: executor-3 (T2와 병렬 가능 — 파일 내 다른 함수)
- **File Ownership**: `pkg/spec/reviewer.go` (parseDiscoverFindings 섹션)
- **주의**: T2와 동일 파일 수정이므로 merge 충돌 주의. 가능하면 T2 완료 후 순차 실행 권장.

### T4: `BuildReviewPrompt` 다중 문서 주입 + few-shot + 판정 기준 (REQ-03/04/05a)
- **파일**: `pkg/spec/prompt.go`, `pkg/spec/prompt_test.go`
- **TDD 순서**:
  1. failing test: 가짜 specDir에 4개 파일을 놓고 `BuildReviewPrompt(doc, ctx, opts, specDir)` 호출 → 프롬프트 문자열에 "## plan.md", "## research.md", "## acceptance.md" 섹션 포함 확인
  2. failing test: 300줄짜리 plan.md → trim 표기 "... (trimmed 100 more lines)" 존재 확인
  3. failing test: 프롬프트 내 "### Verdict Decision Rules" 섹션 존재 + critical=0, major<=2 규칙 포함
  4. failing test: "### Finding Format Examples" 섹션에 positive 2개 + negative 1개 예시 포함
  5. 시그니처 변경: `BuildReviewPrompt(doc *SpecDocument, codeContext string, opts ReviewPromptOptions, specDir string) string`
     - 대안: opts에 `SpecDir string`, `PassCriteria string`, `DocContextMaxLines int` 필드 추가 (시그니처 안정성 우선)
  6. 구현: plan.md/research.md/acceptance.md를 `os.ReadFile`로 읽어 trim 후 섹션 추가
- **Agent**: executor-4 (T2/T3과 완전 병렬 가능)
- **File Ownership**: `pkg/spec/prompt.go`, 해당 test
- **선행조건**: T1 완료

## Phase 2: 루프 재설계 + 통합 (순차)

### T5: `runSpecReview` 루프 재설계 (REQ-02/05b/06)
- **파일**: `internal/cli/spec_review.go`, 필요 시 `internal/cli/spec_review_loop.go` 신규 생성
- **선행조건**: T2, T3, T4 완료
- **TDD 순서**:
  1. failing test(integration): `runSpecReview` 호출 시 `doc.RawContent == ""`이면 즉시 에러 반환 (REQ-05b)
  2. failing test(integration): 3-provider mock이 동일 finding (ScopeRef=X, Category=correctness, Description=Y)을 발급 → merged.Findings 길이 1 (REQ-06)
  3. failing test(integration): revision 0 완료 후 spec.md 파일 내용을 외부 수정 → revision 1에서 `spec.Load` 재호출 확인 (REQ-02)
  4. failing test: 최종 review-findings.json의 모든 finding.ID가 전역 유니크 (REQ-07 end-to-end)
  5. 구현 (spec_review.go):
     - 루프 최상단에 `doc.RawContent == ""` guard
     - 루프 iteration 시작 시 `doc, err = spec.Load(specDir)` 재호출 (revision > 0)
     - findings 병합 블록(line 130-138)을 `MergeSupermajority` → `DeduplicateFindings` 호출로 교체
     - `MergeVerdicts(reviews, gate.VerdictThreshold, len(providers))` 호출로 업데이트
     - `BuildReviewPrompt` 호출 시 `specDir` 또는 확장된 `opts` 전달
  6. 파일 크기 체크: 300줄 초과 우려 시 루프 본체를 `runSpecReviewLoop` 함수로 분리하여 `spec_review_loop.go`로 이동
- **Agent**: executor-5 (단독, 다른 executor 완료 후)
- **File Ownership**: `internal/cli/spec_review.go`, `internal/cli/spec_review_loop.go` (신규)

### T6: autopus.yaml 샘플 주석 업데이트
- **파일**: `autopus.yaml`
- **내용**: 신규 config key 3개의 주석 예시 추가. 기본값으로 동작하도록 주석 처리.
- **Agent**: executor-5 (T5와 동일 PR에 포함)
- **File Ownership**: `autopus.yaml`

## Phase 3: 검증 (tester 단독)

### T7: 회귀 검증 + 성능 스모크
- **파일**: 없음 (실행 only)
- **내용**:
  - `go test ./...` 전체 통과 확인
  - `go vet ./...` 무경고
  - 기존 SPEC-REVCONV-001 테스트 regression 없음
  - `auto spec review --multi`로 실 SPEC 1개(예: 현재 REVISE로 종료된 SPEC-HARN-PIPE-001)를 대상으로 end-to-end 실행 → PASS 또는 의미 있는 REVISE (동일 finding 반복 없이 진행)
- **Agent**: tester-1
- **선행조건**: T5, T6 완료

## Agent Assignment 요약

| Task | Agent | 병렬/순차 | 선행 |
|------|-------|----------|------|
| T1 | executor-1 | — | — |
| T2 | executor-2 | T3과 순차 권장 (동일 파일) | T1 |
| T3 | executor-3 | T2 이후 | T1, T2 |
| T4 | executor-4 | T2/T3과 병렬 | T1 |
| T5 | executor-5 | 순차 | T2, T3, T4 |
| T6 | executor-5 | T5와 동일 단계 | T5 |
| T7 | tester-1 | 최종 | T5, T6 |

최대 동시 worktree: 3 (T2-T4 병렬 단계) — worktree-safety.md 제한(5) 이하.

## 구현 전략

- **최소 침습**: REVCONV-001의 공개 API는 손대지 않는다. 오로지 호출 경로와 프롬프트 품질, config 확장으로 해결.
- **TDD**: 모든 변경은 failing test 먼저. 특히 T5 integration test는 mock orchestra result 주입 유틸이 필요할 수 있음(기존 test 참고).
- **관찰 가능성**: 루프 내 재로드 실패, empty RawContent, supermajority 미성립 등은 stderr에 명확한 메시지 출력. 기존 "경고: 서킷 브레이커 작동" 패턴 따름.
- **후방 호환**: `verdict_threshold=1.0` 설정 시 현재 unanimous 동작과 동일해야 함 (escape hatch 유지).

## 위험 요소 (research.md와 cross-ref)

- R1: `MergeSupermajority` 순서 뒤집기로 인한 DeduplicateFindings 내 ID 재할당 결과 불안정 → research.md D2에서 완화책
- R2: plan/research/acceptance.md 파일이 생성되지 않은 신규 SPEC에서는 해당 섹션 생략. 단, 이 때 provider가 "plan.md 없음"을 critical로 잡을 위험 → 판정 기준(REQ-04)에 "문서 부재는 minor"로 명시
- R3: spec_review.go 300줄 경계 초과 시 split 필수. T5에서 사전 확인.

