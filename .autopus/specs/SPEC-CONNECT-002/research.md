# SPEC-CONNECT-002 리서치

## 기존 코드 분석

### Device Code Flow 인프라 (재사용 가능)

- `pkg/worker/setup/auth.go`
  - `GeneratePKCE()` — PKCE code_verifier/challenge 생성 (L48-58)
  - `RequestDeviceCode(backendURL, codeVerifier)` — device code 요청 (L61-94)
  - `PollForToken(ctx, backendURL, deviceCode, codeVerifier, interval)` — 폴링 (L124-153)
  - `DeviceCode` 구조체 — verification_uri, user_code, expires_in 포함 (L28-35)
  - `OpenBrowser(url)` — headless에서 스킵해야 할 대상 (L257-268)
  - `SaveCredentials(creds)` — `~/.config/autopus/credentials.json` 저장 (L271-292)

### Connect 패키지 (확장 대상)

- `pkg/connect/server_auth.go`
  - `AuthenticateServer(ctx, cfg, deps)` — `AuthDeps` 인터페이스로 DI 가능 (L111-147)
  - `AuthDeps` 인터페이스: `GeneratePKCE()`, `RequestDeviceCode()`, `PollForToken()`, `OpenBrowser()`, `SaveCredentials()` (L78-84)
  - headless용 AuthDeps: `OpenBrowser()`를 no-op으로 구현하면 기존 로직 재사용 가능
  - `Client` 구조체 + `ListWorkspaces()`, `SubmitToken()` (L57-177)

- `pkg/connect/oauth_openai.go`
  - `WaitForCallback()` — local callback server + browser open (L133-176)
  - headless에서는 이 함수 대신 서버 프록시 device code flow 사용
  - `OAuthResult` 구조체 재사용 가능 (L52-58)

### CLI 진입점 (수정 대상)

- `internal/cli/connect.go`
  - 현재 `--server`, `--workspace` 플래그만 정의 (L71-72)
  - `stepServerAuth()`, `stepWorkspaceSelect()`, `stepOAuth()`, `stepSubmitToken()` 4단계 (L76-158)
  - `connectTimeout = 5 * time.Minute` — headless에서는 10분 기본으로 별도 설정 필요

- `internal/cli/prompts.go`
  - `isStdinTTY()` — TTY 감지 (L18-20), non-TTY + non-headless 안내에 활용

### EnsureWorker 패턴 참고

- `pkg/worker/setup/ensure.go`
  - `EnsureResult` — `action`/`data` JSON 구조 (L11-14)
  - `ensureDeviceAuth()` — device code 요청 → JSON 즉시 출력 → 폴링 대기 패턴 (L67-119)
  - headless connect의 JSON 이벤트 출력 패턴과 매우 유사 — 동일 접근법 적용

## 설계 결정

### 결정 1: OpenAI OAuth를 서버 프록시 device code로 전환

**선택**: Autopus 서버가 device code를 발급하고, 서버 측에서 OpenAI OAuth redirect를 처리

**근거**:
- OpenAI는 RFC 8628 device code flow를 지원하지 않음 (authorization code + redirect_uri만 지원)
- 에이전트 환경에서 local callback server는 포트 접근 불가
- 서버 프록시 패턴은 이미 Autopus 서버 인증(Step 1)에서 검증된 방식

**대안 검토**:
- CLI가 직접 로컬 서버를 열고 사용자에게 URL을 알려주는 방식 → 에이전트가 브라우저를 열 수 없고 포트 포워딩이 보장되지 않아 부적합
- OpenAI에 device code flow 지원 요청 → 외부 의존성이라 통제 불가

### 결정 2: NDJSON (newline-delimited JSON) 출력 형식

**선택**: 한 줄에 하나의 JSON 객체 (NDJSON)

**근거**:
- 에이전트가 stdout을 라인 단위로 파싱하기 가장 쉬운 형식
- `EnsureWorker()`가 이미 동일 패턴 사용 (`fmt.Println(string(out))`)
- streaming 출력이므로 전체 완료 전에도 중간 상태를 에이전트가 파악 가능

**대안 검토**:
- 전체 결과를 하나의 JSON으로 출력 → 진행 중 상태를 알 수 없어 에이전트 UX 저하
- 별도 포맷 (Protocol Buffers 등) → 오버킬, 텍스트 파싱 불가

### 결정 3: headless에서 --workspace 필수

**선택**: `--headless` 시 `--workspace` 미지정이면 에러

**근거**:
- 워크스페이스 선택은 TUI prompt가 필수 (huh form)
- 에이전트는 사전에 workspace ID를 알고 있거나 `auto workspace list` 등으로 조회 가능
- 자동 선택(단일 workspace일 때)도 가능하지만, 명시적 지정이 에이전트 시나리오에서 더 예측 가능

### 결정 4: AuthDeps 인터페이스 활용한 browser skip

**선택**: 기존 `AuthDeps` 인터페이스에 headless deps 구현체 주입

**근거**:
- `AuthenticateServer()`가 이미 DI를 지원함 (`deps AuthDeps` 파라미터)
- `OpenBrowser()`를 no-op으로 오버라이드하면 기존 device code flow 전체를 수정 없이 재사용
- 테스트에서도 mock deps로 검증 용이
