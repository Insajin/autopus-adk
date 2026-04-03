# SPEC-ONBOARD-001: 온보딩 UX 개선 — 비개발자 5분 성공 경험

**Status**: completed
**Created**: 2026-04-03
**Domain**: ONBOARD

## 목적

현재 autopus-adk의 온보딩 플로우(install.sh → auto init → auto worker setup)는 개발자 친화적 에러 메시지, 누락된 안내, 기술 용어 노출 등으로 인해 비개발자가 설치 과정에서 이탈하는 문제가 있다. 이 SPEC은 모든 사용자 대면 메시지를 사람이 이해할 수 있는 수준으로 개선하여, 비개발자도 5분 안에 첫 성공 경험을 할 수 있도록 한다.

## 요구사항

### P0 — Critical (이탈 방지)

- **R1**: WHEN install.sh가 지원하지 않는 OS/아키텍처를 감지하면, THE SYSTEM SHALL 현재 환경 정보와 함께 지원 OS 목록(macOS, Linux)을 표시하고, Windows 사용자에게는 WSL2 안내 링크를 제공해야 한다.
- **R2**: WHEN install.sh가 sudo 권한이 필요할 때, THE SYSTEM SHALL "시스템 폴더에 설치하기 위해 관리자 비밀번호가 필요합니다"라는 설명을 sudo 실행 전에 표시해야 한다.
- **R3**: WHEN install.sh가 sha256sum/shasum을 찾지 못하면, THE SYSTEM SHALL "다운로드 파일 무결성 검증 도구가 없습니다. macOS는 기본 포함이므로 터미널 재시작을 시도하세요."와 같은 행동 가능한 안내를 제공해야 한다.
- **R4**: WHEN worker setup의 Device Auth가 시작되면, THE SYSTEM SHALL 남은 시간을 카운트다운으로 표시해야 한다 (예: "인증 대기 중... (남은 시간: 4분 30초)").
- **R5**: WHEN 브라우저 자동 열기가 실패하면, THE SYSTEM SHALL URL 수동 복사 안내와 함께 "위 URL을 복사하여 브라우저에 붙여넣으세요"를 표시해야 한다.
- **R6**: WHEN Device Auth에서 PKCE 관련 에러가 발생하면, THE SYSTEM SHALL 기술 용어를 숨기고 "서버 인증 중 오류가 발생했습니다. 잠시 후 다시 시도해주세요."로 표시해야 한다.
- **R7**: WHEN 프로바이더가 미설치 상태이면, THE SYSTEM SHALL 설치 명령어와 공식 사이트 URL을 단계별로 안내해야 한다 (예: "1. https://claude.ai 에서 가입 → 2. npm install -g @anthropic-ai/claude-code → 3. claude login 실행").
- **R8**: WHEN 프로바이더 인증 안내에서 환경변수 설정이 필요할 때, THE SYSTEM SHALL "환경변수"라는 용어 대신 단계별 설정 방법을 안내해야 한다 (예: "터미널에 다음 명령어를 입력하세요: export OPENAI_API_KEY=여기에_키_입력").

### P1 — Important (혼란 방지)

- **R9**: WHEN auto init이 완료되면, THE SYSTEM SHALL "Next: run 'auto doctor'" 대신 워크플로우의 실제 다음 단계를 안내해야 한다 (worker setup이 필요하면 "Next: auto worker setup", 그렇지 않으면 "/auto plan 으로 첫 기능을 기획해보세요").
- **R10**: WHEN install.sh가 성공적으로 완료되면, THE SYSTEM SHALL next steps에 worker setup 안내를 포함해야 한다.
- **R11**: WHEN 에러 체인이 사용자에게 노출될 때, THE SYSTEM SHALL 기술적 에러 체인을 사람이 읽을 수 있는 단일 메시지로 변환해야 한다 (예: "authentication failed: request device code: HTTP 500" → "Autopus 서버에 연결할 수 없습니다. 인터넷 연결을 확인하고 다시 시도해주세요.").

### P2 — Polish

- **R12**: WHEN worker setup 명령어가 실행되면, THE SYSTEM SHALL Worker 개념 설명을 표시해야 한다 ("Worker는 Autopus 서버에서 작업을 받아 자동으로 실행하는 백그라운드 서비스입니다").
- **R13**: WHEN Device Auth 대기 중일 때, THE SYSTEM SHALL 매 폴링 주기마다 카운트다운 타이머를 갱신하여 진행 피드백을 제공해야 한다.

## 생성/수정 파일 상세

| 파일 | 역할 |
|------|------|
| `install.sh` | 사람이 읽을 수 있는 에러 메시지, sudo 설명, worker setup next steps 추가 |
| `internal/cli/worker_setup_wizard.go` | Device Auth 카운트다운 타이머, 사람 친화적 에러 메시지, URL 복사 힌트 |
| `internal/cli/worker_commands.go` | worker setup Long description에 Worker 개념 설명 추가 |
| `pkg/worker/setup/provider_auth.go` | 프로바이더별 단계별 설치/인증 가이드 (URL 포함) |
| `internal/cli/init.go` | post-init next steps 수정 (doctor → worker setup 또는 /auto plan) |
| `pkg/worker/setup/messages.go` (NEW) | 기술 에러 → 사용자 친화적 메시지 매핑 테이블 |

## 비목표 (Non-Goals)

- TUI 위젯/레이아웃 변경 (Bubbletea 컴포넌트 유지)
- 새로운 CLI 커맨드 추가
- 백엔드 API 변경
