# SPEC-WKPROC-001 구현 계획

## 태스크 목록

### Phase 1: PID Lock (P0)

- [ ] T1: `pkg/worker/pidlock/` 패키지 생성 — Acquire, Release, IsStale, ReadPID 함수
  - 파일: `pkg/worker/pidlock/pidlock.go` (신규)
  - 의존: 없음

- [ ] T2: WorkerLoop.Start()에 PID lock 획득 통합
  - 파일: `pkg/worker/loop.go` (수정)
  - 의존: T1

- [ ] T3: WorkerLoop.Close()에 PID lock 해제 통합 + signal handler 등록 (SIGTERM/SIGINT)
  - 파일: `pkg/worker/loop.go` (수정)
  - 의존: T1

- [ ] T4: PID lock 유닛 테스트
  - 파일: `pkg/worker/pidlock/pidlock_test.go` (신규)
  - 의존: T1

### Phase 2: Zombie Reaper (P0)

- [ ] T5: `pkg/worker/reaper/` 패키지 생성 — 주기적 zombie 프로세스 감지 및 reap goroutine
  - 파일: `pkg/worker/reaper/reaper.go` (신규)
  - 의존: 없음

- [ ] T6: loop_lifecycle.go startServices에 reaper 시작 통합
  - 파일: `pkg/worker/loop_lifecycle.go` (수정)
  - 의존: T5

- [ ] T7: Reaper 유닛 테스트
  - 파일: `pkg/worker/reaper/reaper_test.go` (신규)
  - 의존: T5

### Phase 3: Daemon 설정 강화 (P0)

- [ ] T8: launchd plist 템플릿에 ProcessType, ThrottleInterval 추가
  - 파일: `pkg/worker/daemon/launchd.go` (수정)
  - 의존: 없음

- [ ] T9: systemd unit 템플릿에 StandardOutput/StandardError 로그 경로 추가
  - 파일: `pkg/worker/daemon/systemd.go` (수정)
  - 의존: 없음

### Phase 4: MCP SSE Transport (P1)

- [ ] T10: `pkg/worker/mcpserver/sse.go` — SSE transport handler 구현 (net/http 기반)
  - 파일: `pkg/worker/mcpserver/sse.go` (신규)
  - 의존: 없음

- [ ] T11: MCPServer에 SSE 엔드포인트 등록, 기존 stdio와 병렬 동작
  - 파일: `pkg/worker/mcpserver/server.go` (수정)
  - 의존: T10

- [ ] T12: SSE transport 테스트
  - 파일: `pkg/worker/mcpserver/sse_test.go` (신규)
  - 의존: T10, T11

### Phase 5: MCP Config Schema (P1)

- [ ] T13: `pkg/worker/mcpserver/config.go` — MCP config 구조체 + JSON Schema 검증
  - 파일: `pkg/worker/mcpserver/config.go` (신규)
  - 의존: 없음

- [ ] T14: 서버 시작 시 config 검증 통합
  - 파일: `pkg/worker/mcpserver/server.go` (수정)
  - 의존: T13

### Phase 6: Worker Status 확장 (P2)

- [ ] T15: `auto worker status`에 PID, uptime, 현재 태스크, 연결 상태, 마지막 heartbeat 보고 추가
  - 파일: `internal/cli/worker_commands.go` (수정)
  - 의존: T1

## 구현 전략

- **PID lock**: `os.OpenFile` + `syscall.Flock`(Unix)으로 advisory lock 구현. `~/.autopus/worker.pid`에 PID 텍스트 기록. 프로세스 존재 확인은 `os.FindProcess` + `syscall.Signal(0)`으로 수행.
- **Zombie reaper**: 30초 간격 ticker goroutine이 `os.Process.Wait`(WNOHANG)로 zombie 자식 프로세스 수거. `loop_exec.go`의 `cmd.Wait()`이 이미 정상 종료를 처리하므로, reaper는 orphan/unexpected zombie만 대상.
- **Daemon**: 기존 plist/unit 템플릿에 필드 추가만으로 최소 변경. 하위 호환 유지.
- **MCP SSE**: `net/http` SSE handler가 `/mcp/sse` 엔드포인트에서 JSON-RPC 메시지를 SSE 이벤트로 전송. 기존 `MCPServer.tools`와 `resources` 레지스트리를 공유하여 중복 없음.
- **MCP Config**: Go struct에 `jsonschema` 태그로 스키마 자동 생성, `NewMCPServer` 호출 시 검증.

## 의존성 그래프

```
T1 ─→ T2, T3, T4, T15
T5 ─→ T6, T7
T8, T9 (독립)
T10 ─→ T11, T12
T13 ─→ T14
```

Phase 1-3(P0)이 Phase 4-6(P1/P2)보다 우선.
