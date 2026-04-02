# SPEC-OSSUX-001: Init Profile 분기 + 비침습 업셀 힌트

**ID**: SPEC-OSSUX-001
**Status**: completed
**Created**: 2026-04-02
**Module**: autopus-adk
**Ref**: BS-020, PRD (prd.md)

## Overview

Init Wizard에 Developer/Fullstack 사용 프로파일(usage profile) 선택을 추가하고, Developer 프로파일 유저에게 비침습적 플랫폼 힌트를 제공하는 SPEC. BS-020 Phase 1 범위만 포함한다.

## Requirements

### P0 — Must Have

R1: WHEN `auto init` is executed, THE SYSTEM SHALL present a "usage profile" step as the FIRST wizard step, with options "Developer" (개발 도구만: plan/go/sync) and "Fullstack" (+ Worker/Platform).

R2: WHEN the user selects "Developer" profile in the init wizard, THE SYSTEM SHALL skip any Worker-related configuration steps that may be added in future.

R3: WHEN the user completes the profile selection, THE SYSTEM SHALL write `usage_profile: developer` or `usage_profile: fullstack` to autopus.yaml under HarnessConfig.

R4: WHEN autopus.yaml has no `usage_profile` field, THE SYSTEM SHALL default to `developer` behavior (backward compatibility).

R5: WHEN `auto go` pipeline completes successfully (Phase 4 APPROVE or harness-only completion) AND `usage_profile` is `developer` AND `hints.platform` is not `false` AND it is the first successful `auto go` completion (tracked in `~/.autopus/state.json`, see design.md §3 for state file format), THE SYSTEM SHALL display a one-line platform hint after the pipeline completion output.

R6: WHEN `auto go` pipeline completes successfully AND `usage_profile` is `developer` AND `hints.platform` is not `false` AND the user has completed `auto go` successfully 3 or more times (tracked in `~/.autopus/state.json`) AND the second hint has not been shown, THE SYSTEM SHALL display a second and final platform hint.

R7: WHEN `auto config set hints.platform false` is executed, THE SYSTEM SHALL set `hints.platform: false` in autopus.yaml, permanently disabling all platform hints. The `auto config set` subcommand shall be implemented as a new Cobra command in `internal/cli/config_cmd.go` that loads autopus.yaml, modifies the target field, and writes back (see design.md §6).

R8: WHEN displaying a platform hint, THE SYSTEM SHALL format it as a single non-intrusive line: `💡 이 작업을 AI 에이전트 팀이 자동화할 수 있습니다 → autopus.co`

R9: WHEN `auto init` runs and autopus.yaml already contains a `usage_profile` value, THE SYSTEM SHALL pre-select the existing profile in the wizard step.

R10: WHEN the `--yes` flag is used with `auto init` (non-interactive mode), THE SYSTEM SHALL default `usage_profile` to `developer`.

### P1 — Should Have

R11: WHEN `auto init` detects that the user selected "Developer" profile, THE SYSTEM SHALL defer Worker/Orchestra provider detection until after profile selection to avoid unnecessary latency for Developer-only users.

### Deferred to SPEC-OSSUX-002

R-DEFERRED: WHILE `usage_profile` is "developer", THE SYSTEM SHALL dim or label Worker-related help entries as [optional] in `auto help` output. (Deferred: Cobra help dynamic modification requires architectural design — see review finding #3.)

## Non-Functional Requirements

NFR-1: Hint state (go_success_count, first_hint_shown, second_hint_shown) SHALL be stored in `~/.autopus/state.json` with project-specific keys (keyed by SHA-256 hash of absolute project path). State file format: `{"version": 1, "projects": {"<hash>": {"go_success_count": N, "first_hint_shown": bool, "second_hint_shown": bool}}}`. Directory `~/.autopus/` is created with `os.MkdirAll(dir, 0o755)` if absent. See design.md §3 for full specification.

NFR-2: Hint check overhead SHALL be < 5ms (single file read + JSON parse).

NFR-3: All new source files SHALL be under 300 lines.

NFR-4: Schema changes to autopus.yaml SHALL be backward compatible — missing fields use defaults.

## Technical Design Notes

- `usage_profile` field name (not `profile`) to avoid collision with existing `Profiles` (executor profiles) in HarnessConfig
- `HintsConf` nested struct for `hints.platform` config
- State file path: `~/.autopus/state.json` using `os.UserHomeDir()`
- State file format: `{"projects": {"<sha256-hash>": {"go_success_count": N, "first_hint_shown": bool, "second_hint_shown": bool}}}`
- Graceful degradation: if state file cannot be read/written, skip hints silently
- Init Wizard profile step is inserted BEFORE the existing language step via `buildStepList()` modification

## Out of Scope

- Help 계층화 / Worker entries dimming 구현 (R11은 P1, SPEC-OSSUX-002에서 본격 구현)
- CLI 브랜딩 변경 (SPEC-OSSUX-002)
- autonomy.level (SPEC-OSSUX-003)
- Telemetry / analytics
- Worker 기능 자체 변경
- Profile 전환 시 Worker setup wizard 자동 실행
