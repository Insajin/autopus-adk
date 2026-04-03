# SPEC-BUDGET-001: Iteration Budget — Worker 도구 호출 예산 관리

**Status**: completed
**Created**: 2026-04-02
**Domain**: BUDGET

## 목적

Worker가 CLI 서브프로세스를 실행할 때, 도구 호출(tool_call/tool_use) 횟수를 제한하여 무한 루프나 과도한 리소스 소비를 방지한다. 현재 Worker는 서브프로세스의 도구 호출 이벤트를 인식하지 못하고, stdin 파이프가 프롬프트 전송 후 즉시 닫히므로 런타임 메시지 주입이 불가능하다. 이 SPEC은 Stream 프로토콜 확장, stdin 파이프 리팩토링, Budget 카운터/경고/강제 종료를 포함한다.

## 요구사항

### REQ-BUDGET-01: Stream 프로토콜 확장 — EventToolCall

WHEN the stream parser receives a JSON event with type "tool_call" or "tool_use",
THE SYSTEM SHALL parse it as an `EventToolCall` event type and propagate it through the event pipeline.

### REQ-BUDGET-02: Adapter별 tool_call 매핑

WHEN a Claude adapter receives a "tool_use" stream event,
THE SYSTEM SHALL map it to the unified `EventToolCall` type.

WHEN a Codex adapter receives a "tool_call" stream event,
THE SYSTEM SHALL map it to the unified `EventToolCall` type.

WHEN a Gemini adapter receives a "tool_call" stream event,
THE SYSTEM SHALL map it to the unified `EventToolCall` type.

### REQ-BUDGET-03: stdin 파이프 유지 (열린 상태)

WHILE a subprocess is running,
THE SYSTEM SHALL keep the stdin pipe open (not close after initial prompt write) to enable runtime message injection.

### REQ-BUDGET-04: IterationBudget 구조체

WHEN an IterationBudget is created with a limit N,
THE SYSTEM SHALL define warning thresholds at 70% and 90% of N, and a hard limit at 100%.

### REQ-BUDGET-05: Tool Call 카운팅

WHEN an EventToolCall event is received during stream parsing,
THE SYSTEM SHALL increment the budget counter by 1.

### REQ-BUDGET-06: 70% 경고 메시지

WHEN the budget counter reaches the 70% threshold,
THE SYSTEM SHALL write a warning message to the subprocess stdin:
`[BUDGET WARNING] 도구 호출 예산 70% 소진. 남은 예산: {remaining}회. 효율적으로 작업을 완료하세요.`

### REQ-BUDGET-07: 90% 위험 메시지

WHEN the budget counter reaches the 90% threshold,
THE SYSTEM SHALL write a critical warning message to the subprocess stdin:
`[BUDGET CRITICAL] 예산 거의 소진. 남은 예산: {remaining}회. 핵심 작업만 완료하세요.`

### REQ-BUDGET-08: 100% EmergencyStop

WHEN the budget counter reaches the hard limit (100%),
THE SYSTEM SHALL invoke `EmergencyStop.Stop(reason: "iteration_budget_exceeded")` to terminate the subprocess.

### REQ-BUDGET-09: Pipeline Phase별 예산 분배

WHEN a PipelineExecutor runs a multi-phase pipeline,
THE SYSTEM SHALL allocate the total budget across phases:
- Planning: 10%
- Execution: 60%
- Testing: 20%
- Review: 10%

### REQ-BUDGET-10: 미사용 예산 이월

WHERE a pipeline phase completes before exhausting its allocated budget,
THE SYSTEM SHALL carry over the remaining budget to the next phase.

## 생성 파일 상세

| 파일 | 패키지 | 역할 |
|------|--------|------|
| `pkg/worker/stream/events.go` | stream | EventToolCall 상수 추가 |
| `pkg/worker/budget/budget.go` | budget | IterationBudget 구조체 (limit, thresholds) |
| `pkg/worker/budget/counter.go` | budget | Counter — EventToolCall 카운팅 + 임계값 판단 |
| `pkg/worker/budget/warning.go` | budget | 경고 메시지 생성 및 stdin 쓰기 |
| `pkg/worker/budget/allocator.go` | budget | Pipeline phase별 예산 분배 및 이월 |
| `pkg/worker/loop_exec.go` | worker | stdin 파이프 유지 + budget 통합 |
| `pkg/worker/pipeline.go` | worker | PhaseAllocator 통합 |
| `pkg/worker/adapter/claude.go` | adapter | tool_use → EventToolCall 매핑 |
| `pkg/worker/adapter/codex.go` | adapter | tool_call → EventToolCall 매핑 |
| `pkg/worker/adapter/gemini.go` | adapter | tool_call → EventToolCall 매핑 |
