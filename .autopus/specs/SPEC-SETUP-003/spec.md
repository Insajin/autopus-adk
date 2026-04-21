# SPEC-SETUP-003: Preview-First Bootstrap and Onboarding Truth Sync

---
id: SPEC-SETUP-003
title: Preview-First Bootstrap and Onboarding Truth Sync
version: 0.1.0
status: completed
priority: Must
---

## Purpose

`init`, `update`, `setup`, `connect`는 모두 onboarding과 bootstrap 품질을 좌우하지만, 현재는 즉시 쓰기와 부분적 truth drift가 남아 있다. 이 SPEC의 목적은 "먼저 보여주고, 그 다음 적용"하는 preview-first 흐름과, README/CLI/help가 실제 구현과 어긋나지 않는 truth-sync를 함께 정리하는 것이다.

## Background

현재 문제:

- `update`와 `setup generate/update`는 preview보다 즉시 반영에 가깝다.
- 메타 workspace와 generated surface가 섞인 저장소에서는 변경 영향 미리보기가 특히 중요하다.
- README의 `auto connect` 설명은 detect/configure/verify를 넓게 말하지만 실제 구현은 server auth + workspace + OpenAI OAuth 3단계에 더 가깝다.
- `pkg/setup/workspace.go`는 multi-workspace 감지를 이미 하지만 bootstrap UX는 single-dir 중심이다.

관련 기존 SPEC:

- `SPEC-CONNECT-002`
- `SPEC-INITUX-001`
- `SPEC-SETUP-002`
- `SPEC-OSSUX-001`

## Implementation Snapshot

2026-04-21 sync 기준 실제 반영 범위:

- `auto setup generate/update` 와 `auto update` 는 `--plan`, `--preview`, `--dry-run` 에서 no-write preview를 출력하고, tracked docs / generated surface / runtime state / config 분류를 함께 보여준다.
- `pkg/setup` 은 `BuildGeneratePlan`, `BuildUpdatePlan`, `ApplyChangePlan` 기반 reusable change-set contract를 사용하고, preview 이후 filesystem drift가 생기면 stale plan으로 재검증한다.
- repo-aware bootstrap hints 는 multi-repo/workspace 감지 결과를 재사용해 owning repo와 source-of-truth 문맥을 preview 출력에 포함한다.
- `auto connect status` 가 local readiness, next action, provider detect 결과를 deterministic하게 보여주고, `auto connect` help/README 문구는 실제 state machine(server auth → workspace → OpenAI OAuth)에 맞게 동기화됐다.

## Requirements

### Must

- **R1 - No-Write Preview Step**  
  WHEN the user invokes a preview-first flag (`--plan`, `--preview`, `--dry-run`, or command-specific diff preview), THE SYSTEM SHALL compute intended changes without writing files.

- **R2 - Preview Classification**  
  WHEN preview output is shown, THE SYSTEM SHALL classify prospective changes as tracked docs, generated surface files, runtime/state files, or config changes so the user can distinguish source-of-truth edits from generated artifacts.

- **R3 - Update Preview**  
  WHEN `auto update --plan` or equivalent preview mode is executed, THE SYSTEM SHALL show which files would be created, updated, skipped, or preserved before any write occurs.

- **R4 - Setup Preview**  
  WHEN `auto setup generate/update` is run in preview mode, THE SYSTEM SHALL show the target documents and the reason each file would change.

- **R5 - Connect Truth Sync**  
  WHEN `auto connect` help text, README, or docs describe the flow, THE SYSTEM SHALL reflect the actual implemented state machine and SHALL NOT imply capabilities that are not present in the current release.

- **R6 - Verification Step for Connect**  
  WHEN onboarding completes, THE SYSTEM SHALL provide a deterministic status/verify surface so the user can confirm what is configured and what still requires manual action.

- **R7 - Meta Workspace Awareness**  
  WHEN the current project is a meta workspace or multi-repo workspace, THE SYSTEM SHALL surface repo-aware hints indicating the owning repo or source-of-truth location before applying bootstrap changes.

- **R8 - Safe Apply Transition**  
  WHEN a preview is followed by apply, THE SYSTEM SHALL use the same computed change set or a revalidated equivalent so that preview/apply drift is minimized.

### Should

- **R9 - Repo-Aware Bootstrap Hints**  
  WHEN bootstrap flows present preview or apply guidance, THE SYSTEM SHALL reuse workspace detection results to explain whether the operation targets a single repo, multi-repo workspace, or generated surface.

- **R10 - README/CLI Truth Tests**  
  WHEN onboarding help text or README guidance changes, THE SYSTEM SHALL run tests or validation checks that prevent README/help text from drifting ahead of implementation.

- **R11 - Non-Interactive Preview Compatibility**  
  WHEN preview-first commands run in non-interactive and CI contexts, THE SYSTEM SHALL produce deterministic output without TUI dependencies.

### Nice

- **R12 - Connect Status Command**  
  WHEN the onboarding surface is expanded beyond the current verify step, THE SYSTEM SHALL prefer a dedicated `auto connect status` surface rather than overloading help text or setup summaries.

## Acceptance Criteria

- [x] `AC-001` preview modes perform no writes and classify tracked versus generated/runtime changes
- [x] `AC-002` setup preview explains target documents and change reasons before apply
- [x] `AC-003` connect docs/help match the actual state machine
- [x] `AC-004` meta workspace context is surfaced before risky bootstrap changes
- [x] `AC-005` preview/apply drift is detected and revalidated before writes
- [x] `AC-006` preview remains deterministic in non-interactive and CI contexts

## Out of Scope

- OpenAI OAuth flow 자체 재설계
- full multi-repo dependency mapping 확장
- all docs site IA rewrite
- telemetry/analytics 기반 onboarding experiments
- provider connection expansion beyond current scope

## Traceability

| Requirement | Test | Status |
|-------------|------|--------|
| R1 | AC-001 | implemented |
| R2 | AC-001 | implemented |
| R3 | AC-001 | implemented |
| R4 | AC-002 | implemented |
| R5 | AC-003 | implemented |
| R6 | AC-003 | implemented |
| R7 | AC-004 | implemented |
| R8 | AC-005 | implemented |
| R9 | AC-004 | implemented |
| R10 | AC-003 | implemented |
| R11 | AC-006 | implemented |
| R12 | AC-003 | implemented |
