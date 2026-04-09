# SPEC-WKAUTH-001 구현 계획

## 태스크 목록

- [ ] T1: `pkg/worker/auth/backoff.go` — Exponential backoff + jitter 유틸리티 생성
- [ ] T2: `pkg/worker/auth/refresher.go` — CredentialStore 통합 및 backoff 적용 리팩터
- [ ] T3: `pkg/worker/auth/reconnect.go` — 조율된 재연결 시퀀스 (Reconnector) 생성
- [ ] T4: `pkg/worker/loop.go` — LoopConfig에 CredentialStore 필드 추가
- [ ] T5: `pkg/worker/loop_lifecycle.go` — CredentialStore 주입, 재연결 조율 콜백 연결
- [ ] T6: `pkg/worker/auth/refresher_test.go` — backoff 및 CredentialStore 통합 테스트
- [ ] T7: `pkg/worker/auth/reconnect_test.go` — 재연결 시퀀스 테스트

## 구현 전략

### T1: Backoff 유틸리티

`backoff.go`에 재사용 가능한 backoff 계산 함수를 분리한다.

```
Backoff(attempt int, base time.Duration, factor float64, jitter float64) time.Duration
```

- attempt=0 → base (3s)
- attempt=1 → base×factor (6s)
- attempt=2 → base×factor² (12s)
- jitter ±20%: `delay × (1 + rand(-0.2, +0.2))`
- 총 최대: 3+6+12 = 21s (±jitter) < NFR-01 25s

### T2: TokenRefresher 리팩터

현재 `refresher.go`의 변경점:

1. **생성자 변경**: `credentialsPath string` → `store setup.CredentialStore` 파라미터
   - `LoadCredentials()`: `os.ReadFile` → `store.Load("autopus-worker")` + JSON unmarshal
   - `SaveCredentials()`: `os.WriteFile` → JSON marshal + `store.Save("autopus-worker")`

2. **checkAndRefresh 로직**:
   - 현재: 1회 실패 → 디스크 재로드 → 1회 재시도 → onReauthNeeded
   - 변경: 최대 3회 backoff 재시도 → 전부 실패 시 구조화된 에러 이벤트(FR-AUTH-11) → onReauthNeeded

3. **doRefresh 분리**: HTTP 호출 자체는 변경 없음, 재시도 루프를 `checkAndRefresh`에서 관리

4. **마이그레이션**: `setup.migratePlaintextCredentials`가 이미 CredentialStore 초기화 시 자동 실행됨 (credstore.go:69). 별도 마이그레이션 코드 불필요.

### T3: Reconnector (재연결 조율)

`reconnect.go`에 새 타입 생성:

```go
type Reconnector struct {
    refresher  *TokenRefresher
    server     ServerReconnecter  // interface { ReconnectTransport(ctx) error; SetAuthToken(string) }
    mu         sync.Mutex         // 중복 재연결 시도 방지
    inProgress bool
}
```

`Reconnect(ctx) error` 메서드:
1. mutex로 중복 호출 방지
2. `refresher.ForceRefresh(ctx)` → 토큰 갱신 (backoff 포함)
3. `server.SetAuthToken(newToken)`
4. `server.ReconnectTransport(ctx)` → WebSocket 재연결

### T4: LoopConfig 확장

`LoopConfig`에 필드 추가:
```go
CredentialStore setup.CredentialStore // optional: nil = skip token refresh
```

`CredentialsPath`는 deprecated 처리 (하위 호환: CredentialStore가 nil이고 CredentialsPath가 있으면 기존 동작).

### T5: Lifecycle 통합

`startServices()`에서:
1. CredentialStore가 설정된 경우 → `auth.NewTokenRefresher(backendURL, store, callbacks)`
2. `auth.NewReconnector(refresher, server)` 생성
3. NetMonitor의 onChange 콜백을 `reconnector.Reconnect(ctx)` 호출로 변경
4. 기존 `server.ReconnectTransport` 직접 호출 제거

## 변경 범위

- 신규 파일 2개 (backoff.go, reconnect.go) — 각 ~60-80줄 예상
- 수정 파일 3개 (refresher.go, loop.go, loop_lifecycle.go)
- refresher.go: ~199줄 → ~180줄 예상 (평문 I/O 제거로 감소)
- 모든 파일 300줄 미만 유지

## 의존성

- `setup.CredentialStore` 인터페이스 (이미 구현됨)
- `setup.NewCredentialStore()` 팩토리 (이미 구현됨)
- `a2a.Server.ReconnectTransport()`, `a2a.Server.SetAuthToken()` (이미 구현됨)
