# SPEC-CLIJSON-001 Acceptance: Stable Machine-Readable CLI Surfaces

## Test Scenarios

### AC-001: doctor emits stable JSON envelope

Given a project with configured doctor checks
When `auto doctor --json` is executed
Then stdout contains one valid JSON document with `schema_version`, `command`, `status`, and `data`
And no human banner, ANSI color codes, or prose-only diagnostics appear on stdout

### AC-002: status remains machine-readable with non-fatal findings

Given a workspace with draft and approved SPECs
When `auto status --json` is executed
Then the payload includes structured SPEC counts and entries
And non-fatal status findings are represented in JSON without requiring text parsing

### AC-003: telemetry compare preserves exit semantics

Given telemetry comparison completes with warning-level observations but no fatal invocation error
When `auto telemetry compare --json` is executed
Then the process preserves its command semantics
And the JSON payload carries the warning or degraded status explicitly

### AC-004: text mode remains unchanged

Given the same command is executed without `--json`
When the user runs `auto doctor`
Then the existing human-readable output remains available
And JSON-only fields are not leaked into text mode

## Edge Cases

### AC-005: sensitive values are redacted in JSON mode

Given a supported command observes credential state, home-directory paths, or environment-backed secrets
When the command emits JSON output
Then token-like values and secret-bearing fields are omitted or masked
And absolute home-directory paths are masked or converted to safe relative identifiers before serialization

### AC-006: unsupported command-specific field expansion

Given a future field is added to a command-specific payload
When an older consumer reads the payload
Then the common envelope still parses successfully because additive fields do not break the top-level contract

### AC-007: fatal configuration error in JSON mode

Given a command is invoked with invalid flags or configuration
When JSON mode is active
Then the process exits non-zero
And stdout or stderr clearly carries a machine-readable fatal error payload or a documented fatal error path

## Sync Verification

| Criterion | Sync Status | Evidence |
|-----------|-------------|----------|
| AC-001 | verified | `TestJSONContract_PhaseOneCommandsRequireEnvelopeSupport`, `TestJSONContract_CommonEnvelopeSupportsPhaseOneCommands` |
| AC-002 | verified | `runStatusJSON`, `buildStatusPayload`, `TestJSONContract_PhaseOneCommandsRequireEnvelopeSupport` |
| AC-003 | verified | `buildTelemetryComparisonWarnings`, `writeJSONResultAndExit`, `TestJSONContract_FatalErrorPathUsesJSONEnvelope` |
| AC-004 | verified | `runDoctorText`, `runStatusText`, `go test -race ./internal/cli -run 'Test(Doctor|Status)'` |
| AC-005 | verified | `sanitizeJSONString`, `maskHomePath`, `TestJSONContract_RedactsSecretsAndMasksHomePaths` |
| AC-006 | verified | `writeJSONEnvelope`, `assertCommonJSONEnvelope`, `TestJSONContract_ExistingJSONCommandsRequireEnvelopeAlignment` |
| AC-007 | verified | `jsonFatalError`, `internal/cli/root.go`, `TestJSONContract_FatalErrorPathUsesJSONEnvelope` |

## Definition of Done

- [x] phase-1 commands support JSON envelopes
- [x] stdout/stderr contract is deterministic
- [x] snapshot tests prevent accidental schema drift
- [x] non-JSON output remains backward compatible
- [x] sensitive values are redacted in JSON mode
