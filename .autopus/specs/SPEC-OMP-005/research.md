# SPEC-OMP-005 리서치

## Outcome Lock

`.omp/config.yml`과 `.omp/agents/*.md`가 에이전트별 `autopus_<agent>` 역할만으로 모델을 결정하고, OMP native role은 사용자 소유로 남는다.

## 실측 근거

- `omp/18.1.10`, `omp --config custom-roles.yml --model @autopus_executor --mode rpc --no-tools --no-skills --no-extensions` + `get_state` → `{"model":"claude-opus-5","thinkingLevel":"xhigh"}`. `@autopus_planner` → `claude-fable-5-1`/`max`. 커스텀 역할은 modelRoles 키만 있으면 해석된다.
- OMP docs(`task-agent-discovery.md` Role-backed custom agents): frontmatter `model: "@review"`가 `modelRoles.review`로 해석된다. `settings.md`: `modelTags`로 추가 역할을 정의할 수 있고 `/model` Roles 뷰가 커스텀 역할을 보인다.
- SPEC-OMP-003 적용 결과(2026-09-05, autopus-workspace): `task: anthropic/claude-fable-5-1:max`(executor 승격), `tiny: anthropic/claude-opus-5:xhigh`(백그라운드 작업 비용 상승), `default: anthropic/claude-fable-5-1:max`(전역 `xhigh` 덮음). 이 세 관측이 본 SPEC의 동기다.

## 현재 코드 경로

| 단계 | 파일 | 키 |
|---|---|---|
| 프로필 → 라우트 | `omp_model_integration_bridge.go` `bridgeOMPIntegrationRoutes` | capability |
| 라우트 → resolution | `omp_model_routing_compile.go` | request 단위(agent 필드 있음) |
| resolution → 투영 입력 | `omp_model_integration_bridge.go` `projectOMPIntegrationCapabilities` | capability |
| 투영 | `omp_model_projection.go`, `omp_model_projection_policy.go` `ompProjectionRoleSpecs` | native role |
| agent 렌더 | `omp_agents.go` → `content.TransformAgentForOMPWithModel` | `@role` |
| receipt | `omp_model_integration_receipt.go` | capability로 resolution 조회 |
| doctor | `internal/cli/doctor_omp_model_routing_probe.go` `compileOMPModelDoctorRouting` | agent(이미) |
| agent catalog | `internal/cli/platform_omp_agent_catalog.go` | `OMPNativeRoleCapability(role)` |

doctor 경로만 이미 agent 키라서, integration 경로를 doctor 경로와 같은 모양으로 맞추면 된다.

## 대안 검토

- **frontmatter에 exact selector 기록**(SPEC-OMP-003 본문이 허용): modelRoles가 필요 없어지지만 `/model` Roles 뷰에서 재지정할 수 없고 fallback chain·modelFallback 설정은 여전히 config에 있어야 한다. 역할 간접화를 잃으므로 기각.
- **native role 행렬만 조정**(debugger/deep-worker → `slow`): task 승격은 줄지만 default/tiny/smol/commit 소유 문제는 남는다. 기각.
- **에이전트별 커스텀 역할**(채택): 두 부작용을 구조적으로 제거하고 OMP 역할 간접화를 유지한다.

## Completion Debt

- RPC readback 호출이 16회로 늘어난다(역할당 1 프로세스). 병렬화는 후속.
- SPEC-OMP-003의 Policy Contract 절은 supersede 노트로만 갱신하고 본문은 이력으로 남긴다.

## Evolution Ideas

- `modelTags`로 역할에 설명 메타데이터를 붙여 `/model` Roles 뷰에서 에이전트 용도를 보이게 한다.
- 프로필 `agents.<name>.thinking` 단독 오버라이드(후보 전체가 아닌 thinking만) 지원.
