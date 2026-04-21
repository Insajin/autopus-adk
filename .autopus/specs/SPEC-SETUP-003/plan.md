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
| `internal/cli/update.go` | 수정 | `--plan`/preview surface |
| `internal/cli/setup.go` | 수정 | generate/update preview and diff explanation |
| `internal/cli/connect.go` | 수정 | help/flow wording truth-sync |
| `internal/cli/init.go` | 수정 | preview-aware next-step messaging |
| `pkg/setup/engine.go` | 수정 | preview/apply separation |
| `pkg/setup/engine_status.go` | 수정 | status/verify surface enrichment |
| `pkg/setup/workspace.go` | 수정 | repo-aware hint input reuse |
| `pkg/connect/*` | 수정 가능 | connect verify/status helpers |
| `README.md` | 수정 | onboarding truth-sync |
| `docs/README.ko.md` | 수정 | onboarding truth-sync |

## Architecture Considerations

- preview-only computation은 write path와 분리되어야 하며, file generation logic는 reusable change-set 형태로 노출되는 편이 좋다.
- workspace detection 결과를 다시 파싱하지 말고 existing `pkg/setup/workspace.go`를 재사용한다.
- docs truth-sync는 implementation-driven이어야 하고, release gate나 tests로 방어해야 한다.
- `auto arch enforce` 기준 현재 아키텍처 규칙 위반은 없다.

## Tasks

- [x] `update` preview/apply change-set 모델을 정의한다.
- [x] `setup generate/update` preview mode와 file classification을 추가한다.
- [x] connect help/README 실제 state machine을 정리한다.
- [x] onboarding verification/status surface를 설계한다.
- [x] workspace detection을 repo-aware hints에 연결한다.
- [x] no-write preview와 truth-sync regression tests를 추가한다.

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
  - `internal/cli/setup.go`
  - `internal/cli/connect.go`
  - `internal/cli/init.go`
  - `pkg/setup/*`
  - `pkg/connect/*`
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
