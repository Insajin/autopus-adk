# SPEC-OMP-003: OMP 역할 기반 멀티프로바이더 모델 라우팅

**Status**: implemented
**Created**: 2026-08-02
**Domain**: OMP

## 목적

사용자가 opt-in OMP runtime profile을 선택하면 Autopus의 agent 역할·위험·품질 의도를
provider-neutral capability policy로 해석하고, 이를 OMP native `modelRoles`, agent selector와
thinking, ordered fallback chain으로 결정적으로 컴파일한다. doctor와 secret-free receipt는 실제
effective provider/model과 선택 이유를 증명한다.

## Outcome Boundary

### Outcome Lock

- **User-visible outcome**: opt-in profile을 선택한 OMP 사용자는 planner/executor/reviewer 등 각
  Autopus 역할이 현재 설치된 OMP와 auth-enabled catalog에서 어떤 provider/model/thinking으로
  실행되는지 확인하고, availability fallback과 provider-family 독립 검토를 구분할 수 있다.
- **Mandatory requirements**: REQ-001~REQ-018 전체다.
- **Explicit non-goals**: OMP gateway, `orchestra.providers.omp`, worker `ProviderAdapter`, subprocess,
  pane, provider quorum 대체, credential 관리, SPEC-OMP-004의 context integrity 최적화다.
- **Completion evidence**: S1~S18 hermetic oracle, routing-owned changed Go files 85%+ coverage,
  T1 package baseline no-regression, strict validation, full Go test/vet/build, receipt ignore hygiene,
  그리고 별도 명시 승인 시 S19 1-call live canary다.
- **Dependency**: SPEC-OMP-002가 completed 상태여야 구현을 시작한다. SPEC-OMP-004는 독립 Primary다.

## Implementation Status

- **Lifecycle state**: 기술 구현은 `implemented`이며 commit/push와 lifecycle `completed` 승격은 승인되지 않았다.
- **Implemented evidence**: T2~T11의 provider-neutral policy, installed catalog normalization, deterministic routing/projection, 0600 overlay, agent mapping, receipt/doctor, safety와 integration surface가 현재 source tree에 존재한다.
- **Predecessor state**: T1의 SPEC-OMP-002 technical gate는 green이지만 exact integrated commit/HEAD에서의 lifecycle status promotion은 deferred다.
- **Pending verification**: T12 integrated coverage, full test/vet/build, strict validation, git hygiene는 parent gate 전이며, S19는 `not-run: explicit approval required`로 S1~S18을 막지 않는다.

## Policy Contract

### Provider-neutral capabilities

`deep_reasoning`, `coding_tool_use`, `fast_validation`, `vision_design`, `independent_dissent`,
`deterministic_transform`를 v1 capability vocabulary로 사용한다. 이 이름은 vendor/model 이름이
아니며 실제 selector 후보와 family metadata는 사용자가 선택한 profile/catalog가 소유한다.

### OMP projection matrix

| OMP native role | Capability | Current generated Autopus agents |
|-----------------|------------|-----------------------------------|
| `default` | `coding_tool_use` | none; non-agent supervisor entry only |
| `smol` | `fast_validation` | explorer, annotator |
| `slow` | `deep_reasoning` | architect |
| `plan` | `deep_reasoning` | planner, spec-writer |
| `vision` | `vision_design` | ux-validator |
| `designer` | `vision_design` | frontend-specialist |
| `commit` | `deterministic_transform` | none; non-agent transform entry only |
| `tiny` | `deterministic_transform` | validator |
| `task` | `coding_tool_use` | executor, debugger, devops, tester, perf-engineer, deep-worker |
| `advisor` | `independent_dissent` | reviewer, security-auditor |

Capability만 주어진 legacy 또는 generic 입력의 canonical native role은
`deep_reasoning→plan`, `coding_tool_use→task`, `fast_validation→smol`,
`vision_design→vision`, `independent_dissent→advisor`, `deterministic_transform→tiny`다.
현재 `LoadAgentSourcesFromFS(content/agents)`의 16개 이름은 위 agent column과 exact set equality를
이뤄야 하며 새 source agent가 explicit mapping 없이 나타나면 `agent_role_unmapped`로 fail closed한다.
Agent override의 exact native role/capability pair가 있으면 이 기본값보다 우선하고 pair가 행렬과
다르면 `role_capability_mismatch`다. 동일 role의 후보 정렬은 profile 순서, exact selector,
provider/model lexical tie-break 순이다. source `opus/sonnet/haiku`는 각각
`deep_reasoning/plan`, `coding_tool_use/task`, `deterministic_transform/tiny`로만 변환한다.

## Requirements

### REQ-001 — 명시적 opt-in과 선행 게이트

WHERE an OMP runtime profile is not selected, THE SYSTEM SHALL preserve the SPEC-OMP-002 OMP output
byte-for-byte; WHEN a profile is selected, THEN THE SYSTEM SHALL compile it only after the
SPEC-OMP-002 compatibility gate succeeds.

- EARS type: Optional + Event-driven / Priority: Must
- 관측 지점: generated diff, dependency gate, receipt `profile`과 `config_source`

### REQ-002 — provider-neutral 정책 스키마

THE SYSTEM SHALL provide a backward-compatible `[NEW] RoleModelPolicyConf` whose profiles map Autopus
agents and OMP native roles to semantic capabilities and ordered selector candidates without adding
`omp` to `QualityConf.Providers`.

- EARS type: Ubiquitous / Priority: Must
- 관측 지점: config round-trip과 validation errors

### REQ-003 — legacy tier 호환 변환

WHEN an opt-in profile omits an agent override and a legacy quality preset supplies `opus`, `sonnet`,
or `haiku`, THEN THE SYSTEM SHALL map that tier through a versioned compatibility table to a semantic
capability and OMP native role; IF the tier is unknown, THEN the system SHALL reject the policy.

- EARS type: Event-driven + Unwanted / Priority: Must
- 관측 지점: compiler result의 `requested_role`, `capability`, `legacy_source`

### REQ-004 — 모델 이름 소유권

THE SYSTEM SHALL keep concrete provider/model selectors out of default agent/rule/template source and
resolve them only from the selected config profile plus the probed runtime catalog.

- EARS type: Ubiquitous / Priority: Must
- 관측 지점: source golden scan과 resolution input

### REQ-005 — 설치 버전과 capability probe

WHEN OMP routing is compiled, THEN THE SYSTEM SHALL run a bounded identity/version/settings/model-catalog
probe, fingerprint normalized non-secret catalog metadata, and gate every used role, selector, thinking
level, fallback key, approval key, and isolation key against the installed version's observed capability.

- EARS type: Event-driven / Priority: Must
- 관측 지점: `omp_version`, `capability_probe`, `catalog_fingerprint`

### REQ-006 — auth-enabled exact selector 해석

WHEN a selector candidate is evaluated, THEN THE SYSTEM SHALL require an exact canonical
`provider/model` match that is not disabled, is keyless or auth-enabled, advertises the requested
capability, supports the requested thinking level, and rejects fuzzy near-matches with an explicit reason.

- EARS type: Event-driven / Priority: Must
- 관측 지점: per-role requested/effective selector와 reason

### REQ-007 — 결정적 fallback과 degraded 경계

IF a primary selector is unavailable, unauthorized, disabled, or lacks thinking support, THEN THE
SYSTEM SHALL evaluate fallback candidates in declared order, select the first exact compatible entry,
and record the skipped reasons; IF no candidate resolves, THEN it SHALL fail closed for required roles
or use only an explicitly configured degraded action with a non-empty reason.

- EARS type: Unwanted / Priority: Must
- 관측 지점: ordered `fallback_attempts`, effective selector, `degraded_reason`

### REQ-008 — fallback과 provider quorum 분리

THE SYSTEM SHALL label OMP fallback as availability recovery only and SHALL NOT count a fallback,
`advisor`, or repeated call as provider quorum, consensus, independent debate, or judge evidence.

- EARS type: Ubiquitous / Priority: Must
- 관측 지점: receipt `evidence_class=availability`와 existing orchestra regression

### REQ-009 — reviewer/security family 다양성

WHEN reviewer or security-auditor is paired with an executor result, THEN THE SYSTEM SHALL prefer an
exact compatible selector whose declared model family differs from the executor's effective family;
IF none is available, THEN it SHALL emit `family_diversity.status=degraded` with both family IDs and
must not claim independent-provider evidence.

- EARS type: Event-driven + Unwanted / Priority: Must
- 관측 지점: paired role receipt의 family diversity object

### REQ-010 — OMP role과 agent projection

WHEN agent mappings are prepared, THEN `[NEW] prepareAgentMappings(cfg, compiledPolicy)` SHALL compare
all names loaded by `LoadAgentSourcesFromFS(content/agents)` with the exact Policy Contract mapping,
fail with `agent_role_unmapped` for any unowned name, emit a supported native role alias or exact
frontmatter selector/thinking derived from the same resolution used for `modelRoles`, and compile ordered
OMP fallback chains without raw `sonnet`/`opus` passthrough.

- EARS type: Event-driven / Priority: Must
- 관측 지점: `.omp/agents/*.md`, effective OMP profile, receipt tuple equality

### REQ-011 — 설정 소유권과 투영 모드

WHERE profile `config_mode=overlay`, THE SYSTEM SHALL generate an owner-only explicit OMP overlay,
leave `.omp/config.yml` byte-identical, pass that exact overlay explicitly at every managed OMP
invocation boundary, and verify effective readback before claiming activation; in
`config_mode=project-managed`, the system updates only explicitly claimed complete key paths after
YAML-aware duplicate/conflict and prior-fingerprint checks, otherwise fail closed without partial writes.

- EARS type: Optional / Priority: Must
- 관측 지점: before/after bytes, mode, invocation argv/config hash, effective readback, managed-key receipt

### REQ-012 — array replacement 안전성

IF a projection would write an OMP array such as `enabledModels`, `disabledProviders`, or `cycleOrder`,
THEN THE SYSTEM SHALL require explicit full-array ownership and preserve the complete intended effective
array and must not append under the assumption that OMP arrays merge.

- EARS type: Unwanted / Priority: Must
- 관측 지점: projected YAML and effective-config oracle

### REQ-013 — 사용자 데이터와 secret 보존

THE SYSTEM SHALL preserve unmanaged keys, comments, environment placeholders, file bytes and existing
permission mode outside owned output, keep newly generated profile files at `0600`, and must not read
or persist credential values, tokens, `.env` contents, auth database contents, or literal secrets in
generated files, logs, doctor output, fingerprints, or receipts.

- EARS type: Ubiquitous / Priority: Must
- 관측 지점: adversarial byte/mode fixture and secret sentinel scan

### REQ-014 — opt-in safety profile

WHERE an OMP safety profile explicitly claims `tools.approvalMode` or `task.isolation.mode`, THE SYSTEM SHALL apply
only values supported by the installed version and record their effective source; otherwise it
preserves user values. Observed local values must not be treated as universal defaults.

- EARS type: Optional / Priority: Must
- 관측 지점: effective settings and `safety` receipt

### REQ-015 — resolution receipt

WHEN routing compilation completes, THEN THE SYSTEM SHALL atomically write a versioned secret-free
`[NEW] .autopus/omp-model-resolution-v1.json` containing OMP version, catalog fingerprint, profile,
config source, activation argv/config hash and effective readback hash, requested/effective role, capability, requested/effective provider/model selector,
thinking, fallback attempts/reason, degraded reason, family diversity, safety source, and generated-at
metadata in canonical role order. Its `resolution_digest` hashes the canonical resolution body while
excluding `generated_at` so identical policy/version/catalog/config inputs retain a stable identity.

- EARS type: Event-driven / Priority: Must
- generated runtime receipt는 `.gitignore`의 exact `.autopus/omp-model-resolution-v1.json` rule로
  ignored 상태여야 하고 tracked index entry가 없어야 한다.
- 관측 지점: parsed receipt schema, deterministic payload hash, `git check-ignore`, `git ls-files`

### REQ-016 — doctor 검증

WHEN `auto doctor` evaluates an opt-in OMP profile, THEN THE SYSTEM SHALL independently probe effective
settings/catalog, verify receipt freshness and projection equality, and report per-role supported,
degraded, or blocked status with exact reason codes without exposing secret material.

- EARS type: Event-driven / Priority: Must
- 관측 지점: doctor text/JSON role rows

### REQ-017 — platform/provider와 context 경계

THE SYSTEM SHALL preserve `PlatformToProvider("omp") == ""`, SPEC-OMP-001 REQ-018, existing orchestra
provider/quorum behavior, and the canonical classification/build/verification authority already owned
by `pkg/promptlayer` and the completed context-engineering SPECs; SPEC-OMP-004 may coordinate only the
OMP session lifecycle bridge without replacing that authority.

- EARS type: Ubiquitous / Priority: Must
- 관측 지점: regression tests and unchanged context profiles

### REQ-018 — 검증과 live 비용 경계

THE SYSTEM SHALL verify routing with hermetic fake OMP/version/catalog/auth fixtures, adversarial config
fixtures, deterministic oracles, at least 85% statement coverage over new or modified routing-owned Go
production files as reported from one coverage profile, and no decrease from the exact pre-change
package baseline captured by T1; when a live canary is explicitly approved, it performs at most one
provider call and records only redacted selection and usage metadata.

- EARS type: Ubiquitous + Optional / Priority: Must
- 관측 지점: test reports, coverage, canary consent and redacted receipt

## 생성 파일 상세

| 경로 | 역할 |
|------|------|
| `[NEW] pkg/config/role_model_policy.go` | provider-neutral policy schema, validation, legacy conversion |
| `[NEW] pkg/adapter/omp/omp_model_catalog.go` | bounded version/settings/models probe와 catalog fingerprint |
| `[NEW] pkg/adapter/omp/omp_model_routing.go` | capability resolver와 OMP role/fallback projection |
| `pkg/adapter/omp/omp_agents.go` | cfg/compiled policy를 받는 agent mapping |
| `pkg/content/agent_transformer_omp.go` | resolved selector/thinking frontmatter 출력 |
| `pkg/adapter/omp/omp_config.go` | overlay/project-managed ownership과 projection |
| `pkg/adapter/omp/omp_lifecycle.go` | doctor용 validation과 receipt freshness |
| `[NEW] pkg/adapter/omp/omp_model_receipt.go` | canonical secret-free receipt schema |
| `[NEW] .autopus/omp-model-resolution-v1.json` | exact ignored/untracked generated runtime evidence; source가 아님 |
| `.gitignore` | exact generated receipt ignore rule |
| `[NEW] scripts/verify-changed-go-coverage.sh` | base SHA/package baseline과 changed-file coverage gate |

## Related SPECs

- **SPEC-OMP-001**: completed 최소 플랫폼 표면. REQ-018 경계를 보존한다.
- **SPEC-OMP-002**: required predecessor. `SPEC-OMP-001 → SPEC-OMP-002 → SPEC-OMP-003` 순서에서
  completed 상태와 parity/safety gates가 구현 시작 조건이다.
- **SPEC-OMP-004**: 독립 Primary. 기존 `pkg/promptlayer` authority를 소비하는 OMP lifecycle bridge를 소유하며 이 SPEC의 sibling이 아니다.

Sibling SPEC은 없다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1, T10 | S1, S19 | INV-001 |
| REQ-002 | T2 | S2, S3 | INV-002, INV-009 |
| REQ-003 | T2, T4 | S3 | INV-003, INV-009 |
| REQ-004 | T2, T11 | S2, S18 | INV-002 |
| REQ-005 | T3 | S4, S5, S15 | INV-011 |
| REQ-006 | T3, T4 | S4, S5 | INV-004 |
| REQ-007 | T4 | S5, S6, S7 | INV-004 |
| REQ-008 | T4, T10 | S6, S16 | INV-010 |
| REQ-009 | T4 | S8, S9 | INV-005 |
| REQ-010 | T5, T7 | S2, S6 | INV-003, INV-004 |
| REQ-011 | T6 | S10, S11 | INV-006, INV-007 |
| REQ-012 | T6 | S11 | INV-007 |
| REQ-013 | T6, T9 | S10, S12, S14 | INV-006, INV-008 |
| REQ-014 | T6, T9 | S13 | INV-012 |
| REQ-015 | T8 | S14, S17 | INV-002, INV-008 |
| REQ-016 | T8 | S14, S15 | INV-008, INV-011 |
| REQ-017 | T10 | S1, S16 | INV-001, INV-010 |
| REQ-018 | T12 | S17, S18, S19 | INV-002, INV-011 |

## Out of Scope

Outcome Boundary의 explicit non-goals와 동일하다.
