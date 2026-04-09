# SPEC-WKPROC-001: Worker 프로세스 및 MCP 안정화

**Status**: completed
**Created**: 2026-04-09
**Domain**: WKPROC
**Parent**: SPEC-WKSTAB-001 (umbrella)
**Module**: autopus-adk

## 목적

ADK Worker는 PID lock, zombie 프로세스 감지, 데몬 설정 강화 등 프로세스 관리 기반이 부재하여 다중 인스턴스 충돌과 리소스 누수가 발생한다. MCP 서버는 stdio 전용이며 SSE transport와 config 스키마가 없어 원격 도구 호출이 불가능하다. 이 SPEC은 Worker 프로세스 안정성 확보와 MCP 서버 확장을 다룬다.

## 현재 상태 분석

| 영역 | 현재 | 문제 |
|------|------|------|
| PID 관리 | 없음 | Worker 중복 실행 가능, 충돌 시 orphan 잔존 |
| Zombie 감지 | 없음 | subprocess가 zombie로 남아 리소스 누수 |
| launchd plist | KeepAlive=true, RunAtLoad=true만 설정 | ProcessType, ThrottleInterval 미설정, 로그 경로 기본값 미흡 |
| systemd unit | Type=simple, Restart=always | 로그 리다이렉션 미설정 |
| MCP transport | stdio only (JSON-RPC over stdin/stdout) | 원격 MCP 도구 호출 불가 |
| MCP config | 하드코딩 | JSON Schema 검증 없음 |
| worker status | 데몬 설치 여부만 확인 | PID, uptime, 현재 태스크 등 미보고 |

## 요구사항

### P0 — Must Have

| ID | Requirement |
|----|-------------|
| FR-PROC-01 | WHEN `auto worker start` is executed, THE SYSTEM SHALL acquire a PID lock file (`~/.autopus/worker.pid`) containing the current process PID; IF the lock is held by a running process, THE SYSTEM SHALL exit with error "Worker already running (PID: {pid})" |
| FR-PROC-02 | WHEN the Worker process exits (normal shutdown via SIGTERM/SIGINT or crash), THE SYSTEM SHALL release the PID lock file by deleting `~/.autopus/worker.pid` |
| FR-PROC-03 | WHEN the Worker detects a stale PID lock (file exists but the process no longer exists), THE SYSTEM SHALL log a warning, reclaim the lock, and start normally |
| FR-PROC-04 | WHEN a subprocess spawned by the Worker becomes zombie (exited but not reaped by the parent), THE SYSTEM SHALL detect and reap it within 30 seconds via a periodic reaper goroutine |
| FR-PROC-05 | WHEN registering as a launchd service, THE SYSTEM SHALL generate a plist with `ProcessType=Background`, `KeepAlive=true`, `ThrottleInterval=10`, and stdout/stderr log redirection to `~/.config/autopus/logs/` |

### P1 — Should Have

| ID | Requirement |
|----|-------------|
| FR-PROC-10 | THE SYSTEM SHALL expose an SSE-based MCP transport endpoint (`/mcp/sse`) alongside the existing stdio transport, using the same tool/resource registry |
| FR-PROC-11 | THE SYSTEM SHALL define and validate MCP server configuration via JSON Schema, rejecting invalid config at startup |

### P2 — Could Have

| ID | Requirement |
|----|-------------|
| FR-PROC-20 | WHEN `auto worker status` is called, THE SYSTEM SHALL report: PID, uptime, current task (or idle), connection status (WebSocket/polling), last heartbeat timestamp |

## 비기능 요구사항

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-03 | PID lock 획득/해제 지연 | < 100ms |
| NFR-04 | Zombie 프로세스 감지 주기 | ≤ 30초 |

## 기술 제약

- Worker 라이프사이클: `loop_lifecycle.go`의 기존 부팅 순서(audit→auth→knowledge→scheduler→net→poll) 유지
- PID lock은 로컬 파일시스템(`~/.autopus/`) 한정, NFS 미지원
- MCP SSE는 기존 stdio 위에 추가, 기존 stdio transport 동작 변경 없음
- Go 1.26, 파일 크기 300줄 제한

## 범위 밖

- MCP SSE transport 전면 재설계 (기존 stdio 위에 SSE 엔드포인트 추가만)
- TUI 대시보드 (SPEC-ADKW-001 범위)
- Frontend 변경
