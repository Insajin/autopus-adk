# SPEC-INITUX-001: Init Quality Gate Wizard & Doctor 진단 강화

**Status**: completed
**Created**: 2026-03-24
**Domain**: INITUX

## 목적

`auto init` 실행 시 Quality Gate 관련 설정(quality mode, review gate, methodology)이 CLI에서 전혀 노출되지 않아, 사용자가 이 설정의 존재를 인지하지 못한 채 하드코딩된 기본값이 적용된다. 또한 init TUI가 기존 branded 컴포넌트(Banner, Step, SummaryBox)를 활용하지 않아 일관성이 부족하다. `auto doctor`에도 quality gate 진단이 누락되어 있다.

이 SPEC은 init에 branded TUI wizard를 도입하여 quality gate를 interactive하게 설정하고, orchestra 프로바이더를 자동 감지하며, doctor에 quality gate 진단 섹션을 추가한다.

## 요구사항

### P0 - Must Have

- **R1**: WHEN `auto init` is executed in interactive mode, THE SYSTEM SHALL prompt the user to select a quality mode from available presets (ultra/balanced).
- **R2**: WHEN `auto init` is executed, THE SYSTEM SHALL detect installed orchestra provider binaries (claude, codex, gemini) using `exec.LookPath()` and configure `spec.review_gate.providers` with only detected providers.
- **R3**: WHEN fewer than 2 orchestra providers are detected, THE SYSTEM SHALL set `spec.review_gate.enabled` to false and display a warning message.
- **R4**: WHEN `auto init` is executed, THE SYSTEM SHALL display the Autopus Banner at the start and a SummaryBox with all configured settings at completion.
- **R5**: WHEN `auto init` is executed, THE SYSTEM SHALL display step-by-step progress using `tui.Step()` for each major init phase (Language, Quality Gate, Platform Files, Gitignore).
- **R6**: WHEN `--yes` flag is provided to `auto init`, THE SYSTEM SHALL skip all interactive prompts and use `DefaultFullConfig()` values.

### P1 - Should Have

- **R7**: WHEN `auto init` is executed in interactive mode, THE SYSTEM SHALL prompt for methodology mode (tdd/none) and enforcement preference.
- **R8**: WHEN `auto doctor` is executed, THE SYSTEM SHALL include a "Quality Gate" diagnostic section checking: quality preset validity, review gate configuration consistency, and provider binary availability.
- **R9**: WHEN `--quality <value>` flag is provided to `auto init`, THE SYSTEM SHALL set the quality mode to the specified value without prompting.
- **R10**: WHEN `--no-review-gate` flag is provided to `auto init`, THE SYSTEM SHALL disable `spec.review_gate.enabled` without prompting.

### P2 - Could Have

- **R11**: WHEN init completes, THE SYSTEM SHALL suggest `auto doctor` as the next verification step.

## 생성 파일 상세

### 수정 파일

| 파일 | 현재 줄 수 | 역할 | 변경 범위 |
|------|-----------|------|----------|
| `internal/cli/init.go` | 183 | Init 오케스트레이션 | wizard 플로우 통합, --quality/--no-review-gate/--yes 플래그 추가, Step 진행 표시 |
| `internal/cli/prompts.go` | 145 | 인터랙티브 프롬프트 | `promptQualityMode()`, `promptReviewGate()`, `promptMethodology()` 추가 |
| `pkg/detect/detect.go` | 138 | 바이너리 감지 | `DetectOrchestraProviders()` 함수 추가 |
| `internal/cli/doctor.go` | 191 | Doctor 진단 | Quality Gate 진단 섹션 추가 |

### 신규 파일

| 파일 | 역할 |
|------|------|
| `internal/cli/tui/wizard.go` | SummaryTable, WizardHeader TUI 컴포넌트 |

### 파일 크기 제약

- `init.go`: wizard 플로우 통합 후에도 200줄 이내 유지 (프롬프트 로직은 prompts.go에 위임)
- `prompts.go`: 3개 프롬프트 함수 추가 시 ~220줄 예상 (300줄 이내)
- `doctor.go`: Quality Gate 섹션 추가 시 ~240줄 예상 (300줄 이내)
- `detect.go`: DetectOrchestraProviders 추가 시 ~170줄 예상
- `tui/wizard.go`: 신규 ~80줄 예상
