# SPEC-OMP-002 수락 기준

## Test Scenarios

아래 Must oracle의 예상 값은 각 Then/And에 exact set, bytes, mode, report row, ordered trace, JSON으로 명시한다.

### S1: Workflow command와 skill file의 20대20 대응

Given OMP-only 기본 config와 canonical `workflowSpecs` 20개 이름 집합이 있다.
When OMP adapter가 scratch workspace를 Generate한다.
Then command 집합은 `.agents/commands/<name>.md`의 `<name>`이고 workflow skill 집합은 `.agents/skills/<name>/SKILL.md`의 parent `<name>` 중 `workflowSpecs` 멤버인 항목이며, 두 집합은 canonical 20개와 각각 정확히 같다.
And 두 집합의 차집합은 양방향 모두 빈 집합이며 orphan command와 orphan workflow skill은 0개다.
And `agent-pipeline`을 포함한 compiled extended skill은 workflow 집합 계산에서 제외된다.
And 모든 workflow와 extended mapping을 합친 emitted target relative-path의 중복은 0개다.
And `auto`는 thin router body oracle을 정확히 만족하고, 나머지 19개 `auto-*` 파일은 detailed workflow body oracle을 만족한다.
And `auto-plan`, `auto-test`, `auto-canary`, `auto-go` body에는 각각 하나의 parse 가능한 `## Context Profile`이 있다.

### S2: Router와 direct alias가 같은 상세 계약으로 수렴

Given payload `plan "cache race" --model vendor/model-x --variant high`와 동일 인자를 받은 `/auto-plan` alias가 있다.
When generated router, alias command, router skill body를 구조적으로 파싱한다.
Then 세 본문이 지시하는 exact target은 `workflowSpecs`의 `auto-plan`이며 target map에 정확히 한 번 존재한다.
And model, variant, feature payload placeholder는 각 진입 경로에서 각각 정확히 한 번 전달된다.
And router body는 unknown subcommand를 fuzzy-correct하지 않는다는 명시적 clause를 포함한다.
And 이 static oracle은 프로덕션에 없는 runtime parser나 exact runtime 오류 문자열을 주장하지 않는다.

### S3: OMP context profile matrix가 canonical oracle과 같다

Given Claude, Codex, OpenCode, Gemini, Antigravity mirror와 OMP generated workflow surfaces가 있다.
When plan, test, canary, go의 required, optional, worker-optional, excluded 집합을 파싱한다.
Then OMP plan은 required `[architecture,core,relevant_spec]`, optional `[learning,signature]`, excluded `[canary,test]`와 정확히 같다.
And OMP test는 required `[core,test]`, optional `[learning,signature]`, excluded `[canary]`와 정확히 같다.
And OMP canary는 required `[canary,core]`, optional `[learning]`, excluded `[signature,test]`와 정확히 같다.
And OMP go는 required `[acceptance,available_architecture,core,plan,resolved_spec]`, worker-optional `[learning,signature,task_declared_extra]`, excluded `[canary,test]`와 정확히 같다.
And 기존 다섯 surface의 기존 expected matrix도 바뀌지 않는다.

### S4: OMP worker receipt와 trust-boundary polarity가 canonical contract와 같다

Given existing canonical source `content/skills/agent-pipeline.md`, default OMP compile target, generated OMP `auto-go`가 있다.
When catalog state와 generated pipeline을 resolve하고 return fields와 security/evidence clauses를 추출한다.
Then catalog state는 `Registered=true`, `Compiled=true`, `TargetPath=.agents/skills/agent-pipeline/SKILL.md`와 정확히 같다.
And generated target은 존재하며 `auto-go` body는 그 상대 경로를 정확히 1회 참조한다.
And field 집합은 정렬 후 `[blockers,changed_files,next_required_step,owned_paths,verification]`과 정확히 같다.
And stable project-relative refs만 허용하고 absolute path, `..`, symlink, non-regular file을 거부하는 clause가 존재한다.
And sanitization·redaction·injection evidence 보존과 raw tool/provider/required-body non-replay clause가 존재한다.
And selected refs, hashes, omitted count, 2,000 estimated tokens, short-correct-without-padding semantics가 존재한다.
And 이들 제한을 `permitted`, `do not sanitize`, `may replay raw`로 뒤집은 fixture는 전부 실패한다.

### S5: Generate와 Update가 config bytes와 0600 mode를 보존

Given fixture A에는 user `skills` mapping이 없고 marker가 top-level `skills` subtree 전체를 감싸며, fixture B에는 user `skills` mapping과 그 안에서 `customDirectories` entry만 감싸는 같은 indentation의 marker 한 쌍이 있고 두 파일 mode는 `0600`이다.
When OMP Generate 후 Update를 두 fixture에 각각 순서대로 실행한다.
Then A의 Clean-owned 범위는 top-level subtree 전체이고 B의 Clean-owned 범위는 entry 하나뿐이다.
And marker 밖 user-owned byte slice와 B의 sibling `skills` key/value는 두 단계 전후 정확히 같다.
And 두 fixture 모두 begin/end marker는 각각 1개이며 parsed `skills.customDirectories`는 정확히 `[".agents/skills"]`다.
And 각 단계 후 `Perm()`은 `0600`이다.
And config가 없던 fixture의 Generate 결과 mode도 `0600`이다.
And marker 밖에 `skills.customDirectories`가 중복되거나 `skills`가 scalar인 fixture는 오류를 반환하고 원본 bytes와 mode를 그대로 둔다.

### S6: Clean이 config marker만 제거하고 기존 mode를 보존

Given OMP가 남아 있는 `autopus.yaml`, mode `0600`의 두 marker 표현 config, user-owned YAML, `.omp/rules/user.md`가 있다.
When `auto platform remove omp`가 mutation preflight, Clean, config save 순서로 실행된다.
Then config에는 begin/end marker가 0개이고 user-owned YAML의 parsed key/value와 pre-Clean bytes는 정규 trailing newline 외에 정확히 같다.
And 남은 config의 `Perm()`은 `0600`이다.
And `.omp/rules/user.md`의 bytes와 mode는 정확히 같다.
And user-owned content 없이 marker block만 있던 별도 fixture에서는 config file만 제거되고 `.omp/`의 다른 사용자 파일은 남는다.
And preflight 또는 Clean이 sentinel error를 반환하는 fixture에서는 `autopus.yaml`과 generated files가 byte-identical이고 OMP가 config에 남으며 성공 메시지가 없다.
And Clean 뒤 config save가 실패하는 late-error fixture는 성공을 반환하지 않고 changed-path receipt와 `repair_required=true`를 남겨 재실행 가능한 OMP config를 보존한다.
And 오류 폐기를 전제한 OMP lifecycle 주석은 0개다.

### S7: Validate가 존재하는 디렉터리 안의 잘못된 값을 탐지

Given expected rules는 `content/rules/*.md`, agents는 `LoadAgentSourcesFromFS(content/agents)`, workflows는 `workflowSpecs`, pipeline은 compiled catalog에서 파생되고, fixture는 각 derived set에서 stable-first 항목 하나씩을 제거하고 config를 malformed YAML로 바꾸지만 모든 parent directory를 유지한다.
When OMP Validate를 실행한다.
Then findings에는 YAML parse error와 각 종류의 `expected=<derived count> got=<derived count-1>`이 별도 error로 있다.
And missing command-skill pair 이름이 stable-sorted detail에 있다.
And 현재 source snapshot의 derived evidence는 rules `14`, agents `16`, commands `20`, workflow skills `20`이지만 validator authority로 하드코딩되지 않는다.
And directory existence가 참이라는 이유로 findings가 0개가 되지 않는다.

### S8: Model selector syntax와 discovery readiness를 요청 없이 분류

Given actual installed OMP, isolated test-owned profile의 dummy catalog model `s7dummy/s7-probe`, credential-unavailable 상태, selectors `s7-probe`, `s7dummy/`, `unknown-family`가 있다.
When readiness가 bounded `omp models --json`을 실행하고 exact top-level `{"models":[...]}`를 파싱한 뒤 selector syntax와 catalog resolution을 검사한다.
Then `s7-probe`는 discovered candidate와 credential-unavailable warning으로 분류된다.
And `s7dummy/`는 malformed selector error이고 `unknown-family`는 unresolved selector error다.
And fake provider의 completion endpoint request count는 `0`이다.
And `.omp/config.yml` marker bytes에는 `modelRoles`, `enabledModels`, `disabledProviders`, `fallbackChains`가 추가되지 않는다.
And empty list, malformed top-level JSON, timeout, oversized output, nonzero exit은 각각 `catalog_empty`, `catalog_invalid`, `catalog_timeout`, `catalog_oversized`, `catalog_exit_nonzero`로 구분된다.

### S9: Version·capability provenance와 F-010 authority 경계

Given process ancestry argv[0]가 bare `omp`이지만 PATH executable이 `openmp wrapper 4.5`를 출력하는 fixture와 `omp/17.1.8`을 출력하는 fixture가 있다.
When runtime ancestry, adapter Detect, readiness capability probe, orchestra invoker resolution을 각각 실행한다.
Then 첫 fixture의 ancestry hint는 `AgentRuntimeOMP`일 수 있지만 Detect는 false이고 readiness authority는 unavailable이다.
And 두 번째 fixture의 expected receipt는 `identity.version`, `launch.rpc`, `launch.no_session`, `launch.cwd`, `launch.model`, `config.overlay_readback`, `catalog.models_json`, `rpc.command_discovery`, `rpc.tool_events`, `rpc.terminal`이 모두 `supported=true`다.
And 각 값은 각각 `omp --version`, `omp --help`, 17.1.8의 task-owned `PI_CONFIG_FILES` overlay `config get` exact wrapper, `omp models --json`, RPC `available_commands_update`, paired `tool_execution_start/end`, terminal `message_end`로 관측된다.
And 두 fixture 모두 OMP의 orchestra invoking provider, automatic judge provider, permission authority 결과는 정확히 빈 값이다.
And `--version` hang과 missing capability는 bounded timeout 후 구분된 finding으로 끝난다.

### S10: Doctor text와 JSON이 value failure를 동일하게 투영

Given generated workspace의 모든 디렉터리는 있지만 `customDirectories`가 `["wrong/path"]`이고 `auto-review` detail skill이 빠진 fixture가 있다.
When `auto doctor --dir <scratch>`와 `auto doctor --dir <scratch> --json`을 실행한다.
Then 두 출력은 `skills.customDirectories expected=[.agents/skills] got=[wrong/path]`와 missing `auto-review`를 각각 보고한다.
And JSON check ID와 severity는 text finding과 1대1로 대응한다.
And healthy fixture에서는 OMP value finding이 0개다.
And healthy OMP+OpenCode mixed fixture에서도 yielded shared surface를 OMP 누락으로 보고한 finding은 0개다.

### S11: Actual OMP RPC가 대표 workflow를 끝까지 실행

Given `omp --version`과 capability probe를 통과한 actual installed OMP, isolated `PI_CODING_AGENT_DIR`, OMP-only scratch workspace, actual request content-gated auth-none loopback fake provider가 있다.
When RPC mode에서 `/auto plan "fixture parity"`와 `/auto-plan "fixture parity"`를 별도 bounded turn으로 실행한다.
Then 각 다음 scripted tool은 직전 actual request가 기대 command 또는 generated skill body hash를 포함한 경우에만 허용된다.
And sanitized trace의 ordered projection은 `[skill:auto,skill:auto-plan,tool_execution_start:write,tool_execution_end:write,message_end]`와 정확히 같다.
And loaded `auto-plan` body hash는 generated file hash와 같고 context matrix는 S3의 plan expected set과 같다.
And write target은 scratch root 안의 fixture receipt이며 receipt JSON은 `{"command":"auto-plan","context_profile":"plan","status":"completed"}`와 정확히 같다.
And fake provider request count는 fixture가 정한 bounded count 이내이고 external credential lookup과 external network request는 0건이다.
And 이 결과는 두 대표 surface의 expansion/wiring만 증명하며 일반 runtime parser나 LLM 품질을 증명하지 않는다.
And available command 목록, content gate 없는 scripted event, prompt dump, non-empty stdout, exit code만 관측한 실행은 이 시나리오를 PASS시키지 못한다.

### S12: User ownership과 mixed-platform 양보가 보존

Given user-owned `.omp/rules/user.md`, `.omp/RULES.md`, marker 밖 config, 그리고 OpenCode가 소유하는 `.agents/skills/auto/SKILL.md`가 있는 mixed workspace가 있다.
When OMP Generate, Update, Clean을 실행한다.
Then OMP manifest는 OMP가 실제 소유한 path만 기록하고 OpenCode-owned skill path를 기록하지 않는다.
And user-owned 세 파일과 yielded shared file은 lifecycle 전후 byte-identical이다.
And backup은 checksum이 달라진 OMP-manifest-owned file에만 생기며 foreign/user file backup은 0개다.

### S13: 회귀·coverage·generated-surface hygiene gate

Given exact integrated candidate HEAD가 있다.
When targeted tests, race tests, `go test ./...`, strict SPEC validation, coverage, file-size, git hygiene를 실행한다.
Then 모든 test와 strict validation은 exit `0`이고 OMP 관련 changed package statement coverage는 `85.0%` 이상이다.
And 변경·신규 Go source file은 각각 300줄 이하이며 SPEC Markdown은 이 제한에서 제외된다.
And credential, raw provider payload, privileged absolute path를 담은 retained trace는 0개다.
And root meta workspace delivery에 `.omp/`, `.agents/`, `.codex/`, `.opencode/`, runtime manifest drift가 staged된 항목은 0개다.

## Oracle Acceptance Notes

- S1·S3·S4·S7은 set equality와 exact expected 값을 사용하며 파일 존재만으로 PASS하지 않는다.
- S5·S6·S12는 bytes, parsed values, permission mode, manifest membership을 함께 비교한다.
- S8·S9·S10은 syntax, catalog, credential, identity, capability 원인을 분리한다.
- S11은 actual process의 ordered behavior trace와 concrete receipt JSON을 요구하며 discovery-only 증거를 명시적으로 거부한다.
- S13의 85%는 OMP 관련 changed package statement coverage의 하한이다.
