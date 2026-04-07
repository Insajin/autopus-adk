# SPEC-CONNECT-002 구현 계획

## 태스크 목록

- [ ] T1: `pkg/connect/headless_event.go` — NDJSON 이벤트 타입 정의 및 출력 헬퍼
- [ ] T2: `pkg/connect/device_oauth.go` — 서버 프록시 device code flow 클라이언트 (request + poll)
- [ ] T3: `internal/cli/connect_headless.go` — headless 모드 3-step 흐름 구현
- [ ] T4: `internal/cli/connect.go` 수정 — `--headless`, `--timeout` 플래그 추가 및 분기
- [ ] T5: `pkg/connect/device_oauth_test.go` — device code flow 단위 테스트
- [ ] T6: `internal/cli/connect_headless_test.go` — headless 흐름 통합 테스트

## 구현 전략

### 기존 코드 활용

- **서버 인증**: `pkg/worker/setup/auth.go`의 `RequestDeviceCode()`, `PollForToken()` 그대로 재사용. headless에서는 `OpenBrowser()` 호출만 스킵.
- **connect.AuthenticateServer()**: `AuthDeps` 인터페이스로 DI 가능 — headless용 deps 구현체에서 `OpenBrowser()`를 no-op으로 오버라이드.
- **워크스페이스 조회**: `connect.Client.ListWorkspaces()` 그대로 사용.
- **토큰 저장**: `connect.Client.SubmitToken()` 서버 프록시 방식에서는 불필요 (서버가 직접 저장). 로컬 credential은 Step 1 인증 토큰만 저장.

### OpenAI Device Code Flow — 서버 프록시 패턴

OpenAI는 표준 device code flow를 지원하지 않으므로, Autopus 서버가 중간자 역할:

1. CLI → 서버: `POST /ai-oauth/device-code` → 서버가 device code + verification URL 발급
2. 사용자가 verification URL에서 OpenAI OAuth 완료 (서버 측 redirect_uri로 콜백)
3. CLI → 서버: `POST /ai-oauth/device-token` (polling) → 서버가 OAuth 완료 확인 후 성공 응답

이 패턴은 `pkg/worker/setup/auth.go`의 device code polling과 동일한 구조이므로 동일한 polling 로직을 추상화하여 재사용.

### 변경 범위

- `connect.go`: 플래그 2개 추가 + if/else 분기 1개 (최소 변경)
- 신규 파일 4개: 각각 150줄 이하 목표
- 기존 인터랙티브 모드: 코드 변경 없음
