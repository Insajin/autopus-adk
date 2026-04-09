# SPEC-WKAUTH-001 수락 기준

## 시나리오

### S1: Token refresh with backoff on transient failure

- Given: TokenRefresher가 실행 중이고, 토큰 만료까지 5분 미만
- When: 첫 번째 refresh 요청이 네트워크 오류로 실패
- Then: 3s 후 재시도 (attempt 2), 실패 시 6s 후 재시도 (attempt 3), 실패 시 12s 후 재시도 (attempt 4)
- And: 재시도 간격에 ±20% jitter가 적용됨
- And: 총 소요 시간이 25초 미만 (NFR-01)

### S2: Token refresh succeeds on retry

- Given: 백엔드가 일시적으로 불가하고, 2번째 시도에서 복구
- When: TokenRefresher가 refresh를 시도
- Then: 1회 실패 → 3s backoff → 2회 시도 성공
- And: CredentialStore에 새 토큰이 저장됨
- And: `onTokenRefresh` 콜백이 새 토큰으로 호출됨

### S3: Permanent auth failure emits structured error

- Given: 백엔드가 401 Unauthorized를 반환 (non-retryable)
- When: TokenRefresher가 refresh를 시도
- Then: 즉시 `auth.permanent_failure` 이벤트가 발생 (FR-AUTH-11)
- And: 이벤트에 failure_reason, attempt_count, last_error가 포함
- And: `onReauthNeeded` 콜백이 호출됨
- And: 불필요한 backoff 재시도가 발생하지 않음

### S4: CredentialStore integration replaces plain JSON

- Given: 기존 평문 `credentials.json` 파일이 존재
- When: Worker가 시작되고 CredentialStore가 초기화됨
- Then: `credentials.json` 내용이 CredentialStore("autopus-worker")에 마이그레이션됨
- And: 원본 파일이 `.bak`으로 이동됨
- And: TokenRefresher의 Load/Save가 CredentialStore를 통해 동작
- And: 디스크에 평문 토큰 파일이 존재하지 않음

### S5: Coordinated reconnection on WebSocket loss

- Given: Worker가 A2A WebSocket으로 연결된 상태
- When: WebSocket 연결이 끊김
- Then: Reconnector가 순차적으로 실행: (1) token refresh → (2) SetAuthToken → (3) WebSocket reconnect
- And: 토큰이 유효한 경우 refresh를 건너뛰고 바로 WebSocket reconnect
- And: 전체 시퀀스가 30초 미만에 완료 (NFR-02)

### S6: NetMonitor triggers coordinated reconnection

- Given: Worker가 실행 중이고 네트워크 인터페이스가 변경됨 (Wi-Fi → LAN 전환 등)
- When: NetMonitor가 주소 변경을 감지
- Then: `server.ReconnectTransport()` 직접 호출 대신 `reconnector.Reconnect(ctx)` 호출
- And: 토큰 유효성 확인 후 WebSocket 재연결이 수행됨

### S7: Duplicate reconnection prevention

- Given: 재연결 시퀀스가 진행 중
- When: NetMonitor가 다시 네트워크 변경을 감지하여 두 번째 재연결 요청 발생
- Then: 두 번째 요청은 무시됨 (mutex로 방지)
- And: 첫 번째 재연결 시퀀스만 완료됨

### S8: API Key mode unchanged

- Given: Worker가 API Key 모드(`acos_worker_` prefix)로 실행 중
- When: Worker가 시작됨
- Then: TokenRefresher가 생성되지 않음 (기존 동작 유지)
- And: CredentialStore 마이그레이션이 실행되지 않음

### S9: Boot order preserved

- Given: Worker 라이프사이클이 시작됨
- When: `startServices(ctx)` 실행
- Then: 서비스 시작 순서가 audit→auth→knowledge→scheduler→net→poll 유지
- And: auth 단계에서 CredentialStore 기반 TokenRefresher가 시작됨
- And: net 단계에서 NetMonitor가 Reconnector 콜백으로 시작됨
