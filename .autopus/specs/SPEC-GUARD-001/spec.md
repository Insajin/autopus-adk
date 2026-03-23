# SPEC-GUARD-001: Harness Pipeline Guard System

**Status**: done
**Created**: 2026-03-23
**Domain**: GUARD

## 목적

AI 에이전트가 하네스 파이프라인(auto-router.md.tmpl) 실행 시 필수 단계를 누락하는 구조적 결함을 해결한다. 근본 원인 분석(P0 3건, P1 3건)에 기반한 3-Layer Defense 시스템을 도입하여, 에이전트가 파이프라인의 모든 필수 단계를 순서대로 실행하도록 강제한다.

## 배경

auto-router.md.tmpl에서 다음 구조적 결함이 확인됨:

- **P0-1**: Plan Step 3 조건부 분기가 산문으로 작성되어 에이전트가 Review Gate를 건너뜀
- **P0-2**: Phase 2.1 Worktree Merge가 소단계(Step 2.3.1)로 표기되어 선택사항으로 인식됨
- **P0-3**: Error Recovery 섹션이 Completion Guidance와 분리되어 에러 처리를 무시함
- **P1-1**: 재시도 제한이 3곳에 분산되어 횟수 추적 실패
- **P1-2**: Step과 Phase 번호 혼용으로 인지 혼란
- **P1-3**: MANDATORY/OPTIONAL 시각적 구분 없음

## 요구사항

### Layer 1: Post-Agent Continuation Marker

- **R1**: WHEN a spec-writer Agent returns in the plan subcommand, THE SYSTEM SHALL display a POST-AGENT continuation marker directing to the next required step (Step 3: Review Gate Decision).
- **R2**: WHEN a planner Agent returns in the go subcommand, THE SYSTEM SHALL display a POST-AGENT continuation marker directing to Gate 1: Approval.
- **R3**: WHEN executor Agent(s) return in the go subcommand, THE SYSTEM SHALL display a POST-AGENT continuation marker directing to Phase 2.1: Worktree Merge (not to validation).
- **R4**: WHEN a validator Agent returns in the go subcommand, THE SYSTEM SHALL display a POST-AGENT continuation marker directing to Phase 3: Testing.
- **R5**: WHEN a tester Agent returns in the go subcommand, THE SYSTEM SHALL display a POST-AGENT continuation marker directing to Gate 3: Coverage check.
- **R6**: WHEN a reviewer Agent returns in the go subcommand, THE SYSTEM SHALL display a POST-AGENT continuation marker directing to Gate 4 result handling.

### Layer 2: Pre-Completion Verification Checklist

- **R7**: WHEN the plan subcommand reaches Completion Guidance, THE SYSTEM SHALL first evaluate a mandatory Pre-Completion Verification checklist covering Steps 1-4 before displaying completion output.
- **R8**: WHEN the go subcommand reaches Completion Guidance, THE SYSTEM SHALL first evaluate a mandatory Pre-Completion Verification checklist covering all Phases, Gates, and Phase 2.1 merge before displaying completion output.
- **R9**: WHERE any checklist item was NOT evaluated, THE SYSTEM SHALL direct the agent back to the unevaluated step instead of proceeding to completion.

### Layer 3: GATE/REQUIRED Markers

- **R10**: THE SYSTEM SHALL mark all mandatory pipeline steps with `[REQUIRED]` tag.
- **R11**: THE SYSTEM SHALL mark all conditional branch points with `[GATE]` or `[CONDITIONAL]` tag.
- **R12**: WHERE a step is intentionally skipped due to a condition being false, THE SYSTEM SHALL mark it with `[INTENDED SKIP]` tag.

### P0 Fixes

- **R13**: THE SYSTEM SHALL render Plan Step 3 as an IF-THEN diagram with explicit branch paths for both true and false conditions (P0-1).
- **R14**: THE SYSTEM SHALL promote Phase 2.1 Worktree Merge from "Step 2.3.1" to a top-level `[REQUIRED] Phase 2.1` with a CHECKPOINT marker before Gate 2 (P0-2).
- **R15**: WHEN executor results are displayed, THE SYSTEM SHALL show "Phase 2.1 Worktree Merge (merge required)" as the next step instead of "validation" (P0-2).
- **R16**: THE SYSTEM SHALL relocate Error Recovery blocks from a separate section to inline positions directly after each respective Gate (P0-3).

### P1 Fixes

- **R17**: THE SYSTEM SHALL consolidate all retry limits into a single reference table at the top of the go pipeline section (P1-1).
- **R18**: THE SYSTEM SHALL apply `[REQUIRED]` and `[GATE]` markers to all steps and gates throughout the template (P1-3).

## 생성 파일 상세

| 파일 | 역할 |
|------|------|
| `templates/claude/commands/auto-router.md.tmpl` | 주요 변경 대상. 3-Layer Defense 적용, P0/P1 수정 |
| `.claude/commands/auto.md` | 로컬 렌더링 버전. 템플릿 변경 후 동기화 |

## 변경 범위

- Go 코드 변경 없음 (harness-only)
- `.md` 및 `.md.tmpl` 파일만 수정
- 기능 동작 변경 없음 (에이전트 지시 텍스트 구조 개선)
