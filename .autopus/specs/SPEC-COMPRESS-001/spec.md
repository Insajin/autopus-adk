# SPEC-COMPRESS-001: Context Compression for Pipeline Phase Transitions

**Status**: completed
**Created**: 2026-04-02
**Domain**: COMPRESS

## 목적

Worker 파이프라인(`PipelineExecutor`)은 현재 페이즈 간 출력을 전문(full text)으로 전달한다
(`pipeline.go:89` — `prevOutput = pr.Output`). 후반 페이즈(tester, reviewer)에 도달할수록
누적 컨텍스트가 기하급수적으로 증가하여 context window overflow가 발생한다.
현재 `IsContextOverflow()`는 에러 발생 **후** 사후 대응만 하므로, 사전적(proactive) 압축이 필요하다.

## 요구사항

### REQ-COMP-001: 페이즈 간 구조화 요약 (P0)
WHEN a pipeline phase completes AND the cumulative context exceeds the compression threshold,
THE SYSTEM SHALL replace the previous phase output with a structured summary
containing Goal, Progress, Decisions, Files Modified, and Next Steps sections.

### REQ-COMP-002: 토큰 예산 계산 (P1)
WHEN the pipeline initializes,
THE SYSTEM SHALL estimate the token budget based on the provider's model context window size
and apply a configurable threshold ratio (default: 50% of window) to determine when compression triggers.

### REQ-COMP-003: 도구 결과 가지치기 (P2)
WHEN compressing context AND tool output results are present,
THE SYSTEM SHALL prune older tool results by replacing them with placeholder summaries,
preserving only the most recent tool outputs.

### REQ-COMP-004: 누적 압축 (P3)
WHEN multiple consecutive phases trigger compression,
THE SYSTEM SHALL apply progressive summarization that preserves key decisions and file changes
from all prior phases, preventing information loss across compression boundaries.

### REQ-COMP-005: 원문 보존 조건
WHEN the cumulative context is below the compression threshold,
THE SYSTEM SHALL pass the full phase output to the next phase without modification.

### REQ-COMP-006: 요약 크기 제한
THE SYSTEM SHALL limit each structured summary to a maximum of 5% of the model's context window
(capped at 12,288 tokens), ensuring summaries themselves do not consume excessive budget.

## 생성 파일 상세

### `pkg/worker/compress/compressor.go`
- `ContextCompressor` 인터페이스 정의
- `CompressConfig` 구조체 (임계값 비율, 최대 요약 크기 등)
- `NewDefaultCompressor()` 팩토리 함수

### `pkg/worker/compress/budget.go`
- 모델별 context window 크기 매핑 (claude: 200K, codex: 128K, gemini: 1M)
- `TokenBudget` 구조체: 현재 추정 토큰 수, 남은 예산, 임계값 계산
- 간이 토큰 추정 함수 (외부 tiktoken 의존 없이 문자/단어 기반)

### `pkg/worker/compress/summarizer.go`
- 구조화 요약 생성 로직
- 템플릿 기반 요약: Goal / Progress / Decisions / Files Modified / Next Steps
- 텍스트 파싱으로 섹션 추출 (LLM 호출 없이 규칙 기반 1차 구현)

### `pkg/worker/compress/pruner.go`
- 도구 결과 가지치기 로직
- 오래된 tool output을 `[pruned: {tool_name} — {summary}]` 형태로 교체
- Head/Tail 보호: 첫 번째와 마지막 tool output은 보존

### `pipeline.go` 수정
- `runPhase` 반환 후 `prevOutput` 할당 전에 compressor 호출 삽입
- `PipelineExecutor` 구조체에 `compressor` 필드 추가
