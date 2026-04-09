# SPEC-WKAUTH-001 리서치

## 기존 코드 분석

### TokenRefresher (`pkg/worker/auth/refresher.go`, 199줄)

**현재 동작**:
- 60s ticker로 `checkAndRefresh()` 호출
- 만료 5분 전에 refresh 시도
- `http.Client{Timeout: 10s}` — backoff 없음
- 실패 시 1회 디스크 재로드 후 1회 재시도, 그래도 실패 시 즉시 `onReauthNeeded()`
- `LoadCredentials()`: `os.ReadFile(credentialsPath)` → JSON unmarshal → 평문 파일
- `SaveCredentials()`: JSON marshal → `os.WriteFile(tmp)` → `os.Rename()` → 평문 파일
- `doRefresh()`: POST `/api/v1/auth/cli-refresh` with refresh_token body

**문제점**:
1. 단일 HTTP 시도 + 1회 재시도만 — 일시적 네트워크 오류에 취약
2. `os.ReadFile`/`os.WriteFile`로 평문 JSON 직접 읽기/쓰기 — CredentialStore 미사용
3. backoff/jitter 없음 — 서버 장애 시 모든 Worker가 동시 재시도

### CredentialStore (`pkg/worker/setup/credstore.go`, 119줄)

**현재 상태**: 완전히 구현됨, **refresher.go에서 미사용**

- 인터페이스: `Save(service, value)`, `Load(service)`, `Delete(service)`
- 키체인 우선 시도 → 실패 시 AES-256-GCM 파일 fallback
- `migratePlaintextCredentials()`: `DefaultCredentialsPath()` 파일을 읽어 CredentialStore에 저장 후 zero-fill + 삭제
- 서비스 키: `"autopus-worker"`

**통합 전략**: `TokenRefresher`가 `credentialsPath string` 대신 `CredentialStore` 인터페이스를 주입받도록 변경. 기존 마이그레이션 로직은 `NewCredentialStore()` 팩토리에서 자동 실행되므로 별도 처리 불필요.

### NetMonitor (`pkg/worker/net/monitor.go`, 86줄)

**현재 동작**:
- 5s 폴링으로 네트워크 인터페이스 주소 변경 감지
- `onChange(oldAddrs, newAddrs)` 콜백 호출
- `onValidate() error` 콜백으로 연결 유효성 검증

**연결 상태**: `loop_lifecycle.go:88-98`에서 onChange → `server.ReconnectTransport()` 직접 호출. TokenRefresher와 조율 없음.

### A2A Transport (`pkg/worker/a2a/ws_transport.go`, 223줄)

**재연결 관련**:
- `Reconnect(ctx)`: base=3s, factor=2, max=4 retries로 exponential backoff 이미 구현
- `Connect(ctx)`: Authorization header에 Bearer token 포함
- `SetAuthToken()`은 `Server` 레벨 (`a2a/server.go:133`)

**핵심 발견**: Transport는 자체 reconnect backoff가 있지만, reconnect 시점에 token이 만료되었으면 무의미하게 4회 실패한다. Token refresh가 선행되어야 한다.

### WorkerLoop 라이프사이클 (`pkg/worker/loop_lifecycle.go`, 136줄)

**서비스 시작 순서**: audit → auth(TokenRefresher) → knowledge → scheduler → net(NetMonitor) → poll

- L38-49: API Key 모드 체크 후 TokenRefresher 생성, `credentialsPath` 기반
- L88-100: NetMonitor 생성, onChange에서 직접 `server.ReconnectTransport()` 호출

## 설계 결정

### D1: Reconnector를 별도 타입으로 분리

**결정**: `reconnect.go`에 `Reconnector` 구조체를 새로 만든다.

**이유**: 재연결 조율은 TokenRefresher와 Transport 사이의 교차 관심사(cross-cutting concern)이다. refresher.go에 넣으면 a2a 패키지 의존성이 생기고, transport에 넣으면 auth 의존성이 생긴다. 별도 조율자가 두 컴포넌트를 느슨하게 연결한다.

**대안**: refresher에 `onBeforeReconnect` 콜백 추가 → 거부: 콜백 지옥이 되고, 재연결 순서 보장이 어려움.

### D2: CredentialStore를 인터페이스로 주입

**결정**: `NewTokenRefresher`에 `setup.CredentialStore` 인터페이스를 직접 전달.

**이유**: 
- CredentialStore 인터페이스가 이미 존재하고, Keychain/GCM fallback이 구현됨
- 테스트에서 mock 주입 용이
- `credentialsPath` 기반 코드를 완전히 제거하여 평문 파일 접근 경로 자체를 없앰

**대안**: adapter 패턴으로 `credentialsPath`를 CredentialStore로 감싸기 → 거부: 불필요한 복잡도, 평문 파일 경로가 코드에 남음.

### D3: Backoff 파라미터 (base=3s, factor=2, max=3)

**결정**: PRD Q&A에서 확인된 값 사용 — 3s → 6s → 12s (총 ~21s ≤ NFR-01 25s).

**이유**: 
- `http.Client{Timeout:10s}` 기준으로, 요청 실패 후 3s 대기가 서버 복구에 충분한 최소 간격
- factor=2는 업계 표준 exponential backoff
- jitter ±20%로 thundering herd 방지

### D4: 4xx 응답은 non-retryable로 분류

**결정**: HTTP 4xx (특히 401, 403)는 즉시 `onReauthNeeded` 호출, backoff 재시도하지 않음.

**이유**: 4xx는 클라이언트 오류이므로 재시도해도 결과가 동일. 401은 토큰 자체가 무효화된 것이므로 refresh_token으로 재시도가 무의미.

### D5: 마이그레이션 후 .bak 파일 72시간 유지

**결정**: PRD Risk 섹션의 롤백 전략 준수 — `.bak`으로 이동, 72시간 후 삭제.

**현재 코드 확인**: `credstore.go:103-106`에서 즉시 zero-fill + remove 수행. 이 동작을 72시간 지연 삭제로 변경해야 함. 단, 이 변경은 `setup` 패키지의 `migratePlaintextCredentials()`에 해당하므로 SPEC-WKAUTH-001 범위에서는 TokenRefresher가 CredentialStore를 사용하도록 변경하는 것이 핵심이고, 마이그레이션 정책 변경은 선택적 개선사항이다.

## 리스크

| 리스크 | 심각도 | 대응 |
|--------|--------|------|
| CredentialStore 주입으로 기존 `credentialsPath` 기반 코드와 호환 깨짐 | Medium | `CredentialStore`가 nil이면 기존 `credentialsPath` 기반 fallback 유지 (하위 호환) |
| 재연결 시 mutex deadlock 가능성 | Low | `Reconnector.mu`는 reconnect 시퀀스 전체를 감싸되, 내부에서 다른 lock 획득하지 않음 |
| jitter 범위가 NFR-01 25s를 초과할 가능성 | Low | 최악의 경우: (3+6+12)×1.2 = 25.2s → jitter 상한을 +15%로 조정하면 24.15s |
