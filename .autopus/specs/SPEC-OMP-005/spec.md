# SPEC-OMP-005: OMP 에이전트별 모델 역할 투영

---
id: SPEC-OMP-005
title: OMP 에이전트별 모델 역할 투영
version: 0.2.0
status: implemented
priority: HIGH
created: 2026-09-05
domain: OMP
supersedes: SPEC-OMP-003 Policy Contract의 native role 행렬
---

## Purpose

SPEC-OMP-003은 16개 생성 에이전트를 OMP native role 10개(`default`, `smol`, `slow`, `plan`, `vision`, `designer`, `commit`, `tiny`, `task`, `advisor`)로 접어 `modelRoles`에 투영했다. OMP는 `modelRoles`에 임의 역할 키를 받고 agent frontmatter `model: "@<role>"`로 해석하므로(실측: `--model @autopus_executor` → `claude-opus-5:xhigh`) 이 접기는 OMP 제약이 아니라 설계 선택이었고, 두 가지 부작용을 만들었다.

1. 같은 native role을 공유하는 에이전트가 max-wins로 승격된다. ultra 프리셋에서 debugger/deep-worker(fable)가 `task`를 공유해 executor/tester/devops/perf-engineer(opus)가 OMP에서만 fable로 실행된다.
2. `default`/`tiny`/`smol`/`commit`은 OMP가 메인 세션·백그라운드 작업(제목, 메모리, 커밋 메시지)에 쓰는 역할인데 프로젝트 관리 대상이 되어 사용자 전역값을 덮는다. validator(opus) 때문에 `tiny`가 `opus:xhigh`로 바뀌는 식이다.

이 SPEC은 에이전트마다 고유 역할 `autopus_<agent>`를 투영해 1:1 매핑을 만들고, native role은 어떤 산출물에도 쓰지 않는다. capability 레이어(provider-neutral 6종)는 family diversity와 hand-written 프로필의 기본 후보로 유지한다.

## Outcome Boundary

- **Outcome Lock**: `auto update`가 `.omp/config.yml`(project-managed) 또는 overlay의 `modelRoles`에 `autopus_<agent>` 16개 키만 기록하고, 각 `.omp/agents/<agent>.md`가 `model: '@autopus_<agent>'`와 그 에이전트의 quality 티어에서 파생된 `thinking`을 가지며, OMP native role 키는 Autopus가 소유하거나 기록하지 않는다.
- **Mandatory requirements**: 역할 이름 규칙, 에이전트→capability 행렬 유지, 프로필 `agents.<name>.candidates` 오버라이드, built-in 파생의 max-wins 제거, family diversity를 에이전트 역할 기준으로, receipt·doctor·agent catalog의 역할 값 교체, native role 상수 제거, 기존 native 키 ledger를 통한 교체, 역할 16개 readback의 병렬화와 신뢰 경계 유지, project-managed 모드를 떠날 때 config preimage 복원.
- **Explicit non-goals**: OMP native role 자체의 의미 변경, 사용자 전역 `modelRoles` 편집, capability 어휘 변경, Claude/Codex/Gemini 투영 변경, SPEC-OMP-004 컨텍스트 최적화, `modelTags` 메타데이터 투영.
- **Completion evidence**: S1-S11 Must oracle 통과, `go test ./...` 통과, autopus-workspace에서 `auto update` 후 `.omp/config.yml`에 native 키 0개·`autopus_*` 16개, `omp --model @autopus_executor --mode rpc get_state`가 프리셋 티어 모델을 반환, doctor `model-routing.receipt` pass, 모드 왕복 후 `.omp/config.yml` byte-identical.

## Policy Contract

| Agent | Role | Capability |
|---|---|---|
| annotator | `autopus_annotator` | `fast_validation` |
| architect | `autopus_architect` | `deep_reasoning` |
| debugger | `autopus_debugger` | `coding_tool_use` |
| deep-worker | `autopus_deep_worker` | `coding_tool_use` |
| devops | `autopus_devops` | `coding_tool_use` |
| executor | `autopus_executor` | `coding_tool_use` |
| explorer | `autopus_explorer` | `fast_validation` |
| frontend-specialist | `autopus_frontend_specialist` | `vision_design` |
| perf-engineer | `autopus_perf_engineer` | `coding_tool_use` |
| planner | `autopus_planner` | `deep_reasoning` |
| reviewer | `autopus_reviewer` | `independent_dissent` |
| security-auditor | `autopus_security_auditor` | `independent_dissent` |
| spec-writer | `autopus_spec_writer` | `deep_reasoning` |
| tester | `autopus_tester` | `coding_tool_use` |
| ux-validator | `autopus_ux_validator` | `vision_design` |
| validator | `autopus_validator` | `deterministic_transform` |

역할 이름은 `autopus_` + 에이전트 이름의 `-`를 `_`로 바꾼 값이다. capability의 대표 역할(`CanonicalOMPRoleForCapability`)은 `deep_reasoning→autopus_planner`, `coding_tool_use→autopus_executor`, `fast_validation→autopus_explorer`, `vision_design→autopus_ux_validator`, `independent_dissent→autopus_reviewer`, `deterministic_transform→autopus_validator`다. Legacy tier 변환(`LegacyTierRoute`)은 같은 capability와 대표 역할을 돌려준다.

## Requirements

### Ubiquitous

**REQ-ROLE-001 — 에이전트별 역할 이름**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL derive every OMP model role name as `autopus_` followed by the agent name with `-` replaced by `_`, SHALL emit exactly one role per canonical agent, and SHALL NOT write any OMP native role key (`default`, `smol`, `slow`, `plan`, `vision`, `designer`, `commit`, `tiny`, `task`, `advisor`) into `modelRoles`, agent frontmatter, receipts, or doctor rows.
Observability: projected `modelRoles` key set equals the 16 Policy Contract roles; `.omp/agents/<agent>.md` `model` equals `@autopus_<agent>`.

**REQ-ROLE-002 — capability 행렬 유지**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL keep the agent→capability matrix of the Policy Contract as the provider-neutral layer, SHALL fail closed with `agent_role_unmapped` for any generated agent outside the matrix, and SHALL reject a profile `agents.<name>` entry whose `role` or `capability` disagrees with the matrix with `role_capability_mismatch`.
Observability: `OMPAgentCapability`, `OMPRoleCapability`, and `ValidateOMPAgentRoleSet` results.

**REQ-ROLE-003 — 에이전트 후보 오버라이드**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL resolve an agent's ordered candidates from `agents.<name>.candidates` when present and otherwise from `capabilities.<capability>.candidates`, SHALL apply the capability route's `required`/`degraded_action` to both, and SHALL validate override candidates with the same selector, thinking, family, and operator-attestation rules as capability candidates.
Observability: `RoleModelProfileConf.AgentCandidates(agent)` and validation error codes.

**REQ-ROLE-004 — family diversity는 역할 단위**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL accept `family_diversity.roles` values only from the Policy Contract role set and SHALL request a distinct executor family for exactly those agents.
Observability: route request `prefer_distinct_executor_family` per agent.

**REQ-READBACK-001 — 역할 readback의 병렬화와 신뢰 경계**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL read back every projected role through one OMP RPC process per role with at most four processes in flight, SHALL accept only the argv shape `--config <absolute path> --model @<identifier> --mode rpc --no-tools --no-skills --no-extensions` (`SafeOMPModelRoleRPCArgs`), SHALL bound each process's output by the probe's `maxOutput`, SHALL forward the overlay only through `--config` plus `PI_CONFIG_FILES`, and SHALL fail the whole readback on the first role whose resolved selector differs from the projection.
Observability: `readOMPModelRolesViaRPC` worker bound, argv allowlist rejection, `activation role readback mismatch: <role>` error, and `auto update` wall time.

### Event-driven

**REQ-BUILTIN-001 — built-in 파생은 에이전트 티어를 그대로 투영한다**
Type: Event-driven | Priority: Must
WHEN `role_model_policy.profile` names a built-in profile (`ultra`, `balanced`), THE SYSTEM SHALL populate `agents.<name>.candidates` for every canonical agent from that agent's own quality tier on the anchor family ladder (`independent_dissent` agents on the counterpart family), SHALL keep the capability routes as max-wins defaults only, and SHALL set `family_diversity.roles` to `autopus_reviewer` and `autopus_security_auditor`.
Observability: for the shipped ultra preset, `autopus_executor` resolves to the opus rung while `autopus_debugger` resolves to the fable rung.

**REQ-PROJ-001 — 투영·receipt·doctor는 에이전트 키로 동작한다**
Type: Event-driven | Priority: Must
WHEN routes are compiled, THE SYSTEM SHALL key route requests, resolutions, projection rows, receipt role rows, and doctor role checks by agent, SHALL keep fallback chains keyed by effective selector with the existing conflict detection, and SHALL keep the RPC readback of every projected role.
Observability: receipt `roles[].role` values are Policy Contract roles; doctor emits 16 `model-routing.role.*` checks.

**REQ-OWN-002 — project-managed 모드를 떠나면 preimage를 복원한다**
Type: Event-driven | Priority: Must
WHEN an update's file set no longer manages `.omp/config.yml` while an ownership ledger exists, THE SYSTEM SHALL validate the current file against the ledger's last emitted bytes, SHALL restore the ledger preimage (delete the file when the preimage was missing, otherwise write the original bytes with the original mode), and SHALL prune the ledger, so that a later project-managed apply starts from a clean takeover.
Observability: after switching to overlay mode neither `.omp/config.yml` (when it did not pre-exist) nor `.autopus/omp-model-project-ownership-v1.json` remains; switching back succeeds and yields byte-identical managed output.

### Unwanted

**REQ-OWN-001 — native 키는 소유하지 않는다**
Type: Unwanted | Priority: Must
IF a project-managed `.omp/config.yml` already carries native role keys written by SPEC-OMP-003, THEN THE SYSTEM SHALL replace the whole ledger-owned `modelRoles` value with the agent role set so that native keys fall back to the user's global settings; IF `.omp/config.yml` carries a `modelRoles` value the ledger does not own, THEN THE SYSTEM SHALL fail closed with `managed_key_conflict` and SHALL leave the file byte-identical.
Observability: post-update `.omp/config.yml` contains no native role key; `omp config get modelRoles` shows global native values plus project agent roles; the conflict path leaves the original bytes untouched and writes no integration artifact.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-ROLE-001 | T1, T3 | S1, S6 | INV-001, INV-002 |
| REQ-ROLE-002 | T1 | S2 | INV-003 |
| REQ-ROLE-003 | T1, T3 | S3 | INV-004 |
| REQ-ROLE-004 | T1, T3 | S4 | INV-003 |
| REQ-READBACK-001 | T7 | S11 | INV-008 |
| REQ-BUILTIN-001 | T2 | S5 | INV-004, INV-005 |
| REQ-PROJ-001 | T3, T4 | S6, S7 | INV-001, INV-006 |
| REQ-OWN-002 | T8 | S10 | INV-007 |
| REQ-OWN-001 | T5 | S8, S9 | INV-002, INV-007 |

## Out of Scope

Outcome Boundary의 explicit non-goals와 동일하다.
