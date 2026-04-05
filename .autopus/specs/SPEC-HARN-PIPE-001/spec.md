# SPEC-HARN-PIPE-001: Go 바이너리 기반 플랫폼-무관 파이프라인 오케스트레이터

**Status**: completed
**Created**: 2026-04-05
**Domain**: HARN-PIPE
**Depends on**: SPEC-MULTIPLATFORM-001, SPEC-PARITY-001

## 목적

현재 ADK의 5-Phase 파이프라인(`agent-pipeline.md`)은 Claude Code의 `Agent()` 도구에 의존한다. Codex CLI와 Gemini CLI에는 네이티브 서브에이전트 스폰 메커니즘이 없어서 파이프라인이 동작하지 않는다. Go 바이너리가 각 Phase를 해당 플랫폼의 CLI를 subprocess로 호출하여 실행하면, 어떤 코딩 CLI에서든 동일한 파이프라인을 실행할 수 있다.

## 배경

- `pkg/orchestra/` — `ExecutionBackend` 인터페이스와 `subprocessBackend`가 이미 CLI subprocess 실행을 추상화하고 있음
- `pkg/adapter/` — `PlatformAdapter` 인터페이스와 claude/codex/gemini 어댑터가 각 플랫폼의 바이너리, 인자, 모델을 알고 있음
- `pkg/pipeline/` — `Checkpoint`, `Event`, `PipelineMonitor` 등 상태 관리 인프라가 이미 존재
- `autopus.yaml` — `OrchestraConf.Providers`에 바이너리/인자/모델이 정의됨
- `.claude/skills/autopus/agent-pipeline.md` — 5-Phase 구조 정의 (534줄, 스킬 레벨)

## 요구사항

### MUST (필수)

- **REQ-1**: WHEN the user runs `auto pipeline run SPEC-ID --platform codex`, THE SYSTEM SHALL execute the 5-Phase pipeline using the codex CLI subprocess for each phase.
- **REQ-2**: WHEN a phase completes, THE SYSTEM SHALL parse the subprocess stdout output and inject relevant results into the next phase's prompt.
- **REQ-3**: WHEN the `--platform` flag is omitted, THE SYSTEM SHALL default to the current platform (detected via `pkg/adapter` registry).
- **REQ-4**: THE SYSTEM SHALL support `--strategy sequential` (default) and `--strategy parallel` execution modes.
- **REQ-5**: WHEN `--strategy parallel` is used, THE SYSTEM SHALL execute independent phases (e.g., multiple executors in Phase 2) in parallel subprocess workers, each in an isolated git worktree.
- **REQ-6**: WHEN a quality gate (Gate 2: validation, Phase 4: review) returns FAIL, THE SYSTEM SHALL retry the preceding phase up to the configured retry limit (default: 3 for validation, 2 for review).
- **REQ-7**: THE SYSTEM SHALL persist checkpoint state to `.autopus/pipeline-state/{specID}.yaml` after each phase, enabling `--continue` resume.
- **REQ-8**: THE SYSTEM SHALL reuse the existing `ExecutionBackend` interface from `pkg/orchestra/backend.go` for subprocess execution.
- **REQ-9**: THE SYSTEM SHALL reuse provider configuration from `autopus.yaml` (`OrchestraConf.Providers`).

### SHOULD (권장)

- **REQ-10**: WHEN `--strategy parallel` is used with tmux/cmux, THE SYSTEM SHOULD display each executor subprocess in a separate terminal pane using `pkg/pipeline/MonitorSession`.
- **REQ-11**: THE SYSTEM SHOULD emit JSONL events (`pkg/pipeline/events.go`) for each phase transition, enabling external monitoring.
- **REQ-12**: THE SYSTEM SHOULD support a `--dry-run` flag that prints the prompts that would be sent to each phase without executing.
- **REQ-13**: WHEN running on Claude Code, THE SYSTEM SHOULD coexist with the existing `Agent()` based pipeline as an alternative mode (`--engine subprocess`), not replacing it.

### COULD (선택)

- **REQ-14**: THE SYSTEM COULD support cross-platform execution where different phases use different platforms (e.g., plan with Claude, implement with Codex).
- **REQ-15**: THE SYSTEM COULD integrate with the Context7 doc-fetch pipeline (Phase 1.8) by passing fetched documentation as prompt context.

### WON'T (범위 외)

- **REQ-16**: This SPEC will NOT implement a new GUI or web dashboard — CLI + terminal pane only.
- **REQ-17**: This SPEC will NOT replace the `Agent()` based pipeline on Claude Code — both will coexist.

## 생성 파일 상세

| 파일 | 패키지 | 역할 |
|------|--------|------|
| `pkg/pipeline/engine.go` | `pipeline` | `PipelineEngine` 인터페이스 및 `SubprocessEngine` 구현 |
| `pkg/pipeline/phase.go` | `pipeline` | Phase 정의 (`Plan`, `TestScaffold`, `Implement`, `Validate`, `Review`) |
| `pkg/pipeline/phase_prompt.go` | `pipeline` | Phase별 프롬프트 빌더 (결과 파싱 → 다음 Phase 주입) |
| `pkg/pipeline/phase_gate.go` | `pipeline` | Quality Gate 판정 로직 (PASS/FAIL/RETRY) |
| `pkg/pipeline/runner.go` | `pipeline` | 순차/병렬 실행 오케스트레이션 |
| `pkg/pipeline/worktree.go` | `pipeline` | 병렬 실행용 git worktree 관리 |
| `internal/cli/pipeline_run.go` | `cli` | `auto pipeline run` CLI 커맨드 |
| `internal/cli/pipeline_run_test.go` | `cli` | CLI 커맨드 테스트 |
