# SPEC-ADKWIRE-001 구현 계획

## 태스크 목록

- [ ] T1: `internal/cli/learn.go` — Learn CLI 커맨드 그룹 생성 (query, record, prune, summary)
- [ ] T2: `internal/cli/root.go` — `newLearnCmd()` 등록 (1줄 추가)
- [ ] T3: `pkg/worker/loop.go` — LoopConfig에 CredentialsPath 필드 추가
- [ ] T4: `pkg/worker/loop_lifecycle.go` — auth/poll/net 라이프사이클 관리 분리 파일 생성
- [ ] T5: `internal/cli/pipeline_dashboard.go` — stub 제거, checkpoint 기반 렌더링
- [ ] T6: `pkg/pipeline/status_map.go` — CheckpointStatus → PhaseStatus 매핑 유틸리티
- [ ] T7: 단위 테스트 — learn CLI, status_map, lifecycle wiring

## 에이전트 배정

| Task | Agent | 의존성 | 예상 라인 |
|------|-------|--------|-----------|
| T1 | executor-1 | 없음 | ~120 |
| T2 | executor-1 | T1 | 1 |
| T3 | executor-2 | 없음 | ~5 |
| T4 | executor-2 | T3 | ~100 |
| T5 | executor-3 | T6 | ~30 |
| T6 | executor-3 | 없음 | ~40 |
| T7 | tester | T1-T6 | ~150 |

## 구현 전략

### Learn CLI (T1-T2)
- `internal/cli/learn.go`에 `newLearnCmd()` 함수 작성, 4개 subcommand 등록
- 각 subcommand는 `pkg/learn/` 패키지의 public API를 직접 호출
- Store 초기화: `learn.NewStore(cwd)` — cwd는 실행 디렉토리 기준
- query: `--files`, `--packages`, `--keywords` 플래그 → `RelevanceQuery` 구성 → `QueryRelevant(store, query, 1.0)`
- record: `--type` 필수, `--pattern` 필수, 나머지 optional → `Record*(store, opts)` 호출 (type에 따라 분기)
- prune: `--days` 필수 → `Prune(store, days)` → 삭제 건수 출력
- summary: `--top` optional (default 5) → `GenerateSummary(store, topN)` → 포맷 출력
- root.go에 `root.AddCommand(newLearnCmd())` 1줄 추가

### Worker 패키지 Wiring (T3-T4)
- `LoopConfig`에 `CredentialsPath string` 필드 추가
- 새 파일 `loop_lifecycle.go`에서 라이프사이클 분리:
  - `startServices(ctx)` — TokenRefresher, NetMonitor 시작
  - `stopServices()` — 정리
  - `activateFallbackPoller(ctx)` — A2A 실패 시 REST poller 활성화
- TokenRefresher: `onTokenRefresh` 콜백에서 A2A Server의 auth token 갱신
- NetMonitor: `onChange`에서 `a2a.Transport.Reconnect()` 호출, `onValidate`에서 ping 체크
- TaskPoller: `server.OnReconnectFailed()`에 fallback 연결, `onTask` 콜백에서 `handleTask` 호출

### Pipeline Dashboard (T5-T6)
- `pkg/pipeline/status_map.go`에 `MapCheckpointToPhases(cp *Checkpoint) DashboardData` 함수 생성
  - Checkpoint.Phase → 현재 phase running, 이전 phase done, 이후 phase pending
  - TaskStatus의 failed 존재 시 해당 phase failed
- `pipeline_dashboard.go`에서 `pipeline.Load(cwd)` 호출
  - 성공: `MapCheckpointToPhases()` → `RenderDashboard()`
  - 실패 (파일 없음): 기존 all-pending 유지 + 경고 메시지 출력

### 기존 코드 변경 최소화 원칙
- `loop.go`의 Start()/Close()는 `loop_lifecycle.go`의 함수를 호출하는 1-2줄만 추가
- A2A Server에 SetAuthToken() 메서드가 없으면 추가 필요 (확인 결과 없음 → T4에 포함)
