# SPEC-SESSCONT-001 리서치

## 기존 코드 분석

### 세션 ID 생성 (변경 대상)

**`pkg/worker/pipeline.go:99`** — 핵심 변경 지점:
```go
sessionID := fmt.Sprintf("pipeline-%s-%s", taskID, phase)
```
각 페이즈마다 별도 sessionID를 생성. 이 1줄이 세션 분리의 근본 원인.

### --resume 플래그 전달 경로

1. `pipeline.go:99` → sessionID 생성
2. `pipeline.go:102` → `adapter.TaskConfig.SessionID`에 할당
3. `adapter/claude.go:25-27` → SessionID가 비어있으면 fallback 생성
4. `adapter/claude.go:34` → `--resume sessionID`로 CLI 인자 구성
5. `adapter/gemini.go:30`, `adapter/codex.go:32` — 동일 패턴

### 관련 타입 정의

- `adapter/interface.go:22-29` — `TaskConfig` 구조체 (SessionID 필드, L24)
- `adapter/interface.go:39-45` — `TaskResult` 구조체 (SessionID 필드, L42)
- `pipeline.go:25-31` — `PhaseResult` 구조체 (SessionID 필드, L30)
- `stream/events.go:22-28` — `ResultData` 구조체 (SessionID 필드, L25)

### 단일 실행 경로 (미영향)

- `loop.go:99-106` — `handleTask`에서 `adapter.TaskConfig` 직접 구성
- `loop.go:74` — `taskPayloadMessage.SessionID`를 그대로 전달
- PipelineExecutor와 독립적인 경로이므로 세션 변경에 영향 없음

### 페이즈 프롬프트 함수

- `pipeline.go:193-207` — plannerPrompt, executorPrompt, testerPrompt, reviewerPrompt
- 세션 재사용 시에도 이 프롬프트가 stdin으로 전달되어 페이즈 역할 구분 유지

## 설계 결정

### D1: sessionID에서 phase 접미사 제거 (채택)

**이유**: Claude Code의 --resume은 동일 session ID일 때 이전 대화를 이어받음.
phase 접미사를 제거하면 planner의 출력이 executor 세션에 자연스럽게 존재하여
중복 전송이 불필요해짐.

**대안 검토**:
- A) 모든 이전 페이즈 출력을 프롬프트에 포함 — 현재 방식. 토큰 낭비 심함.
- B) 파일 기반 컨텍스트 전달 — 중간 결과를 파일로 저장. 복잡성 증가.
- C) API 모드 전환 — CLI subprocess 대신 Anthropic API 직접 호출. 아키텍처 변경 과다.

### D2: TaskConfig.TaskID는 phase 접미사 유지

**이유**: TaskID는 로그 추적, 비용 집계, A2A 보고에 사용됨.
페이즈별 구분이 필요하므로 `{taskID}-{phase}` 유지.
SessionID만 통합하여 CLI 세션 재사용을 달성.

### D3: 재시도 시 sessionID 보존 (P1)

**이유**: Claude Code --resume 세션에는 실패 시점의 컨텍스트가 남아있음.
동일 세션으로 재시도하면 "이전 시도가 실패했다"는 맥락에서 재시작 가능.
새 세션 생성 시 모든 컨텍스트를 처음부터 다시 전달해야 함.

### D4: SPEC-COMPRESS-001 연계 필수

세션 재사용 시 컨텍스트가 4개 페이즈에 걸쳐 누적됨.
Context Compression 없이는 긴 태스크에서 context window overflow 위험.
P0은 독립 배포 가능하나, 모니터링 후 SPEC-COMPRESS-001 우선순위 상향 검토 필요.

## 영향 범위

| 파일 | 변경 규모 | 위험도 |
|------|----------|--------|
| `pkg/worker/pipeline.go` | L99 1줄 수정 (P0), 재시도 로직 ~30줄 추가 (P1) | 낮음 |
| `pkg/worker/session_cache.go` | 신규 ~80줄 (P2) | 낮음 |
| `pkg/worker/pipeline_test.go` | sessionID assertion 수정 | 낮음 |
| `pkg/worker/pipeline_integration_test.go` | sessionID assertion 수정 | 낮음 |
| `pkg/worker/adapter/*` | 변경 없음 | 없음 |
| `pkg/worker/loop.go` | 변경 없음 | 없음 |
