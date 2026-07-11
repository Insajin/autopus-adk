# SPEC-CODEXQUAL-001: GPT-5.6 품질별 Codex 실행 프로필

**Status**: completed
**Created**: 2026-07-10
**Updated**: 2026-07-11
**Domain**: CODEXQUAL

## 목적

Autopus-ADK의 Codex 모델 및 reasoning effort 결정을 persistent quality, 명시적 orchestra
runtime override, 실행 역할, agent tier, declared worker effort, 현재 Codex capability에 따라
일관되게 해석한다. Codex 주 세션은 기본적으로 사용자 runtime 모델을 상속하고, 명시적으로
  quality-managed 정책을 선택한 supervisor와 managed subagent/native multi-agent,
quality-managed orchestra는 같은 프로필 resolver를 사용한다. 사용자 소유 설정과 다른 플랫폼
정책은 보존한다.

## Outcome Boundary

### Outcome Lock

- **User-visible outcome**: 사용자의 Codex 주 세션 모델은 새 프로젝트에서 그대로 유지되며,
  수정되지 않은 legacy generated root는 `auto update`에서 사용자 기본 모델 상속으로 안전하게 이행된다.
  사용자는 `auto quality <mode> --apply`로 Balanced와 Ultra에 맞는 관리형 GPT-5.6 프로필을
  저장하고 반영한다. Ultra에서는 `max`, `xhigh`, `ultra`를 역할과 delegation 의미에 따라
  구분하며, capability 부족 시 선택 결과와 이유를 확인할 수 있다.
- **Mandatory requirements**: REQ-001~REQ-010을 모두 구현해야 한다.
- **Explicit non-goals**: Claude/OpenCode 정책, Codex fan-out·`max_threads`·`max_depth`, per-spawn
  custom-agent override, 사용자 소유 설정 강제 이행, 역사적 기록 재작성, entitlement telemetry는
  변경하지 않는다.
- **Scenario evidence**: `acceptance.md`의 S1~S22를 모두 통과해야 한다. S1~S9는 동일한
  full-support catalog fixture와 명시된 declared-effort 입력을 사용하며, S13~S18은 capability
  분기와 모든 consumer 투영을 검증하고, S21~S22는 legacy 이행과 실패 복구를 검증한다.
- **Execution evidence**: 실제 경로를 사용한 strict validation, focused/full Go tests, vet, build,
  generated surface parity, 현재 로컬 `codex debug models` smoke를 완료해야 한다. 명령은
  `acceptance.md`의 Verification Commands를 단일 기준으로 사용한다.

## Policy Matrix

| Scope | Balanced | Ultra | Rationale |
|---|---|---|---|
| Supervisor (`inherit`, default) | User Codex runtime default | User Codex runtime default | 사용자 주 세션 모델 소유권 보존 |
| Supervisor (`quality`) | Sol + `xhigh` | Sol + `ultra` | 명시적으로 opt-in한 전략 판단; Ultra depth 0에서 자동 위임 허용 |
| `planner`/`architect`/`security-auditor` | Sol + `xhigh` | Sol + `max` | Ultra의 전략·보안 worker에 최대 추론을 배정하되 중첩 자동 위임은 금지 |
| Other Opus agent | Sol + `xhigh` | Sol + `xhigh` | 전략 3개 외 managed worker의 비용 상한 고정 |
| Other Sonnet agent | Terra + normalized declared effort | Sol + `xhigh` | Balanced 반복 작업 비용 절감, Ultra 일반 worker 비용 제어 |
| Other Haiku agent | Luna + normalized declared effort | Sol + `xhigh` | Balanced 단순·고빈도 작업 비용 절감, Ultra 일반 worker 비용 제어 |
| Quality-managed orchestra | Sol + `xhigh` | Sol + `ultra` | 독립 depth 0 분석 프로세스 |

Sol, Terra, Luna는 각각 `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna`를 뜻한다.

## Requirements

### REQ-001 (Ubiquitous / Priority: Must)

the system SHALL provide one canonical Codex profile and capability resolver for supervisor,
managed agent, and quality-managed orchestra model/effort decisions.

### REQ-002 (Event-driven / Priority: Must)

WHEN a quality-managed Codex orchestra profile is resolved, THEN the system SHALL determine effective
quality in the order explicit runtime `--quality`, persistent `quality.default`, `balanced`, treat only
exact `ultra` as Ultra, and use the Balanced supervisor/orchestra profile for every other preset while
retaining that preset's role mapping for persistently generated managed agents.

### REQ-003 (Event-driven / Priority: Must)

WHEN a fresh Codex supervisor config is rendered with `supervisor_model_policy: inherit`, THEN the
system SHALL omit project-local model and effort assignments so the Codex runtime default is inherited;
WHEN the policy is `quality`, THEN the system SHALL emit Sol plus `xhigh` for Balanced and Sol plus
`ultra` for Ultra after applying the capability resolver. A missing legacy policy is interpreted as
`quality` for backward compatibility.

### REQ-004 (Event-driven / Priority: Must)

WHEN a managed Codex agent definition is rendered, THEN the system SHALL map its effective Opus,
Sonnet, or Haiku tier and declared effort through the Policy Matrix, normalize an empty or unknown
declared effort to `medium`, cap declared worker `ultra` at `max`, use Sol plus `max` for
`planner`, `architect`, and `security-auditor` in Ultra, and use Sol plus `xhigh` for every other
Ultra managed or unknown agent role.

### REQ-005 (Event-driven / Priority: Must)

WHEN a Codex orchestra provider has `model_policy: quality`, THEN the system SHALL apply explicit
runtime `--effort` after effective quality so that `--effort` wins over quality-derived effort, update
only the managed model/effort options before an argv `--` terminator in both subprocess Args and
interactive PaneArgs, leave the disk config unchanged, and exclude `model_policy: pinned` providers and
already generated managed-agent files from runtime quality/effort changes.

### REQ-006 (Unwanted / Priority: Must)

IF an existing Codex root model/effort key is user-owned, THEN the system SHALL preserve that key's
parsed right-hand-side literal, including its quoted value, across repeated updates and record durable
per-key ownership; IF a manifest-tracked root has an explicit supervisor policy and an exact known
generated model/effort tuple, THEN the system SHALL refresh or remove that tuple even when an unrelated
config key changed; IF a legacy config has no supervisor policy and its markerless model tuple is
ambiguous, THEN the system SHALL preserve it until the user explicitly selects `inherit` or `quality`;
IF a legacy config has no supervisor policy and all managed-root ownership evidence is present, THEN the
system SHALL require the generated header, no user model marker, an exact historical `gpt-5.5+xhigh` or
managed GPT-5.6 tuple, a merge-policy manifest entry, and a matching whole-file checksum before
`auto update` persists
`supervisor_model_policy: inherit` and remove the managed root model/effort assignments; IF any ownership
condition is absent, THEN the system SHALL preserve the assignments; IF Codex rendering fails after the
policy migration is staged, THEN the system SHALL restore the missing legacy policy and SHALL NOT report
the migration as successful; WHEN `auto update --plan` evaluates the migration, THEN it SHALL report the
pending change without writing project files; WHEN `auto doctor` observes a migratable legacy override,
THEN the system SHALL report a read-only warning; WHEN `auto doctor` observes an explicit `inherit`
policy with an unapplied project override, THEN the system SHALL report a read-only warning;
IF a legacy config has no supervisor policy and its provider exactly matches the v0.50.66
auto-pinned model-only signature, THEN the system SHALL reclassify that provider as `quality` and apply
the persistent quality profile; IF any other provider has `model_policy: pinned`, THEN the system SHALL
preserve its Binary, Args, PaneArgs, every slice element, ordering, quoting, unrelated flag, and `--`
suffix.

### REQ-007 (Unwanted / Priority: Must)

IF the requested effort is absent from an available requested model, THEN the system SHALL retain that
model and select the highest supported effort no greater than the canonical request in
`ultra,max,xhigh,high,medium,low` order with reason `effort_unavailable`, or retain the model, omit the
effort override, and expose `runtime_default` when no supported effort is no greater than the request.

### REQ-008 (Unwanted / Priority: Must)

IF the requested GPT-5.6 model is absent from a valid catalog and compatible `gpt-5.5` is present,
THEN the system SHALL select `gpt-5.5`, cap effort at `xhigh`, and expose exact reason
`model_unavailable`, omit managed model/effort fields and expose `runtime_default` when the valid catalog
has neither the requested model nor a compatible legacy tuple, or use the legacy `gpt-5.5` profile with
effort capped at `xhigh` and expose `catalog_unknown` when the catalog probe is unavailable, times out,
exceeds its bounds, or returns invalid JSON.

### REQ-009 (Optional / Priority: Must)

WHERE a running Codex session has already loaded custom agent files, THEN the system SHALL describe
per-run quality/effort overrides as unsupported for those workers and provide
`auto quality <mode> --apply` plus a new Codex session instead of claiming a per-spawn model override.

### REQ-010 (Ubiquitous / Priority: Must)

the system SHALL leave Claude and OpenCode model, effort, model override, and variant behavior unchanged.

## Ownership And Precedence

Quality-managed orchestra precedence is:

1. Effective quality: explicit runtime `--quality` > persistent `quality.default` > `balanced`.
2. Effective effort: explicit runtime `--effort` > effort derived from effective quality.
3. Capability: the catalog resolver may lower or omit the requested effort and may select the compatible
   legacy model according to REQ-007 and REQ-008.

The runtime overlay applies only to Codex providers marked `model_policy: quality`. It is ephemeral and
updates neither `autopus.yaml` nor already generated agent files. Managed agent files use persistent
`quality.default` because the Codex subagent call schema has no per-spawn model or effort fields.
Fresh configs use `supervisor_model_policy: inherit`; existing configs with no policy retain the legacy
`quality` interpretation until migration. Legacy root assignments are preservation-first because older
manifests cannot distinguish every markerless user tuple from generated content. The one automatic
exception requires all ownership evidence in REQ-006: generated header, no user marker, merge-policy
manifest entry, matching whole-file checksum, and an exact known historical managed tuple. `auto update`
then records `inherit` and removes the root override. Any missing or conflicting evidence keeps the
legacy assignment until `auto quality supervisor inherit|quality --apply` records an explicit choice.
Under an explicit policy, exact known generated tuples are Autopus-owned and may refresh even if an
unrelated setting caused whole-file checksum drift. Untracked custom tuples and durable key-list markers
establish user ownership only for their named root assignments across later manifest rewrites. Managed
`.codex/agents/*.toml` files remain Autopus-owned and refresh on update. Preview performs the ownership
decision in memory only. A failed Codex update restores the pre-migration policy, and workspace rollback
restores both generated transaction journals and the original `autopus.yaml`. Doctor never writes these
files and warns when legacy shadowing remains or an explicit `inherit` policy has not reached the project
config.

New canonical Codex providers use `model_policy: quality`. An explicit `pinned` marker is never overlaid
except for the v0.50.66 one-time repair. That repair requires a missing raw supervisor policy and the
complete auto-pinned signature: Binary=`codex`, Args=`exec --sandbox workspace-write -m gpt-5.5`,
PaneArgs=`-m gpt-5.5`, no prompt/input/working customization, and the canonical subprocess schema and
timeout. During migration, the exact unmarked historical Autopus `gpt-5.5+xhigh` Args and PaneArgs tuple
is also promoted to `quality`. Any near-match or provider in a config with an explicit supervisor policy
remains `pinned` byte-for-byte, and every other unmarked provider becomes `pinned`.

Root preservation and provider preservation deliberately use different units. Root merge preserves the
parsed right-hand-side literal for each user-owned assignment while the generated template may normalize
whitespace. Pinned provider migration and runtime resolution preserve the complete Binary, Args, and
PaneArgs values as slice-equality oracles. For a quality-managed provider, only model/effort tokens before
the first `--` are managed; the terminator and its complete suffix remain unchanged.

## Compatibility

The resolver consumes a structured, size-bounded capability catalog. A managed worker normalizes its
declared effort before capability resolution: blank or unknown becomes `medium`, `ultra` becomes `max`,
Balanced Opus remains fixed at `xhigh`, and Ultra selects role-bound `max` or `xhigh` independently of
declared effort. The capability resolver first preserves the requested model
and chooses the highest supported effort no greater than the request. If none exists, it preserves the
model but omits effort with `runtime_default`.

The resolver changes to `gpt-5.5` only when the requested GPT-5.6 model is absent and a compatible legacy
tuple exists. Legacy effort never exceeds `xhigh`, and this branch reports `model_unavailable`. A valid
catalog with no compatible requested or legacy tuple returns empty managed fields with `runtime_default`.
An unavailable, timed-out, oversized, empty, or malformed catalog is distinct: it selects the legacy
profile with the same `xhigh` cap and reports `catalog_unknown`. Fallback receipts expose `requested`,
`selected`, and `reason`; adapter rendering deduplicates identical receipts, while each independently
resolved orchestra provider emits one receipt.

## Traceability Matrix

| Requirement | Invariant | Plan Task | Acceptance |
|---|---|---|---|
| REQ-001 | INV-001 | T1, T5 | S1, S18 |
| REQ-002 | INV-001, INV-006 | T1, T4 | S2, S9 |
| REQ-003 | INV-001 | T2, T5 | S3, S4, S18 |
| REQ-004 | INV-001, INV-004 | T3, T5 | S5, S6, S7, S18 |
| REQ-005 | INV-001, INV-002, INV-006 | T4, T5 | S8, S9, S11, S12, S18 |
| REQ-006 | INV-002 | T2, T4 | S10, S11, S12, S21, S22 |
| REQ-007 | INV-003 | T1, T5 | S13, S14, S18 |
| REQ-008 | INV-003, INV-005 | T1, T5 | S15, S16, S17, S18 |
| REQ-009 | INV-004 | T3, T6 | S19 |
| REQ-010 | INV-007 | T6, T7 | S20 |

## Related SPECs

- None. This primary SPEC closes the model policy outcome within `autopus-adk`.

## Completion Verdict

- **Outcome Lock**: satisfied.
- **Mandatory requirements**: REQ-001~REQ-010 implemented and verified.
- **Must acceptance**: S1~S22 passed with the evidence mapped in `acceptance.md`.
- **Verification**: focused and race tests, `go test ./... -count=1`, `go vet ./...`, and
  `go build ./...` passed. Strict SPEC review passed all 43 checklist items and resolved all seven
  discovered findings.
- **Live evidence**: `codex-cli 0.144.0` reported Sol and Terra through `ultra`, Luna through `max`,
  and GPT-5.5 through `xhigh`; fresh-init smoke matched the Balanced and Ultra policy matrix.
- **Residual provider health**: the Codex formal-review provider timed out at its configured seven-minute
  boundary; Claude and Gemini completed, the combined review gate returned PASS, and no open finding
  depends on the timed-out response.
- **Completion Debt**: none. Optional evolution ideas remain outside the Outcome Lock.
