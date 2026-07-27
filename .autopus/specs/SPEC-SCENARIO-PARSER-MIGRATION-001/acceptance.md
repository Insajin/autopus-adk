# SPEC-SCENARIO-PARSER-MIGRATION-001 Acceptance

## Test Scenarios

### S1: Numeric and alphanumeric refs do not bleed
Priority: Must
Given `S1`, `S15A`, and `S-CANARY-1` headers
When scenarios are parsed and rendered
Then expected refs remain distinct and round-trip
And expected fields remain attached to their own header

### S2: Explicit active scenarios require executable fields
Priority: Must
Given an explicit active scenario or a scenario with missing status
When validation runs
Then expected explicit `Status: active`, non-empty Command, and at least one
supported Verify primitive are required
And missing values produce stable scenario/field reason codes
And missing status is invalid rather than implicit active

### S3: Warn quarantines invalid scenarios
Priority: Must
Given valid and invalid entries in default warn mode
When `auto test run` executes
Then expected invalid entries run no build or shell command
And expected valid active entries may run
And expected invalid count/reason codes appear in text and JSON diagnostics
And quarantined entries count only as `invalid`, not run/passed/skipped/failed
And zero runnable entries return exit zero with warning status and no PASS result

### S4: Enforce fails before execution
Priority: Must
Given at least one invalid entry
When `--scenario-validation=enforce` runs
Then expected command fails before runner/build/shell execution
And diagnostics equal warn mode diagnostics

### S5: Verification never defaults unknown to PASS
Priority: Must
Given exit-code, stdout, stderr, file, and unknown primitives
When evaluation runs
Then supported primitives execute their real checks
And expected unknown primitive fails validation and evaluation
And expected `exit_code(2)` fails when actual exit code is `0`

### S6: Compatibility parse and statuses remain
Priority: Must
Given valid active, deprecated, skip, and reference scenarios
When lenient parse/render and executable validation run
Then expected round-trip remains stable
And only valid active entries are runnable

## Edge Cases

### Edge Case 1: Malformed header follows valid scenario
Priority: Must
Given a malformed `### S...` header after a valid scenario
When parsing continues
Then expected its fields do not overwrite the valid scenario

### Edge Case 2: Duplicate field or reference
Priority: Must
Given repeated Command or repeated scenario ref/ID
When validation runs
Then expected deterministic duplicate reason code is emitted

### Edge Case 3: No valid active entries in warn mode
Priority: Must
Given only sparse invalid or non-runnable entries
When warn mode runs
Then expected no build/shell command executes
And expected diagnostics state zero runnable scenarios without fabricating PASS

## Diagnostic Wire Contract

Text diagnostics use one line per issue:
`<code> <scenario_ref> <field>`.

JSON diagnostics are emitted under:

```json
{
  "data": {
    "summary": {
      "passed": 0,
      "failed": 0,
      "skipped": 0,
      "invalid": 1,
      "total": 0
    },
    "diagnostics": [
      {
        "code": "scenario_missing_command",
        "scenario_ref": "S15A",
        "field": "Command",
        "line": 12
      }
    ]
  }
}
```

Warn and enforce compare the same sorted diagnostics value; only execution and
exit behavior differ.

## Oracle Acceptance Notes

- These Must scenarios assert concrete expected output: exact refs, exact reason
  codes, exact JSON fields and counts, and the `exit_code(2)` versus actual `0`
  result.
- Parser tests use numeric, alphanumeric, malformed, duplicate, and unknown
  field fixtures and assert exact issue codes.
- Runner tests prove supported primitives execute real checks and unknown
  primitives fail.
- CLI tests inject command/build sentinels to prove validation occurs before
  either boundary in warn and enforce modes.
- Existing valid parse/render tests remain compatibility oracles.

## Definition of Done

- [x] S1-S6 and Edge Case 1-3 PASS
- [x] focused/race/vet/build/SPEC/architecture/diff gates PASS
- [x] review/security findings resolved
- [x] strict-default debt explicit
