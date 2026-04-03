# SPEC-COMPRESS-001 리서치

## 기존 코드 분석

### 페이즈 간 전달 경로
- **`pkg/worker/pipeline.go:54-95`** — `Execute()` 메서드: 4개 페이즈를 순차 실행
  - `:70` — `prevOutput := prompt` (초기값은 원본 프롬프트)
  - `:79` — `phasePrompt := p.promptFunc(prevOutput)` (이전 출력을 다음 프롬프트로)
  - `:89` — `prevOutput = pr.Output` (전문 교체, 압축 없음)
  - 이 지점(89번째 줄)이 compressor 삽입 지점

### 프롬프트 조립
- **`pkg/worker/context.go:23-54`** — `ContextBuilder.Build()`: Layer 4 프롬프트 조립
  - `strings.Builder` 기반, 섹션별 조건부 출력
  - 토큰 크기 인식 없음, 입력 크기 제한 없음

### Overflow 감지 (현재 사후 대응)
- **`pkg/worker/pipeline.go:211-217`** — `IsContextOverflow()`: error 이벤트에서 "context window"/"token limit" 문자열 매칭
  - 사후 감지만 수행, 사전 방지 로직 없음

### 프롬프트 래퍼
- **`pipeline.go:193-207`** — `plannerPrompt()`, `executorPrompt()`, `testerPrompt()`, `reviewerPrompt()`
  - 단순 `fmt.Sprintf`로 role prefix + 이전 출력 연결
  - 출력 크기에 대한 고려 없음

### Provider 어댑터
- **`pkg/worker/adapter/interface.go:10-19`** — `ProviderAdapter` 인터페이스
  - `Name() string` — provider 이름 반환 (모델 window 크기 매핑에 활용)
  - 현재 구현체: `claude.go`, `codex.go`, `gemini.go`

### 결과 집계
- **`pipeline.go:179-189`** — `aggregateResults()`: 모든 페이즈 출력을 `## Phase: {name}` 마크다운으로 연결
  - 압축된 출력도 동일하게 집계 가능 (변경 불필요)

### 관련 패키지
- **`pkg/worker/parallel/`** — `worktree.go`, `semaphore.go`: 병렬 실행 인프라
  - 파이프라인 내 병렬 실행 시에도 compress 패키지 재사용 가능

## 설계 결정

### 1. 외부 토크나이저 미사용
**결정**: tiktoken-go 등 외부 의존 없이 문자/단어 기반 추정 사용
- **근거**: `go.mod`에 현재 tiktoken 의존 없음. 추정 정확도 80-90%로 충분 (임계값 기반이므로 정확한 값 불필요)
- **대안**: tiktoken-go 사용 → 정확하지만 새 의존성 추가, 빌드 복잡도 증가. `TokenEstimator` 인터페이스로 추후 교체 가능하도록 설계.

### 2. 규칙 기반 요약 (1차)
**결정**: LLM 호출 없이 텍스트 패턴 매칭으로 구조화 요약 생성
- **근거**: LLM 요약은 추가 비용/지연 발생. 파이프라인 페이즈 출력은 구조화되어 있어 패턴 매칭으로 핵심 추출 가능.
- **대안**: LLM 기반 요약 → 더 정확하지만 비용/지연 증가. P3에서 `ContextCompressor` 인터페이스의 LLM 구현체로 추가.

### 3. 인터페이스 기반 설계
**결정**: `ContextCompressor` 인터페이스로 추상화, 기본 구현체(RuleBasedCompressor) 제공
- **근거**: 향후 LLM 기반 압축, 외부 토크나이저, 다른 압축 전략 교체 용이
- **패턴**: Go의 관용적 인터페이스 패턴, `adapter.ProviderAdapter`와 동일 접근

### 4. Compressor 삽입 지점
**결정**: `pipeline.go:89` (`prevOutput = pr.Output`) 직전에 compressor 호출
- **근거**: 단일 삽입 지점으로 모든 페이즈 전환에 적용. 기존 코드 변경 최소화.
- **대안**: 각 `*Prompt()` 함수 내부에서 압축 → 4곳 수정 필요, 관심사 혼재

### 5. Nil-safe 하위 호환
**결정**: compressor가 nil이면 기존 동작(전문 전달) 유지
- **근거**: 기존 `NewPipelineExecutor()` 호출자 변경 불필요. 점진적 도입 가능.

## Hermes 패턴 참고

Brainstorm BS-021에서 참조한 Hermes의 `agent/context_compressor.py` 패턴:
- 토큰 임계값: context window의 50% 도달 시 사전적 압축
- 단계: 도구 결과 가지치기 → Head/Tail 보호 → 중간 턴 LLM 요약
- 요약 크기: context window의 5% (최대 12K 토큰)

본 SPEC은 이 패턴을 Go 파이프라인 구조에 맞게 적용하되,
1차 구현에서는 LLM 호출을 제외하고 규칙 기반으로 시작한다.
