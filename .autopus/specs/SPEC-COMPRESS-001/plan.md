# SPEC-COMPRESS-001 구현 계획

## 태스크 목록

- [ ] T1: `pkg/worker/compress/budget.go` — 모델별 window 매핑 및 토큰 추정 함수
- [ ] T2: `pkg/worker/compress/compressor.go` — ContextCompressor 인터페이스 및 기본 구현체
- [ ] T3: `pkg/worker/compress/summarizer.go` — 규칙 기반 구조화 요약 생성기
- [ ] T4: `pkg/worker/compress/pruner.go` — 도구 결과 가지치기 로직
- [ ] T5: `pipeline.go` 수정 — compressor 통합 (prevOutput 할당 전 압축 삽입)
- [ ] T6: `pkg/worker/compress/compressor_test.go` — 단위 테스트
- [ ] T7: `pkg/worker/compress/budget_test.go` — 토큰 추정 및 임계값 테스트
- [ ] T8: `pkg/worker/compress/summarizer_test.go` — 요약 생성 테스트
- [ ] T9: `pkg/worker/compress/pruner_test.go` — 가지치기 테스트
- [ ] T10: `pipeline_test.go` 확장 — 압축 통합 테스트

## 구현 전략

### 접근 방법
1. **외부 의존 최소화**: tiktoken-go 대신 문자/단어 기반 토큰 추정 사용 (Go 표준 라이브러리만). 추후 정확도 필요 시 외부 토크나이저 교체 가능하도록 인터페이스 분리.
2. **규칙 기반 우선**: 1차 구현은 LLM 호출 없이 텍스트 파싱 + 템플릿으로 요약 생성. P3에서 LLM 기반 summarizer를 `ContextCompressor` 인터페이스의 다른 구현체로 추가.
3. **기존 코드 최소 변경**: `pipeline.go`의 변경은 compressor 필드 추가 + `Execute()` 루프 내 2줄 삽입으로 제한.

### 기존 코드 활용
- `adapter.ProviderAdapter.Name()` → 모델 이름으로 window 크기 조회
- `PipelineExecutor` 구조체 확장 (compressor 필드)
- `NewPipelineExecutor()` 시그니처에 옵셔널 compressor 파라미터 (nil이면 기본값)

### 변경 범위
- **신규 파일**: 4개 소스 + 4개 테스트 (`pkg/worker/compress/`)
- **수정 파일**: 1개 (`pipeline.go` — 약 15줄 추가)
- **수정 파일**: 1개 (`pipeline_test.go` — 압축 관련 테스트 추가)

### 의존 관계
- T1 (budget) → T2 (compressor)가 참조
- T3 (summarizer), T4 (pruner) → T2 (compressor)가 조합
- T5 (pipeline 통합) → T1~T4 완료 후
- T6~T10 (테스트) → 각 대상 태스크 완료 후
