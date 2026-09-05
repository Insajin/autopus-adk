# SPEC-OMP-006 구현 계획

## Implementation Strategy

`ExecutionBackend` 인터페이스와 리뷰 루프는 그대로 두고 세 지점만 연다. (1) structured 경로의 backend 선택 지점에 요청 단위 라우팅 래퍼를 끼우고, `RunOrchestra` 직접 경로에는 `OrchestraConfig.ProviderBackends`로 같은 OMP backend를 주입한다. RPC backend는 SPEC-OMP-004의 `pipelineOMPProcess`/`pipelineOMPRPCProtocol`을 argv 옵션과 18.1.x prompt ack 허용만 확장해 재사용한다. (2) SPEC 리뷰 러너에 judge 단계를 추가하고 루프 merge에서 judge 결과를 우선한다. (3) 세션은 도구 allowlist + hardening overlay로 read-only를 강제한다. 첫 게이트(2026-09-05, OMP backend 3-provider + judge 실행 성공)가 낸 F-001~F-016을 두 번째 iteration에서 닫는다.

## Visual Planning Brief

```mermaid
flowchart LR
  C[autopus.yaml providers<br/>backend: omp, model, tools] --> V[config.Validate<br/>fail-closed]
  V --> R[RoutedBackend<br/>structured review + judge]
  V --> D[OrchestraConfig.ProviderBackends<br/>RunOrchestra direct path]
  R --> O[ompReviewBackend<br/>--tools --no-lsp --config hardening]
  D --> O
  O --> S[structured spec review]
  S --> J[judge stage<br/>review_judge validated]
  J --> M[merge: judge > supermajority<br/>verify-mode ID preservation]
  M --> W[review.md / receipt<br/>providers[].executed_backend + judge]
```

## Feature Completion Scope

| Outcome slice | Included | Evidence |
|---|---|---|
| provider `backend/model/tools` 스키마와 검증, migration 보존 | Yes | S1 |
| structured 리뷰·judge 라우팅 | Yes | S2 |
| `RunOrchestra` 전략 6종 라우팅(`ProviderBackends`) | Yes | S12 |
| OMP RPC read-only 세션(argv·격리·모델·thinking·17/18.1 ack) | Yes | S3, S4 |
| hardening overlay(LSP/MCP/web/memory off, secrets on) | Yes | S13 |
| family 도출과 실패 분류(provider 오류 턴, transient 재시도, 모델 드리프트) | Yes | S5, S15 |
| judge 설정 해석·프롬프트·스키마·검증 | Yes | S6, S11 |
| judge 우선순위·verify 모드 ID 보존·fallback | Yes | S7, S8, S10 |
| receipt 관측면 | Yes | S8, S9 |
| dogfood | Yes | S9 |
| 읽기 경로 sandbox, pane 변경, orchestra 전략 변경, Agent Hub 패널 | No | Residual Risk / Evolution Ideas |

## Tasks

- [x] **T1: provider 스키마·검증·orchestra 타입.** `pkg/config/schema.go`(`ProviderEntry.Backend/Model/Tools`), `[NEW] pkg/config/schema_provider_backend.go`(검증·`EffectiveTools`), `pkg/config/{migrate.go,codex_provider.go}`(backend 엔트리는 CLI 기본값 복원·codex 분류에서 제외), `pkg/orchestra/types.go`(`ProviderConfig` 필드), `[NEW] pkg/orchestra/backend_routed.go`, `[NEW] pkg/orchestra/model_family.go`, `pkg/orchestra/{schema_types.go,schema_builder.go,output_parser.go,prompt_builder.go}`, `[NEW] pkg/orchestra/review_judge_prompt.go`, `[NEW] templates/shared/spec-review-judge.md.tmpl`.
- [x] **T2: OMP RPC 리뷰 backend.** `[NEW] internal/cli/omp_review_backend.go`, `[NEW] omp_review_backend_process.go`, `pipeline_backend_omp_process.go`(`startPipelineOMPProcessWithOptions`), `pipeline_backend_omp_protocol.go`(`set_thinking_level`, 18.1.x bare prompt ack).
- [x] **T3: structured 경로 배선.** `internal/cli/spec_review_runtime.go`(factory 기본값 `selectRoutedBackend`), `orchestra_run_runtime.go`, `orchestra_helpers.go`(`providerConfigFromEntry`), `spec_review_providers.go`(omp 설치 확인), `orchestra_brainstorm_judge.go`(configured family 우선), `codex_catalog_runtime.go`(omp면 catalog probe 생략), `spec_review_schema.go`(omp면 schema inline), `pkg/orchestra/subprocess_*`(omp는 CLI schema flag 미지원으로 embed).
- [x] **T4: judge 단계.** `[NEW] internal/cli/spec_review_judge.go`, `spec_review_structured.go`.
- [x] **T5: merge 우선순위·receipt.** `internal/cli/spec_review_loop.go`, `spec_review_loop_merge.go`, `spec_review_receipt.go`, `spec_review_runtime.go`(receipt 매핑), `[NEW] pkg/spec/judge.go`, `pkg/spec/types.go`(`ReviewResult.Judge`), `pkg/spec/review_persist.go`.
- [x] **T6: dogfood.** autopus-adk·autopus-workspace `autopus.yaml` providers를 `backend: omp`로 바꾸고 `auto spec review SPEC-OMP-006 --subprocess --auto`로 receipt를 남긴다. 1차(2026-09-05): 3 provider success·judge ok, 판정 REVISE(F-001~F-016). 2-4차: T7-T9 후 재실행, F-001 잔여만 남음. 5차: revision 0 judge PASS 뒤 revision 1이 claude provider 오류(`empty_output` 오분류)로 서킷 브레이커 REVISE. 6차(T10 후): 3 provider success·`executed_backend=omp`×3, judge ok(anthropic), 판정 PASS, 체크리스트 69/69, F-018(표기 드리프트)만 deferred → 정정.
- [x] **T7: `RunOrchestra` 모든 전략 라우팅(F-005).** `pkg/orchestra/types.go`(`ProviderBackends`), `[NEW] pkg/orchestra/provider_backend_route.go`(`runConfiguredProvider`), `runner.go`(preflight skip, `runParallel`·`runFastest` 호출 교체), `debate.go`(rebuttal), `debate_judge.go`, `relay.go`, `sequential_pipeline.go`, `recheck.go`(transport), `internal/cli/{orchestra.go,orchestra_run_runtime.go}`(ProviderBackends 주입). 전략 6종 라우팅과 backend 부재 시 fail-closed를 `[NEW] provider_backend_route_strategies_test.go`로 고정.
- [x] **T8: 세션 hardening(F-001, F-002).** `internal/cli/omp_review_backend*.go`: `--no-lsp`, private overlay `review-hardening.yml` + `--config`, 종료 시 삭제.
- [x] **T9: judge hardening(F-003, F-004, F-006, F-007, F-008, F-011, F-015).** `internal/cli/spec_review.go`(judge config 해석), `spec_review_loop*.go`, `spec_review_judge*.go`(REVISE/REJECT⇒accept≥1, 미지 alias, `merged=` 로그), `spec_review_receipt*.go`, `pkg/spec/{judge.go,types.go,review_persist.go,provider_health.go}`(verify 모드 ID 보존, merge 출처 결합, `AcceptedIDs`/`Rationale`, 성공·실패 provider `executed_backend`), `pkg/orchestra/output_parser.go`(`ParseReviewJudge` 의미 검증), 템플릿 문장 추가.
- [x] **T10: 세션 턴 의미·재시도·backend 귀속(F-008 잔여, 5차 dogfood).** `internal/cli/pipeline_backend_omp_protocol.go`(`agent_end.messages` 판독), `[NEW] pipeline_backend_omp_turn.go`(`pipelineOMPTurnError`, transient 판정, assistant identity), `[NEW] pipeline_backend_omp_json.go`(분리), `omp_review_backend.go`(`get_state.dumpTools` 검증, 모델 드리프트 fail-closed), `[NEW] omp_review_backend_session.go`(fresh-session 재시도·backoff), `spec_review_structured.go`(`provider_error`/`provider_model_error`), `pkg/orchestra/backend_routed.go`(`ProviderBackendResolver`), `spec_review_provider_policy.go`·`spec_review_structured_runtime.go`(부모 취소·큐잉 outcome의 provider별 backend 이름).

## Ownership

| Slice | Paths |
|---|---|
| RouteDirect (T7) | `pkg/orchestra/{types.go,runner.go,debate.go,debate_judge.go,relay.go,sequential_pipeline.go,recheck.go}`, `[NEW] pkg/orchestra/{provider_backend_route.go,provider_backend_route_strategies_test.go}`, `internal/cli/{orchestra.go,orchestra_run_runtime.go}` |
| SessionHardening (T8) | `internal/cli/omp_review_backend*.go` |
| TurnSemantics (T10) | `internal/cli/{pipeline_backend_omp_protocol.go,pipeline_backend_omp_turn.go,pipeline_backend_omp_json.go,omp_review_backend*.go,spec_review_structured*.go,spec_review_provider_policy.go}`, `pkg/orchestra/backend_routed.go` |
| JudgeHardening (T9) | `internal/cli/{spec_review.go,spec_review_loop*.go,spec_review_judge*.go,spec_review_receipt*.go,spec_review_runtime.go}`, `pkg/spec/{judge.go,types.go,review_persist.go,provider_health.go}`, `pkg/orchestra/output_parser.go`, `templates/shared/spec-review-judge.md.tmpl` |
| integration (T6) | 빌드·전체 테스트·config·dogfood·SPEC/CHANGELOG |

## Risks

| Risk | Mitigation |
|---|---|
| OMP RPC 세션이 read 도구로 대형 파일을 읽어 컨텍스트를 넘김 | `--max-time`과 provider timeout으로 상한; auto compaction off이므로 초과 시 실패로 분류되고 supermajority가 나머지로 판정 |
| judge가 리뷰어와 같은 family라 독립성이 약함 | receipt에 judge family 기록; `spec.review_gate.judge`는 사용자 선택. family 분리 강제는 Evolution Idea |
| `openai-codex` provider가 codex CLI와 같은 계정 한도를 공유할 수 있음 | 1차 dogfood에서는 OMP 경로가 성공했다(CLI는 한도). backend는 한도를 우회하지 않음(non-goal) |
| judge가 리뷰어 finding을 임의로 버림 | `reject`는 `reason` 필수, receipt에 rejected 수와 accepted_ids 기록, 리뷰어 원문은 review.md에 보존 |
| omp 버전별 RPC 응답 차이(17.x `agentInvoked` vs 18.1.x bare ack) | 두 형태를 모두 수락하고 lifecycle 프레임으로 턴 완료를 판정(테스트 고정) |
| provider 일시 오류(429/529)가 리뷰 revision 전체를 서킷 브레이커로 끊음 | `stopReason=error` 턴을 `provider_error`로 분류하고 transient status만 같은 pin의 새 프로세스로 최대 3회 재시도(OMP auto-retry는 model fallback 때문에 계속 off) |
