# SPEC-OMP-005 수락 기준

## Oracle Acceptance Notes

- 모든 S1-S11은 **Must**다. 역할 이름·selector·thinking은 정확 일치로 비교한다.
- fake 카탈로그는 operator-attested 경로를 쓰며 `anthropic/claude-{fable-5-1,opus-5,sonnet-5,haiku-4-5}`와 `openai-codex/gpt-{6-astra,5.6-sol,5.6-terra,5.6-luna}`를 포함한다.
- 실측 scenario(S8, S10의 워크스페이스 부분)는 autopus-workspace의 실제 `omp/18.1.10`으로 수행한다.
- 바이트 동일성은 `diff` 무출력 또는 `os.ReadFile` 결과 완전 일치로 판정한다.

## Test Scenarios

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

### S9: 소유하지 않은 modelRoles는 건드리지 않는다
Given ledger가 없는 `.omp/config.yml`에 사용자가 쓴 `modelRoles:\n  task: user/model:high\nunknown: keep\n`.
When project-managed 프로필로 `Generate`한다.
Then `managed_key_conflict: prior fingerprint mismatch`로 실패한다.
And 파일 바이트는 원본과 완전히 같고 `.autopus/omp-model-*`·overlay 산출물이 하나도 생성되지 않는다.

### S10: project-managed 이탈 시 preimage 복원과 왕복
Given `.omp/config.yml`이 없던 워크스페이스에 project-managed 프로필이 적용돼 파일과 ownership ledger가 생겼다.
When 프로필을 overlay 모드(`agents.executor.candidates = [anthropic/claude-sonnet-5:high]`)로 바꿔 `Update`한다.
Then `.omp/config.yml`과 `.autopus/omp-model-project-ownership-v1.json`이 모두 사라지고 overlay의 `autopus_executor`는 `anthropic/claude-sonnet-5:high`, `autopus_tester`는 capability 기본값 `anthropic/claude-opus-5:xhigh`다.
When 프로필을 다시 project-managed로 되돌려 `Update`한다.
Then `Update`가 성공하고 `.omp/config.yml`은 최초 적용 결과와 byte-identical이며 overlay 파일은 prune된다.

### S11: readback 신뢰 경계와 병렬 상한
Given 역할 16개 projection과 세션 겹침을 기록하는 fake RPC runner(호출당 20ms 지연).
When `readOMPModelRolesViaRPC`를 호출한다.
Then RPC 호출은 정확히 16회, 결과 `modelRoles`는 projection과 같고, 동시에 열린 세션의 최댓값은 `1 < peak <= 4`다.
And `SafeOMPModelRoleRPCArgs`는 `--config`가 절대경로가 아니거나 개행을 포함하거나, `--model`이 `@identifier`가 아니거나, `--mode rpc`·`--no-tools`·`--no-skills`·`--no-extensions` 중 하나라도 빠지거나 추가 인자가 있는 argv를 거부한다.
And 프로브 프로세스는 `--config <path>` argv에 대해 `PI_CONFIG_FILES=<path>`를 함께 전달하고 metadata 프로브에는 전달하지 않으며, 출력이 `maxOutput`을 넘으면 프로세스 그룹을 종료한다.
When 한 역할(`autopus_tester`)의 RPC 응답만 `thinkingLevel=low`로 바뀐다.
Then `readOMPModelRolesViaRPC`는 결과 없이 `activation role readback mismatch: autopus_tester`로 실패한다.

## Current Acceptance State

- S1-S7, S9는 `pkg/config`(`role_model_policy_test.go`, `role_model_policy_agents_test.go`, `role_model_policy_builtin_agents_test.go`, `role_model_policy_attested_test.go`)와 `pkg/adapter/omp`(`omp_model_integration_builtin_test.go`, `omp_model_projection_test.go`, `omp_model_integration_lifecycle_test.go` S11 conflict 테스트, doctor 테스트)로 고정됐고 통과했다.
- S11은 `omp_model_activation_rpc_test.go`의 `TestReadOMPModelRolesViaRPC_BoundsInFlightSessionsToWorkerCount`(16회 호출, `1 < peak <= 4`), `TestReadOMPModelRolesViaRPC_FailsWholeReadbackOnOneMismatchedRole`(`activation role readback mismatch: autopus_tester`), `TestSafeOMPModelRoleRPCArgs_AcceptsOnlyProviderFreeRoleSessions`(argv allowlist 8종 거부)와 `omp_model_probe_process_test.go`의 `PI_CONFIG_FILES` 전달 테스트, `pkg/processprobe`의 출력 상한 테스트로 고정됐고 `-race`로 통과했다.
- S10은 `TestOMPModelIntegration_ProjectManagedToOverlayRestoresPreimageAndRoundTrips`로 고정됐고(수정 전 실패 확인), autopus-workspace에서 같은 왕복을 실측해 `.omp/config.yml` byte-identical·ledger/overlay 정리를 확인했다.
- S8은 2026-09-05 autopus-workspace(`omp/18.1.10`)에서 실측했다: `auto update --local` 후 `.omp/config.yml`의 native 키 0개·`autopus_*` 16개, `omp config get modelRoles`가 전역 `default=claude-fable-5-1:xhigh`/`tiny=gpt-5.6-luna:max`/`task=gpt-5.6-sol:max`와 프로젝트 `autopus_executor=claude-opus-5:xhigh`를 함께 반환, `--model @autopus_executor` RPC가 `claude-opus-5`/`xhigh`, `@autopus_planner`가 `claude-fable-5-1`/`max`, `@autopus_reviewer`가 `gpt-6-astra`/`max`, `auto doctor` `model-routing.receipt`가 `supported/fresh`.
- `family: openai` anchor는 `auto update --local --preview`(RPC readback 포함)로 검증했다.
- 리뷰 게이트: `auto spec review SPEC-OMP-005 --subprocess --auto -p claude`(2026-09-05) 판정 **PASS**, 체크리스트 23/23, 발견 13건 전부 resolved. codex는 사용량 한도(9/7 리셋 예정), gemini는 debate 프롬프트에서 25분 타임아웃이라 정족수를 만족하는 claude 단독으로 실행했다.
