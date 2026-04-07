# SPEC-CONNECT-002: auto connect --headless 에이전트 모드

**Status**: draft
**Created**: 2026-04-07
**Domain**: CONNECT
**Scope**: Module (autopus-adk)
**Extends**: SPEC-CONNECT-001

## 목적

코딩 에이전트(Claude Code, Codex 등)는 브라우저를 열 수 없고 TUI 입력도 불가하다. 현재 `auto connect`의 3단계 인터랙티브 위자드는 에이전트 환경에서 동작하지 않는다. `--headless` 플래그를 추가하여 에이전트가 비대화형으로 OAuth 연결을 완수할 수 있도록 한다.

### 핵심 제약

- OpenAI는 표준 RFC 8628 Device Code Flow를 지원하지 않음 — authorization code + redirect_uri 기반만 지원
- 따라서 OpenAI OAuth는 Autopus 서버를 프록시로 활용하는 서버 측 device code flow가 필요함
- Autopus 서버 인증(Step 1)은 이미 device code flow 기반이므로 그대로 재사용

## 요구사항

### CLI 플래그 및 진입점

- REQ-HL-01: WHEN the user runs `auto connect --headless`, THE SYSTEM SHALL execute all 3 steps in non-interactive mode with JSON stdout output and no browser opening.
- REQ-HL-02: WHEN `--headless` is active, THE SYSTEM SHALL require `--workspace` flag (workspace selection prompt 불가).
- REQ-HL-03: WHEN `--headless` is active without `--workspace`, THE SYSTEM SHALL exit with a JSON error `{"error": "headless mode requires --workspace flag"}`.
- REQ-HL-04: WHEN `--headless` is active, THE SYSTEM SHALL use `--timeout` value (default 10 minutes) as the overall flow timeout.
- REQ-HL-05: WHEN `--timeout` flag is provided, THE SYSTEM SHALL use the specified duration (e.g., `--timeout 5m`, `--timeout 300s`).

### Step 1: Autopus 서버 인증 (headless)

- REQ-HL-10: WHEN headless mode begins Step 1, THE SYSTEM SHALL call `RequestDeviceCode()` and output a JSON event: `{"step": "server_auth", "action": "login_required", "url": "<verification_uri>", "code": "<user_code>", "expires_in": <seconds>}`.
- REQ-HL-11: WHEN headless mode outputs login_required, THE SYSTEM SHALL NOT open a browser.
- REQ-HL-12: WHEN headless Step 1 polling succeeds, THE SYSTEM SHALL output `{"step": "server_auth", "status": "success"}` and proceed to Step 2.
- REQ-HL-13: WHEN headless Step 1 polling times out or fails, THE SYSTEM SHALL output `{"step": "server_auth", "status": "error", "error": "<message>"}` and exit non-zero.

### Step 2: 워크스페이스 확인 (headless)

- REQ-HL-20: WHEN headless mode reaches Step 2, THE SYSTEM SHALL validate the `--workspace` ID against the server and output `{"step": "workspace", "status": "success", "workspace_id": "<id>", "workspace_name": "<name>"}`.
- REQ-HL-21: WHEN the workspace ID is invalid, THE SYSTEM SHALL output `{"step": "workspace", "status": "error", "error": "workspace not found"}` and exit non-zero.

### Step 3: OpenAI OAuth (headless — 서버 프록시 device code)

- REQ-HL-30: WHEN headless mode reaches Step 3, THE SYSTEM SHALL request a server-side device code for OpenAI OAuth via `POST /api/v1/workspaces/{wsID}/ai-oauth/device-code`.
- REQ-HL-31: WHEN the server returns a device code, THE SYSTEM SHALL output `{"step": "openai_oauth", "action": "login_required", "url": "<verification_uri>", "code": "<user_code>", "expires_in": <seconds>}`.
- REQ-HL-32: WHEN the server device code is issued, THE SYSTEM SHALL poll `POST /api/v1/workspaces/{wsID}/ai-oauth/device-token` with the device code until completion.
- REQ-HL-33: WHEN OpenAI OAuth completes via server proxy, THE SYSTEM SHALL output `{"step": "openai_oauth", "status": "success"}`.
- REQ-HL-34: WHEN OpenAI OAuth polling times out or fails, THE SYSTEM SHALL output `{"step": "openai_oauth", "status": "error", "error": "<message>"}` and exit non-zero.

### JSON 출력 프로토콜

- REQ-HL-40: WHILE `--headless` is active, THE SYSTEM SHALL output one JSON object per line (NDJSON) to stdout.
- REQ-HL-41: WHEN the entire flow completes successfully, THE SYSTEM SHALL output a final event: `{"step": "complete", "status": "success", "workspace_id": "<id>", "provider": "openai"}` and exit 0.
- REQ-HL-42: WHILE `--headless` is active, THE SYSTEM SHALL NOT write TUI formatting, spinners, or color codes to stdout.
- REQ-HL-43: WHILE `--headless` is active, THE SYSTEM SHALL write debug/progress logs to stderr only.

### 기존 모드 보호

- REQ-HL-50: WHEN `--headless` flag is absent, THE SYSTEM SHALL execute the existing interactive wizard without any behavioral change.
- REQ-HL-51: WHEN stdin is not a TTY and `--headless` is absent, THE SYSTEM SHALL print a hint suggesting `--headless` flag and exit non-zero.

## 생성 파일 상세

| 파일 | 역할 |
|------|------|
| `internal/cli/connect.go` | 수정: `--headless`, `--timeout` 플래그 추가, headless 분기 |
| `internal/cli/connect_headless.go` | 신규: headless 모드 전체 흐름 (3 steps, JSON output) |
| `pkg/connect/device_oauth.go` | 신규: 서버 프록시 device code flow (request + poll) |
| `pkg/connect/headless_event.go` | 신규: NDJSON 이벤트 타입 및 출력 헬퍼 |
| `internal/cli/connect_headless_test.go` | 신규: headless 모드 단위 테스트 |
| `pkg/connect/device_oauth_test.go` | 신규: device code flow 단위 테스트 |

## 서버 의존성 (범위 밖)

이 SPEC은 ADK CLI 측만 다룬다. 서버 측 엔드포인트(`/ai-oauth/device-code`, `/ai-oauth/device-token`)는 별도 SPEC(Autopus 백엔드)에서 구현해야 한다. CLI 구현 시 이 엔드포인트가 존재한다고 가정하며, 서버가 미구현 상태일 경우 `{"step": "openai_oauth", "status": "error", "error": "server does not support device code flow"}` 출력 후 종료한다.
