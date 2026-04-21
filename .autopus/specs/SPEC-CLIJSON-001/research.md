# SPEC-CLIJSON-001 Research: Stable Machine-Readable CLI Surfaces

## Codebase Analysis

현재 CLI는 명령별로 출력 표면이 나뉘어 있다. 핵심 운영 명령이 text-only인 반면, 일부 명령은 이미 JSON/NDJSON을 사용한다. 즉, "패턴이 없는 것"이 아니라 "부분적으로만 있는 것"이 문제다.

### Target Files

| 파일 | 역할 | 변경 필요 |
|------|------|-----------|
| `internal/cli/doctor.go` | health/diagnostic summary | 높음 |
| `internal/cli/status.go` | SPEC dashboard | 높음 |
| `internal/cli/setup.go` | setup status/validate surface | 높음 |
| `internal/cli/telemetry.go` | summary/cost/compare | 높음 |
| `internal/cli/permission.go` | existing JSON-capable reference | 중간 |
| `internal/cli/test.go` | existing JSON-capable reference | 중간 |
| `internal/cli/worker_commands.go` | existing `worker status --json` | 중간 |
| `internal/cli/connect_headless.go` | NDJSON precedent | 낮음 |

### Dependencies

현재 precedent:

- `connect --headless`: NDJSON
- `permission detect --json`
- `test run --json`
- `worker status --json`

Phase-1 gap:

- `doctor`
- `status`
- `setup` status/validate
- `telemetry` summary/cost/compare

외부 사례:

- GitHub CLI formatting: `--json`, `--jq`, `--template`
- `gh auth status --json hosts`
- `gh run watch --exit-status`

## Lore Decisions

`auto lore context`로 별도 출력된 lore는 없었다. changelog를 보면 CLI usability와 runtime wording parity 복구가 반복되었고, 출력 contract 표준화는 자연스러운 다음 단계다.

## Architecture Compliance

`auto arch enforce` 결과 현재 아키텍처 규칙 위반은 없다.

## Key Findings

1. partial JSON commands가 이미 존재하므로 기술적 feasibility는 높다.
2. 문제는 JSON capability 부재보다 공통 contract 부재다.
3. stdout/stderr 분리가 느슨하면 CI와 desktop surface에서 text scraping이 계속 남는다.
4. `schema_version`이 없으면 나중에 field drift를 제어하기 어렵다.

## Recommendations

- 먼저 top-level envelope를 고정하고 command-specific payload는 `data`로 격리한다.
- jq/template는 후속 단계로 미루고, phase-1은 stable JSON과 exit semantics 정리에 집중한다.
- snapshot/contract tests를 도입해 release마다 CLI schema drift를 바로 잡는다.

## Sync Notes

2026-04-21 sync에서 실제로 확인된 구현 포인트:

- shared formatter는 새 public package를 만들지 않고 `internal/cli/output_json.go`에 두어 command payload builder와 가까운 층에서 재사용했다.
- `connect --headless`는 단일 envelope로 바꾸지 않고 NDJSON streaming을 유지한 채 공통 metadata만 붙이는 compatibility wrapper를 택했다.
- fatal path는 JSON payload 출력 후 `jsonFatalError`를 반환하고 root command가 추가 human stderr line을 생략하는 방식으로 정리했다.
- 계약 테스트는 temp binary를 빌드해 실제 CLI surface를 실행하는 방식으로 고정해 in-process Cobra 호출 차이와 workdir 의존성을 피했다.
