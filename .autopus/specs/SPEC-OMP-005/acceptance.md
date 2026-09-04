# SPEC-OMP-005 수락 기준

## Oracle Acceptance Notes

- 모든 S1-S8은 **Must**다. 역할 이름·selector·thinking은 정확 일치로 비교한다.
- fake 카탈로그는 operator-attested 경로를 쓰며 `anthropic/claude-{fable-5-1,opus-5,sonnet-5,haiku-4-5}`와 `openai-codex/gpt-{6-astra,5.6-sol,5.6-terra,5.6-luna}`를 포함한다.
- 실측 scenario(S8)는 autopus-workspace의 실제 `omp/18.1.10`으로 수행한다.

## Test Scenarios

## Current Acceptance State

- S1-S7은 `pkg/config`(`role_model_policy_test.go`, `role_model_policy_agents_test.go`, `role_model_policy_builtin_agents_test.go`, `role_model_policy_attested_test.go`)와 `pkg/adapter/omp`(`omp_model_integration_builtin_test.go`, `omp_model_projection_test.go`, doctor 테스트)로 고정됐고 통과했다.
- S8은 2026-09-05 autopus-workspace(`omp/18.1.10`)에서 실측했다: `auto update --local` 후 `.omp/config.yml`의 native 키 0개·`autopus_*` 16개, `omp config get modelRoles`가 전역 `default=claude-fable-5-1:xhigh`/`tiny=gpt-5.6-luna:max`/`task=gpt-5.6-sol:max`와 프로젝트 `autopus_executor=claude-opus-5:xhigh`를 함께 반환, `--model @autopus_executor` RPC가 `claude-opus-5`/`xhigh`, `@autopus_planner`가 `claude-fable-5-1`/`max`, `@autopus_reviewer`가 `gpt-6-astra`/`max`, `auto doctor` `model-routing.receipt`가 `supported/fresh`.
- 실측 중 발견: 역할 16개의 직렬 RPC readback이 doctor의 공유 20s 데드라인을 넘겨 `projection_mismatch`가 났다. readback을 4-way 병렬로 바꿔 `auto update`가 22s→12s로 줄었고 doctor가 통과한다(plan.md Risks의 첫 항목 해소).

### S1: 역할 이름 규칙과 native 키 부재
Given canonical agent 16개.
When `OMPAgentRoleMapping()`을 읽는다.
Then 값 집합은 Policy Contract의 `autopus_*` 16개와 정확히 같고 `deep-worker`는 `autopus_deep_worker`다.
And `OMPRoleCapability("task")`, `OMPRoleCapability("tiny")`는 `role_unknown`을 반환한다.

### S2: 행렬 불일치 fail-closed
Given 프로필 `agents.executor: {role: autopus_planner}`와 `agents.executor: {capability: deep_reasoning}`.
When `Validate()`를 호출한다.
Then 둘 다 `role_capability_mismatch`로 실패한다.
And `agents.future-agent: {}`는 `agent_role_unmapped`로 실패한다.

### S3: 후보 오버라이드 우선순위
Given `capabilities.coding_tool_use.candidates = [anthropic/claude-opus-5:xhigh]`, `agents.executor.candidates = [anthropic/claude-sonnet-5:medium]`.
When `AgentCandidates("executor")`와 `AgentCandidates("tester")`를 읽는다.
Then executor는 sonnet 후보, tester는 opus 후보다.
And 오버라이드 후보에 `family`가 없으면 operator-attested 검증이 `operator_attestation_family_required`로 실패한다.

### S4: family diversity 역할 검증
Given `family_diversity.roles = [autopus_reviewer]`.
When 라우트를 브리지한다.
Then reviewer 라우트만 `prefer_distinct_executor_family=true`이고 security-auditor는 false다.
And `family_diversity.roles = [advisor]`는 `role_unknown`으로 거부된다.

### S5: built-in 파생은 max-wins가 없다
Given ultra 프리셋(planner/debugger fable, executor opus)과 `family: anthropic`.
When `BuiltinRoleModelProfile("ultra", ...)`를 파생하고 라우트를 브리지·컴파일·투영한다.
Then `autopus_executor`는 `anthropic/claude-opus-5:xhigh`, `autopus_debugger`는 `anthropic/claude-fable-5-1:max`, `autopus_validator`는 `anthropic/claude-opus-5:xhigh`다.
And `autopus_reviewer`와 `autopus_security_auditor`는 `openai-codex/gpt-6-astra:max`다.
And `family_diversity.roles`는 정확히 `[autopus_reviewer, autopus_security_auditor]`다.

### S6: 투영 산출물
Given S5의 projection.
When `.omp/agents/*.md`와 overlay를 렌더링한다.
Then `modelRoles` 키 집합은 `autopus_*` 16개이고 native 키는 0개다.
And `executor.md` frontmatter는 `model: '@autopus_executor'`, `thinking: xhigh`다.
And `retry.fallbackChains`는 selector 키(`anthropic/claude-fable-5-1:max` → `[anthropic/claude-opus-5:xhigh]` 등)이며 같은 selector에 다른 chain이 오면 `fallback_chain_conflict`다.

### S7: receipt와 doctor
Given S5의 integration.
When receipt를 만들고 doctor routing을 컴파일한다.
Then receipt `roles`는 16행이고 각 `role`은 Policy Contract 값이며 `evidence_class=operator_attested`다.
And doctor는 16개 `model-routing.role.*` 검사를 내고 `agent=executor role=autopus_executor capability=coding_tool_use`를 포함한다.

### S8: 워크스페이스 실측(native 키 교체)
Given SPEC-OMP-003이 기록한 native 키 10개를 가진 project-managed `.omp/config.yml`.
When 새 바이너리로 `auto update --local`을 실행한다.
Then `.omp/config.yml`의 `modelRoles`에 native 키가 0개, `autopus_*` 키가 16개다.
And `omp config get modelRoles`(cwd=repo)는 전역 `tiny`/`task` 값과 프로젝트 `autopus_*` 값을 함께 보인다.
And `omp --model @autopus_executor --mode rpc get_state`는 `claude-opus-5`/`xhigh`, `@autopus_planner`는 `claude-fable-5-1`/`max`를 반환한다.
And `auto doctor`의 `model-routing.receipt`가 pass다.
