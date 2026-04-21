# SPEC-SETUP-003 Research: Preview-First Bootstrap and Onboarding Truth Sync

## Codebase Analysis

bootstrap/onboarding 관련 로직은 이미 넓게 존재하지만, preview-first contract와 truth-sync contract는 느슨하다.

### Target Files

| 파일 | 역할 | 변경 필요 |
|------|------|-----------|
| `internal/cli/update.go` | harness update 진입점 | 높음 |
| `internal/cli/setup.go` | setup generate/update CLI | 높음 |
| `internal/cli/connect.go` | connect interactive flow | 높음 |
| `internal/cli/init.go` | onboarding next-step UX | 중간 |
| `pkg/setup/engine.go` | generation/update core | 높음 |
| `pkg/setup/engine_status.go` | status helper | 중간 |
| `pkg/setup/workspace.go` | workspace detection | 중간 |
| `README.md` | onboarding truth surface | 높음 |
| `docs/README.ko.md` | onboarding truth surface | 높음 |

### Dependencies

관련 기존 SPEC:

- `SPEC-CONNECT-002`
- `SPEC-INITUX-001`
- `SPEC-SETUP-002`
- `SPEC-OSSUX-001`

관련 코드 관찰:

- `pkg/setup/workspace.go`는 workspace 감지를 이미 지원한다.
- `update`와 `setup`은 preview보다 apply 중심 UX에 가깝다.
- `connect`는 구현된 state machine보다 README 설명 범위가 더 넓다.

## Lore Decisions

`auto lore context`로 별도 출력된 lore는 없었다. changelog에서는 install/update/connect/doctor wording 및 bootstrap sequencing 복구가 반복적으로 나타난다.

## Architecture Compliance

`auto arch enforce` 결과 현재 아키텍처 규칙 위반은 없다.

## Key Findings

1. preview-first 기능은 새 플랫폼 기능이 아니라 기존 bootstrap core 위에 얹는 usability/reliability 계층이다.
2. workspace detection은 이미 존재하므로 repo-aware hints는 feasibility가 높다.
3. 가장 큰 신뢰성 문제는 "즉시 쓰기"와 "문서가 구현보다 앞서감"의 조합이다.
4. connect truth-sync는 flow 확장보다 먼저 필요하다.

## Recommendations

- preview/apply를 같은 change-set 모델로 묶어 drift를 최소화한다.
- README/help는 구현이 제공하는 state machine만 설명하도록 강제한다.
- meta workspace나 generated surface 혼합 저장소에서 source-of-truth 힌트를 우선적으로 노출한다.
- CI/agent에서도 preview가 deterministic하게 동작해야 한다.
