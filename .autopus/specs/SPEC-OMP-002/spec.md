# SPEC-OMP-002: OMP 워크플로 실행 파리티 및 설정 안전성

**Status**: completed
**Created**: 2026-08-02
**Domain**: OMP

## 목적

`SPEC-OMP-001`이 설치·발견 최소 표면을 제공한 뒤에도 OMP-only 생성물은 상세 workflow skill을 만들지 않는다. 현재 `.agents/commands/auto*.md`는 matching `auto-*` skill을 로드하라고 지시하지만 `pkg/adapter/omp/omp_skills.go`는 얇은 `auto` 하나만 생성한다. 이 SPEC은 실제 command → skill → context → tool → completion 경로를 닫고, `.omp/config.yml`의 사용자 내용·mode를 lifecycle 전체에서 보존하며, doctor가 디렉터리 존재가 아니라 실행 readiness 값을 판정하도록 만든다.

## Outcome Boundary

**User-visible outcome**: OMP-only workspace에서 `/auto`, `/auto-plan`, `/auto-go`, `/auto-review`를 포함한 workflow command가 matching 상세 계약을 발견하고, plan/test/canary/go context profile과 worker receipt 계약을 동일하게 전달한다. 대표 `/auto plan`은 인증 없는 fake provider를 사용한 실제 OMP RPC turn에서 상세 skill을 로드하고 tool을 실행한 뒤 완료한다. Generate/Update/Clean 이후 사용자 config bytes와 기존 mode가 보존되며 doctor는 YAML, surface set/count, command-skill binding, model selector discovery, version/capability readiness를 값으로 검증한다.

**Mandatory requirements**: REQ-001부터 REQ-012까지 전체다.

**Explicit non-goals**

- OMP `modelRoles`, fallback chain, provider allowlist 또는 역할별 모델 정책 생성. 이는 `[NEW] SPEC-OMP-003`의 독립 Outcome Lock이다.
- OMP compaction, memory, advisor를 이용한 context-native 최적화. 이는 `[NEW] SPEC-OMP-004`의 독립 Outcome Lock이다.
- OMP를 `orchestra.providers.omp` 또는 worker `ProviderAdapter`로 등록하는 일.
- 실 credential, OAuth, 상용 모델 요청 및 응답 품질 평가.
- `.omp/config.yml` Autopus marker에 `skills.customDirectories` 외 키를 추가하는 일.
- root meta workspace의 generated platform surface를 commit 대상으로 바꾸는 일.

**Completion evidence**: OMP workflow/context/config/readiness 단위·통합 테스트와 기존 adapter regression이 green이고, OMP 관련 변경 패키지 statement coverage가 85% 이상이며, 설치 OMP의 version/capability receipt를 남긴 auth-free actual RPC workflow smoke가 ordered skill load, context body, tool event, response completion을 모두 증명한다. `auto spec validate ... --strict`, `go test ./...`, root generated-surface hygiene gate가 통과한다.

## Requirements

### REQ-001 — Workflow 상세 skill 전량 방출

WHEN an OMP-only configuration generates the shared skill surface, THEN THE SYSTEM SHALL emit exactly 20 workflow skill files whose names equal the 20-entry command set: one thin canonical `auto` router and 19 detailed `auto-*` workflow skills.

- EARS type: Event-driven / Priority: Must
- 19개 detailed skill은 각 workflow의 실행 단계와 플랫폼 정규화된 tool·path contract를 포함한다. 얇은 `auto` router만 있고 matching detail이 없는 상태는 허용하지 않는다.
- extended skill emitter는 `workflowSpecs` 멤버 이름을 제외해 workflow emitter와 같은 target을 중복 생성하지 않는다.
- 관측 지점: generated workflow skill relative-path set, `workflowSpecs` name set, target-path duplicate count `0`, router body oracle, 19개 detailed skill body hash와 필수 section.

### REQ-002 — Router와 direct alias의 단일 emitted routing contract

WHEN OMP command and skill surfaces are emitted, THEN THE SYSTEM SHALL encode `/auto <subcommand> ...` and the corresponding `/auto-<subcommand> ...` alias with the same exact target detail name from `workflowSpecs`, preserve each supplied `--model <provider/model>` and `--variant <value>` placeholder exactly once, and include an explicit no-fuzzy-fallback clause for unknown subcommands.

- EARS type: Event-driven / Priority: Must
- `/auto`는 canonical router이고 direct alias는 같은 target을 지시하는 짧은 진입점이다. 이 SPEC은 OMP 내부에 존재하지 않는 deterministic runtime parser나 exact runtime error string을 발명하지 않는다.
- 관측 지점: rendered command/router body의 exact target map, flag placeholder count, no-fuzzy clause. 실제 turn은 REQ-011의 content-gated representative wiring 증거로 별도 검증한다.

### REQ-003 — Context profile cross-adapter parity

WHERE OMP workflow skills define plan, test, canary, or go execution, THE SYSTEM SHALL expose the same required, optional or worker-optional, and excluded context sets that the canonical cross-adapter context engineering oracle expects.

- EARS type: State-driven / Priority: Must
- exact matrix: plan required `architecture,core,relevant_spec`, optional `learning,signature`, excluded `canary,test`; test required `core,test`, optional `learning,signature`, excluded `canary`; canary required `canary,core`, optional `learning`, excluded `signature,test`; go required `acceptance,available_architecture,core,plan,resolved_spec`, worker-optional `learning,signature,task_declared_extra`, excluded `canary,test`.
- 관측 지점: OMP generated plan/test/canary/go skill의 `## Context Profile` parse result와 기존 surface matrix 비교.

### REQ-004 — Worker receipt와 context trust boundary parity

WHEN an OMP go workflow delegates work, THEN THE SYSTEM SHALL require the five worker return fields `owned_paths`, `changed_files`, `verification`, `blockers`, and `next_required_step`, and SHALL preserve the canonical project-relative ref, traversal rejection, sanitization, redaction, raw-provider non-replay, selected-ref, hash, omitted-count, and bounded-token clauses.

- EARS type: Event-driven / Priority: Must
- receipt field는 이름 집합이 정확히 5개이며 추가·누락 없이 canonical contract와 같다.
- existing canonical source는 `content/skills/agent-pipeline.md`이고 `coreSkillSet`, default OMP compile target, `resolveDefaultSkillTarget`가 `.agents/skills/agent-pipeline/SKILL.md`로 방출한다. OMP `auto-go` detail은 이 상대 경로를 정확히 1회 참조해야 한다.
- 관측 지점: catalog state `Registered=true, Compiled=true, TargetPath=.agents/skills/agent-pipeline/SKILL.md`, generated file 존재, auto-go exact-one ref, route-resolved body의 field set과 security/evidence clause polarity.

### REQ-005 — Generate와 Update의 좁은 structural config 소유권

WHEN Generate or Update rewrites `.omp/config.yml`, THEN THE SYSTEM SHALL structurally own only the full desired list at `skills.customDirectories`, preserve bytes and sibling YAML keys outside that managed ownership, preserve the existing regular file permission bits, and create a new config with mode `0600`.

- EARS type: Event-driven / Priority: Must
- Autopus managed YAML은 기존 `skills.customDirectories: [.agents/skills]` 범위로 고정한다. 다른 array key를 병합하거나 소유하지 않는다.
- user `skills` mapping이 없으면 marker가 top-level `skills` subtree 전체를 감싼다. user `skills` mapping이 있으면 동일 marker 한 쌍을 그 mapping 내부에 같은 indentation으로 삽입해 `customDirectories` entry만 감싼다. Clean은 첫 형태에서 subtree 전체를, 둘째 형태에서 entry만 제거한다.
- marker 밖에 같은 owned key가 있거나 `skills`가 mapping이 아니거나 marker가 root 또는 `skills` direct-child 이외 위치에 있거나 YAML·marker 경계가 모호하면 원본을 byte-identical로 두고 충돌을 반환한다.
- 관측 지점: pre/post marker-outside byte slices, preserved sibling `skills` nodes, `os.FileMode.Perm()`, parsed YAML, conflict result.

### REQ-006 — Clean의 config 내용·mode 보존

WHEN Clean removes the Autopus-owned `skills.customDirectories` node from a `.omp/config.yml` that retains user content, THEN THE SYSTEM SHALL write the remainder with the file's pre-Clean permission bits and byte-preserved user-owned YAML; if no user content remains, the system shall remove only that file and empty managed parents; and the platform CLI shall propagate every Clean error to its caller.

- EARS type: Event-driven / Priority: Must
- mode `0600` fixture가 Clean 후 `0644`로 확대되면 실패다. structural conflict·unsafe path·write failure를 성공으로 삼아도 실패다.
- `auto platform remove omp`는 mutation 전 structural/ownership preflight를 수행하고 Clean 성공 뒤에만 platform config를 저장한다. preflight/Clean 실패 시 `autopus.yaml`은 byte-identical로 OMP를 계속 포함하며, late partial failure는 changed-path receipt와 repair-required 상태를 남긴다. 기존 "caller discards error" 전제 주석은 함께 갱신한다.
- 관측 지점: Clean 전후 user-owned YAML bytes/nodes, marker absence, exact mode, adjacent `.omp/` user file set, `internal/cli/platform.go` 반환 오류.

### REQ-007 — 값 기반 OMP surface 검증

WHEN OMP Validate or doctor inspects a generated workspace, THEN THE SYSTEM SHALL parse the config as YAML; require `skills.customDirectories` to equal `[".agents/skills"]`; derive canonical rule filenames from `content/rules/*.md`, agent filenames from `LoadAgentSourcesFromFS(content/agents)`, workflow names from `workflowSpecs`, and required pipeline targets from the compiled skill catalog; compare each generated set to that source-derived set; and emit separate expected/got findings even when every parent directory exists.

- EARS type: Event-driven / Priority: Must
- current baseline evidence는 rules 14, agents 16, commands 20, workflow skills 20이지만 숫자는 validator의 source of truth가 아니다. catalog 변화 시 expected set과 count가 함께 파생된다. mixed-platform 구성은 ownership predicate가 계산한 OMP 소유 set과 실제 set을 비교해 양보된 shared surface를 OMP 누락으로 오판하지 않는다.
- 관측 지점: `ValidationError` list, doctor text/JSON IDs and details, canonical vs actual relative-path sets.

### REQ-008 — Model selector discovery readiness 검증

WHERE a generated OMP agent declares a model selector, THE SYSTEM SHALL execute bounded `omp models --json` against an isolated test-owned profile, require exact top-level JSON shape `{"models":[...]}`, normalize exact catalog selectors, validate selector syntax, and verify resolution without issuing a completion request; unresolved/malformed/catalog-empty/catalog-invalid/catalog-timeout/catalog-oversized states shall be distinct errors, while missing credentials for an otherwise discovered provider shall be a distinct readiness warning.

- EARS type: State-driven / Priority: Must
- selector 정책 생성이나 provider 선택은 하지 않는다.
- 관측 지점: selector, resolved catalog candidates, provider availability classification, model request count `0`.

### REQ-009 — OMP version·capability·provenance gate

WHEN OMP readiness is evaluated, THEN THE SYSTEM SHALL resolve the executable path, require `--version` output with the `omp/` identity shape, record the observed version, probe the exact capability IDs below through their named interfaces, and reject or mark unsupported any absent capability instead of assuming current `main` documentation behavior.

- EARS type: Event-driven / Priority: Must
- version prefix는 accidental collision 방지이며 hostile binary에 대한 보안 증명이 아니다.
- reference `omp/17.1.8` expected IDs: `identity.version`=`omp --version` matches `^omp/[0-9]+\.[0-9]+\.[0-9]+$`; `launch.rpc`=`omp --help` advertises `--mode` value `rpc`; `launch.no_session`=`--no-session`; `launch.cwd`=`--cwd`; `launch.model`=`--model`; `config.overlay_readback` uses task-owned `PI_CONFIG_FILES=<overlay>` because this installed version rejects root `--config` on the `config` subcommand, and strictly accepts either the compatibility raw array or the observed `{key,value,type,description}` wrapper whose value is `[".agents/skills"]`; `catalog.models_json`=`omp models --json` parses `{"models":[...]}`; `rpc.command_discovery` emits `available_commands_update`; `rpc.tool_events` emits paired `tool_execution_start/end`; `rpc.terminal` emits `message_end`.
- unknown/new versions run the same probes and record supported=false with `flag_missing`, `output_invalid`, `event_missing`, `timeout`, or `exit_nonzero`; a version string alone never sets another capability true.
- 관측 지점: resolved executable, version string, per-ID command/event, expected/got, bounded timeout/error classification.

### REQ-010 — F-010 runtime ancestry authority 제한

WHERE process ancestry yields `AgentRuntimeOMP` from a bare executable name or known Node entrypoint, THE SYSTEM SHALL treat that result as a telemetry or UX hint only and SHALL NOT use it to select an orchestra provider, grant permissions, bypass approval, establish executable provenance, or decide judge/model authority.

- EARS type: State-driven / Priority: Must
- adapter activation과 readiness는 REQ-009의 resolved executable/version/capability evidence를 사용한다.
- 관측 지점: runtime-to-provider mapping result `""`, permission/judge/provider decision table, wrong-version PATH fixture.

### REQ-011 — Actual OMP RPC workflow smoke

WHEN the OMP live smoke gate runs, THEN THE SYSTEM SHALL launch the actual capability-approved OMP binary in RPC mode against an isolated OMP-only scratch workspace and an auth-free content-gated loopback fake provider, execute representative `/auto plan` and `/auto-plan` turns, observe command expansion followed by `skill:auto` and `skill:auto-plan` tool events, verify the loaded body hashes and plan context profile, observe at least one bounded workspace tool event, and receive `message_end`.

- EARS type: Event-driven / Priority: Must
- the fake provider may request the next tool only after the preceding actual OMP request contains the expected command or skill body hash; otherwise it terminates with a failing fixture receipt. This proves representative expansion/wiring, not a general deterministic OMP parser or LLM quality.
- command discovery, scripted events without content gates, system prompt dump, non-empty output, or exit code alone cannot satisfy this requirement.
- 관측 지점: sanitized ordered RPC trace, fake-provider request count, loaded skill body hash/profile, tool target under scratch root, terminal frame.

### REQ-012 — Ownership·hygiene·회귀 게이트

WHILE implementing and verifying OMP execution parity, THE SYSTEM SHALL preserve user-owned `.omp/` files and bytes, mutate only OMP-manifest-owned generated paths, honor mixed-platform ownership of shared `.agents/` surfaces, keep root meta workspace generated surfaces out of commits, and maintain at least 85% statement coverage for OMP-related changed packages.

- EARS type: State-driven / Priority: Must
- 검증 fixture와 trace는 credential, raw provider payload, absolute privileged path를 영구 artifact에 기록하지 않는다.
- 관측 지점: manifest path set, pre/post byte snapshots, `git status`, `git ls-files -c -i --exclude-standard`, coverage report, secret-pattern scan.

## 생성·수정 파일 상세

### Existing source of truth to modify

- `pkg/adapter/omp/omp_skills.go`, `omp_specs.go`, `omp_commands.go`: workflow detail emission과 single resolution contract.
- `pkg/adapter/omp/omp_lifecycle.go`, `omp.go`, `omp_config.go`: value-based Validate와 narrow structural YAML/mode-preserving lifecycle.
- `pkg/adapter/context_engineering_test_helpers_test.go`, `context_engineering_acceptance_test.go`: OMP surface와 context/receipt parity oracle.
- `pkg/detect/runtime.go`, `pkg/detect/platform_identity.go`, `internal/cli/orchestra_invoker_test.go`: hint-only ancestry와 executable identity 경계.
- `internal/cli/doctor*.go`: 필요 시 stable OMP readiness finding의 text/JSON projection.
- `internal/cli/platform.go`: adapter Clean 오류를 호출자에게 전파하는 lifecycle 경계.

### [NEW] planned additions

- `[NEW] pkg/adapter/omp/omp_workflow_parity_test.go`: 20-entry detail, alias, context binding oracle.
- `[NEW] pkg/adapter/omp/omp_readiness.go`: version/capability/surface/model readiness의 OMP-local 구현. 실제 분할은 300줄 제한에 맞춘다.
- `[NEW] pkg/adapter/omp/omp_readiness_test.go`: value mismatch와 selector fixture.
- `[NEW] pkg/adapter/omp/omp_rpc_workflow_test.go`: opt-in actual OMP RPC smoke와 ordered trace oracle.
- `[NEW] pkg/adapter/omp/omp_fake_provider_test.go`: auth-free loopback provider fixture. raw request body를 유지하지 않는다.

Generated `.agents/**`, `.omp/**`, `.autopus/omp-manifest.json`은 scratch 검증 surface이며 source of truth가 아니다.

## Related SPECs

- `SPEC-OMP-001` — completed prerequisite: OMP 최소 설치·발견 표면.
- `[NEW] SPEC-OMP-003` — planned related Primary SPEC: OMP 멀티프로바이더 semantic model routing. 이 SPEC의 sibling이 아니다.
- `[NEW] SPEC-OMP-004` — planned related Primary SPEC: OMP context-native optimization. 이 SPEC의 sibling이 아니다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T2 | S1 | INV-001 |
| REQ-002 | T3 | S2 | INV-002 |
| REQ-003 | T2, T4 | S1, S3, S11 | INV-003 |
| REQ-004 | T4 | S4 | INV-004 |
| REQ-005 | T5 | S5 | INV-005 |
| REQ-006 | T5 | S6 | INV-006 |
| REQ-007 | T6, T8 | S7, S10 | INV-007 |
| REQ-008 | T6, T7 | S8, S10 | INV-008 |
| REQ-009 | T7 | S9, S11 | INV-009 |
| REQ-010 | T7 | S9 | INV-010 |
| REQ-011 | T9 | S11 | INV-011 |
| REQ-012 | T10, T11, T12 | S12, S13 | INV-012 |
