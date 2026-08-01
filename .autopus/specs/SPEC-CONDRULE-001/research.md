# SPEC-CONDRULE-001 Research

## Existing Code Analysis

| Path | Role | Finding |
|------|------|---------|
| `content/rules/*.md` | rule source of truth, 14 files, flat | 8 have frontmatter (`name`, `description`, `category`); 6 have none, including `worktree-safety.md` |
| `pkg/adapter/claude/claude_prepare_files.go` | `prepareContentFiles("rules", ".claude/rules/autopus")` | copies every rule verbatim; only `file-size-limit.md` is special-cased out for dynamic render |
| `pkg/adapter/codex/codex_rules.go` | `ensureCodexRulePlatform` | parses the frontmatter block, appends `platform: codex`, preserves all other keys |
| `pkg/content/hooks.go` | `GenerateProjectHookConfigs`, `generateCLIHooks`, `appendUniqueHook` | builds `[]adapter.HookConfig`; `PreToolUse` with matcher `Bash` is already an established pattern |
| `pkg/adapter/adapter.go` | `HookConfig{Event,Matcher,Type,Command,Timeout,Env}` | the exact struct the dispatcher entry needs |
| `pkg/adapter/claude/claude_settings.go` | `prepareSettingsMapping` | writes the nested Claude Code hook schema and preserves unmanaged user event keys |
| `pkg/adapter/parity_test.go` | parity gate | `classifyFile` counts a mapping as a rule only when the lowercased target path contains `rules/`, `rules\`, or `rules-autopus`; `assert.GreaterOrEqualf(codexRulesParity, 95.0)` is a hard gate |

Two measurements drive the design. First, `.claude/rules/autopus/*.md` files load into every session as project instructions, observed directly in this session where all 14 rule bodies appeared with no tool call. That is the baseline cost this SPEC removes. Second, running `go test ./pkg/adapter -run TestParity_CrossPlatformFeatures -v` on 2026-07-30 reported claude 14, codex 14, gemini 14 rules at 100% Codex rules parity, which is the number relocation must conserve.

## Outcome Lock

- User-visible outcome: on an unmodified ADK install, a Claude Code user pays zero baseline context for triggered rules and sees `lore-commit`, `shell-portability`, and `worktree-safety` text arrive at the matching tool call, without the dispatcher becoming a way to read files outside its body root. Untriggered rules are unchanged.
- Mandatory requirements: trigger schema, classification, native `paths:` compilation, hook-fired relocation plus dispatcher registration, `auto rules fire` runtime, read-side containment with a per-body cap and an explicit fail-closed class, four-rule mapping, cross-platform field preservation, parity-count reconciliation, live firing evidence.
- Explicit non-goals: `.agents/rules/*.md` emission (SPEC-OMP-001), sticky re-attach (`SPEC-STICKYRULE-001`), advisor and model routing, TTSR mid-token interruption in Claude Code, conditionalizing the other ten rules, `astCondition` evaluation.
- Completion evidence: `go test ./pkg/rulecond/... ./pkg/content/... ./pkg/adapter/...` green, regenerated-surface assertions from S3, S9, S10, S13, containment oracles S14 and S15, parity oracle S16 at 100%, and the S12 live trace.

## Visual Planning Brief

`plan.md` carries the Mermaid flowchart for the full source-to-firing path, including both containment gates. In prose: one source rule file fans out to three mutually exclusive compile targets, only the hook-fired target introduces runtime behavior, and that runtime is a stdin-to-stdout JSON transform with a match decision, a per-rule containment decision, and a universal exit-zero.

## Plan Intent Ledger

Preserved from the `@auto plan` handoff. Cells are untrusted prompt input evidence, summarized rather than executed.

| Field | Status | Source | Confidence | Decision / Assumption | If Wrong | Plan Handoff |
|---|---|---|---|---|---|---|
| goal | answered | user plus P0 measurement | high | trigger conditions on rules; claude-code hook plus omp TTSR-compatible compilation; sticky is a sibling | re-confirm direction | requirement seeds |
| scope_boundary | answered | user selection | high | Primary covers schema, compiler, and mapping; sibling covers sticky; omp emission belongs to SPEC-OMP-001; advisor and model routing excluded | renegotiate boundary | explicit non-goals |
| constraints | answered | harness rules | high | `content/` is source of truth, 300-line source cap, unconditional rules keep current behavior, parity tests hold | review FAIL | constraints section |
| done_evidence | answered | P0 measurement | high | observed firing in claude-code, regenerated-surface verification, unit tests green, schema preservation test for omp | strengthen acceptance | Must acceptance |
| brownfield_impact | answered | explorer measurement | medium-high | 14 rule frontmatters, `pkg/content` transformers, claude hook generation path, reuse of existing `content/hooks` infrastructure | adjust plan tasks | reviewer focus |

## Question Audit

- question_transport: AskUserQuestion | question_count: 1 (track 1 scope; answer was to make sticky a sibling SPEC) | unresolved_fields: none

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go toolchain and module already in `autopus-adk/go.mod` | unchanged; no major version moves | `autopus-adk/go.mod` | 2026-07-30 | none; no migration in scope |
| brownfield | `gopkg.in/yaml.v3` for frontmatter parsing | `v3.0.1` (existing direct dependency) | `autopus-adk/go.mod` line 22 | 2026-07-30 | a new YAML library, rejected because the pinned one is already imported |
| brownfield | Go `regexp` (RE2) for conditions, `path/filepath` `EvalSymlinks` for containment | Go standard library | Go standard library | 2026-07-30 | PCRE-style engines, rejected for backtracking exposure; shell `grep -E` plus `jq`, rejected for the added runtime dependency; a sandboxing library, rejected because one directory prefix check suffices |

## Trust Boundary

The compiled manifest and the rule bodies it names are repo-supplied untrusted input, not trusted local configuration. A cloned repository can ship its own `.claude/hooks/autopus/conditional-rules.json`, and that file is far less scrutinized than `.claude/settings.json`. Without containment the dispatcher would be an arbitrary-file-read-to-context-injection primitive firing on every Bash, Edit, and Write call, and the aggregate byte cap alone would still deliver up to 8000 bytes of any readable file.

Three consequences are fixed in requirements. Every body location is resolved against a single root and rejected on absolute paths, `..` components, symlink escape, non-regular files, and non-`.md` suffixes, under a per-body size cap (FIRE-07, FIRE-08, FIRE-11). Body text is model-trusted instruction once injected, so confining bodies to one generated directory keeps that surface reviewable, and COMPILE-06 makes the compiler refuse to mint an entry the dispatcher would reject. Fail-open and fail-closed stay separate (FIRE-09), because collapsing them would make a containment violation indistinguishable from a missing file, which is exactly how an attacker would prefer the failure to look.

## Design Decisions

**Two firing mechanisms, chosen by what the platform already provides.** Claude Code implements `paths:` frontmatter natively, so a `globs`-only rule needs a field translation and no runtime. Tool-input regex matching has no native counterpart, so it needs the dispatcher.

**Relocation, not suppression.** A hook-fired rule stops earning its context cost only if its body leaves every auto-loaded directory, and `.claude/hooks/autopus/` is not an auto-loaded rules path. Bodies stay separate files rather than inlined in the manifest, so the per-call parse stays small and each body remains diff-reviewable and individually containable.

**Go subcommand, not a shell script.** `auto` is already required by existing hook commands, so this adds no dependency, whereas a shell dispatcher would need `jq`. Go `regexp` is RE2, so a bad rule regex cannot backtrack catastrophically, and `filepath.EvalSymlinks` gives a correct containment check that a shell dispatcher would have to approximate.

**Advisory only.** A regex on a command string produces false positives, so a false positive must cost a little context, never a blocked action.

**Parity counting follows delivery, not path shape.** The gate exists to prove Codex stays aligned with Claude on managed rules. After relocation claude-code still delivers all 14 rules, 11 always-loaded plus 3 hook-fired, so the rule set is intact and only the path-based proxy breaks. Extending `classifyFile` to count conditional bodies restores what the gate was always measuring. The alternative of counting `content/rules` source entries was rejected because it decouples the gate from emitted output and would stop catching an adapter that silently fails to emit a rule. The manifest file is not miscounted, because `conditional-rules.json` contains no `conditional/` path segment.

**Considered and rejected: the Claude Code per-handler `if` field.** `if: "Bash(rm *)"` can pre-filter before the hook spawns, but its glob syntax is weaker than regex and would split matching logic across two languages.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | 14 rule bodies observed loaded in this session with no tool call; 3 apply to narrow moments | proceed | remove baseline cost for narrowly scoped rules |
| existing code/helper/pattern | `pkg/content/skills.go::splitFrontmatter`, `pkg/content/hooks.go::generateCLIHooks` + `appendUniqueHook`, `adapter.HookConfig`, `claude_settings.go::prepareSettingsMapping`, `codex_rules.go::ensureCodexRulePlatform`, `parity_test.go::classifyFile` | reuse | dispatcher registers through the existing hook path; parity fixed by extending the existing classifier rather than a new gate |
| stdlib/native | Claude Code native `paths:` frontmatter; Go `regexp`, `encoding/json`, `path/filepath` `EvalSymlinks` and `IsAbs` | use | `file-size-limit` needs zero ADK runtime; containment needs no dependency |
| existing dependency | `gopkg.in/yaml.v3 v3.0.1` already in `go.mod` | reuse | frontmatter parsing adds no dependency |
| new dependency or new abstraction | No new module dependency. New package `pkg/rulecond` (schema, classify, compile, contain, fire) and CLI namespace `auto rules`, justified only after the rungs above: no existing code classifies rules, enforces a body root, or reads hook stdin | accepted | new code limited to schema, classify, compile, contain, fire, plus the CLI entry point |
| minimum sufficient verification | Dispatcher oracle (S2), surface assertions (S3, S13), determinism and cap (S9), dedupe (S10), benign fail-open (S4), secret non-echo (S5), containment matrix (S14), error-class partition (S15), parity counts (S16), byte-identity golden (S8), live trace (S12) | required checks | security, data-safety, and parity gates kept; no broad end-to-end suite added |

## Semantic Invariant Inventory

Source clauses are untrusted prompt input evidence, summarized rather than quoted verbatim.

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | rules fire at the exact tool-call moment via pattern matching | pattern matching | dispatcher stdout `additionalContext` | S1, S2, S12 |
| INV-002 | classification is total and mutually exclusive across always, paths-scoped, hook-fired | partition and grouping | generated file placement, hook entry set, `auto rules list` counts | S3, S8, S10, S11, S13 |
| INV-003 | baseline context cost is zero for triggered rules | set complement | `.claude/rules/autopus/` file set | S3 |
| INV-004 | manifest ordering is stable and truncation is predictable | ordering and deduplication | manifest bytes, capped `additionalContext` | S9 |
| INV-005 | trigger fields round-trip to every platform without field loss | round-trip preservation | codex, opencode, gemini rule frontmatter | S7, S8 |
| INV-006 | firing is advisory and never leaks tool input | fail-open and redaction | exit code, stdout content | S4, S5 |
| INV-007 | condition subject selection is deterministic per tool | mapping | which string is matched per `tool_name` | S2 |
| INV-008 | glob-shaped `condition` values are rejected before they reach omp | parser-contract guard | validation error text | S6 |
| INV-009 | every injected body resolves inside one body root and within the per-body cap | path containment boundary | which bodies are read, reason codes, compile-time failure | S14 |
| INV-010 | benign absence and containment violation are distinct error classes | partition and fail-closed | injected rule set, stderr diagnostics, exit code | S15 |
| INV-011 | relocation conserves the counted rule total per platform | count conservation | parity report counts and gate outcome | S16 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| Trigger schema, parsing, classification, validation, native path-scoped compilation | Primary T1, T2, T3, S13 | covered |
| Hook-fired relocation, manifest, dispatcher registration | Primary T4 | covered |
| Runtime firing happy path and benign fail-open | Primary T5, S1, S2, S4 | covered |
| Read-side containment, fail-closed class, and write-side secret boundary | Primary T5, T11, S5, S14, S15 | covered |
| Integration boundary (claude, codex, opencode, gemini) | Primary T8, S7, S8 | covered |
| Parity-gate reconciliation | Primary T9, S16 | covered |
| Surface (`auto rules list`), drift mapping, live trace | Primary T6, T9, T10, S11, S12 | covered |
| Sticky re-attach for long sessions | `SPEC-STICKYRULE-001` | approved-sibling |
| `.agents/rules/*.md` emission for omp | SPEC-OMP-001 (separate track) | out of scope by explicit non-goal |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

Read-side containment was debt in revision 3 and is now closed inside the Primary SPEC as FIRE-07 through FIRE-11, COMPILE-06, T11, S14, and S15.

## Evolution Ideas

Optional improvements. They do not block sync completion and carry no SPEC, task, or acceptance IDs.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| Use the Claude Code per-handler `if` field to pre-filter before spawning the dispatcher | Single spawn per Bash call is already cheap; unmeasured | A measured spawn-cost regression |
| Sign the manifest against the installed ADK version, or evaluate `astCondition` inside the ADK | Containment already bounds the blast radius to one generated directory; no ast-grep dependency exists and omp owns that semantics | A threat model with a writable body root, or an explicit user request |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| one sibling | Independent user-visible outcome, user-confirmed. Sticky re-attach uses a different hook event (`UserPromptSubmit`), needs per-session counter state that conditional firing does not, and has separately testable acceptance. | `SPEC-STICKYRULE-001` |

Sibling count is 1 of the allowed maximum of 2. Recursive siblings are prohibited. The read-side containment contract defined here is mirrored onto the sibling's own body root by `SPEC-STICKYRULE-001` REQ-STICKYRULE-FIRE-06 and FIRE-07.

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `content/rules/*.md` (14 files, 6 without frontmatter) | existing | `ls` and `head` on each file |
| `content/rules/worktree-safety.md` has no frontmatter | existing | first line is `# Worktree Safety Rules` |
| `pkg/content/hooks.go::GenerateProjectHookConfigs`, `generateCLIHooks`, `appendUniqueHook` | existing | read in full |
| `pkg/adapter/claude/claude_prepare_files.go::prepareContentFilesForConfig` | existing | read at lines 105-153 |
| `pkg/adapter/parity_test.go::classifyFile`, `countFeatures`, `parityPct`, `TestParity_ClassifyFile` | existing | read in full; `classifyFile` rules case at lines 41-43, gate at lines 158-161 |
| Parity baseline claude 14 / codex 14 / gemini 14 at 100% | existing (measured) | `go test ./pkg/adapter -run TestParity_CrossPlatformFeatures -v` on 2026-07-30 |
| `internal/cli/root.go` has no `rules` command; `internal/cli/check_rules_hygiene.go` maps `.claude/rules/` at lines 134-169; `gopkg.in/yaml.v3 v3.0.1` at `go.mod` line 22; `adapter.HookConfig` at lines 28-75; `codex_rules.go::ensureCodexRulePlatform` at lines 77-95; `claude_settings.go::prepareSettingsMapping` at lines 44-115; `opencode_rules.go::prepareRuleMappings` at lines 21-48; `gemini_rules.go::stripFrontmatter` at lines 140-156; `skills.go::splitFrontmatter` at lines 241-258 | existing | each opened with Read at the cited lines |
| Claude Code `.claude/rules/` auto-load and `paths:` frontmatter | existing (platform) | official Claude Code memory documentation, checked 2026-07-30, plus direct observation in this session |
| Claude Code `PreToolUse` stdin schema and `hookSpecificOutput.additionalContext` | existing (platform) | official Claude Code hooks documentation, checked 2026-07-30 |
| `[NEW] pkg/rulecond/{schema,classify,compile_claude,contain,fire}.go` | planned addition | does not exist yet |
| `[NEW] internal/cli/rules.go` with `newRulesCmd`, `auto rules fire`, `auto rules list` | planned addition | does not exist yet |
| `[NEW]` `classifyFile` conditional-body case and its `TestParity_ClassifyFile` rows | planned addition | not present in the current test |
| `[NEW] .claude/hooks/autopus/conditional-rules.json` and `[NEW] .claude/hooks/autopus/conditional/<name>.md` | planned addition | generated artifacts, do not exist yet |

## Reviewer Brief

- Intended scope: make triggered ADK rules cost zero baseline context in Claude Code and fire at the matching tool call, with a contained read path and omp-compatible frontmatter preserved for a separate track.
- Explicit non-goals: `.agents/rules/*.md` emission, sticky re-attach, advisor and model routing, TTSR in Claude Code, conditionalizing the other ten rules, `astCondition` evaluation. Do not expand review into these.
- Self-verified: Traceability Matrix over all 28 requirements, eleven invariants each mapped to a Must oracle, Reference Discipline with existing references confirmed by read, `rg`, or a real test run, Minimality Decision Matrix, and the parity arithmetic checked by executing the gate.
- Reviewer should focus on: whether S14 covers every route into the body root, including normalization and symlink cases; whether the fail-open and fail-closed split in S15 is genuinely total; whether the `classifyFile` change in S16 leaves the manifest uncounted; and whether relocation still removes the three rules from baseline context.

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 3 | files: research.md, spec.md, plan.md | reason: every non-`[NEW]` reference opened or confirmed with rg; parity counts confirmed by running the test
- Q-CORR-02 | status: PASS | attempt: 3 | files: spec.md, plan.md, research.md | reason: `pkg/rulecond`, `internal/cli/rules.go`, the `classifyFile` case, and the generated artifacts carry `[NEW]`
- Q-CORR-03 | status: PASS | attempt: 2 | files: acceptance.md, plan.md | reason: attempt 1 failed strict validation on `## Task List`, a missing `## Oracle Acceptance Notes`, and `### Scenario N:` headings unpaired with the matrix `S<N>` IDs; headings moved to the parser-supported `### S<N>:` form and both sections added; hook JSON verified against official documentation on 2026-07-30
- Q-CORR-04 | status: PASS | attempt: 4 | files: research.md | reason: Reference Discipline separates existing from `[NEW]`, including the planned `classifyFile` case, and separates `content/` source of truth from generated surfaces
- Q-COMP-01 | status: PASS | attempt: 3 | files: all four | reason: each file holds a distinct role and none delegates its content to another
- Q-COH-01 | status: PASS | attempt: 4 | files: all four | reason: one problem, one source directory, one adapter path, one CLI namespace
- Q-COMP-02 | status: FAIL | attempt: 3 | files: spec.md, plan.md, acceptance.md | reason: review revision 3 found body-path containment and per-body size untraced to any requirement, task, or scenario
- Q-COMP-02 | status: PASS | attempt: 4 | files: spec.md, plan.md, acceptance.md | reason: OBS-01 lowered to Should; containment added as FIRE-07 through FIRE-11 and COMPILE-06 with T11, S14, S15; all 28 requirements traced
- Q-COMP-02 | status: PASS | attempt: 5 | files: spec.md, plan.md, acceptance.md | reason: re-review found REQ-CONDRULE-FIRE-09 still describing a missing body file as whole-run empty output, contradicting S15 row 4; FIRE-09 now states per-rule skip, plan T5 carried the same imprecision and was aligned, and S4 was qualified to a single-rule manifest so its whole-run expectation stays true
- Q-COMP-05 | status: PASS | attempt: 5 | files: spec.md, acceptance.md | reason: INV-010 and its oracle S15 were already correct and are unchanged; the requirement text was the only surface out of step, so alignment was one-directional onto FIRE-09
- Q-COMP-03 | status: PASS | attempt: 3 | files: spec.md | reason: each requirement states EARS type, trigger, expected result, and an observation point in the matrix
- Q-COMP-04 | status: FAIL | attempt: 3 | files: spec.md, research.md | reason: the Outcome Lock omitted safe runtime behavior, so a build could satisfy it while exposing arbitrary file reads
- Q-COMP-04 | status: PASS | attempt: 4 | files: spec.md, research.md | reason: Outcome Lock now requires containment, and S14 plus S15 gate it with concrete oracles
- Q-COMP-05 | status: FAIL | attempt: 3 | files: research.md, acceptance.md | reason: no invariant or oracle covered the read side of the boundary
- Q-COMP-05 | status: PASS | attempt: 4 | files: research.md, spec.md, acceptance.md | reason: INV-009, INV-010, INV-011 added, each traced to a requirement, a task, and a Must oracle with concrete expected values
- Q-COMP-06 | status: PASS | attempt: 4 | files: spec.md, research.md | reason: matrix covers all 28 requirements and the Reviewer Brief names four concrete focus areas
- Q-COMP-07 | status: FAIL | attempt: 3 | files: research.md | reason: Completion Debt was declared None while required security hardening was absent
- Q-COMP-07 | status: PASS | attempt: 4 | files: research.md | reason: the hardening is now inside the Primary SPEC and the closure is recorded under Completion Debt
- Q-FEAS-01 | status: PASS | attempt: 3 | files: plan.md | reason: schema is content-layer, compilation is adapter-layer, firing and containment are CLI runtime
- Q-FEAS-02 | status: PASS | attempt: 3 | files: spec.md, plan.md | reason: all edits target `autopus-adk/` source of truth; generated surfaces appear only as outputs
- Q-FEAS-03 | status: FAIL | attempt: 3 | files: plan.md, spec.md, acceptance.md | reason: relocation drops claude rules to 11 against codex 14, so parityPct is 78.6% and the promised parity step cannot pass
- Q-FEAS-03 | status: PASS | attempt: 4 | files: plan.md, spec.md, acceptance.md | reason: REQ-CONDRULE-VERIFY-02 and the T9 `classifyFile` step restore claude 14; S16 fixes the counts as an oracle and the arithmetic was checked by running the gate
- Q-STYLE-01 | status: PASS | attempt: 3 | files: spec.md | reason: requirement text avoids should, might, could, possibly, maybe, perhaps
- Q-STYLE-02 | status: PASS | attempt: 3 | files: spec.md | reason: Priority uses only Must and Should on a line separate from EARS Type
- Q-STYLE-03 | status: PASS | attempt: 3 | files: acceptance.md | reason: bare Given/When/Then/And steps ending in complete sentences
- Q-SEC-01 | status: FAIL | attempt: 3 | files: spec.md, research.md, acceptance.md | reason: the manifest-to-body read path was treated as trusted configuration despite being repo-suppliable
- Q-SEC-01 | status: PASS | attempt: 4 | files: spec.md, research.md, acceptance.md | reason: `## Trust Boundary` names the cloned-repo channel and the instruction-injection consequence; FIRE-07 through FIRE-11 constrain it; S14 exercises it
- Q-SEC-02 | status: FAIL | attempt: 3 | files: spec.md, acceptance.md | reason: nothing stopped a manifest entry naming a credentials file, and only the aggregate cap applied
- Q-SEC-02 | status: PASS | attempt: 4 | files: spec.md, acceptance.md | reason: absolute, traversal, symlink, non-regular, and non-md entries are refused; a 4000-byte per-body cap applies; S14 asserts the decoy secret never appears in stdout or stderr
- Q-SEC-03 | status: FAIL | attempt: 3 | files: spec.md, acceptance.md | reason: the retained manifest was an unvalidated control surface for repeated arbitrary reads
- Q-SEC-03 | status: PASS | attempt: 4 | files: spec.md, acceptance.md | reason: manifest paths are root-relative and validated at compile and dispatch time, the location is fixed, and stderr diagnostics carry only rule name and reason code
- Q-COH-02 | status: FAIL | attempt: 3 | files: research.md | reason: required read-side work sat in neither the Primary SPEC nor Completion Debt
- Q-COH-02 | status: PASS | attempt: 4 | files: research.md, spec.md | reason: the work is in the Primary SPEC; Evolution Ideas hold only optional items such as manifest signing
- Q-COH-03 | status: PASS | attempt: 4 | files: spec.md, research.md | reason: exactly one sibling, justified by an independent outcome slice, with no recursive sibling
