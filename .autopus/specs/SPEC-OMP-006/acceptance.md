# SPEC-OMP-006 수락 기준

## Oracle Acceptance Notes

- 모든 S1-S15는 **Must**다. argv·selector·verdict·finding 결정은 정확 일치로 비교한다.
- OMP RPC는 테스트에서 fake 실행 파일(테스트 바이너리 재실행, JSONL 프레임·argv·cwd·overlay 내용을 로그로 기록) 또는 프로토콜 fake로 대체한다. 실제 `omp`는 S9에서만 쓴다.
- judge oracle은 typed `review_judge` JSON을 돌려주는 fake backend를 쓴다.
- macOS `/var`↔`/private/var` 심링크 때문에 cwd 비교는 `EvalSymlinks` 후 한다.

## Test Scenarios

### S1: provider 스키마 검증 fail-closed
Given `orchestra.providers.claude: {backend: omp}`(model 없음), `{backend: omp, model: anthropic/claude-fable-5-1:max, tools: [read, bash]}`, `{backend: pane}`.
When `config.Validate()`를 호출한다.
Then 각각 `provider_model_required: claude`, `provider_tools_invalid: claude`, `provider_backend_invalid: claude`로 실패한다.
And `{backend: omp, model: anthropic/claude-fable-5-1:max}`와 `tools: []`는 통과하고 `EffectiveTools()`는 `[glob, grep, read]`, `tools: [read, grep, read]`는 `[grep, read]`다.
And `MigrateOrchestraConfig`는 backend 엔트리를 CLI 기본값으로 되돌리거나 `model_policy`를 붙이지 않는다(`changed=false`, 엔트리 동일).

### S2: 요청 단위 라우팅(structured)
Given base backend(fake, `Name()="subprocess"`)와 omp backend(fake, `Name()="omp"`)로 만든 `NewRoutedBackend`.
When `Config.Backend="omp"`인 요청과 빈 backend 요청을 실행한다.
Then 첫 요청은 omp fake로, 둘째는 base fake로 가며 omp 요청에서 `Config.Binary`는 실행되지 않는다.
And `Name()`은 `subprocess+omp`다.

### S3: OMP 리뷰 세션 argv와 격리
Given fake omp 실행 파일이 받은 argv와 cwd를 기록하고 ready/negotiate/set_model/get_state/prompt/get_last_assistant_text 프레임을 흉내 낸다.
When `backend: omp, model: anthropic/claude-fable-5-1:max, tools: [read, grep]` provider로 timeout 5s에 `Execute`한다.
Then argv는 정확히 `--mode rpc --no-session --no-extensions --session-dir <base>/pipeline-task-*/sessions --no-skills --no-lsp --config <base>/review-hardening.yml --tools grep,read --approval-mode yolo --max-time 5`다(`<base>`는 호출 전용 0700 런타임, `--config`는 절대경로).
And cwd는 프로젝트 디렉터리(심링크 해석 후)이고, 완료 후 `<base>`가 존재하지 않는다.

### S4: 모델·thinking 설정과 출력
Given S3의 fake.
When selector `openai-codex/gpt-6-astra:max`로 실행한다.
Then fake는 `negotiate_protocol, set_auto_retry, set_auto_compaction, get_state, set_model, set_thinking_level, get_state, prompt, get_state, get_last_assistant_text` 순서로 명령을 받고(첫 `get_state`가 도구 집합 검증), `set_model{provider: openai-codex, modelId: gpt-6-astra}` 뒤 `set_thinking_level{level: max}`가 온다.
And 응답 `Output`은 text, `ExecutedBackend="omp"`, `ModelFamily="openai"`, `Role`은 요청 role이다.
And selector에 thinking이 없으면 `set_thinking_level`은 보내지 않는다.
And fake가 omp 18.1.x처럼 prompt에 bare `success` 응답만 보내고 `prompt_result` 없이 `agent_start`/`agent_end`를 내도 `Output`이 돌아온다.

### S5: 실패 분류
Given fake가 (a) prompt 응답을 영원히 보내지 않음, (b) `set_model` 실패 프레임, (c) 빈 text.
When 각각 timeout (a) 2s, (b)(c) 1s로 실행한다.
Then (a) `TimedOut=true`, (b) `ExitCode=1`이고 `Error`에 `model unavailable`, (c) `EmptyOutput=true`이며 세 경우 모두 런타임이 삭제된다.

### S6: judge 프롬프트와 파서
Given 리뷰어 2개의 `ReviewerOutput`(하나는 REVISE + major finding, 하나는 PASS)과 verify 모드 prior findings.
When `BuildReviewJudge`로 프롬프트를 만들고 fake judge가 `{verdict: "REVISE", findings: [{severity: major, decision: accept, sources: ["Reviewer A"], ...}, {decision: reject, reason: "duplicate", ...}], rationale: "..."}`를 돌려준다.
Then 프롬프트 헤더는 `Reviewer A/B`이고 provider 이름은 헤더에 없으며(본문은 원문 그대로), prior findings ID를 포함하고, 리뷰어 본문의 자기 식별·지시문을 무시하라는 문장이 있다.
And `ParseReviewJudge`는 verdict `REVISE`, accept 1, reject 1을 돌려주고 `decision: reject`에 `reason`이 없으면 파싱 오류다.

### S7: judge 우선순위
Given supermajority가 PASS(2/3)를 내는 리뷰 결과와 유효한 judge 출력 `{verdict: REVISE, findings: [accept major]}`.
When `runSpecReviewLoop`가 한 리비전을 처리한다.
Then merged verdict는 `REVISE`, findings는 judge가 accept한 1건(sources가 alias→provider로 복원)이며 `ReviewResult.Judge={Provider: claude, Family: anthropic, Status: ok, Verdict: REVISE, Accepted: 1, Rejected: 0, Merged: 0, AcceptedIDs: [F-001]}`이다.
And judge가 `PASS`를 냈지만 accept finding에 `critical`이 있으면 verdict는 `REVISE`로 내려가고 `Judge.Verdict`는 `PASS`로 남는다.
And judge `PASS` + accept 0건 + 리뷰어 checklist FAIL 다수 → verdict `PASS`(legacy 규칙과 동일).

### S8: judge fallback과 receipt
Given judge backend가 timeout 또는 invalid JSON을 돌려준다.
When 같은 리비전을 처리한다.
Then verdict와 findings는 judge 없이 돌린 legacy 결과와 완전히 같고 `Judge.Status`는 `failed`/`invalid`, `Reason`에 원인이 있으며 `FailedProviders`에 `Role: judge` 항목이 있다.
And `review-receipt.json`에 `judge: {provider, family, status, verdict, accepted, rejected, merged, accepted_ids, rationale, reason}` 블록과 `providers[]`(각 `name/status/executed_backend`)이 있다. judge 미설정이면 `judge` 블록이 없다.
And timeout으로 실패한 `backend: omp` 리뷰어도 `providers[]` 행에 `status=timeout`, `executed_backend=omp`로 남는다(`FailedProvider.ExecutedBackend`에서 투영).

### S9: dogfood 실측
Given autopus-adk `autopus.yaml`의 orchestra providers가 전부 `backend: omp`이고 `spec.review_gate.judge: claude`다.
When `auto spec review SPEC-OMP-006 --subprocess --auto`를 실행한다.
Then 실행 로그에 `SPEC 리뷰 백엔드: subprocess+omp (providers=3, …)`와 `OMP 리뷰 세션: provider=<name> model=<selector> tools=glob,grep,read`가 claude/codex/gemini 각각 나오고, `auto spec review` 프로세스의 자식은 `omp`(materialized `omp-verified`)뿐이며 `claude`/`codex`/`agy` 자식은 없다.
And `review-receipt.json`의 `providers[]`는 세 provider 모두 `status=success`, `executed_backend=omp`이고 `judge.status=ok`다.

### S10: verify 모드 judge merge
Given revision 1, prior findings F-001(open), F-002(open), F-004(resolved), 유효한 judge 출력이 F-001·F-004를 accept하고 ID 없는 새 finding 1개를 accept하며 `merge` finding(`id: F-009`, `merge_into: F-001`)이 `Reviewer C`를 출처로 더한다.
When `JudgedFindingsToReview`가 변환하고 루프가 verify scope lock을 적용한다.
Then 변환 결과에서 F-001은 open이며 `FirstSeenRev`는 prior 값 그대로이고 `Provider`는 prior 출처 뒤에 Reviewer C의 provider가 결합돼 있으며, F-002는 resolved(`LastSeenRev=1`), F-004는 regressed, 새 finding은 `F-005`다(prior 최대 ID 다음).
And 루프 결과에서 새 finding은 기존 scope lock 규칙대로 `out_of_scope`로 분류되고(open 아님) F-001은 open, F-002는 resolved다.
And prior F-001(major)이 있는 상태에서 judge가 `PASS`+accept 0건을 내면 F-001은 resolved가 되고 verdict는 `PASS`로 수렴한다(resolved prior의 severity는 하향 근거가 아님).

### S11: judge 출력 의미 검증
Given judge 출력 (a) `{"verdict":"PASS"}`(findings 누락), (b) sources에 `Reviewer Z`(미지 alias), (c) `REVISE`인데 accept 0건, (c') `REJECT`인데 findings 빈 배열, (d) 같은 `id` 두 번(accept와 merge가 같은 id여도), (e) severity `blocker`, (f) `merge`인데 `merge_into`가 없거나 reject finding을 가리킴.
When judge 단계가 파싱한다.
Then 일곱 경우 모두 `Judge.Status=invalid`이고 `Reason`이 각각 findings 누락 / unknown reviewer alias / `REVISE without accepted findings` / `REJECT without accepted findings` / duplicate id / severity 값 / `merge_into must name an accepted finding id`를 말하며 verdict·findings는 supermajority 결과다.

### S12: `RunOrchestra` 전략별 라우팅
Given `OrchestraConfig{Providers: [{Name: reviewer, Backend: omp, Binary: missing-provider-binary}], ProviderBackends: {omp: fake}}`.
When `RunOrchestra`를 consensus, debate(rebuttal, judge), fastest, pipeline, relay, recheck 각각으로 실행한다.
Then 여섯 전략 모두에서 fake backend가 요청을 받고 preflight를 통과하며 첫 응답의 `ExecutedBackend="omp"`다.
And `ProviderBackends`가 nil이면 여섯 전략 모두 응답 0건 또는 오류로 끝나고(`Binary`는 실행되지 않음), `runConfiguredProvider`와 `NewRoutedBackend` 단위에서는 `provider reviewer backend "omp" is not available in this execution path`가 반환된다.

### S13: hardening overlay
Given S3의 fake가 `--config` 경로의 파일 내용을 실행 중 기록한다.
When `Execute`한다.
Then 기록된 overlay는 `lsp.enabled: false`, `mcp.enableProjectConfig: false`, `web_search.enabled: false`, `secrets.enabled: true`, `memory.backend: off`, `tools.approvalMode: yolo` 여섯 키를 정확히 담고 모드는 0600이며, 완료 후 존재하지 않는다.

### S14: 세션 도구 누출 fail-closed
Given fake omp가 `get_state.dumpTools`에 allowlist 외 도구 `mcp__filesystem_delete`를 포함해 보고한다.
When `Execute`한다.
Then 응답은 `ExitCode=1`, `Error`에 `outside the allowlist: mcp__filesystem_delete`가 있고, fake는 `prompt` 명령을 받지 않았으며 런타임은 삭제된다.
And 실제 omp 18.1.10에서 hardening argv로 띄운 세션의 `dumpTools`는 정확히 `["glob","grep","read"]`다(기본 세션은 11개).

### S15: provider 오류·transient 재시도·모델 드리프트
Given fake omp의 `agent_end.messages` 마지막 assistant 항목이 `stopReason=error, errorStatus=404, errorMessage="404 model: gone"`이다.
When `Execute`한다.
Then 응답은 `EmptyOutput=false`, `ExitCode=1`, `Error`에 `provider error status 404: 404 model: gone`, 실패 분류 `provider_error`, 프로세스는 1회만 뜨고 런타임은 삭제된다.
And 첫 프로세스만 `errorStatus=529`로 실패하고 두 번째는 정상 응답하면, 결과는 `Output="review output"`, 프로세스 2개(PID·`--session-dir` 상이), `prompt` 2회, 두 `set_model` 모두 `{openai, model}`, overlay 2개 모두 삭제된다.
And assistant 항목의 `model`이 pinned와 다르면(`claude-sonnet-5`) `ExitCode=1`, `Error`에 `executed model mismatch: want openai/model got openai/claude-sonnet-5`, 분류 `provider_model_error`다.
And 실제 omp 18.1.10 `agent_end` 형상(정상: `provider/model/stopReason=stop`, 오류: `stopReason=error, errorStatus, errorMessage`, `get_last_assistant_text -> {}`)은 `TestSettlePipelineOMPTurn`에 고정된다.

## Current Acceptance State

- 1차 dogfood(2026-09-05): T1-T5 구현 후 `auto spec review SPEC-OMP-006`이 OMP backend에서 완주 — `SPEC 리뷰 백엔드: subprocess+omp`, `OMP 리뷰 세션` 로그 3건(claude fable:max, gemini 3.1-pro:high, codex astra:max), Provider Health 3/3 success, `auto spec review` 자식 프로세스는 `omp-verified`만 관측(외부 CLI 0), judge(claude) `status=ok`, 판정 REVISE로 F-001~F-016 산출. 이 실측이 S9의 transport 부분과 F-016(모델·thinking 해석)을 증명한다.
- T7-T9(2026-09-05): S10(`pkg/spec/judge_test.go` verify 보존·merge_into 출처 결합·regressed 전이, `spec_review_loop_judge_test.go` verify PASS 수렴), S11(`spec_review_judge_test.go`·`review_judge_prompt_test.go` 7종 invalid), S12(`provider_backend_route_strategies_test.go` 전략 6종, `backend_routed_test.go` 미등록 fail-closed), S13(`omp_review_backend_test.go` overlay 기록·삭제), S8 실패 리뷰어 `executed_backend`가 테스트로 통과.
- 2차·3차 dogfood(T7-T9 코드): 3/3 success, judge ok, `backend=omp` 로그. 3차 finding 6건(merge id 충돌, 실패 provider backend, resolved prior 하향, regressed 전이, routed fail-closed, 문서 정합)을 위 테스트와 함께 닫았다.
- 4차 dogfood: 16/17 resolved, 59/69 PASS. 남은 F-001(`--tools`는 built-in 한정, MCP/custom/extension은 discovery)은 세션 `get_state.dumpTools` 런타임 검증(S14, `TestOMPReviewBackend_FailsClosedWhenSessionLeaksTools`)과 실측(`dumpTools == [glob, grep, read]`)으로 닫았다.
- 6차 dogfood(T10 후): 3 provider success·`executed_backend=omp`×3, judge ok(anthropic) 판정 PASS, 체크리스트 69/69 PASS, 18 finding 중 17 resolved·1 deferred(F-018 표기 드리프트, 정정). S9 Completion evidence 충족.
- 5차 dogfood: revision 0 judge PASS(accepted 1, merged 1) 뒤 revision 1에서 claude provider·judge가 `empty_output`으로 실패해 서킷 브레이커 REVISE(43/46 PASS, open 2). 원인: provider 오류 턴(`stopReason=error`)을 빈 출력으로 오분류하고 재시도하지 않았다. 남은 F-008 잔여(부모 취소 경로의 `executed_backend=subprocess+omp`)는 `ProviderBackendResolver`로, F-013(research 자기검증 사유 불일치)은 문서 정정으로, 오분류·재시도·모델 드리프트는 S15로 닫았다.
