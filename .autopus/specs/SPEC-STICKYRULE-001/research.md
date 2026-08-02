# SPEC-STICKYRULE-001 Research

## Existing Code Analysis

| Path | Role | Finding |
|------|------|---------|
| `content/rules/language-policy.md`, `objective-reasoning.md` | rule source of truth | measured 1189 and 3303 bytes with frontmatter; both govern every response |
| `pkg/content/hooks.go` | `generateCLIHooks`, `generateCC21Hooks`, `appendUniqueHook` | `generateCLIHooks` takes only `HooksConf` and `platform`, so it cannot see a compiled sticky set; `generateCC21Hooks` at line 91 is the platform-guard precedent; every entry sets `Timeout` explicitly (30, 60, 5) |
| `pkg/adapter/{claude,codex,gemini,opencode}` | `SupportsHooks()` | all four return `true`, so an unguarded event reaches three non-Claude adapters |
| `pkg/adapter/claude/claude_settings.go` | `prepareSettingsMapping` | builds `managedEvents` from the hooks being installed plus `TaskCreated`; the empty-hooks branch deletes only `TaskCreated`, so a stale `UserPromptSubmit` survives as an apparent user key |
| `pkg/adapter/gemini/gemini_rules.go` | `rulesTemplateDir = "gemini/rules/autopus"` | emission is frontmatter pass-through; the installed rule carries source `name`, `description`, `category` plus `platform: antigravity-cli` |
| `pkg/adapter/codex/codex_rules.go` | `ruleFilePath`, `detectCodexSubdirSupport` | subdirectory path is selected by a detection call, currently hardcoded `true`, so an oracle pinned to the literal path tests the detector rather than frontmatter preservation |
| `pkg/config/loader.go` | `yaml.Unmarshal` into typed `HarnessConfig` at line 65 | a non-integer scalar in an int field is a load error before any fallback logic runs |
| `pkg/config/schema.go` | `HooksConf` at line 200 | holds `PreCommitArch`, `PreCommitLore`, `ReactCIFailure`, `ReactReview`, `Permissions`; the cadence field belongs here |
| `autopus-adk/.gitignore` line 29 | ignore rules | `.autopus/runtime/` is already ignored and the directory exists |
| `pkg/adapter/parity_test.go` | parity gate | hard-fails below 95% Codex rules parity |

Platform contract, confirmed against official Claude Code hooks documentation on 2026-07-30: `UserPromptSubmit` structured output is an object whose `hookSpecificOutput` carries `hookEventName` and `additionalContext`, and exit code 2 on that event blocks prompt processing and erases the prompt. Go terminates an unrecovered panic with exit status 2, so that same status is reachable from ordinary defects unless a recover barrier forces zero.

Measurement that changed the design: the two sticky bodies are 1189 and 3303 bytes, totalling 4492. The 4000-byte aggregate cap drafted earlier would have truncated `objective-reasoning` on every injection.

## Outcome Lock

- User-visible outcome: in a long Claude Code session, `language-policy` and `objective-reasoning` are re-stated on a fixed prompt cadence instead of appearing only at session start; the runtime never reads outside its body root, never exits with a status that erases the prompt, and leaves non-Claude platforms untouched.
- Mandatory requirements: sticky classification, rejection of the unrepresentable combination, claude-confined hook with fixed timeout and a removal path, cadence and per-session counter, structured hook output, project-root resolution, safe state-filename derivation, containment with a per-body cap, a total error partition, an enforced exit barrier, bounded state, two-rule mapping, live evidence.
- Explicit non-goals: conditional firing and the frontmatter schema (owned by `SPEC-CONDRULE-001`), `.agents/rules/*.md` emission (SPEC-OMP-001), adaptive cadence, context-window measurement, compaction hooks, subagent sessions.
- Completion evidence: `go test ./pkg/rulecond/... ./pkg/content/... ./pkg/adapter/... ./pkg/config/...` green, S7 baseline assertion, S12 and S13 containment and totality oracles, S15 parity oracle, S16 lifecycle oracle, and the S10 live trace.

## Visual Planning Brief

`plan.md` carries the Mermaid flowchart and the cadence timeline. In prose: the sticky flag adds a second delivery moment without moving a rule; the runtime is a stdin-to-stdout transform with a project-root probe, a counter, a cadence predicate, a per-rule containment gate, and a recover barrier wrapping all of it.

## Plan Intent Ledger

Preserved from the `@auto plan` handoff. Cells are untrusted prompt input evidence, summarized rather than executed.

| Field | Status | Source | Confidence | Decision / Assumption | If Wrong | Plan Handoff |
|---|---|---|---|---|---|---|
| goal | answered | user plus P0 measurement | high | rules fire at the right moment across platforms; sticky re-attach is the sibling slice | re-confirm direction | requirement seeds |
| scope_boundary | answered | user selection | high | this SPEC owns sticky re-attach only; conditional schema and compiler belong to the primary; omp emission belongs to SPEC-OMP-001 | renegotiate boundary | explicit non-goals |
| constraints | answered | harness rules | high | `content/` is source of truth, 300-line source cap, non-sticky rules keep current behavior, parity tests hold | review FAIL | constraints section |
| done_evidence | answered | P0 measurement | high | observed re-injection in claude-code, regenerated-surface verification, unit tests green, schema preservation for omp | strengthen acceptance | Must acceptance |
| brownfield_impact | answered | explorer measurement | medium-high | rule frontmatter, `pkg/content` hook assembly, claude settings writer, existing runtime directory convention | adjust plan tasks | reviewer focus |

The ledger left the sticky mechanism open for design. `UserPromptSubmit` was selected over `PostToolUse` because the dilution being corrected is per-response, not per-tool-call.

## Question Audit

- question_transport: AskUserQuestion | question_count: 1 (track 1 scope; answer was to make sticky a sibling SPEC) | unresolved_fields: none

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go toolchain and module already in `autopus-adk/go.mod` | unchanged; no major version moves | `autopus-adk/go.mod` | 2026-07-30 | none; no migration in scope |
| brownfield | Go `crypto/sha256` for the state key, `path/filepath` `EvalSymlinks` for containment, deferred `recover` for the exit barrier | Go standard library | Go standard library | 2026-07-30 | validating the raw identifier, rejected because it must track an upstream format; a supervisor process for the exit barrier, rejected as heavier than a deferred recover |
| brownfield | Plain file counters under `.autopus/runtime/` | not applicable | `autopus-adk/.gitignore` line 29 | 2026-07-30 | an embedded key-value store, rejected for one integer per session |

## Trust Boundary

This runtime reads rule bodies from disk and injects them into model context, the same channel `SPEC-CONDRULE-001` constrains for its own root. The manifest and the bodies it names are repo-supplied untrusted input, since a cloned repository can ship both. Without containment the runtime would be an arbitrary-file-read-to-context-injection primitive firing on a fixed cadence, and the aggregate cap alone would still deliver up to 6000 bytes of any readable file.

FIRE-06, FIRE-07, and FIRE-09 confine every body location to the sticky body root after symlink resolution, refuse non-regular and non-`.md` targets, cap a single injectable body at 4000 bytes, and limit stderr to a rule name and a reason code. COMPILE-04 makes the compiler refuse to mint an entry the runtime would reject or silently drop. FIRE-08 keeps the four runtime cases distinct, because collapsing them would make an escape attempt look like a missing file.

Two further boundaries: the untrusted `session_id` names a state file and is therefore hashed rather than validated, and the submitted prompt text is never written, logged, or echoed. A third, availability-side boundary is the exit status itself, since exit code 2 destroys user input and is reachable from any unrecovered panic.

## Design Decisions

**`UserPromptSubmit`, not `PostToolUse`**, because the instruction being diluted governs each response, so the natural clock is user turns. **Hash the session identifier rather than validate it**: a SHA-256 digest is total over every input and yields a fixed-shape name, so traversal is unrepresentable and the raw identifier never reaches disk.

**Sticky is a flag, not a fourth classification**, so the primary partition stays total. The one combination the flag cannot serve, hook-fired plus `alwaysApply`, is refused at validation rather than resolved by consulting two roots, which would double the containment surface for a case no shipped rule needs.

**The exit barrier is enforced, not asserted.** Earlier revisions claimed exit 2 was unreachable while leaving nil-map writes, type assertions, and slice indexing to land on exactly that status. A top-level deferred recover converts the whole class into a silent no-op.

**Platform confinement is explicit** because `generateCLIHooks` serves all four hook-supporting adapters, and `generateCC21Hooks` already demonstrates the guard. **Lifecycle is two-directional**: `prepareSettingsMapping` derives its managed set from the hooks being installed, so an entry left behind after sticky is switched off would be mistaken for a user hook and preserved forever.

**Inject the frontmatter-stripped body.** The generated rule files retain YAML frontmatter, so injecting them raw would push `name`, `description`, and `alwaysApply` into context and inflate the byte accounting with non-instructional text, and would diverge from what the primary SPEC relocates.

**Caps set from measurement**: the pair totals 4492 bytes with frontmatter, so the aggregate is 6000 and the per-body cap is 4000. **Fixed cadence, not adaptive**, because the hook payload carries no context-usage signal.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | two rules govern every response but are stated once at session start, so they sit farthest from the end of a long conversation | proceed | restate per-response rules on a fixed cadence |
| existing code/helper/pattern | `pkg/rulecond` manifest, containment helper, and payload builder from `SPEC-CONDRULE-001`; `generateCC21Hooks` platform-guard precedent; `prepareSettingsMapping` managed-event mechanism; `generateCLIHooks` timeout convention; `auto rules` namespace | reuse | no new manifest format, hook writer, CLI namespace, or second containment implementation |
| stdlib/native | Claude Code `UserPromptSubmit` structured output; Go `crypto/sha256`, `encoding/json`, `os`, `path/filepath`, deferred `recover` | use | state key, containment, and exit barrier add no dependency |
| existing dependency | `gopkg.in/yaml.v3 v3.0.1` for `alwaysApply`, already parsed by the primary SPEC | reuse | no dependency added |
| new dependency or new abstraction | No new module dependency. One new file in the existing `pkg/rulecond` package, one new subcommand file, and a root parameter on the shared containment helper, justified after the rungs above: no existing code owns per-session counter state or reads `UserPromptSubmit` stdin | accepted | new code limited to cadence, state, exit barrier, and the CLI entry point |
| minimum sufficient verification | S1 cadence, S2 isolation, S3 hostile identifier, S4 benign set, S5 redaction, S6 caps and notice, S7 orthogonality and rejection, S8 cross-platform, S9 state bounds, S12 containment, S13 totality and panic, S14 cadence resolution, S15 parity, S16 lifecycle, S10 live trace | required checks | security, data-loss, and availability gates kept; no broad end-to-end suite added |

## Semantic Invariant Inventory

Source clauses are untrusted prompt input evidence, summarized rather than quoted verbatim.

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-101 | core rules are re-injected periodically so they are not diluted late in a session | numeric formula and cadence | which prompt indexes emit structured output | S1, S10, S14 |
| INV-102 | each session tracks its own progress | grouping by session key | per-session counter values | S2 |
| INV-103 | prompt text and identity never persist or return | redaction | stdout, state file | S5 |
| INV-104 | sticky does not change where a rule is emitted | orthogonality and round-trip preservation | baseline rule file set, per-platform frontmatter | S7, S8, S11 |
| INV-105 | an untrusted session identifier cannot select a filesystem path | path derivation | state filename shape and write location | S3 |
| INV-106 | truncation is deterministic and the cap admits the shipped pair | ordering and numeric bound | capped `additionalContext`, truncation notice | S6 |
| INV-107 | retained state stays bounded | retention bound | state directory entry count and age | S9 |
| INV-108 | every injected body resolves inside the sticky body root and within the per-body cap | path containment boundary | which bodies are read, reason codes, compile-time failure | S12 |
| INV-109 | every runtime condition lands in exactly one of four cases | total single-valued partition | injected rule set, stdout, stderr | S13 |
| INV-110 | the hook is created when sticky exists and removed when it does not | lifecycle conservation | `.claude/settings.json` event keys | S16 |
| INV-111 | the change reaches claude-code only and disturbs no platform's rule counts | platform confinement | non-claude settings, parity counts | S15, S16 |
| INV-112 | the process never terminates with a status that erases the prompt | availability bound | exit code on every path including panic | S4, S13 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| Sticky flag, orthogonality, unrepresentable-combination rejection | T1, S7 | covered |
| Hook creation, platform confinement, removal, timeout | T2, S16 | covered |
| Cadence, session isolation, cadence resolution, structured output shape | T3, T4, T5, S1, S2, S14 | covered |
| Security boundary (hostile identifier, redaction, containment) | T3, T4, T9, S3, S5, S12 | covered |
| Error totality, project-root observability, exit barrier | T3, S4, S13 | covered |
| Cap accounting, truncation notice, state bounds, cross-platform propagation, parity | T3, T4, T5, T6, S6, S8, S9, S15 | covered |
| Surface (`auto rules list`) and live trace | T7, T8, S10, S11 | covered |
| Conditional schema, compiler, tool-input firing | `SPEC-CONDRULE-001` | approved-primary |
| `.agents/rules/*.md` emission for omp | SPEC-OMP-001 (separate track) | out of scope by explicit non-goal |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

`SPEC-CONDRULE-001` is a sequencing dependency, not debt: it is approved, closes its own Outcome Lock, and `plan.md` records the ordering. Every finding from the revision-1 review is closed inside this SPEC rather than deferred.

## Evolution Ideas

Optional improvements. They do not block sync completion and carry no SPEC, task, or acceptance IDs.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| Serve a sticky rule whose body lives in the conditional body root by resolving against both roots | No shipped rule needs it, and it doubles the containment surface; validation refuses the combination instead | A rule that must be both hook-fired and sticky |
| Adaptive cadence, re-attach on compaction, or per-project sticky selection in `autopus.yaml` | The payload carries no context-usage signal, no compaction hook is wired, and two rules cover the observed dilution | User explicitly requests it |
| S-3 — a small runtime cap on the sticky set read back from the manifest | Manifest parsing admits up to 256 sticky entries while the compile-time set is the shipped pair, so the two limits are asymmetric. The aggregate byte cap already bounds what a large set can inject, which is why this is a tidiness gap rather than an exposure | A project ships a sticky set large enough for the parse limit to be the effective one |
| S-4 — a provenance preamble on the injected block (LLM01 hardening) | The injected bodies come from the repository's own rule directory under read-side containment, so there is no external author to attribute today. A preamble marking the block as harness-supplied context would harden it if a sticky body ever became third-party | Sticky bodies gain a source outside the checkout |
| S-5 — coverage for Go fatal errors on the hook path | `recover()` cannot catch a Go runtime fatal error such as concurrent map access, so the REQ-STICKYRULE-FIRE-11 exit barrier is a recovery barrier rather than a total one. The current runtime spawns no goroutine and shares no map on that path, so the class is unreachable; this is a documented limitation, not an open defect | The sticky path gains concurrency or shared mutable state |
| S-1 residual — a counter entry that is a symlink to another name **inside** the state directory is followed rather than refused on platforms where `os.Root` is the only guard | `os.Root` resolves the final component itself, so a caller-supplied `syscall.O_NOFOLLOW` never reaches the underlying open (verified empirically on go1.26). The handle is bound to the state directory, so a followed in-root link can only reach another counter file this runtime owns, whose content is a bare integer — the reachable outcome is one session reading another session's index, which changes no trust decision. `openCounter`'s `Lstat` refuses this case in practice; what is missing is a guarantee that survives a concurrent swap in the window between that `Lstat` and the open | Go exposes an `openat`-level `O_NOFOLLOW` through `os.Root`, or the state directory gains a file whose contents are not interchangeable between sessions |
| S-1 residual — the FIFO liveness oracle is unix-only | `syscall.Mkfifo` does not exist on non-unix hosts, so `sticky_state_fifo_test.go` carries a `//go:build unix` tag and a non-unix host gets no coverage for a hostile entry type that turns a refusal bug into a hang rather than an escape. The two `openCounter` guards it exercises are themselves portable, so the gap is in the oracle rather than in the runtime | A non-unix platform becomes a supported hook host |
| S-1 residual — hardlink detection is unavailable on non-unix platforms | `hasMultipleLinks` returns false where `fs.FileInfo.Sys()` exposes no link count, so a hardlinked counter entry is written through on those platforms. The vector needs local write access on the same volume and is not deliverable by a git clone, which is the delivery channel the symlink cases use | A non-unix platform becomes a supported hook host, or Go exposes a portable link count |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| sibling of the primary | Independent user-visible outcome, user-confirmed. Different hook event, different trigger model, per-session state, and separately testable acceptance. | `SPEC-CONDRULE-001` (primary) |

Total sibling count for this pair is 1, within the maximum of 2. This SPEC creates no sibling of its own and reopens no approved primary requirement.

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `content/rules/language-policy.md` 1189 bytes, `objective-reasoning.md` 3303 bytes | existing (measured) | `wc -c` on 2026-07-30 |
| `.gemini/rules/autopus/language-policy.md` carries source keys plus `platform: antigravity-cli` | existing (measured) | `head` on the installed file on 2026-07-30 |
| all four adapters return `SupportsHooks() == true` | existing | `rg` over `pkg/adapter/*/*.go`: claude.go:50, gemini.go:65, opencode.go:44, codex.go:68 |
| `pkg/content/hooks.go` platform guard at line 91; `Timeout` values 30, 60, 5 | existing | read in full |
| `pkg/adapter/claude/claude_settings.go::prepareSettingsMapping` managed-event construction | existing | read at lines 44-115 |
| `pkg/adapter/codex/codex_rules.go::ruleFilePath`, `detectCodexSubdirSupport` returns hardcoded true | existing | read at lines 97-118 |
| `pkg/config/loader.go` typed `yaml.Unmarshal` at line 65; `HooksConf` at `schema.go` lines 200-206; `.gitignore` line 29 ignores `.autopus/runtime/` | existing | read at the cited lines; the runtime directory exists |
| `pkg/adapter/parity_test.go` 95% gate at lines 158-161; post-relocation baseline claude 14 / codex 14 / gemini 14 | existing (measured) | `go test ./pkg/adapter -run TestParity_CrossPlatformFeatures -v` on 2026-07-30 |
| Claude Code `UserPromptSubmit` structured output shape and exit-code-2 prompt erasure | existing (platform) | official Claude Code hooks documentation, checked 2026-07-30 |
| `[NEW] pkg/rulecond/sticky.go`, `[NEW] internal/cli/rules_sticky.go`, and a root parameter on `[NEW] pkg/rulecond/contain.go` | planned addition | the package, namespace, and helper are created by `SPEC-CONDRULE-001`; this SPEC owns adding the root parameter without changing the primary's call-site behavior |
| `[NEW] .autopus/runtime/sticky-rules/<sha256>`, `[NEW]` cadence field on `HooksConf` | planned addition | do not exist yet |

## Reviewer Brief

- Intended scope: re-attach two designated rules on a fixed prompt cadence in Claude Code, using the manifest, containment helper, and CLI namespace the approved primary SPEC creates, without changing where any rule is emitted or touching non-Claude platforms.
- Explicit non-goals: the conditional schema and compiler, `.agents/rules/*.md` emission, adaptive cadence, context measurement, compaction hooks, subagent sessions. Do not reopen `SPEC-CONDRULE-001` requirements.
- Self-verified: Traceability Matrix over all 25 requirements, twelve invariants each mapped to a Must oracle, Reference Discipline where every existing claim was confirmed by read, `rg`, `wc -c`, `head`, or a real test run, and byte caps checked against real file sizes.
- Reviewer should focus on: whether the four-case partition in S13 is genuinely total and single-valued; whether the recover barrier closes every route to exit 2; whether S16 proves removal as well as creation; and whether S8 now matches real adapter behavior rather than an assumed invariance.

## Live Re-Injection Evidence (2026-08-01)

REQ-STICKYRULE-VERIFY-01 and acceptance S10. A dev binary was driven against a throwaway project built by the real `auto init` flow, so the installed surface is the generated one rather than a hand-placed fixture. `<scratchpad>` redacts the absolute session scratchpad path; `~/.local/bin/auto` was not touched and no real project received hooks.

```
go build -o <scratchpad>/auto-dev ./cmd/auto
<scratchpad>/auto-dev init --dir <scratchpad>/sticky-live --project sticky-live --platforms claude-code --yes
cd <scratchpad>/sticky-live
printf '%s' "$PAYLOAD" | <scratchpad>/auto-dev rules sticky --event UserPromptSubmit    # repeated 9 times
```

Installed surface: `.claude/hooks/autopus/conditional-rules.json` carries `rules` as the unchanged `lore-commit`, `shell-portability`, `worktree-safety` and `sticky` as exactly `language-policy` and `objective-reasoning` with root-relative bodies. `.claude/settings.json` holds one `UserPromptSubmit` entry, `"command": "auto rules sticky --event UserPromptSubmit"`, `"timeout": 5`, empty matcher.

Cadence run, one fixed `session_id`, default `N = 8`:

| prompt index | exit | stdout bytes | stderr bytes |
|--------------|------|--------------|--------------|
| 1 | 0 | 4467 | 0 |
| 2..8 | 0 | 0 | 0 |
| 9 | 0 | 4467 | 0 |

Index 1 and index 9 produced byte-identical payloads. Decoded, the payload's only top-level key is `hookSpecificOutput`, whose keys are exactly `hookEventName` (value `UserPromptSubmit`) and `additionalContext`. `additionalContext` measures 4264 bytes against the 6000-byte aggregate cap, opens with `## Rule: language-policy`, and carries `## Rule: objective-reasoning`; no truncation notice is present, as expected for a set with headroom. Injectable bodies measure 1058 and 3152 bytes.

Redaction held: `additionalContext` contains none of the submitted prompt text, the planted `EXAMPLEFAKESECRET` value, the raw `session_id`, the supplied `cwd`, or the transcript path, and no frontmatter key (`alwaysApply`, `name:`) reached context.

State: one file under `.autopus/runtime/sticky-rules/`, named `ed475fc6287a3cbb17ce8ec74eae3d1739cb9c0d9ff7f2b188b4288cf1d00fdf`, which is the SHA-256 digest of the raw `session_id`, holding the single line `9`. The raw identifier appears nowhere in the file name or its content.

S10 as written, cadence 2 via `hooks.sticky_cadence: 2` in `autopus.yaml`, fresh session, three prompts: prompt 1 and prompt 3 each injected the `language-policy` body, prompt 2 wrote nothing, and every invocation exited 0 with empty stderr. `auto rules sticky` never emitted `decision` or `block`. `auto rules list` reported `2 sticky, re-attached on an effective cadence of 2 prompts` with the sticky column true for exactly the designated pair, closing the S11 and S14 observability claims from the same run.

## Implementation Deviation — per-body cap basis (accepted 2026-08-01)

REQ-STICKYRULE-FIRE-07 words the 4000-byte per-body cap over the injectable body. The landed implementation measures the whole source file including frontmatter: `pkg/rulecond/contain.go::ReadBody` compares `info.Size()` against `MaxBodyBytes`, and `pkg/rulecond/compile_sticky.go::stickySourceSize` mirrors that basis at compile time.

Accepted as-is for three reasons. It is strictly fail-closed, admitting fewer rules than the SPEC wording and never more. REQ-STICKYRULE-COMPILE-04 forbids the compiler emitting an entry the runtime would refuse, and `ReadBody`'s raw-size ceiling is shared with the SPEC-CONDRULE-001 conditional runtime whose contract must stay unchanged under REQ-STICKYRULE-VERIFY-01, so moving the runtime to an injectable-body basis would require forking the shared containment read path. Both shipped sticky rules sit far under the cap on either basis, at 1207 and 3321 bytes raw against 1058 and 3152 bytes injectable, so shipped behavior is identical.

The basis is pinned by two characterization tests in `pkg/rulecond/sticky_cap_test.go`: `TestStickyFire_PerBodyCapMeasuresTheWholeSourceFile` and `TestStickySourceFiles_KeepHeadroomUnderThePerBodyCap`.

## Self-Verify Summary

- Q-CORR-01 | status: FAIL | attempt: 5 | files: plan.md, acceptance.md | reason: re-review found the 4000-to-6000 cap fix never reached plan.md T3 or one Oracle Acceptance Note, so an executor following T3 would truncate the shipped pair; S8 also asserted gemini invariance that the installed file disproves
- Q-CORR-01 | status: PASS | attempt: 6 | files: spec.md, plan.md, acceptance.md, research.md | reason: swept every byte number across all four files; T3 and the notes now read 6000 aggregate over injectable bodies with a 4000 per-body cap, and S8 was rewritten to the verified pass-through behavior
- Q-CORR-02 | status: PASS | attempt: 6 | files: spec.md, plan.md, research.md | reason: `sticky.go`, `rules_sticky.go`, the contain.go root parameter, the state files, and the cadence field carry `[NEW]`
- Q-CORR-03 | status: PASS | attempt: 6 | files: spec.md, acceptance.md | reason: FIRE-01 now fixes the verified structured shape including `hookEventName`, and S1 asserts it; headings keep the parser-supported `### S<N>:` form
- Q-CORR-04 | status: PASS | attempt: 6 | files: research.md | reason: Reference Discipline separates existing from `[NEW]` and cites the command used for every measured claim
- Q-COMP-01 | status: PASS | attempt: 6 | files: all four | reason: each file holds a distinct role; Q-COH-01 also passes, since this is one problem, one event, one runtime file, one state model
- Q-COMP-02 | status: PASS | attempt: 6 | files: spec.md, acceptance.md | reason: all 25 requirements appear in the matrix; VERIFY-01 split from VERIFY-02 so baseline, live trace, and parity each have a mapped oracle
- Q-COMP-03 | status: PASS | attempt: 6 | files: spec.md | reason: each requirement states EARS type, trigger, expected result, and an observation point
- Q-COMP-04 | status: FAIL | attempt: 5 | files: spec.md | reason: the Outcome Lock omitted the exit barrier, the hook removal path, and platform confinement, so a build could satisfy it while erasing prompts or leaking an event to three adapters
- Q-COMP-04 | status: PASS | attempt: 6 | files: spec.md, acceptance.md | reason: all three are mandatory now and gated by S13, S16, and S15
- Q-COMP-05 | status: FAIL | attempt: 5 | files: research.md, acceptance.md | reason: INV-103 and INV-109 assigned the same missing-body input two different outputs across S4 and S13, so the partition was neither total nor single-valued
- Q-COMP-05 | status: PASS | attempt: 6 | files: spec.md, research.md, acceptance.md | reason: FIRE-08 defines four disjoint cases, S13 fixes all of them in one table, S4 is scoped to a single-rule manifest, and INV-110 through INV-112 each gained a Must oracle
- Q-COMP-06 | status: PASS | attempt: 6 | files: spec.md, research.md | reason: matrix covers all 25 requirements and the Reviewer Brief names four concrete focus areas
- Q-COMP-07 | status: PASS | attempt: 6 | files: research.md | reason: Completion Debt is empty, every review finding is closed here, and Evolution Ideas carry no IDs
- Q-FEAS-01 | status: PASS | attempt: 6 | files: plan.md | reason: rule mapping is content-layer, hook assembly and settings are adapter-layer, cadence, state, containment, and the barrier are CLI runtime
- Q-FEAS-02 | status: PASS | attempt: 6 | files: spec.md, plan.md | reason: all edits target `autopus-adk/` source of truth including the gemini template regeneration step
- Q-FEAS-03 | status: FAIL | attempt: 5 | files: spec.md, plan.md | reason: the hook lifecycle was not implementable as written, since `generateCLIHooks` cannot see a sticky set and `prepareSettingsMapping` would preserve a stale entry; the timeout was also unspecified
- Q-FEAS-03 | status: PASS | attempt: 6 | files: spec.md, plan.md, acceptance.md | reason: COMPILE-01 through COMPILE-03 name the guard, the timeout of 5, and the managed-event change, T2 names both call sites, and S16 fixes the transitions
- Q-STYLE-01 | status: PASS | attempt: 6 | files: spec.md | reason: requirement text avoids should, might, could, possibly, maybe, perhaps
- Q-STYLE-02 | status: PASS | attempt: 6 | files: spec.md | reason: Priority uses only Must and Should on a line separate from EARS Type
- Q-STYLE-03 | status: PASS | attempt: 6 | files: acceptance.md | reason: bare Given/When/Then/And steps; S2 now fixes an explicit 12-step order instead of an impossible strict alternation
- Q-SEC-01 | status: PASS | attempt: 6 | files: spec.md, research.md, acceptance.md | reason: `## Trust Boundary` names the cloned-repo channel, the redaction boundary, and the availability boundary; FIRE-06 through FIRE-12 constrain them; S12 and S13 exercise them
- Q-SEC-02 | status: PASS | attempt: 6 | files: spec.md, acceptance.md | reason: containment refuses absolute, traversal, symlink, non-regular, and non-md entries under a 4000-byte per-body cap; S12 asserts the decoy secret never appears; the state key stays a SHA-256 digest
- Q-SEC-03 | status: PASS | attempt: 6 | files: spec.md, acceptance.md | reason: the retained counter holds no prompt text and is bounded by age and count; injected bodies are frontmatter-stripped so no metadata is retained in context
- Q-COH-02 | status: PASS | attempt: 6 | files: research.md | reason: nothing required by the Outcome Lock is deferred; the two-directional lifecycle landed here rather than in a follow-up
- Q-COH-03 | status: PASS | attempt: 6 | files: spec.md, research.md | reason: single approved sibling of the primary, independent outcome slice, no sibling of its own, no primary requirement reopened

## Completion Verdict (2026-08-02)

- Outcome Lock: satisfied. Designated sticky rules re-state on a fixed prompt cadence in a long Claude Code session, the runtime reads no file outside its designated body root, it never terminates with a status that erases the user's prompt, and non-Claude platforms gain no entry.
- Mandatory requirements: 25/25.
- Must acceptance: 15/15 (16 scenarios, 15 Must).
- Completion Debt: none.
- Evolution Ideas: surfaced as optional, not scheduled.

Evidence: build and vet clean; coverage pkg/rulecond 88.7%, pkg/config 89.1%, pkg/content 93.8%, pkg/adapter/claude 86.9%, sticky CLI files 87.0% and 92.0%; parity unchanged; `auto check --hygiene --arch --quiet` exit 0; no source file over the 300-line limit; code review APPROVE with zero blockers; adversarial security audit PASS at round 3 after a reopened leaf-containment finding was closed and re-attacked; live re-injection observed and recorded above.
