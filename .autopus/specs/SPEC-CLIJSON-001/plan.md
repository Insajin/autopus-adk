# SPEC-CLIJSON-001 Plan: Stable Machine-Readable CLI Surfaces

## Implementation Strategy

Phase 1은 "모든 명령을 JSON화"가 아니라, 운영 가치가 큰 상태/진단 명령에 공통 envelope를 도입하는 것이다.

1. **Shared formatter**
   - envelope 타입과 writer를 공통 helper로 만든다
2. **Phase-1 command rollout**
   - `doctor`, `status`, `setup status`, `setup validate`, `telemetry summary/cost/compare`
3. **Compatibility alignment**
   - 기존 JSON-capable commands와 envelope alignment 방향을 문서화한다
4. **Contract tests**
   - command output snapshot과 envelope version tests를 추가한다

## Sync Outcome

2026-04-21 sync 기준 구현 결과:

- `internal/cli/output_json.go`에 shared formatter와 sanitizer를 도입했다.
- `doctor`, `status`, `setup status`, `setup validate`, `telemetry summary/cost/compare`에 공통 JSON mode를 연결했다.
- `permission detect`, `test run`, `worker status`를 같은 envelope로 정렬했고 `connect --headless`는 NDJSON compatibility metadata를 추가했다.
- fatal JSON path는 `jsonFatalError` + `root.go` 처리로 human stderr suffix 없이 종료되도록 정리했다.
- 검증은 `go test ./internal/cli ./pkg/connect` 와 `go test -race ./internal/cli -run 'Test(Doctor|Status)'` 기준으로 통과했다.

## File Impact Analysis

| 파일 | 작업 (생성/수정/삭제) | 설명 |
|------|---------------------|------|
| `internal/cli/doctor.go` | 수정 | JSON mode 지원 |
| `internal/cli/status.go` | 수정 | JSON mode 지원 |
| `internal/cli/setup.go` | 수정 | setup status/validate JSON surface 연결 |
| `internal/cli/telemetry.go` | 수정 | summary/cost/compare JSON output |
| `internal/cli/permission.go` | 수정 | shared envelope alignment |
| `internal/cli/test.go` | 수정 | existing JSON envelope alignment |
| `internal/cli/worker_commands.go` | 수정 | worker status alignment |
| `internal/cli/root.go` | 수정 | JSON fatal path stderr suppression |
| `internal/cli/output_json.go` | 생성 | shared JSON envelope/writer helper |
| `internal/cli/{doctor_json,doctor_json_platforms,doctor_json_checks,status_json,setup_json,telemetry_json,test_json,worker_status_json}.go` | 생성 | command-specific payload/check builder 분리 |
| `pkg/connect/headless_event.go` | 수정 | NDJSON compatibility metadata 추가 |
| `internal/cli/json_contract_test.go` | 생성 | shared contract/redaction/fatal-path regression tests |
| `internal/cli/{permission_test.go,test_test.go,test_coverage_test.go,test_profile_test.go}` | 수정 | aligned JSON surface regressions |

## Architecture Considerations

- CLI surface 표준화이므로 `internal/cli` 내부 shared helper로 시작하는 것이 현실적이다.
- domain payload 생성은 각 command가 계속 담당하고, envelope/writer만 공통화한다.
- JSON mode가 text mode 로직을 오염시키지 않도록 output assembly를 분리한다.
- `auto arch enforce` 기준 현재 아키텍처 규칙 위반은 없다.

## Tasks

- [x] 공통 JSON envelope 타입과 writer helper를 정의한다.
- [x] phase-1 명령별 payload 구조를 정리한다.
- [x] `doctor`와 `status`에 JSON mode를 우선 도입한다.
- [x] `setup`과 `telemetry` 계열에 JSON mode를 확장한다.
- [x] 기존 JSON-capable commands와 envelope alignment 방식을 문서화한다.
- [x] snapshot/contract tests를 추가한다.

## Risks & Mitigations

| 리스크 | 영향도 | 대응 |
|--------|--------|------|
| 명령별 payload shape가 제각각이라 공통 envelope가 얇아질 수 있음 | 중간 | envelope와 command-specific `data`를 분리 |
| text mode 유지 보수성이 떨어질 수 있음 | 중간 | text writer와 JSON writer를 분리 |
| exit code semantics 변경으로 automation 회귀가 생길 수 있음 | 높음 | 기존 fatal/non-fatal semantics 유지, JSON에 추가 정보만 보강 |
| 한번 공개한 schema를 오래 유지해야 하는 부담 | 중간 | `schema_version`과 additive change policy 도입 |

## Dependencies

- 내부:
  - `internal/cli/doctor.go`
  - `internal/cli/status.go`
  - `internal/cli/setup.go`
  - `internal/cli/telemetry.go`
  - `internal/cli/permission.go`
  - `internal/cli/test.go`
  - `internal/cli/worker_commands.go`
- 외부 참고:
  - GitHub CLI formatting/auth/watch contracts

## Exit Criteria

- [x] phase-1 supported commands emit stable JSON envelopes
- [x] stdout/stderr separation is covered by tests
- [x] text mode remains unchanged for non-JSON users
- [x] schema snapshots catch unintended breaking changes
