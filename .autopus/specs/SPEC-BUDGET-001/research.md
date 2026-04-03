# SPEC-BUDGET-001 리서치

## 기존 코드 분석

### Stream 프로토콜 (현재 상태)

- `pkg/worker/stream/events.go:6-13` — 이벤트 타입 상수: `system.init`, `system.task_started`, `system.task_progress`, `system.task_notification`, `result`, `error`만 정의. tool_call/tool_use 타입 없음.
- `pkg/worker/stream/parser.go:49-73` — `ParseLine`: JSON 라인의 `type` 필드를 파싱하여 `Event{Type, Subtype, Raw}` 반환. `splitType`로 "system.init" → ("system", "init") 분리. 이 로직은 tool_call 타입도 자동으로 처리 가능 (추가 파싱 불필요).

### Adapter ParseEvent (현재 상태)

- `pkg/worker/adapter/claude.go:59-69` — `ParseEvent`: `stream.ParseLine`을 위임 호출. tool_use 이벤트가 들어와도 현재는 "tool_use" type으로 그대로 전달됨 (EventToolCall 매핑 없음).
- `pkg/worker/adapter/codex.go:57-72` — `ParseEvent`: 자체 JSON unmarshal. type 필드만 추출.
- `pkg/worker/adapter/gemini.go:61-76` — `ParseEvent`: Codex와 동일 패턴.

### stdin 파이프 (현재 상태)

- `pkg/worker/loop_exec.go:40-47` — `executeSubprocess` 내 goroutine:
  ```go
  go func() {
      defer stdin.Close()
      io.Copy(stdin, strings.NewReader(taskCfg.Prompt))
  }()
  ```
  프롬프트를 쓴 뒤 stdin을 즉시 닫음. 런타임 메시지 주입 불가.
- `pkg/worker/pipeline.go:124-127` — `runPhase`도 동일한 패턴으로 stdin 즉시 닫음.

### parseStream (현재 상태)

- `pkg/worker/loop_exec.go:74-121` — switch문:
  - `system.init`, `system.task_started`, `system.task_progress`, `system.task_notification` → 로깅
  - `result` → ExtractResult로 결과 추출
  - `error` → 로깅
  - **tool_call 매칭 없음** — tool_call 이벤트가 수신되면 무시됨 (default case 없음, 단순 skip)

### EmergencyStop (현재 상태)

- `pkg/worker/security/emergency.go:14-89` — `EmergencyStop` 구조체: `SetProcess(cmd)` → `Stop(reason)` → SIGTERM → 5초 대기 → SIGKILL. 프로세스 그룹(-pid) 대상. thread-safe (sync.Mutex).
- **호출처 없음**: SetProcess/Stop이 구현되어 있지만 현재 코드에서 어디서도 호출하지 않음. Budget이 첫 번째 통합 대상이 됨.

### Pipeline (현재 상태)

- `pkg/worker/pipeline.go:54-95` — `Execute`: planner→executor→tester→reviewer 순차 실행. 각 phase별 독립 subprocess. 예산 관리 없음.
- `pkg/worker/pipeline.go:98-143` — `runPhase`: 단일 subprocess 실행. Budget 주입점은 cmd.Start() 이후, parsePhaseStream 호출 시점.

### WorkerLoop 설정

- `pkg/worker/loop.go:18-26` — `LoopConfig`: Budget 관련 필드 없음. limit 값을 추가해야 함.
- `pkg/worker/loop.go:78-121` — `handleTask`: executeSubprocess 호출 전에 budget 초기화 필요.

## 설계 결정

### D1: EventToolCall 통합 타입

Claude는 `tool_use`, Codex/Gemini는 `tool_call`을 사용한다. 통합 상수 `EventToolCall = "tool_call"`을 정의하고, Claude adapter에서 `tool_use`를 `tool_call`로 정규화한다.

**대안 검토**: 각 프로바이더별 별도 이벤트 타입 유지 → 카운팅 로직이 프로바이더 의존적이 되어 복잡도 증가. 정규화가 더 깔끔.

### D2: stdin 파이프 유지 방식

`StdinWriter` 래퍼를 도입하여 io.WriteCloser를 보유한다. 초기 프롬프트는 Write로 전달하고, budget 경고도 Write로 주입한다. 서브프로세스 종료 시 Close를 호출한다.

**대안 검토**: 별도의 사이드 채널(signal, named pipe) → CLI 프로바이더가 지원하지 않으므로 불가. stdin이 유일한 입력 채널.

### D3: Budget 임계값 하드코딩 vs 설정

70%/90%는 하드코딩한다. 대부분의 사용 사례에서 합리적인 값이며, 설정 가능하게 만들면 복잡도만 증가한다.

**대안 검토**: 설정 파일에서 임계값 로드 → 추후 필요 시 확장 가능하도록 구조체 필드로는 노출하되, 기본값 사용 권장.

### D4: EmergencyStop 통합 방식

Budget counter가 100%에 도달하면 EmergencyStop.Stop을 직접 호출한다. EmergencyStop은 이미 thread-safe하고 프로세스 그룹 종료를 지원하므로 그대로 사용한다.

**대안 검토**: context.Cancel로 우아한 종료 → CLI 프로바이더가 context cancellation을 인식하지 못할 수 있음. SIGTERM이 더 확실.

### D5: Pipeline Allocator 이월 전략

Phase 종료 시 미사용 예산을 다음 phase에 단순 합산한다. 이월 상한은 두지 않는다 (다음 phase가 활용하는 것이 최선).

**대안 검토**: 이월 상한 설정 (e.g., 원래 예산의 150%) → 불필요한 제약. 전체 예산이 이미 상한이므로 추가 제약 불필요.

## 참고 자료

- Hermes Agent의 AIAgent 90회 제한 패턴: 고정 limit + 경고 메시지 접근법
- BS-021 #2 브레인스톰: 핵심 흐름 및 확장 방향 정의
