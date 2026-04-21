# SPEC-CLIJSON-001: Stable Machine-Readable CLI Surfaces

---
id: SPEC-CLIJSON-001
title: Stable Machine-Readable CLI Surfaces
version: 0.1.0
status: completed
priority: Must
---

## Purpose

`autopus-adk`는 이미 일부 명령에서 JSON/NDJSON을 제공하지만, 핵심 운영 명령의 출력 계약이 제각각이다. 이 SPEC의 목적은 주요 진단/상태 명령에 공통 machine-readable envelope를 도입해 CI, desktop, support tooling, agent chaining이 text scraping 없이 안정적으로 재사용하도록 만드는 것이다.

## Background

현재 상태:

- JSON/NDJSON 존재:
  - `connect --headless`
  - `permission detect --json`
  - `test run --json`
  - `worker status --json`
- text-only 중심:
  - `doctor`
  - `status`
  - `setup` 계열 상태/검증
  - `telemetry summary`, `telemetry cost`, `telemetry compare`

이 불일치는 자동화 경로를 약하게 만든다. 외부 CLI 사례도 공통 formatting contract의 가치를 보여준다.

- GitHub CLI: `--json`, `--jq`, `--template`
- `gh auth status --json`은 auth 상태를 구조화해도 동작 semantics를 유지
- `gh run watch --exit-status`는 watch 표면과 exit status를 명확히 연결

이번 SPEC은 jq/template 엔진 자체보다는 먼저 stable JSON envelope와 exit contract를 표준화한다.

## Implementation Snapshot

2026-04-21 sync 기준 실제 반영 범위:

- `internal/cli/output_json.go`에서 공통 JSON envelope(`schema_version`, `command`, `status`, `generated_at`, `data`)와 optional `warnings`, `checks`, `error`를 정의하고 redaction/home-path masking을 공통 처리한다.
- Phase-1 대상인 `doctor`, `status`, `setup status`, `setup validate`, `telemetry summary`, `telemetry cost`, `telemetry compare`에 `--json`/`--format json` 경로를 연결했다.
- 기존 JSON 표면인 `permission detect --json`, `test run --json`, `worker status --json`도 같은 envelope로 정렬했다.
- `connect --headless`는 line-delimited NDJSON을 유지하되 각 event에 `schema_version`, `command`, `generated_at`를 추가하는 compatibility wrapper를 적용했다.
- `internal/cli/root.go`는 `jsonFatalError`를 인식해 JSON fatal payload 뒤에 human `Error:` line을 덧붙이지 않도록 정리했다.
- `internal/cli/json_contract_test.go`와 기존 command tests를 통해 phase-1 rollout, 기존 JSON surface alignment, redaction, fatal path contract를 회귀 검증한다.

## Requirements

### Must

- **R1 - Common JSON Envelope**
  WHEN a supported command is executed with `--json` or `--format json`, THE SYSTEM SHALL emit a single stable JSON document containing at minimum `schema_version`, `command`, `status`, `generated_at`, and `data`.

- **R2 - Supported Commands in Phase 1**
  THE SYSTEM SHALL support the common JSON envelope for:
  - `auto doctor`
  - `auto status`
  - `auto setup status`
  - `auto setup validate`
  - `auto telemetry summary`
  - `auto telemetry cost`
  - `auto telemetry compare`

- **R3 - Stdout/Stderr Separation**
  WHEN JSON mode is active, THE SYSTEM SHALL write machine-readable payloads to stdout only. Human-oriented logs, progress, and warnings not represented in the JSON payload SHALL go to stderr.

- **R4 - Backward-Compatible Text Mode**
  WHEN `--json` and `--format json` are absent, THE SYSTEM SHALL preserve existing human-readable output behavior.

- **R5 - Exit Status Contract**
  WHEN JSON mode is active, THE SYSTEM SHALL preserve the command's existing fatal/non-fatal process semantics and also encode the outcome in the payload. Fatal invocation/configuration errors SHALL exit non-zero. Non-fatal diagnostic findings SHALL be represented in JSON without requiring text scraping.

- **R6 - Partial Success/Warn Surface**
  WHEN a command completes with warnings, skipped checks, or soft failures, THE SYSTEM SHALL include machine-readable `warnings` or `checks` entries instead of relying on terminal prose only.

- **R7 - Schema Versioning**
  THE SYSTEM SHALL version the JSON envelope and document changes as additive-by-default. Breaking changes SHALL require a schema version increment.

- **R8 - Shared Formatter Layer**
  THE SYSTEM SHALL implement the envelope/formatter logic in a shared internal package or helper rather than re-implementing JSON output independently per command.

- **R9 - Redaction and Safe Path Policy**
  WHEN JSON mode serializes diagnostic, setup, telemetry, or workspace state, THE SYSTEM SHALL redact or omit credential material, access tokens, cookie values, environment secrets, and other sensitive fields. The system SHALL also avoid exposing user-home absolute paths by default and SHALL prefer stable IDs, safe relative paths, or explicitly masked values.

### Should

- **R10 - Existing JSON Command Alignment**
  Existing JSON-capable commands (`connect --headless`, `permission detect --json`, `test run --json`, `worker status --json`) SHALL converge on the same envelope or an explicitly documented compatibility wrapper before the shared formatter contract is declared complete.

- **R11 - Machine-Readable Check Results**
  Commands that execute multiple checks (for example `doctor`) SHALL expose an ordered list of structured check results with check ID, severity, status, and detail.

- **R12 - Snapshot Contract Tests**
  The JSON schema for each supported command SHALL be covered by snapshot or contract tests that fail on unintended shape drift.

### Nice

- **R13 - Additional Formatting Layer**
  The system SHALL keep higher-level formatting helpers such as template or filter integration behind a later rollout gate until the JSON contract is stable.

## Acceptance Criteria

- [x] `AC-001` supported phase-1 commands emit the common JSON envelope
- [x] `AC-002` non-fatal findings remain machine-readable without parsing prose
- [x] `AC-003` exit semantics remain machine-readable without parsing prose
- [x] `AC-004` text mode remains backward compatible
- [x] `AC-005` JSON payloads redact secrets and mask unsafe absolute paths
- [x] `AC-006` additive field expansion preserves top-level compatibility
- [x] `AC-007` fatal configuration errors follow a documented machine-readable path

## Out of Scope

- built-in jq query engine 구현
- general-purpose Go template formatting DSL 구현
- every CLI command의 일괄 JSON화
- remote API schema 표준화
- telemetry storage schema 재설계

## Traceability

| Requirement | Test | Status |
|-------------|------|--------|
| R1 | AC-001 | implemented |
| R2 | AC-001 | implemented |
| R3 | AC-002 | implemented |
| R4 | AC-004 | implemented |
| R5 | AC-003, AC-007 | implemented |
| R6 | AC-002, AC-003 | implemented |
| R7 | AC-001, AC-006 | implemented |
| R8 | AC-001 | implemented |
| R9 | AC-005 | implemented |
| R10 | AC-001 | implemented |
| R11 | AC-001, AC-002 | implemented |
| R12 | AC-001, AC-006 | implemented |
| R13 | deferred follow-up formatting rollout | deferred |
