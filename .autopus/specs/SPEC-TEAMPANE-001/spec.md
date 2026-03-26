# SPEC-TEAMPANE-001: Agent Teams 터미널 패널 시각화

**Status**: completed
**Created**: 2026-03-26
**Revision**: 2
**Domain**: TEAMPANE

## 목적

Agent Teams(`--team`) 모드에서 각 팀원(Lead, Builder, Guardian)의 작업 진행 상황을 cmux/tmux 패널 분할로 실시간 시각화한다. 현재 `--multi` 모드에는 `pane_runner.go`를 통한 프로바이더별 패널 분할이 존재하지만, `--team` 모드에는 팀원별 시각화가 전혀 없어 사용자가 파이프라인 진행 상황을 파악하기 어렵다.

## 요구사항

### R1: 터미널 감지 및 패널 생성
- WHEN the `--team` flag is active, THE SYSTEM SHALL detect the available terminal multiplexer (cmux > tmux > plain) using the existing `DetectTerminal()` function.
- WHEN a cmux or tmux terminal is detected, THE SYSTEM SHALL create one pane per active teammate (Lead, Builder(s), Guardian) plus one dashboard pane.
- WHEN only a plain terminal is available, THE SYSTEM SHALL fall back to unified sequential output without pane splitting.

### R2: 팀원별 로그 스트리밍
- WHILE a teammate is executing its assigned phase, THE SYSTEM SHALL stream that teammate's log output to its dedicated pane in real time.
- WHERE a teammate's log pane exists, THE SYSTEM SHALL display the teammate's role name and current phase as a pane title prefix.
- THE SYSTEM SHALL use a unique log file naming convention: `autopus-team-{specID}-{role}.log` to prevent collisions.

### R3: 대시보드 패널
- WHEN the team pipeline starts, THE SYSTEM SHALL create a dashboard pane that displays phase transitions and all teammate statuses using the existing `RenderDashboard` box-drawing format.
- WHILE the pipeline is running, THE SYSTEM SHALL update the dashboard pane when phase or agent status changes occur.
- THE SYSTEM SHALL implement a width-aware compact rendering mode for narrow panes (< 38 chars).

### R4: 순차적 분할 레이아웃 (Sequential Split Layout)

Terminal 인터페이스의 `SplitPane(ctx, direction)` 은 현재 활성 패널만 분할하며, 특정 패널 ID 지정 분할을 지원하지 않는다. 따라서 그리드 레이아웃 대신 **순차적 분할(sequential split)** 전략을 사용한다.

#### 분할 방향

`terminal.Vertical` (상하 분할)을 사용한다. `terminal.go` 정의:
- `Horizontal` = 좌우(side by side)
- `Vertical` = 상하(top and bottom)

수직 스택 레이아웃이므로 **Vertical split**이 올바른 방향이다.

#### SplitPane 활성 패널 가정

`SplitPane(ctx, Vertical)`은 새로 생성된 패널의 ID를 반환하며, 이후의 SplitPane 호출은 가장 최근에 생성된(활성) 패널을 분할한다. 이 가정은 `pane_runner.go`의 `splitProviderPanes()`에서 동일하게 사용되고 있으며 cmux/tmux 양쪽에서 검증된 동작이다.

#### 분할 전략

`pane_runner.go`와 동일한 패턴으로 순차 Vertical split을 수행한다. 각 split은 현재 활성 패널의 하단에 새 패널을 생성한다.

- **3명 팀 (Lead + Builder + Guardian)**: 4개 패널
  ```
  ┌───────────────────────────┐
  │ Dashboard (initial pane)  │
  ├───────────────────────────┤
  │ Lead                      │
  ├───────────────────────────┤
  │ Builder                   │
  ├───────────────────────────┤
  │ Guardian                  │
  └───────────────────────────┘
  Split: Vertical × 3
  ```

- **4명 팀 (Lead + 2 Builders + Guardian)**: 5개 패널
  ```
  ┌───────────────────────────┐
  │ Dashboard (initial pane)  │
  ├───────────────────────────┤
  │ Lead                      │
  ├───────────────────────────┤
  │ Builder-1                 │
  ├───────────────────────────┤
  │ Builder-2                 │
  ├───────────────────────────┤
  │ Guardian                  │
  └───────────────────────────┘
  Split: Vertical × 4
  ```

- **5명 팀 (Lead + 3 Builders + Guardian)**: 6개 패널
  ```
  Split: Vertical × 5 (same pattern)
  ```

#### 레이아웃 고정 원칙
- THE SYSTEM SHALL create the layout once at pipeline start based on the initial team composition.
- THE SYSTEM SHALL NOT dynamically add or remove panes mid-pipeline. If a teammate is spawned after layout creation, it shares the Lead pane's log.
- Late-spawned teammates (e.g., builder-2, annotator) SHALL be logged to the Lead pane with a role prefix instead of creating new panes.

### R5: 정리(Cleanup)
- WHEN the pipeline completes (success or failure), THE SYSTEM SHALL close all teammate panes and the dashboard pane, and remove temporary log files.
- WHEN an individual teammate fails, THE SYSTEM SHALL send a failure indicator message to that pane (`echo "[FAILED] {role}: {error}"`).
- **cmux**: Individual pane close via `close-surface` is supported. Failed panes remain open for inspection until pipeline Close().
- **tmux limitation**: `TmuxAdapter.Close()` only supports session-level kill. Individual pane preservation on failure is NOT supported on tmux. THE SYSTEM SHALL document this as a known limitation and close all panes together on pipeline completion.

### R6: 기존 모니터링과의 호환성
- THE SYSTEM SHALL NOT modify the existing `MonitorSession` behavior for non-team pipelines.
- THE SYSTEM SHALL extend (not replace) the `MonitorSession` pattern for team-mode pane management.
- THE SYSTEM SHALL define a `PipelineMonitor` interface that both `MonitorSession` and `TeamMonitorSession` implement, enabling the pipeline code to use a single injection point.

### R7: tmux 지원
- WHEN tmux is the detected terminal (cmux not available), THE SYSTEM SHALL support the same sequential split layout and log streaming functionality as cmux mode.
- **Known limitations on tmux**:
  - Individual pane close not supported (R5 degradation — see above)
  - Session-level cleanup only on Close()

### R8: CLI 통합 지점

- WHEN the pipeline's `go` subcommand is invoked with `--team`, THE SYSTEM SHALL instantiate `TeamMonitorSession` instead of `MonitorSession`.
- The branching point is in the main session's pipeline orchestrator (not in CLI flag parsing, which is handled by the harness skill).
- THE SYSTEM SHALL pass the detected `Terminal` instance and team composition (role list) to `NewTeamMonitorSession()`.
- THE SYSTEM SHALL wire `EventAgentSpawn` / `EventAgentDone` pipeline events to `TeamMonitorSession.UpdateTeammate()`.

## 생성 파일 상세

### `autopus-adk/pkg/pipeline/monitor.go` (기존 파일에 추가)
- `PipelineMonitor` 인터페이스: `Start()`, `UpdateAgent(name, status)`, `Close()`, `LogPath()` — `MonitorSession`과 `TeamMonitorSession` 공통
- 기존 `MonitorSession`은 이미 이 메서드들을 가지므로 인터페이스 선언만 추가 (구현 변경 없음)

### `autopus-adk/pkg/pipeline/team_monitor.go` (~180 lines)
- `TeamMonitorSession` 구조체: `terminal.Terminal`, 팀원별 pane 관리, 로그 파일
- `NewTeamMonitorSession(specID, term, teammates []string)`: 팀 구성 정보를 받아 세션 생성
- `Start()`: 순차적 Horizontal split으로 패널 생성 + `tail -f` 스트리밍
- `UpdateAgent(name, status string)`: PipelineMonitor 인터페이스 구현 — 대시보드 갱신
- `Close()`: 모든 팀원 패널 + 대시보드 정리 + 로그 파일 삭제
- `isMultiplexer()`: `term.Name() != "plain"` (cmux + tmux 지원)

### `autopus-adk/pkg/pipeline/team_layout.go` (~120 lines)
- `LayoutPlan` 구조체: 패널 분할 순서 (role 이름 목록)
- `planLayout(teammates []string)`: 팀원 목록을 기반으로 split 순서 결정 (dashboard를 초기 패널로 사용)
- `applyLayout(ctx, term, plan)`: `LayoutPlan`을 순차적 `SplitPane(ctx, Horizontal)` 호출로 실행, 각 PaneID 수집
- Shell-escape 헬퍼: `pkg/orchestra/pane_shell.go`에서 필요한 함수를 export하여 공유 (별도 PR로 선행, 또는 `pkg/shellutil/` 공통 패키지 추출)

### `autopus-adk/pkg/pipeline/team_pane.go` (~100 lines)
- `TeammatePaneInfo` 구조체: role, PaneID, logPath
- `createTeammatePanes(ctx, term, layout)`: applyLayout + 각 패널에 `tail -f {logPath}` 전송
- `streamToPane(ctx, term, paneID, logPath)`: `SendCommand()` 로 tail -f 실행
- `cleanupTeammatePanes(term, panes)`: 패널 닫기 + 로그 파일 삭제
- Log file naming: `os.CreateTemp("", "autopus-team-{specID}-{role}-")`

### `autopus-adk/pkg/pipeline/team_dashboard.go` (~100 lines)
- `TeamDashboardData` 구조체: 기존 `DashboardData` 확장, `TeammateStatuses []TeammateStatus`
- `TeammateStatus`: role, phase, status, icon
- `RenderTeamDashboard(data TeamDashboardData, maxWidth int)`: 폭 인식 렌더링 (maxWidth < 38이면 compact 모드)
