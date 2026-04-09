# SPEC-WKPROC-001 리서치

## 기존 코드 분석

### 1. Daemon (launchd/systemd)

**launchd.go** (`pkg/worker/daemon/launchd.go`)
- `GeneratePlist()`: XML 템플릿 기반 plist 생성, `xmlEscape` 함수로 안전한 값 삽입
- 현재 설정: `Label`, `ProgramArguments`, `KeepAlive=true`, `RunAtLoad=true`, `EnvironmentVariables(PATH)`, `StandardOutPath`, `StandardErrorPath`
- **누락**: `ProcessType`, `ThrottleInterval` — launchd가 Worker를 foreground로 취급할 수 있음
- `InstallLaunchd()`: `launchctl load`로 서비스 등록, `UninstallLaunchd()`: `launchctl unload`로 해제
- plist 경로: `~/Library/LaunchAgents/co.autopus.worker.plist`

**systemd.go** (`pkg/worker/daemon/systemd.go`)
- `GenerateUnit()`: Go template으로 unit 파일 생성
- 현재 설정: `Type=simple`, `Restart=always`, `RestartSec=5`, `After=network-online.target`
- **누락**: 로그 경로 설정 (journald 기본이지만 명시적 설정 없음)
- `InstallSystemd()`: `systemctl --user daemon-reload && enable --now`
- unit 경로: `~/.config/systemd/user/autopus-worker.service`

### 2. Worker 라이프사이클

**loop.go** (`pkg/worker/loop.go`)
- `WorkerLoop` struct: config, server, builder, authRefresher, netMonitor, pollFallback 등
- `Start()`: `server.Start(ctx)` → `startServices(ctx)` → parallel init
- `Close()`: `stopServices()` → `server.Close()`
- **PID lock 없음**: Start()에서 중복 실행 검사 없이 바로 서버 시작

**loop_lifecycle.go** (`pkg/worker/loop_lifecycle.go`)
- `startServices()`: audit → auth → knowledge → scheduler → net/poll 순서
- `stopServices()`: `lifecycleCancel()` → `auditWriter.Close()`
- **zombie reaper 없음**: subprocess 정리가 `cmd.Wait()`에만 의존

**loop_exec.go** (`pkg/worker/loop_exec.go`)
- `executeWithParallel()`: semaphore → worktree → subprocess 실행
- `executeSubprocess()` → `executeWithBudget()`: `cmd.Start()`, stdin write, stdout parse, `cmd.Wait()`
- `cmd.Wait()`이 정상 종료 수거를 담당하지만, Wait 호출 전 parent가 크래시하면 zombie 발생 가능

### 3. MCP 서버

**server.go** (`pkg/worker/mcpserver/server.go`, 195줄)
- `MCPServer`: backendURL, authToken, workspaceID, tools map, resources registry
- `Start()`: `bufio.Scanner`로 stdin에서 line-delimited JSON-RPC 읽기 — **stdio 전용**
- `dispatch()`: initialize, tools/list, tools/call, resources/list, resources/read 처리
- `registerTools()`: 6개 도구 등록 (execute_task, search_knowledge, get_execution_status, list_agents, approve_execution, manage_workspace)
- `writeResponse()`: mutex로 동기화된 stdout 쓰기

**tools.go** (`pkg/worker/mcpserver/tools.go`, 161줄)
- HTTP 클라이언트 기반: doGet, doPost, doPut → executeRequest(Bearer auth)
- 6개 핸들러: 각각 backend API를 프록시

**resources.go** (`pkg/worker/mcpserver/resources.go`, 142줄)
- 4개 리소스: status, workspaces, agents, executions/{id}
- TTL 기반 캐시 (30초), stale 캐시 서빙

### 4. Worker Status 커맨드

**worker_commands.go** (`internal/cli/worker_commands.go:70-96`)
- `newWorkerStatusCmd()`: `--json` 플래그 지원
- JSON 모드: `setup.CollectStatus()` 호출
- 일반 모드: `isDaemonInstalled()`, `printDaemonStatus()` — 데몬 설치 여부만 표시
- **PID, uptime, 태스크 상태 미보고**

## 설계 결정

### PID Lock 구현 방식

**결정**: `os.OpenFile` + PID 텍스트 기록 + `os.FindProcess(pid)` + `syscall.Kill(pid, 0)` 검증

**대안 검토**:
1. **`syscall.Flock` advisory lock**: Unix에서만 동작, Windows 미지원. Worker가 macOS/Linux 전용이므로 가능하지만, PID 파일 기반이 더 간단하고 디버깅이 쉬움 (파일 내용으로 PID 확인 가능).
2. **Unix domain socket**: 실행 중인 Worker에 연결 시도로 중복 검사. 더 robust하지만 구현 복잡도 높음. PID lock으로 충분한 단계에서 과도.
3. **fcntl lock**: `syscall.Flock`과 유사하나 NFS에서도 동작. PID lock이 `~/.autopus/`(로컬 전용)에 한정되므로 불필요.

**선택 근거**: PID 파일은 (1) 파일 존재 여부로 빠르게 확인 가능, (2) 파일 내용으로 PID 확인 가능, (3) `auto worker status`에서 PID 읽기 용이, (4) launchd/systemd가 자체 프로세스 관리를 하므로 advisory lock까지 필요 없음.

### Zombie Reaper 구현 방식

**결정**: 30초 간격 ticker goroutine + `syscall.Wait4(-1, &status, WNOHANG, nil)`

**대안 검토**:
1. **SIGCHLD 핸들러**: 시그널 기반으로 즉시 감지 가능하나, Go에서 시그널 핸들러 관리가 복잡하고 runtime과 충돌 가능. `os/signal.Notify`는 SIGCHLD를 직접 제공하지 않음.
2. **cmd.Wait() 강화**: 기존 `cmd.Wait()`에 타임아웃 추가. Wait 자체가 blocking이므로 goroutine wrap 필요. 현재 구조에서 이미 goroutine 내에서 실행되므로 추가 이점 없음.

**선택 근거**: 30초 polling은 NFR-04 요구사항을 충족하며, Go runtime과의 호환성 문제 없이 안전하게 동작. 성능 오버헤드 무시 가능 (30초에 1회 syscall).

### MCP SSE Transport 구현 방식

**결정**: `net/http` SSE handler를 별도 goroutine으로 실행, 기존 MCPServer의 tools/resources 레지스트리 공유

**대안 검토**:
1. **Fiber 기반 SSE**: Worker가 이미 Fiber를 사용하지 않으므로 불필요한 의존성 추가.
2. **WebSocket transport**: SSE보다 양방향이지만, MCP spec이 SSE를 권장하고 기존 WebSocket은 A2A 서버에서 사용 중이므로 포트/경로 충돌 우려.
3. **gRPC**: 프로토콜 변환 오버헤드, MCP 표준과 불일치.

**선택 근거**: net/http SSE는 (1) 추가 의존성 없음, (2) MCP 프로토콜 표준 준수, (3) 기존 stdio transport와 동일한 dispatch 로직 재사용 가능.

### MCP SSE 인증 방식 (OQ-3 답변 제안)

**제안**: Worker 토큰 재사용 — SSE 요청의 `Authorization: Bearer {worker-token}` 헤더로 인증. Worker 토큰은 이미 backend에서 발급받은 것이므로 별도 토큰 체계 불필요. SSE 연결 시 토큰 유효성 검증 후 연결 유지.

## 영향 받는 파일 요약

| 파일 | 변경 유형 | 설명 |
|------|-----------|------|
| `pkg/worker/pidlock/pidlock.go` | 신규 | PID lock acquire/release/stale 감지 |
| `pkg/worker/pidlock/pidlock_test.go` | 신규 | PID lock 테스트 |
| `pkg/worker/reaper/reaper.go` | 신규 | Zombie subprocess reaper |
| `pkg/worker/reaper/reaper_test.go` | 신규 | Reaper 테스트 |
| `pkg/worker/loop.go` | 수정 | PID lock 통합 (Start/Close) |
| `pkg/worker/loop_lifecycle.go` | 수정 | Reaper goroutine 시작 |
| `pkg/worker/daemon/launchd.go` | 수정 | ProcessType, ThrottleInterval 추가 |
| `pkg/worker/daemon/systemd.go` | 수정 | 로그 경로 명시 |
| `pkg/worker/mcpserver/sse.go` | 신규 | SSE transport handler |
| `pkg/worker/mcpserver/server.go` | 수정 | SSE 엔드포인트 등록 |
| `pkg/worker/mcpserver/sse_test.go` | 신규 | SSE transport 테스트 |
| `pkg/worker/mcpserver/config.go` | 신규 | Config struct + JSON Schema 검증 |
| `internal/cli/worker_commands.go` | 수정 | status 보고 확장 |
