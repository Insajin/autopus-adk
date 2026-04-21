# ADK SPEC Snapshot — 2026-04-21

이 문서는 `autopus-adk/.autopus/specs/` 안의 열린 항목과 stale 문서를 빠르게 구분하기 위한 운영 메모다.

## Active

- `SPEC-CLIJSON-001`
  - 사용자 확인 기준 현재 구현 진행 중
  - 공통 JSON envelope, stdout/stderr 분리, phase-1 command coverage가 현재 활성 작업

## Next

- `SPEC-SETUP-002`
  - 실제 미구현 backlog
  - 현재 있는 monorepo `DetectWorkspaces()`를 대체하지 않고, nested git repo 감지와 cross-repo dependency mapping을 추가하는 확장 작업이다

## Later

- `SPEC-SESSCONT-001`
  - 실제 backlog지만 최적화 성격
  - 현재 pipeline phase session은 여전히 `pipeline-{taskID}-{phase}` 형식이라 문서와 코드가 일치한다

## Closed Or Rolled Up

- `SPEC-ADKSTUB-001`
  - `SPEC-ADKWIRE-003`로 rolled up
- `SPEC-ADKWIRE-001`
  - `SPEC-ADKWIRE-003`로 rolled up
- `SPEC-ADKWIRE-002`
  - `SPEC-ADKWIRE-003`로 rolled up
- `SPEC-CONNECT-002`
  - module-side CLI scope completed
- `SPEC-PATH-001`
  - module-local SPEC resolution contract completed
- `SPEC-PARITY-001`
  - completed sync
- `SPEC-SETUP-003`
  - completed
  - preview-first bootstrap preview/apply contract, repo-aware hints, connect truth-sync가 `v0.40.39` 기준 반영됐다

## Notes

- `rolled up`은 "예전 초안의 intent가 더 큰 통합 SPEC에 흡수되어, 같은 문서를 다시 집행 backlog로 보지 않는다"는 뜻이다.
- 새 구현을 시작할 때는 이 문서보다 각 SPEC 본문과 실제 코드 상태를 우선한다.
