# SPEC-SCENARIO-PARSER-MIGRATION-001: Executable scenario parser migration

---
id: SPEC-SCENARIO-PARSER-MIGRATION-001
title: Executable scenario parser migration
version: 0.1.0
status: completed
priority: HIGH
---

## Purpose

Stop malformed scenarios from reaching build/shell execution while providing a
warn-to-enforce migration path for existing sparse indexes.

## Outcome Boundary

- Outcome Lock: invalid scenarios are diagnosed and quarantined by default and
  rejected in opt-in enforce mode before build or shell execution.
- Mandatory: alphanumeric refs, stable issues, active field/status validation,
  duplicate detection, supported primitive validation, warn/enforce modes.
- Non-goals: data migration, strict-default promotion, inferred bodies, and
  validation of top-level `Build` command contents.
- Completion evidence: S1-S6 and Edge Case 1-3.

## Requirements

### Ubiquitous

REQ-REF-001: THE SYSTEM SHALL parse and render numeric and alphanumeric `S...`
references without attaching unrecognized header fields to a previous scenario.
`Scenario.Ref` preserves the exact reference, `Scenario.Number` remains the
numeric compatibility projection, and render, sync, and CLI consumers use
`DisplayRef()`.

REQ-VALIDATE-001: THE SYSTEM SHALL emit stable structured issues for missing
command, missing/unsupported verification, invalid status, duplicate ref/ID,
duplicate field, and unknown field.

REQ-DIAGNOSTIC-001: THE SYSTEM SHALL use exact reason codes
`scenario_missing_status`, `scenario_invalid_status`,
`scenario_missing_command`, `scenario_missing_verify`,
`scenario_unsupported_verify`, `scenario_duplicate_ref`,
`scenario_duplicate_id`, `scenario_duplicate_field`,
`scenario_unknown_field`, and `scenario_malformed_header`.

REQ-ACTIVE-001: THE SYSTEM SHALL classify only explicit valid `active`
scenarios as runnable; `deprecated`, `skip`, and `reference` do not run.

REQ-ROUNDTRIP-001: THE SYSTEM SHALL preserve valid parse/render/parse behavior
and the existing lenient parser for non-execution consumers.

### Event-Driven

REQ-WARN-001: WHEN validation mode is `warn` THEN THE SYSTEM SHALL quarantine
invalid entries before build/shell execution, emit reason codes/counts, and
preserve compatibility exit behavior. Quarantined entries count only as
`invalid`, not run/passed/skipped/failed; zero runnable entries return exit zero
with warning status and no fabricated PASS result.

REQ-ENFORCE-001: WHEN validation mode is `enforce` and any invalid runnable
entry exists THEN THE SYSTEM SHALL fail before build/shell execution.

REQ-VERIFY-001: WHEN a supported primitive is evaluated THEN THE SYSTEM SHALL
execute its real check; an unsupported primitive never defaults to PASS. The
supported grammar is `exit_code(<integer>)`,
`stdout_contains(<JSON string>)`, `stderr_empty()`,
`file_exists(<JSON string>)`, and
`file_contains(<JSON string>, <JSON string>)`.

### Unwanted

REQ-NOINFER-001: IF command or verification is missing THEN THE SYSTEM SHALL
NOT infer, synthesize, or execute it.

REQ-NOBLEED-001: IF a header is malformed or unsupported THEN subsequent fields
do not overwrite a previously parsed scenario.

## Acceptance Criteria

The authoritative Must set is S1-S6 and Edge Case 1-3 in `acceptance.md`.

## Traceability Matrix

| Requirement | Tasks | Acceptance |
|---|---|---|
| REQ-REF-001, REQ-NOBLEED-001 | T1, T2 | S1, Edge 1 |
| REQ-VALIDATE-001, REQ-DIAGNOSTIC-001, REQ-ACTIVE-001 | T1, T2 | S2-S4, Edge 2 |
| REQ-WARN-001, REQ-ENFORCE-001 | T3, T4 | S3-S4, Edge 3 |
| REQ-VERIFY-001 | T1, T3 | S5 |
| REQ-ROUNDTRIP-001, REQ-NOINFER-001 | T2, T4 | S6 |

## Related SPECs

- `SPEC-CONTEXT-ENGINEERING-EVOLUTION-001`
- `SPEC-CONTEXT-ENGINEERING-001`

## Completion

- Status: completed
- Completed: 2026-07-27
- Acceptance: S1-S6 and Edge Case 1-3 PASS
- Guardians: reviewer APPROVE; shared security findings RESOLVED; AX scan
  completed
- Verification: focused/full affected packages, race, vet, build, strict SPEC,
  architecture, and diff gates PASS
- Completion Debt: strict-default promotion requires external inventory cleanup
