# SPEC-ADKWIRE-001 리서치

## 기존 코드 분석

### Learn 패키지 (`pkg/learn/`)

완전한 구현이 존재하지만 CLI 진입점이 없음:

| 파일 | Public API | 용도 |
|------|-----------|------|
| `store.go` | `NewStore(dir)`, `Append()`, `Read()`, `NextID()`, `UpdateReuseCount()` | JSONL 파일 기반 저장소 |
| `types.go` | `LearningEntry`, `EntryType`, `Severity`, `RelevanceQuery`, `Summary`, `RecordOpts` | 타입 정의 |
| `query.go` | `MatchRelevance()`, `QueryRelevant(store, query, threshold)` | 관련도 기반 검색 (file/package/keyword 매칭) |
| `record.go` | `RecordGateFail()`, `RecordCoverageGap()`, `RecordReviewIssue()`, `RecordExecutorError()`, `RecordFixPattern()` | 타입별 기록 함수 |
| `prune.go` | `Prune(store, days)` | 오래된 엔트리 삭제 |
| `summary.go` | `GenerateSummary(store, topN)` | 요약 생성 (타입카운트, 패턴통계, 개선영역) |
| `rewrite.go` | `rewriteStore(store, entries)` (unexported) | 전체 파일 재작성 |

CLI 커맨드 패턴 참고: `internal/cli/root.go`에서 각 커맨드는 `newXxxCmd() *cobra.Command`를 반환하고 `root.AddCommand()`에 등록. 현재 `newLearnCmd`는 없음.

### Worker auth 패키지 (`pkg/worker/auth/refresher.go`)

- `TokenRefresher` — 60초 간격으로 credentials 파일 확인, 만료 5분 전에 `/api/v1/auth/refresh` POST로 갱신
- `Start(ctx)` — blocking loop, goroutine으로 실행해야 함
- `onTokenRefresh(newToken string)` — 콜백으로 새 토큰 전파
- `onReauthNeeded()` — 갱신 실패 시 재인증 요구 콜백
- 현재 `loop.go`에서 import도 없고 인스턴스 생성도 없음

### Worker poll 패키지 (`pkg/worker/poll/poller.go`)

- `TaskPoller` — adaptive backoff (2s-60s)으로 REST API polling
- `Start(ctx)` — blocking loop
- `Reset()` — backoff 리셋 (WebSocket push 수신 시)
- `onTask([]byte)` — 태스크 수신 콜백
- A2A WebSocket 실패 시 fallback으로 설계되었으나 연결 없음

### Worker net 패키지 (`pkg/worker/net/monitor.go`)

- `NetMonitor` — 5초 간격으로 네트워크 인터페이스 주소 변경 감지
- `Start(ctx)` — goroutine 내부에서 실행 (non-blocking)
- `onChange(oldAddrs, newAddrs)` — 주소 변경 시 콜백
- `onValidate() error` — 변경 감지 시 먼저 유효성 검사 (에러면 onChange 호출)
- WebSocket reconnect 트리거로 설계되었으나 연결 없음

### A2A Transport Reconnect

- `pkg/worker/a2a/ws_transport.go:177` — `Transport.Reconnect(ctx)` 메서드 존재
- `pkg/worker/a2a/ws_client.go:74` — `Client.OnReconnectFailed(fn func(error))` 콜백 등록 메서드 존재
- 테스트도 존재: reconnect success, all retries fail, context canceled

### Pipeline Dashboard Stub

- `internal/cli/pipeline_dashboard.go:30` — `@AX:WARN @AX:CYCLE:3` 어노테이션, 3사이클 방치된 stub
- 모든 phase를 `PhasePending`으로 하드코딩
- `pkg/pipeline/checkpoint.go` — `Load(dir)` 함수로 `.autopus-checkpoint.yaml` 로드 가능
- `pkg/pipeline/types.go` — `Checkpoint` struct에 `Phase string`과 `TaskStatus map[string]CheckpointStatus`
- `CheckpointStatus`와 `PhaseStatus`는 별도 타입이므로 매핑 함수 필요

## 설계 결정

### D1: Learn CLI를 단일 파일로 구현

**결정**: `internal/cli/learn.go`에 4개 subcommand를 모두 포함
**이유**: 각 subcommand는 15-30줄 수준이며 총 ~120줄로 200줄 미만. 파일 분리 시 오히려 탐색이 어려워짐.
**대안**: subcommand별 파일 분리 — 불필요한 복잡성

### D2: Worker 라이프사이클을 별도 파일로 분리

**결정**: `pkg/worker/loop_lifecycle.go` 신규 생성
**이유**: `loop.go`가 이미 184줄이므로 auth/poll/net wiring 코드를 추가하면 300줄 한도 초과. 라이프사이클 관리 로직을 별도 파일로 분리하면 각 파일이 200줄 미만 유지.
**대안**: loop.go에 직접 추가 — 300줄 한도 위반

### D3: A2A Server에 SetAuthToken 메서드 추가

**결정**: `a2a.Server`에 `SetAuthToken(token string)` 추가 필요
**이유**: TokenRefresher의 `onTokenRefresh` 콜백에서 Server의 auth token을 업데이트해야 하나 현재 해당 메서드가 없음. ServerConfig.AuthToken은 초기값만 설정.
**대안**: Server 재생성 — 비효율적이고 연결 끊김 발생

### D4: Phase 매핑 로직을 pipeline 패키지에 배치

**결정**: `pkg/pipeline/status_map.go`에 `MapCheckpointToPhases()` 함수 생성
**이유**: Checkpoint와 DashboardData 모두 pipeline 패키지 내 타입이므로 같은 패키지에서 매핑하는 것이 자연스러움. CLI 레이어에 도메인 로직이 스며드는 것을 방지.
**대안**: CLI에서 직접 매핑 — 도메인 로직 누출

### D5: Checkpoint 없을 때 fallback 유지

**결정**: checkpoint 파일이 없으면 all-pending + 경고 메시지 출력
**이유**: 파이프라인 시작 전에 dashboard를 볼 수 있어야 하므로 에러 대신 graceful fallback이 적절. 단, 사용자에게 실제 데이터가 아님을 알려야 함.
**대안**: 에러 반환 — UX 저하
