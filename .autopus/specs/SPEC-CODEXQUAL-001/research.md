# SPEC-CODEXQUAL-001 조사 기록

## Outcome Lock

- **User outcome**: 사용자는 로컬 Codex 설정을 직접 고치지 않아도 persistent Balanced/Ultra와
  명시적 orchestra runtime override에 맞는 GPT-5.6 model/effort를 얻는다. 기존 user-owned root와
  pinned provider는 보존되고, capability 부족 시 effective selection과 reason이 보인다.
- **Mandatory scope**: `spec.md`의 REQ-001~REQ-010 전체를 하나의 Primary SPEC에서 닫는다.
- **Explicit non-goals**: Claude/OpenCode model 정책, Codex fan-out·`max_threads`·`max_depth`,
  per-spawn custom-agent override, 사용자 설정 강제 이행, 과거 기록 재작성, entitlement telemetry는
  변경하지 않는다.
- **Scenario evidence**: `acceptance.md`의 S1~S20 전체가 completion oracle이다. S1~S9는
  `C_FULL`과 declared-effort 입력을 명시하고, S13~S18은 fallback 분기와 root·agent·Args·PaneArgs
  capability 소비를 검증한다.
- **Execution evidence**: `acceptance.md`의 실제 경로 strict command, focused/full/race Go tests,
  vet, build, template parity, local `codex debug models`, git hygiene commands를 실행한다.

## Visual Planning Brief

```text
runtime --quality > quality.default > balanced
                         │
                         ▼
                  desired role profile <── runtime --effort (quality-managed orchestra only)
                         │
                         ▼
                bounded capability catalog
          ┌──────────────┼───────────────┐
          ▼              ▼               ▼
    same 5.6 model   gpt-5.5 fallback   runtime default
     lower effort      xhigh ceiling      omit controls
          │              │               │
          └──────────────┼───────────────┘
                         ▼
             root / agent / Args / PaneArgs
```

## Evidence

### Runtime catalog

로컬 `codex-cli 0.144.0`의 structured `codex debug models` JSON을 확인했다.

| Model | Supported effort |
|---|---|
| `gpt-5.6-sol` | `low, medium, high, xhigh, max, ultra` |
| `gpt-5.6-terra` | `low, medium, high, xhigh, max, ultra` |
| `gpt-5.6-luna` | `low, medium, high, xhigh, max` |
| `gpt-5.5` | `low, medium, high, xhigh` |

공식 Codex Models 문서는 `max`를 가장 어려운 문제를 위한 maximum reasoning depth로,
`ultra`를 automatic task delegation을 포함한 maximum reasoning으로 구분한다. Sol은 복잡하고
개방형인 고가치 작업, Terra는 일상적인 균형형 작업, Luna는 명확하고 반복 가능한 고빈도 작업에
권장된다. 공식 Subagents 문서는 custom agent의 `model`과 `model_reasoning_effort`를 파일에서
지정하고, 생략하면 parent 값을 상속한다고 설명한다. 현재 spawn call 자체에는 per-spawn
model/effort field가 없다.

### Baseline before this SPEC

- `pkg/config/defaults.go`의 Codex aliases와 canonical orchestra provider는 `gpt-5.5`를 사용했다.
- `pkg/content/agent_transformer_mapping.go`는 Opus/Sonnet/Haiku를 한 Codex 모델로 변환했다.
- `templates/codex/config.toml.tmpl`과 16개 agent template은 `gpt-5.5`를 직접 지정했다.
- `internal/cli/orchestra_helpers.go`의 config 부재 fallback도 `gpt-5.5+xhigh`로 고정했다.
- `pkg/adapter/codex/codex_config_merge.go`는 기존 root model/effort를 사용자 소유 키로 보존했다.

### Implemented worktree evidence

- `pkg/config/codex_profile.go`, `codex_profile_legacy.go`: desired profile, effort normalization,
  structured capability resolution, exact reason enum을 구현한다.
- `pkg/config/codex_provider.go`: `ApplyCodexProviderProfile`, `model_policy`, historical migration,
  `--` terminator-aware Args/PaneArgs 변환을 구현한다.
- `pkg/config/codex_catalog_bounds.go`, `pkg/codexruntime/probe.go`: catalog size·shape bounds와 timeout이
  있는 probe를 구현한다.
- `pkg/adapter/codex/codex_catalog.go`: root와 agent render가 같은 capability resolver와 deduplicated
  receipt를 사용한다.
- `internal/cli/orchestra_run_runtime.go`, `codex_catalog_runtime.go`: ephemeral quality/effort overlay와
  provider runtime capability resolution을 구현한다.
- `pkg/content/agent_transformer.go`와 Codex agent templates: agent name, source tier, declared effort를
  하나의 profile tuple로 렌더한다.
- unit/integration tests는 policy, root generation, agent generation, provider migration, argv suffix,
  fallback reason, bounds, general orchestra, `orchestra run`, structured review 경로를 검증한다.

## Decision Record

| Decision | Reason |
|---|---|
| Balanced supervisor를 Sol+xhigh로 사용 | supervisor는 계획, gate, 합성을 담당하는 전략 역할이다 |
| Balanced Opus worker를 declared effort와 무관하게 Sol+xhigh로 고정 | 핵심 전략 품질을 유지하고 Ultra 비용·delegation 경계와 구분한다 |
| Balanced Sonnet/Haiku worker는 normalized declared effort 사용 | 반복 작업 비용을 낮추면서 source role의 추론 의도를 보존한다 |
| unknown/blank declared effort는 medium, worker ultra는 max | 비표준 입력을 결정적으로 처리하고 nested automatic delegation을 막는다 |
| Ultra supervisor/orchestra를 Sol+ultra로 사용 | 둘 다 depth 0 실행이며 자동 task delegation을 활용할 수 있다 |
| Ultra managed worker를 Sol+max로 사용 | supervisor가 이미 worker를 배치하므로 worker의 중첩 delegation을 요청하지 않는다 |
| runtime `--effort`가 quality-derived effort보다 우선 | 명시적 실행 override가 preset보다 구체적인 사용자 의도다 |
| runtime override는 quality-managed orchestra로 제한 | pinned ownership과 세션 시작 시 로드된 agent file contract를 지킨다 |
| root는 RHS literal, provider는 complete slice로 보존 | 실제 merge는 root whitespace를 정규화하지만 provider argv ordering과 quoting은 실행 의미를 바꾼다 |
| 같은 모델 effort downgrade를 먼저 시도 | 모델 역할을 보존하고 지원되지 않는 control만 낮춘다 |
| no-lower effort에서는 model 유지+effort 생략 | 더 높은 effort로 올리지 않고 Codex runtime default에 위임한다 |
| GPT-5.6 부재 시 legacy `gpt-5.5`와 xhigh ceiling 사용 | 구형 CLI/account 호환성을 유지하고 미지원 `max`/`ultra`를 보내지 않는다 |
| `model_unavailable`, `runtime_default`, `catalog_unknown`을 분리 | valid capability 부재와 catalog 신뢰 실패를 운영상 구분한다 |
| provider `model_policy` 도입 | exact canonical migration과 user-pinned argv 보존을 구분한다 |
| OpenCode 정책 제외 | OpenCode provider/model/variant 계약은 별도 surface다 |

## Fallback Reason Inventory

| Reason | Trigger | Effective selection |
|---|---|---|
| `supported` | requested model/effort가 catalog에 있음 | requested tuple 유지 |
| `effort_unavailable` | requested model은 있고 요청 이하 supported effort가 있음 | 같은 model + 가장 높은 compatible effort |
| `model_unavailable` | requested GPT-5.6 model은 없고 compatible `gpt-5.5`가 있음 | `gpt-5.5` + 요청 이하 effort, ceiling=`xhigh` |
| `runtime_default` | 같은 model에 요청 이하 effort가 없거나 valid catalog에 compatible target/legacy tuple이 없음 | model만 유지하거나 model/effort 모두 생략 |
| `catalog_unknown` | probe failure/timeout/bounds violation/empty/malformed catalog | legacy `gpt-5.5` + 요청 effort의 `xhigh` ceiling |

## Minimality Matrix

| Surface | Required change | Excluded change |
|---|---|---|
| `pkg/config` | model constants, desired/capability resolver, provider marker/migration | Claude cost resolver 변경 |
| `pkg/content` | Codex tier/declared-effort render | source agent 역할 재설계 |
| Codex adapter | fresh config, managed agent render, root RHS literal 보존 | 사용자 root 강제 migration |
| Orchestra CLI | effective quality/effort overlay, capability fallback | provider 전략/투표 수 변경 |
| Docs/templates | 현재 정책 행렬과 runtime 경계 | historical SPEC/CHANGELOG rewrite |

## Semantic Invariant Inventory

| ID | Invariant | Requirements | Oracle |
|---|---|---|---|
| INV-001 | 같은 desired tuple과 catalog는 모든 consumer에서 같은 `CodexProfileResolution`을 만든다 | REQ-001~005 | S1~S9, S18 |
| INV-002 | root RHS literal과 pinned provider complete slices는 각 소유권 단위에서 보존된다 | REQ-005, REQ-006 | S9~S12 |
| INV-003 | capability resolver는 model을 먼저 보존하고 요청보다 높은 effort를 선택하지 않는다 | REQ-007, REQ-008 | S13~S18 |
| INV-004 | worker는 unknown/blank=`medium`, declared ultra=`max`이며 per-spawn override를 지원한다고 안내하지 않는다 | REQ-004, REQ-009 | S5~S7, S19 |
| INV-005 | invalid catalog와 valid-but-missing catalog는 각각 `catalog_unknown`과 `runtime_default`로 구분된다 | REQ-008 | S16~S18 |
| INV-006 | orchestra는 quality와 effort precedence를 지키며 runtime overlay를 disk에 쓰지 않는다 | REQ-002, REQ-005 | S2, S8, S9 |
| INV-007 | Codex 정책 변경은 Claude/OpenCode model contract를 바꾸지 않는다 | REQ-010 | S20 |

## Feature Coverage Map

| User outcome | Runtime surface | Requirement | Acceptance |
|---|---|---|---|
| Balanced/Ultra supervisor 기본 | Codex root template + adapter merge | REQ-003, REQ-006 | S3, S4, S10, S18 |
| tier별 subagent/native team | content transformer + managed agent TOML | REQ-004, REQ-009 | S5~S7, S18, S19 |
| quality별 orchestra | provider config + three CLI entry paths | REQ-002, REQ-005, REQ-006 | S2, S8, S9, S11, S12, S18 |
| availability fallback | bounded catalog probe + profile resolver + receipt | REQ-007, REQ-008 | S13~S18 |
| platform isolation | Claude/OpenCode regression surfaces | REQ-010 | S20 |

## Question Audit

- Balanced supervisor 품질: 사용자가 Sol+xhigh로 상향을 확정했다.
- `max` 대 `ultra`: live catalog와 공식 문서를 재확인했다. Ultra는 단순 상위 token budget이 아니라
  automatic task delegation을 포함하므로 실행 역할별로 구분한다.
- Balanced Opus: declared `max`를 그대로 쓰지 않고 `xhigh`로 고정한다. 전략 품질은 유지하면서
  Ultra worker의 `max`와 비용 경계를 분명히 하기 위한 의도된 정책이다.
- Per-run custom agent 전환: agent 파일은 세션 시작 시 로드된다. persistent 변경에는
  `auto quality set`, `auto update`, 새 Codex 세션이 필요하다.
- Runtime override: `--quality`와 `--effort`는 quality-managed orchestra에만 즉시 적용한다.
  `--effort`가 quality-derived effort보다 우선한다.
- Custom quality preset: exact `ultra`만 Ultra supervisor/orchestra profile을 선택한다. 나머지는
  Balanced profile을 사용하면서 persistent generation에서는 preset의 role tier mapping을 유지한다.
- 미해결 제품 결정: 없음.

## Reference Discipline

| Reference | State | Verification |
|---|---|---|
| OpenAI Codex Models, `https://learn.chatgpt.com/docs/models` | official current | 2026-07-10 확인 |
| OpenAI Codex Subagents, `https://learn.chatgpt.com/docs/agent-configuration/subagents` | official current | 2026-07-10 확인 |
| local `codex debug models` JSON | runtime current | codex-cli 0.144.0에서 확인 |
| `pkg/adapter/codex/codex_config_merge.go` | existing before SPEC | root user-owned RHS literal merge 확인 |
| `internal/cli/global_flags.go` | existing before SPEC | `--quality`, `--effort` 입력 surface 확인 |
| `pkg/config/codex_profile.go`, `codex_profile_legacy.go` | [IMPLEMENTED] by SPEC | desired/capability resolver와 exact reason 확인 |
| `pkg/config/codex_provider.go` | [IMPLEMENTED] by SPEC | `ApplyCodexProviderProfile`, marker, migration, terminator-safe argv 확인 |
| `pkg/config/codex_catalog_bounds.go` | [IMPLEMENTED] by SPEC | payload bounds와 validation 확인 |
| `pkg/codexruntime/probe.go` | [IMPLEMENTED] by SPEC | timeout과 bounded stdout probe 확인 |
| `pkg/config/codex_profile_template.go` | [IMPLEMENTED] by SPEC | root/agent template resolver bridge 확인 |
| `pkg/adapter/codex/codex_catalog.go` | [IMPLEMENTED] by SPEC | root/agent capability consumption과 receipt 확인 |
| `internal/cli/orchestra_run_runtime.go` | [IMPLEMENTED] by SPEC | quality/effort precedence와 pinned exclusion 확인 |
| `internal/cli/codex_catalog_runtime.go` | [IMPLEMENTED] by SPEC | Args/PaneArgs capability consumption과 receipt 확인 |
| `pkg/content/agent_transformer.go` and mapping | [IMPLEMENTED] by SPEC | declared effort를 포함한 generated agent tuple 확인 |

## Reviewer Brief

- **Scope**: desired profile matrix, precedence, ownership, capability resolution, consumer parity,
  fallback receipt, persistent worker lifecycle만 검토한다.
- **Non-goals**: Claude/OpenCode policy redesign, Codex concurrency/depth, per-spawn override, telemetry,
  unrelated design changes는 finding 범위에서 제외한다.
- **Evidence**: REQ-001~010 ↔ INV-001~007 ↔ S1~S20 traceability, `C_FULL`/partial/invalid fixtures,
  Scenario Evidence Map, Reference Discipline의 실제 구현 경로를 사용한다.
- **Self-verification**: strict command를 실제 SPEC directory 경로로 실행한 뒤 focused/full/race tests,
  vet, build, live catalog, git hygiene 순서로 확인한다. 문서만 존재하거나 stale 문자열이 없다는
  사실만으로 PASS하지 않는다.

## Formal Finding Closure

| Finding | Closure |
|---|---|
| F1 / `model_unavailable` 미정의 | REQ-008, Compatibility, Fallback Reason Inventory, S15에 exact reason과 legacy ceiling을 정의했다 |
| F2 / REQ-009 EARS type | 허용 type `Optional`과 `WHERE ... THEN the system SHALL` 형식으로 고쳤다 |
| F3 / worker effort normalization | REQ-004, plan T3, S5/S6에 blank/unknown=`medium`, ultra=`max`를 고정했다 |
| F4 / Balanced Opus 근거 | Policy Matrix, Decision Record, Question Audit에 fixed `xhigh` 근거를 명시했다 |
| F5 / unknown effort 결정성 | managed worker normalization과 canonical capability order를 REQ-004/REQ-007로 분리했다 |
| F6 / MapEffort ultra bypass | model/effort를 같은 declared tuple로 resolver에 전달하고 worker resolver에서 `max`로 cap하도록 T3/S5/S6에 고정했다 |
| F7 / `ApplyCodexProviderProfile` reference | Reference Discipline에 `pkg/config/codex_provider.go`의 [IMPLEMENTED] symbol로 등록했다 |

## Codex Timeout Convergence Record

| ID | Closure location |
|---|---|
| CS-001 | spec/research Outcome Lock에 user outcome, REQ-001~010, non-goals, S1~S20, execution evidence를 고정했다 |
| CS-002 | REQ-002와 Ownership And Precedence에 `runtime --quality > quality.default > balanced`를 고정했다 |
| CS-003 | REQ-005와 S2/S9에 `--effort > quality-derived effort`, quality-managed orchestra-only 범위를 고정했다 |
| CS-004 | REQ-008과 S15~S17에 `model_unavailable`, legacy ceiling, runtime/catalog reason을 구분했다 |
| CS-005 | REQ-004/REQ-007과 S5/S6/S14에 unknown normalization, worker cap, no-lower 동작을 고정했다 |
| CS-006 | REQ-006과 S10/S11에 root RHS literal과 provider complete slice 보존 단위를 분리했다 |
| CS-007 | 모든 문서의 acceptance 범위를 S1~S20으로 통일하고 Traceability/Feature Coverage를 갱신했다 |
| CS-008 | plan T5와 S18에 root·agent·Args·PaneArgs 통합 capability oracle을 추가했다 |
| CS-009 | S16의 unavailable/malformed와 S17의 valid-but-missing fixture를 분리했다 |
| CS-010 | Completion Debt를 없음으로 닫고 unsupported-account smoke를 Verification Limitation으로 옮겼다 |
| CS-011 | Verification Commands를 `.autopus/specs/SPEC-CODEXQUAL-001` 실제 경로로 고쳤다 |

## Self-Verify Summary

| ID | Status | Evidence |
|---|---|---|
| Q-CORR-01 | PASS | existing 및 [IMPLEMENTED] path/symbol을 현재 worktree에서 확인했다 |
| Q-CORR-02 | PASS | SPEC 신규 구현을 Reference Discipline에서 [IMPLEMENTED]로 구분했다 |
| Q-CORR-03 | PASS | REQ-009를 허용 EARS type `Optional`/`WHERE` 형식으로 정정했다 |
| Q-CORR-04 | PASS | `ApplyCodexProviderProfile`을 포함한 reference의 실제 파일을 확인했다 |
| Q-COMP-01 | PASS | prd/spec/plan/acceptance/research가 outcome, contract, task, oracle, evidence 역할을 분리한다 |
| Q-COMP-02 | PASS | REQ-001~010이 invariant와 S1~S20에 모두 연결된다 |
| Q-COMP-03 | PASS | trigger, behavior, exact selected tuple/reason, 관측 지점을 명시했다 |
| Q-COMP-04 | PASS | Outcome Lock이 user outcome, scope, non-goals, completion evidence를 포함한다 |
| Q-COMP-05 | PASS | INV-001~007을 concrete consumer/fallback oracle에 연결했다 |
| Q-COMP-06 | PASS | Reviewer Brief가 scope, non-goals, evidence, self-verification을 제한한다 |
| Q-COMP-07 | PASS | Completion Debt는 없으며 환경 제약은 Verification Limitation으로 분리했다 |
| Q-FEAS-01 | PASS | resolver, templates, adapter, CLI가 실제 구현 계층과 일치한다 |
| Q-FEAS-02 | PASS | root literal과 provider slice ownership을 실제 module boundary에 맞췄다 |
| Q-FEAS-03 | PASS | Verification Commands가 repo에서 실행 가능한 명령과 실제 SPEC 경로를 사용한다 |
| Q-STYLE-01 | PASS | 모든 requirement가 `SHALL`로 단정되고 모호한 should/might를 쓰지 않는다 |
| Q-STYLE-02 | PASS | EARS type과 Priority를 별도 축으로 유지한다 |
| Q-STYLE-03 | PASS | S1~S20이 bare Given/When/Then/And 형식을 사용한다 |
| Q-SEC-01 | PASS | external catalog는 timeout, byte, model, slug, effort bounds로 제한한다 |
| Q-SEC-02 | PASS | user-owned 설정을 정해진 literal/slice 단위로 보존하고 secret을 다루지 않는다 |
| Q-SEC-03 | PASS | receipt는 requested/selected/reason만 출력하고 persistent artifact를 만들지 않는다 |
| Q-COH-01 | PASS | 모든 변경이 Codex quality별 profile outcome 하나에 수렴한다 |
| Q-COH-02 | PASS | 필수 작업은 Primary SPEC에서 닫고 optional future idea에는 task ID를 부여하지 않는다 |
| Q-COH-03 | PASS | sibling SPEC 의존 없이 REQ-001~010을 완결한다 |

## Evolution Ideas

| Idea | Why not required now | Promotion trigger |
|---|---|---|
| role-balanced/role-ultra agent 파일을 동시에 생성 | 파일 수와 router contract가 두 배가 되고 current spawn schema에는 per-call model field가 없다 | Codex가 dynamic custom-agent selection API를 제공하거나 사용자가 per-run worker 전환을 요구할 때 |
| Luna worker를 기본 preset에 추가 | 현재 default preset에는 Haiku role이 없어 synthetic resolver oracle로 계약을 검증할 수 있다 | 실제 economy role을 preset에 도입할 때 |
| 계정별 fallback telemetry 집계 | local harness outcome에 필요하지 않다 | 운영에서 미지원 비율을 제품 지표로 사용할 때 |

## Completion Debt

없음.

## Verification Limitation

현재 계정은 GPT-5.6 전체 capability를 제공하므로 GPT-5.6 entitlement가 없는 실제 계정에서 live
smoke를 수행할 수 없다. unsupported-account 동작은 hermetic catalog fixture와 exact runtime receipt로
검증하고, 현재 계정에서는 local catalog smoke를 수행한다. 이 제한은 미구현 작업을 남기지 않는다.
