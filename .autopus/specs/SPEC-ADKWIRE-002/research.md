# SPEC-ADKWIRE-002 리서치

## 기존 코드 분석

### Worker 메인 루프 (pkg/worker/loop.go, 184줄)
- `LoopConfig` struct (19-28줄): BackendURL, WorkerName, Skills, Provider, MCPConfig, WorkDir, AuthToken, Router
- `WorkerLoop` struct (33-37줄): config, server, builder, tuiProgram — 5개 패키지 참조 없음
- `NewWorkerLoop()` (40-56줄): A2A Server만 생성. audit/scheduler/parallel/knowledge/workspace 초기화 없음
- `Start()` (59-62줄): `wl.server.Start(ctx)` 한 줄. 추가 서비스 시작 없음
- `Close()` (65-67줄): `wl.server.Close()` 한 줄
- `handleTask()` (80-130줄): 순차 실행. semaphore 없음, audit 기록 없음, KnowledgeCtx 미사용 (74줄에 필드 정의만 있음)
- `taskPayloadMessage.KnowledgeCtx` (74줄): 필드가 존재하지만 `Build()` 호출 시 msg.KnowledgeCtx가 그대로 전달될 뿐, 검색 결과로 보강되지 않음

### Task Execution (pkg/worker/loop_exec.go, 204줄)
- `executeSubprocess()` (54-56줄): `executeWithBudget(ctx, taskCfg, nil)` — budget 없이 위임
- `executeWithBudget()` (60-120줄): Provider.BuildCommand → stdin/stdout pipe → Start → parseStream → Wait
- 세마포어, worktree, audit 어디에도 없음

### audit/rotation.go (156줄)
- `RotatingWriter`: io.Writer 구현, size-based rotation, maxAge cleanup
- `NewRotatingWriter(path, maxSize, maxAge)` → `(*RotatingWriter, error)`
- `StartCleanup(ctx)` — background goroutine, ticker 1시간
- `Write(p)` — mutex 보호, rotation on overflow
- `Close()` — file close
- Wiring 필요: `NewWorkerLoop`에서 생성, `Start()`에서 `go w.StartCleanup(ctx)`, `Close()`에서 `w.Close()`

### scheduler/dispatcher.go (137줄) + cron.go (133줄)
- `Dispatcher`: 60초 간격 poll, cron match, dedup (lastTrigger map)
- `NewDispatcher(backendURL, authToken, workspaceID, loc, onTrigger)` → `*Dispatcher`
- `Start(ctx)` — blocking loop (goroutine으로 호출해야 함)
- `onTrigger(scheduleID, taskPayload string)` 콜백 — handleTask로 연결 필요
- scheduler는 WorkspaceID가 필요 → LoopConfig에 WorkspaceID 필드 추가 필요
- cron.go: 5-field cron parser, `ParseCron()`, `CronExpr.Match(time.Time)`

### parallel/semaphore.go (53줄) + worktree.go (86줄)
- `TaskSemaphore`: `NewTaskSemaphore(limit)`, `Acquire(ctx)`, `Release()`, `Available()`, `Limit()`
- `WorktreeManager`: `NewWorktreeManager(baseDir)`, `Create(taskID)`, `Remove(path, force)`, `List()`
- Create: `git -c gc.auto=0 worktree add {path} -b worker-{taskID}`
- Remove: `git -c gc.auto=0 worktree remove [--force] {path}`
- Wiring: handleTask에서 Acquire → Create → execute → Remove → Release

### knowledge/ (4 files: search.go 66줄, syncer.go 120줄, watcher.go 106줄, excluder.go 104줄)
- `KnowledgeSearcher`: `NewKnowledgeSearcher(backendURL, authToken)`, `Search(ctx, query) ([]SearchResult, error)`
- SearchResult: ID, Title, Content, Score
- `Syncer`: `NewSyncer(backendURL, authToken, workspaceID)`, `SyncFile(ctx, path)`
- `FileWatcher`: `NewFileWatcher(dir, interval, onChange, excluder)`, `Start(ctx)` (blocking), `Stop()`
- onChange 콜백에서 `Syncer.SyncFile()` 호출하면 파일 변경 → 자동 sync 완성
- `Excluder`: `NewExcluder(gitignorePath)`, `IsExcluded(path)` — .gitignore + built-in rules

### workspace/multi.go (84줄)
- `MultiWorkspace`: `NewMultiWorkspace()`, `Add(conn)`, `Get(id)`, `RouteTask(id)`, `List()`, `Remove(id)`
- `WorkspaceConn`: WorkspaceID, ProjectDir, BackendURL, AuthToken, Connected
- RouteTask: Connected 검사 후 ProjectDir 반환
- Wiring: 각 WorkspaceConfig에 대해 별도 A2A Server goroutine 스폰, RouteTask로 WorkDir 결정

## SPEC-ADKWIRE-001 패턴 참조

ADKWIRE-001은 `loop_lifecycle.go` 파일을 새로 만들어 auth/poll/net 서비스를 관리한다:
- `startServices(ctx)` — 서비스 초기화 및 goroutine 시작
- `stopServices()` — 역순 종료
- loop.go의 Start()/Close()에서 1-2줄로 호출

이 SPEC은 동일한 패턴을 확장하여 5개 서비스를 추가한다.

## 설계 결정

### D1: 새 파일 3개 분리 vs loop_lifecycle.go 단일 확장
- **결정**: 새 파일 3개 (loop_audit.go, loop_knowledge.go, loop_workspace.go) + lifecycle.go 수정
- **이유**: lifecycle.go에 모든 로직을 넣으면 300줄을 초과할 가능성이 높음. 각 패키지의 wiring 로직을 별도 파일로 분리하면 관심사 분리가 명확하고 파일 크기 제한을 준수 가능
- **대안**: lifecycle.go 하나에 모두 넣기 — 500줄+ 예상으로 file-size-limit 위반

### D2: Semaphore wiring 위치
- **결정**: loop_exec.go의 handleTask/executeSubprocess 수정
- **이유**: semaphore는 태스크 실행 경로에 직접 삽입해야 하므로 별도 파일보다 기존 실행 경로 수정이 자연스러움
- **대안**: loop_parallel.go 별도 파일 — handleTask 호출 흐름이 불필요하게 분산됨

### D3: Knowledge search의 blocking vs non-blocking
- **결정**: Non-blocking (검색 실패 시 빈 KnowledgeCtx로 진행)
- **이유**: knowledge search는 보조 정보이며, API 장애로 태스크 실행이 차단되면 안 됨. 5초 timeout 내 실패 시 빈 문자열 반환
- **대안**: Blocking + retry — 태스크 지연 리스크가 검색 가치보다 큼

### D4: Multi-workspace A2A 서버 구조
- **결정**: 워크스페이스별 독립 A2A Server goroutine
- **이유**: workspace/multi.go가 이미 per-workspace 연결 모델을 사용. 각 워크스페이스가 독립된 백엔드 URL과 auth token을 가질 수 있으므로 별도 서버가 필요
- **대안**: 단일 A2A 서버에 workspace routing — 백엔드 URL이 다를 수 있어 불가

### D5: Scheduler의 onTrigger → handleTask 연결
- **결정**: onTrigger 콜백에서 json.RawMessage로 변환 후 handleTask 직접 호출
- **이유**: 스케줄러가 트리거하는 태스크도 외부 수신 태스크와 동일한 경로로 처리되어야 audit, semaphore 등이 적용됨
- **대안**: 별도 실행 경로 — audit/parallel 미적용으로 일관성 깨짐
