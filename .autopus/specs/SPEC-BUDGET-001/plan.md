# SPEC-BUDGET-001 구현 계획

## 태스크 목록

- [ ] T1: Stream 프로토콜 확장 — `stream/events.go`에 `EventToolCall` 상수 추가
- [ ] T2: Adapter tool_call 매핑 — `claude.go`, `codex.go`, `gemini.go`의 ParseEvent에서 tool_call/tool_use를 EventToolCall로 변환
- [ ] T3: stdin 파이프 리팩토링 — `loop_exec.go`의 `executeSubprocess`에서 stdin 파이프를 열린 상태로 유지, `StdinWriter` 구조체 도입
- [ ] T4: Budget 패키지 코어 — `budget/budget.go` IterationBudget 구조체 (limit, 70%/90% thresholds)
- [ ] T5: Counter 구현 — `budget/counter.go` EventToolCall 카운팅 및 임계값 도달 판단
- [ ] T6: Warning 구현 — `budget/warning.go` 경고/위험 메시지 생성 및 stdinWriter 주입
- [ ] T7: EmergencyStop 연계 — Counter가 100% 도달 시 `security/emergency.go`의 Stop 호출
- [ ] T8: parseStream 통합 — `loop_exec.go:74-121` switch문에 EventToolCall 케이스 추가, budget counter 연동
- [ ] T9: Pipeline Allocator — `budget/allocator.go` phase별 예산 분배 (10/60/20/10) + 이월 로직
- [ ] T10: Pipeline 통합 — `pipeline.go`의 runPhase에 PhaseAllocator 연결
- [ ] T11: 단위 테스트 — budget 패키지 전체 (counter, warning, allocator)
- [ ] T12: 통합 테스트 — loop_exec + budget + emergency 연동 테스트

## 구현 전략

### 의존 관계

```
T1 → T2 (stream 이벤트가 먼저 있어야 adapter에서 매핑 가능)
T1 → T4 → T5 → T6, T7 (budget 패키지 순차 빌드)
T3 → T6 (stdinWriter가 있어야 warning 주입 가능)
T3 + T5 + T7 → T8 (parseStream 통합은 모든 선행 완료 후)
T4 + T5 → T9 → T10 (allocator는 budget core 후 구현)
T5 + T6 + T9 → T11 (단위 테스트)
T8 + T10 → T12 (통합 테스트)
```

### 병렬 실행 가능 그룹

- Group A: T1 (stream 확장)
- Group B: T3 (stdin 리팩토링) — T1과 독립적
- Group C: T4 (budget core) — T1 완료 후
- Group D: T2 + T5 + T6 + T7 — T1 + T4 완료 후 병렬
- Group E: T8 + T9 — Group D 완료 후
- Group F: T10 + T11 — Group E 완료 후
- Group G: T12 — 마지막

### 기존 코드 활용

- `stream.ParseLine` (parser.go:49-73): 기존 JSON 파싱 로직 활용, type 필드로 tool_call 감지
- `EmergencyStop` (security/emergency.go:14-89): 이미 구현된 SIGTERM→SIGKILL 패턴 그대로 사용
- `WorkerLoop.parseStream` (loop_exec.go:74-121): switch문에 case 추가로 확장
- `PipelineExecutor.runPhase` (pipeline.go:98-143): budget allocator 주입점

### 변경 범위

- 신규 파일: 4개 (`budget/budget.go`, `counter.go`, `warning.go`, `allocator.go`)
- 수정 파일: 5개 (`stream/events.go`, `loop_exec.go`, `pipeline.go`, `claude.go`, `codex.go`, `gemini.go`)
- 테스트 파일: 2개 이상 (`budget/*_test.go`, `loop_exec` 통합 테스트)
