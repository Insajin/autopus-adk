# SPEC-WKAUTH-001: Worker 인증 및 재연결 안정화

**Status**: completed
**Created**: 2026-04-09
**Domain**: WKAUTH
**Parent**: SPEC-WKSTAB-001 (umbrella)
**Module**: autopus-adk

## 목적

ADK Worker의 TokenRefresher가 현재 단일 HTTP 시도(10s timeout) 실패 시 즉시 `onReauthNeeded`를 호출하여 Worker 강제 재인증과 진행 중 태스크 중단을 유발한다. 또한 SPEC-OSSSEC-001에서 구현된 CredentialStore(macOS Keychain / libsecret / AES-256-GCM)가 존재함에도 refresher.go가 여전히 평문 JSON 파일을 사용하고, NetMonitor와 TokenRefresher가 독립적으로 동작하여 서버 재배포 시 토큰+WebSocket 동시 복구가 불가능하다.

이 SPEC은 세 가지 문제를 해결한다:
1. Token refresh에 exponential backoff 적용으로 일시적 네트워크 오류 내성 확보
2. CredentialStore 통합으로 평문 토큰 파일 제거
3. NetMonitor↔TokenRefresher↔WebSocket 재연결 조율 메커니즘 구축

## 요구사항

### P0 — Must Have

| ID | Requirement |
|----|-------------|
| FR-AUTH-01 | WHEN TokenRefresher detects an expired or near-expiry token, THE SYSTEM SHALL attempt refresh with exponential backoff (base=3s, factor=2, max=3 retries, jitter=±20%) before triggering onReauthNeeded |
| FR-AUTH-02 | WHEN storing or reading credentials, THE SYSTEM SHALL use CredentialStore interface (`setup.CredentialStore`) instead of plain JSON file I/O (`os.ReadFile`/`os.WriteFile`) |
| FR-AUTH-03 | WHEN CredentialStore migration completes successfully (write + read-back verification), THE SYSTEM SHALL move existing plain-text credential files to `{path}.bak`, then auto-delete `.bak` after 72 hours |
| FR-AUTH-04 | WHEN a WebSocket connection is lost, THE SYSTEM SHALL execute a coordinated reconnection sequence in a single goroutine: (1) verify/refresh token via CredentialStore → (2) update Server auth token → (3) reconnect WebSocket with valid token |
| FR-AUTH-05 | WHEN NetMonitor detects a network interface change, THE SYSTEM SHALL trigger the coordinated reconnection sequence (FR-AUTH-04) instead of directly calling `server.ReconnectTransport` |

### P1 — Should Have

| ID | Requirement |
|----|-------------|
| FR-AUTH-10 | WHEN the coordinated reconnection sequence completes, THE SYSTEM SHALL resume any in-flight tasks by re-querying their status from the backend and restoring `TaskLifecycle` state |
| FR-AUTH-11 | WHEN token refresh fails after all retries (3 attempts with backoff), THE SYSTEM SHALL emit a structured error event (`auth.permanent_failure`) with failure reason, attempt count, and last error before triggering onReauthNeeded |

## 비기능 요구사항

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-01 | Token refresh 재시도 총 소요 시간 (3회 backoff + jitter 포함) | < 25초 |
| NFR-02 | 재연결 시퀀스 (토큰 refresh + WebSocket 재연결) 완료 시간 | < 30초 |
| NFR-06 | CredentialStore 읽기/쓰기 지연 (Keychain 포함) | < 500ms |

## 생성/수정 파일 상세

| 파일 | 변경 유형 | 역할 |
|------|-----------|------|
| `pkg/worker/auth/refresher.go` | **대폭 수정** | backoff 추가, CredentialStore 통합, 평문 I/O 제거 |
| `pkg/worker/auth/backoff.go` | **신규** | exponential backoff + jitter 유틸리티 |
| `pkg/worker/auth/reconnect.go` | **신규** | 조율된 재연결 시퀀스 (token refresh → WS reconnect) |
| `pkg/worker/loop_lifecycle.go` | **수정** | CredentialStore 주입, 재연결 조율 콜백 연결 |
| `pkg/worker/loop.go` | **수정** | LoopConfig에 CredentialStore 필드 추가 |
| `pkg/worker/net/monitor.go` | **수정 없음** | 기존 콜백 구조 유지, lifecycle에서 콜백만 교체 |

## 제약 사항

- 기존 `loop_lifecycle.go` 부팅 순서 (audit→auth→knowledge→scheduler→net→poll) 유지
- CredentialStore 인터페이스(`setup.CredentialStore`)를 그대로 사용, 인터페이스 변경 없음
- A2A WebSocket 프로토콜(`/ws/a2a`) 하위 호환 유지
- API Key 모드(`acos_worker_` prefix)는 token refresh 대상 아님 (기존 동작 유지)
