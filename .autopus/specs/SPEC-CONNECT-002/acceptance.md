# SPEC-CONNECT-002 수락 기준

## 시나리오

### S1: headless 모드 정상 흐름

- Given: Autopus 서버가 동작 중이고 device code flow를 지원하며, 유효한 workspace ID가 있음
- When: 에이전트가 `auto connect --headless --workspace ws_123` 실행
- Then: stdout에 NDJSON 형식으로 step별 이벤트가 출력됨
  - `{"step": "server_auth", "action": "login_required", "url": "...", "code": "...", ...}`
  - `{"step": "server_auth", "status": "success"}`
  - `{"step": "workspace", "status": "success", ...}`
  - `{"step": "openai_oauth", "action": "login_required", "url": "...", "code": "...", ...}`
  - `{"step": "openai_oauth", "status": "success"}`
  - `{"step": "complete", "status": "success", "workspace_id": "ws_123", "provider": "openai"}`
- And: exit code 0

### S2: headless 모드 workspace 플래그 누락

- Given: `--headless` 플래그가 설정됨
- When: `auto connect --headless` (--workspace 없이) 실행
- Then: `{"error": "headless mode requires --workspace flag"}` 출력 후 exit non-zero

### S3: headless 모드 타임아웃

- Given: headless 모드가 실행 중이고 사용자가 인증을 완료하지 않음
- When: `--timeout 30s`로 설정된 시간이 경과
- Then: 현재 진행 중인 step에서 `{"step": "...", "status": "error", "error": "timeout"}` 출력 후 exit non-zero

### S4: 잘못된 workspace ID

- Given: headless 모드 실행, 서버 인증 성공
- When: 존재하지 않는 workspace ID 전달
- Then: `{"step": "workspace", "status": "error", "error": "workspace not found"}` 출력 후 exit non-zero

### S5: 기존 인터랙티브 모드 비간섭

- Given: `--headless` 플래그 없음, stdin이 TTY
- When: `auto connect` 실행
- Then: 기존 3단계 인터랙티브 위자드가 변경 없이 동작

### S6: non-TTY에서 headless 미지정

- Given: stdin이 TTY가 아님 (파이프 등), `--headless` 없음
- When: `auto connect` 실행
- Then: `--headless` 플래그 사용을 안내하는 메시지 출력 후 exit non-zero

### S7: 서버가 device code flow 미지원

- Given: headless 모드 실행, 서버 인증 성공, workspace 확인 성공
- When: `/ai-oauth/device-code` 엔드포인트가 404 또는 501 반환
- Then: `{"step": "openai_oauth", "status": "error", "error": "server does not support device code flow"}` 출력 후 exit non-zero

### S8: 커스텀 타임아웃

- Given: headless 모드
- When: `auto connect --headless --workspace ws_123 --timeout 5m` 실행
- Then: 전체 흐름이 5분 타임아웃으로 동작
