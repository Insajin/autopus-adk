# SPEC-WKPROC-001 수락 기준

## PID Lock

### S1: 정상 PID lock 획득
- Given: `~/.autopus/worker.pid` 파일이 존재하지 않음
- When: `auto worker start` 실행
- Then: `~/.autopus/worker.pid`가 생성되고 현재 프로세스 PID가 기록됨
- And: Worker가 정상 시작됨

### S2: 중복 실행 방지
- Given: Worker가 이미 실행 중이며 `~/.autopus/worker.pid`에 유효한 PID가 기록됨
- When: 두 번째 `auto worker start` 실행
- Then: "Worker already running (PID: {pid})" 오류 메시지 출력
- And: 두 번째 프로세스가 즉시 종료됨 (exit code != 0)

### S3: 정상 종료 시 PID lock 해제
- Given: Worker가 실행 중이며 PID lock을 보유
- When: SIGTERM 시그널 수신
- Then: `~/.autopus/worker.pid` 파일이 삭제됨
- And: Worker가 graceful shutdown 완료

### S4: Stale PID lock 감지 및 복구
- Given: `~/.autopus/worker.pid`에 존재하지 않는 PID(예: 99999)가 기록됨
- When: `auto worker start` 실행
- Then: "[worker] stale PID lock detected (PID: 99999), reclaiming" 경고 로그 출력
- And: 새로운 PID로 lock 파일 갱신
- And: Worker가 정상 시작됨

### S5: PID lock 획득 성능
- Given: `~/.autopus/` 디렉토리가 로컬 파일시스템에 존재
- When: PID lock 획득 시도
- Then: 100ms 이내에 완료됨

## Zombie Reaper

### S6: Zombie 프로세스 감지 및 수거
- Given: Worker가 실행 중이며 reaper goroutine이 활성
- When: Worker가 spawn한 subprocess가 zombie 상태가 됨 (exit했지만 wait되지 않음)
- Then: 30초 이내에 zombie가 감지되어 reap됨
- And: "[reaper] reaped zombie process (PID: {pid})" 로그 출력

### S7: 정상 subprocess는 reaper 대상 아님
- Given: Worker가 subprocess를 spawn하고 정상적으로 cmd.Wait()으로 수거
- When: reaper 주기 도래
- Then: 해당 subprocess에 대해 아무 동작 없음

## Daemon 설정

### S8: launchd plist 필수 필드
- Given: macOS 환경
- When: `auto worker install` (launchd) 실행
- Then: 생성된 plist에 다음 키가 포함됨:
  - `ProcessType` = `Background`
  - `KeepAlive` = `true`
  - `ThrottleInterval` = `10`
  - `StandardOutPath` = `~/.config/autopus/logs/autopus-worker.out.log`
  - `StandardErrorPath` = `~/.config/autopus/logs/autopus-worker.err.log`

### S9: systemd unit 로그 경로
- Given: Linux 환경
- When: `auto worker install` (systemd) 실행
- Then: 생성된 unit 파일에 `StandardOutput=journal` 또는 로그 파일 경로가 설정됨

## MCP SSE Transport (P1)

### S10: SSE 엔드포인트 응답
- Given: MCP 서버가 SSE transport를 활성화하여 실행 중
- When: `GET /mcp/sse` 요청
- Then: `Content-Type: text/event-stream` 응답
- And: SSE 연결이 유지됨

### S11: SSE를 통한 도구 호출
- Given: SSE 연결이 활성 상태
- When: JSON-RPC `tools/call` 메시지를 SSE 채널로 전송
- Then: 기존 stdio와 동일한 결과가 SSE 이벤트로 반환됨

### S12: stdio와 SSE 병렬 동작
- Given: MCP 서버 실행 중
- When: stdio 클라이언트와 SSE 클라이언트가 동시에 도구 호출
- Then: 두 클라이언트 모두 독립적으로 정상 응답 수신

## MCP Config Schema (P1)

### S13: 유효한 config 검증 통과
- Given: 올바른 형식의 `worker-mcp.json` 파일
- When: MCP 서버 시작
- Then: config 검증 통과, 서버 정상 시작

### S14: 잘못된 config 거부
- Given: 필수 필드 누락 또는 타입 불일치의 `worker-mcp.json`
- When: MCP 서버 시작 시도
- Then: 검증 오류 메시지 출력
- And: 서버 시작 실패 (fail-fast)

## Worker Status (P2)

### S15: 상세 status 보고
- Given: Worker가 실행 중이며 태스크를 처리 중
- When: `auto worker status` 실행
- Then: 출력에 다음 정보 포함:
  - PID
  - Uptime (e.g., "2h 15m")
  - Current task (task ID 또는 "idle")
  - Connection status ("websocket" 또는 "polling")
  - Last heartbeat timestamp

### S16: Worker 미실행 시 status
- Given: Worker가 실행되지 않음 (`~/.autopus/worker.pid` 없음 또는 stale)
- When: `auto worker status` 실행
- Then: "Worker is not running" 메시지 출력
