# Changelog — autopus-adk

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- **Orchestra Relay Pane Mode** (SPEC-ORCH-005): relay 전략에서 cmux/tmux pane 기반 인터랙티브 실행 지원
  - `pkg/orchestra/relay_pane.go` — 순차 pane relay 실행 엔진: SplitPane → 인터랙티브 실행 → sentinel 완료 감지 → 결과 수집 → 맥락 주입
  - `-p` 플래그 없이 프로바이더 CLI를 실행하여 전체 TUI/인터랙티브 기능 활용 가능
  - 이전 프로바이더 결과를 heredoc으로 다음 pane에 프롬프트 주입
  - 프로바이더 실패 시 skip-continue 처리 (SPEC-ORCH-004 REQ-3a 패턴 재사용)
  - `runner.go` relay pane fallback 경고 제거 — relay도 `RunPaneOrchestra`로 통합 라우팅
  - pane 라이프사이클 관리: 완료 후 defer로 모든 pane 및 임시 파일 정리
  - plain 터미널 환경에서는 기존 standard relay 실행으로 자동 fallback

- **Agent Teams Terminal Pane Visualization** (SPEC-TEAMPANE-001): `--team` 모드에서 팀원별 cmux/tmux 패널 분할 및 실시간 로그 스트리밍
  - `pkg/pipeline/team_monitor.go` — TeamMonitorSession: PipelineMonitor 인터페이스 구현, plain 터미널 graceful degradation
  - `pkg/pipeline/team_layout.go` — LayoutPlan: 순차적 Vertical split 전략, 3~5인 팀 지원
  - `pkg/pipeline/team_pane.go` — 팀원별 패널 생성/정리, tail -f 로그 스트리밍, shell-escape 보안
  - `pkg/pipeline/team_dashboard.go` — 폭 인식(width-aware) 대시보드 렌더링, compact 모드(< 38자)
  - `pkg/pipeline/monitor.go` — PipelineMonitor 인터페이스 추가 (MonitorSession + TeamMonitorSession 공통 계약)
  - SplitPane 실패 시 자동 cleanup 및 plain 터미널 폴백
  - tmux 지원 (개별 패널 닫기 미지원 제한사항 문서화)

- **Orchestra Agentic Relay Mode** (SPEC-ORCH-004): 프로바이더를 agentic one-shot 모드로 순차 실행하는 relay 전략
  - `pkg/orchestra/relay.go` — 릴레이 실행 로직, 프롬프트 주입, 결과 포맷팅
  - 프로바이더별 agentic 플래그 자동 매핑 (claude: `--allowedTools`, codex: `--approval-mode full-auto`)
  - 이전 프로바이더 분석 결과를 `## Previous Analysis by {provider}` 섹션으로 다음 프로바이더에 주입
  - 부분 실패 시 skip-continue 처리 (REQ-3a)
  - `--keep-relay-output` 플래그로 결과 파일 보존 옵션
  - `/tmp/autopus-relay-{jobID}/` 임시 디렉토리 관리

- **Orchestra Detach Mode** (SPEC-ORCH-003): pane 터미널(cmux/tmux) 감지 시 auto-detach 비동기 실행
  - `pkg/orchestra/job.go` — Job persistence model, status tracking, stale job GC
  - `pkg/orchestra/detach.go` — ShouldDetach() 판정, RunPaneOrchestraDetached() 진입점
  - `internal/cli/orchestra_job.go` — `auto orchestra status/wait/result` CLI 서브커맨드
  - `--no-detach` 플래그로 blocking 실행 강제 가능
  - REQ-11: 1시간 이상 된 abandoned job 자동 정리 (opportunistic GC)
