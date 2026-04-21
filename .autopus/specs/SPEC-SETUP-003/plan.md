# SPEC-SETUP-003 Plan: Preview-First Bootstrap and Onboarding Truth Sync

## Implementation Strategy

이 SPEC은 bootstrap과 onboarding을 한 번에 재작성하지 않는다. 대신 "먼저 preview를 넣고, 그 다음 truth-sync를 맞춘다"는 두 단계로 간다.

1. **Preview-first rollout**
   - `update`, `setup generate/update`에 no-write preview를 추가
   - tracked/generated/runtime 분류를 함께 보여준다
2. **Truth-sync rollout**
   - `connect` help/README/summary를 실제 state machine에 맞춘다
   - verify/status surface를 추가하거나 기존 요약을 정형화한다
3. **Repo-aware hints**
   - workspace detection을 bootstrap UX에 재사용한다

## File Impact Analysis

| 파일 | 작업 (생성/수정/삭제) | 설명 |
|------|---------------------|------|
| `internal/cli/update.go` | 수정 | `--plan`/preview surface 진입점과 apply/preview 분기 |
| `internal/cli/update_preview.go` | 생성 | update preview 항목 계산과 분류 출력 |
| `internal/cli/update_config_preview.go` | 생성 | config no-write preview와 change reason 계산 |
| `internal/cli/setup.go` | 수정 | generate/update preview entrypoint와 change-plan 연결 |
| `internal/cli/setup_preview.go` | 생성 | `ChangePlan` 을 CLI preview item으로 변환 |
| `internal/cli/connect.go` | 수정 | help wording truth-sync와 `status` 진입점 연결 |
| `internal/cli/connect_status.go` | 생성 | deterministic local verify/status surface |
| `internal/cli/init.go` | 수정 | preview-aware next-step messaging |
| `pkg/setup/change_plan.go` | 생성 | `BuildGeneratePlan` / `BuildUpdatePlan` no-write change-set builder |
| `pkg/setup/change_apply.go` | 생성 | `ApplyChangePlan` stale preview revalidation + write path |
| `pkg/setup/types.go` | 수정 | `ChangePlan`, `PlannedChange`, `WorkspaceHint` 계약 추가 |
| `pkg/setup/workspace_hints.go` | 생성 | repo-aware hint 생성 |
| `README.md` | 수정 | onboarding truth-sync |
| `docs/README.ko.md` | 수정 | onboarding truth-sync |
| `internal/cli/connect_truth_sync_test.go` | 생성 | CLI/README truth drift regression guard |

## Architecture Considerations

- preview-only computation은 write path와 분리되어야 하며, file generation logic는 `ChangePlan{Changes []PlannedChange, WorkspaceHints []WorkspaceHint, Fingerprint}` 형태의 reusable change-set 으로 노출되는 편이 좋다.
- apply 경로는 preview 결과를 재사용하되 `ApplyChangePlan(plan)` 한 곳에서만 쓰기를 허용하고, stale fingerprint는 `ErrStaleChangePlan` 으로 막아야 한다.
- workspace detection 결과를 다시 파싱하지 말고 existing `pkg/setup/workspace.go`를 재사용한다.
- docs truth-sync는 implementation-driven이어야 하고, release gate나 tests로 방어해야 한다.
- `auto arch enforce` 기준 현재 아키텍처 규칙 위반은 없다.

## Tasks

- [x] `update` preview/apply change-set 모델(`ChangePlan`, `PlannedChange`, `WorkspaceHint`, `ApplyChangePlan`)을 정의한다.
- [x] `setup generate/update` preview mode와 file classification을 추가한다.
- [x] connect help/README 실제 state machine을 정리한다.
- [x] onboarding verification/status surface를 설계한다.
- [x] workspace detection을 repo-aware hints에 연결한다.
- [x] no-write preview와 `connect_truth_sync_test.go` 기반 truth-sync regression tests를 추가한다.

## Risks & Mitigations

| 리스크 | 영향도 | 대응 |
|--------|--------|------|
| preview와 apply 결과가 달라질 수 있음 | 높음 | preview change-set 재사용 또는 revalidation contract 명시 |
| command별 flag naming이 들쭉날쭉해질 수 있음 | 중간 | shared preview-first naming rule 정의 |
| truth-sync가 문서 수정에만 그칠 위험 | 중간 | help/tests/release checks로 구현과 문서를 함께 묶음 |
| meta workspace hint가 noisy할 수 있음 | 중간 | repo-aware hints는 preview/apply 직전 핵심 상황에만 노출 |

## Dependencies

- 내부:
  - `internal/cli/update.go`
  - `internal/cli/update_preview.go`
  - `internal/cli/update_config_preview.go`
  - `internal/cli/setup.go`
  - `internal/cli/setup_preview.go`
  - `internal/cli/connect.go`
  - `internal/cli/connect_status.go`
  - `internal/cli/init.go`
  - `pkg/setup/change_plan.go`
  - `pkg/setup/change_apply.go`
  - `pkg/setup/types.go`
  - `pkg/setup/workspace_hints.go`
  - `README.md`
  - `docs/README.ko.md`
- 관련 기존 SPEC:
  - `SPEC-CONNECT-002`
  - `SPEC-INITUX-001`
  - `SPEC-SETUP-002`
  - `SPEC-OSSUX-001`
- 외부 참고:
  - Terraform plan
  - GitHub CLI auth/status
  - Vercel link
  - npm doctor

## Exit Criteria

- [x] preview-first commands guarantee no writes in preview mode
- [x] tracked/generated/runtime classification is shown in preview output
- [x] connect help/README no longer overstate unsupported flow behavior
- [x] repo-aware hints surface source-of-truth context in bootstrap flows
- [x] regression tests protect preview/apply and truth-sync contracts
