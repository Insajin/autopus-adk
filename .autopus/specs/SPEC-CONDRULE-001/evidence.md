# SPEC-CONDRULE-001 — Live Completion Evidence (S12)

**Captured**: 2026-07-31
**Task**: CR-T8 (plan T10 live half)
**Binary under test**: built from the working tree at `cmd/auto`, executed from the canonical path
`/private/tmp/claude-502/-Users-bitgapnam-Documents-github-autopus-co/71bb2b80-99d0-4b0b-a4ec-3569f0749de6/scratchpad/auto-condrule`
**Workspace under test**: `/Users/bitgapnam/Documents/github/autopus-co` (root workspace)

## Verdict

The compiler, the manifest, the dispatcher registration, and the dispatcher runtime all behave as specified.
The **upgrade path does not**: `auto update` leaves the three relocated rule bodies in
`.claude/rules/autopus/`, so an existing install keeps paying baseline context for them. The Outcome Lock
("zero baseline context for triggered rules") is met on a fresh install and **not met on upgrade**.
See [Blocker B1](#blocker-b1--stale-baseline-rule-files-are-never-pruned-on-upgrade).

## Step 1-2 — Build and workspace update

```
go build -o <scratchpad>/auto-condrule ./cmd/auto      # exit 0
cd /Users/bitgapnam/Documents/github/autopus-co
<binary> update --local -y                             # exit 0
  ✓ claude-code updated ... Update complete: 5 platform(s) updated
```

The `--local --plan` preview run beforehand predicted exactly the SPEC-intended change set:

```
emit  .claude/hooks/autopus/conditional-rules.json          (new)
emit  .claude/hooks/autopus/conditional/lore-commit.md      (new)
emit  .claude/hooks/autopus/conditional/shell-portability.md (new)
emit  .claude/hooks/autopus/conditional/worktree-safety.md  (new)
prune .claude/rules/autopus/lore-commit.md        — stale managed artifact would be pruned
prune .claude/rules/autopus/shell-portability.md  — stale managed artifact would be pruned
prune .claude/rules/autopus/worktree-safety.md    — stale managed artifact would be pruned
[runtime_state] .autopus/claude-code-manifest.json — 16 emit, 103 retain, 3 prune actions
```

The four emits happened. **The three prunes did not.**

## Step 3 — On-disk surface

### (a) Baseline rule directory — FAIL

```
$ ls -1 .claude/rules/autopus/
branding.md  context7-docs.md  deferred-tools.md  doc-storage.md  file-size-limit.md
language-policy.md  lore-commit.md  objective-reasoning.md  project-identity.md
shell-portability.md  spec-quality.md  subagent-delegation.md  techstack-freshness.md
worktree-safety.md
count=14        # expected 11
```

`lore-commit.md`, `shell-portability.md`, `worktree-safety.md` are all still PRESENT. Their mtimes are
`2026-07-26 07:15:01` (the previous update) while the relocated bodies are `2026-07-31 14:49:26`, so these
are stale leftovers, not fresh writes. Two of the three differ in content from the relocated body.

### (b) Conditional body root — PASS

```
$ ls -1 .claude/hooks/autopus/conditional/
lore-commit.md  shell-portability.md  worktree-safety.md
$ ls -1 .claude/hooks/autopus/conditional-rules.json
.claude/hooks/autopus/conditional-rules.json
```

### (c) Dispatcher registration — PASS

Exactly one PreToolUse dispatcher entry (2 PreToolUse entries total, 1 of which is the dispatcher):

```json
{ "matcher": "Bash", "type": "command",
  "command": "auto rules fire --event PreToolUse", "timeout": 10 }
```

## Step 4 — Dispatcher firing trace (deterministic)

Matching subject, real hook payload shape:

```
$ printf '%s' '{"tool_name":"Bash","tool_input":{"command":"git gc --prune=now"}}' \
    | <binary> rules fire --event PreToolUse
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":
 "## Rule: worktree-safety\n# Worktree Safety Rules\n\nIMPORTANT: These rules apply whenever parallel
  executor agents run in isolated worktrees during pipeline Phase 2.\n\n## Prohibited Commands During
  Parallel Execution\n\n... - `git gc` — may delete objects referenced by other worktrees ..."}}
EXIT=0
```

Non-matching subject:

```
$ printf '%s' '{"tool_name":"Bash","tool_input":{"command":"ls"}}' \
    | <binary> rules fire --event PreToolUse
(no output)
EXIT=0
```

Both exit 0 and neither emits `permissionDecision`, `continue: false`, or exit 2, satisfying
REQ-CONDRULE-FIRE-06. The matched command text `git gc --prune=now` does not appear in the payload —
only the rule name and rule body — satisfying REQ-CONDRULE-FIRE-04.

### `auto rules list`

```
RULE                 CLASS         TRIGGER                          CLAUDE-CODE-DESTINATION
branding             always        -                                .claude/rules/autopus/branding.md
context7-docs        always        -                                .claude/rules/autopus/context7-docs.md
deferred-tools       always        -                                .claude/rules/autopus/deferred-tools.md
doc-storage          always        -                                .claude/rules/autopus/doc-storage.md
file-size-limit      paths-scoped  **/*.go,**/*.ts,...,**/*.rs      .claude/rules/autopus/file-size-limit.md
language-policy      always        -                                .claude/rules/autopus/language-policy.md
lore-commit          hook-fired    tool:bash                        .claude/hooks/autopus/conditional/lore-commit.md
objective-reasoning  always        -                                .claude/rules/autopus/objective-reasoning.md
project-identity     always        -                                .claude/rules/autopus/project-identity.md
shell-portability    hook-fired    tool:bash                        .claude/hooks/autopus/conditional/shell-portability.md
spec-quality         always        -                                .claude/rules/autopus/spec-quality.md
subagent-delegation  always        -                                .claude/rules/autopus/subagent-delegation.md
techstack-freshness  always        -                                .claude/rules/autopus/techstack-freshness.md
worktree-safety      hook-fired    tool:bash                        .claude/hooks/autopus/conditional/worktree-safety.md

14 rules: 10 always, 1 paths-scoped, 3 hook-fired
```

## Step 5 — Live Claude Code session — NOT OBSERVED

Claude Code `2.1.220`, run from the root workspace after the update. Three flag variations were attempted:

1. `claude -p --debug "<prompt>" --allowedTools Bash` — failed: `--debug` consumed the prompt as its
   optional filter argument (`Error: Input must be provided ... when using --print`).
2. `claude -p "<prompt>" --debug --allowedTools Bash < /dev/null` — exit 0, the command ran, stderr empty
   (0 bytes); no hook diagnostics surfaced.
3. `claude -p "<prompt>" --output-format stream-json --verbose --allowedTools Bash < /dev/null` — exit 0.

Variation 3 confirms the tool call that should have triggered the dispatcher did happen:

```
TOOL_USE: Bash {"command": "git gc --help", "description": "Show git gc manual page"}
```

`git gc --help` matches `\bgit\s+(gc|prune|repack)\b`. But the only hook events in the stream were
SessionStart:

```
{"subtype":"hook_started","hook_name":"SessionStart:startup", ...}
{"subtype":"hook_response","hook_name":"SessionStart:startup","exit_code":0,"outcome":"success", ...}
```

No `PreToolUse` `hook_started` / `hook_response` event appeared. **Live PreToolUse firing is therefore
not yet observed.** Per the task instruction this is recorded, not fabricated; the Step 4 deterministic
trace stands as the firing evidence. Two candidate explanations, neither yet confirmed:

- Claude Code gates newly written hook definitions behind review before executing them, and
  `.claude/settings.json` was rewritten by `auto update` minutes before the session started.
- Even if it had executed, it would have failed open — see the Step 6 caveat below.

Interactive-session observation remains pending the user's next session start, after binary promotion.

## Step 6 — Caveat: the installed `auto` predates `rules fire`

The hook command is the bare string `auto rules fire --event PreToolUse`, which resolves through `PATH`
to the installed binary, not the build under test:

```
$ command -v auto
/Users/bitgapnam/.local/bin/auto
$ auto --help | grep -c rules
0                                    # the `rules` command does not exist there
$ printf '%s' '{"tool_name":"Bash","tool_input":{"command":"git gc"}}' \
    | auto rules fire --event PreToolUse
Error: unknown flag: --event
EXIT=1
```

Until the new binary is promoted through the managed ADK flow, every new session's PreToolUse dispatcher
exits 1 with an unknown-flag error. That is non-blocking (Claude Code treats a non-zero, non-2 hook exit
as an advisory failure), so the failure mode is fail-open and no tool call is denied or delayed — but no
rule text is injected either. **Promotion of the new `auto` binary is a required follow-up before the
Outcome Lock holds in a live session.**

## Step 7 — Verification table

| Command | Result |
|---------|--------|
| `go test ./pkg/rulecond/...` | `ok github.com/insajin/autopus-adk/pkg/rulecond` |
| `go test ./pkg/content/... ./pkg/adapter/claude/...` | `ok .../pkg/content`, `ok .../pkg/adapter/claude` |
| `go test ./pkg/adapter -run TestParity` | `ok github.com/insajin/autopus-adk/pkg/adapter` |
| `go build ./... && go vet ./...` | exit 0, no output |
| `<binary> spec validate .autopus/specs/SPEC-CONDRULE-001 --strict` | `SPEC 검증 통과`, exit 0 |

No GNU `timeout` wrapper was used on any command (macOS shell-portability rule).

## Blocker B1 — Stale baseline rule files are never pruned on upgrade

**Severity**: blocks the Outcome Lock on every existing install. Fresh installs are unaffected.

**Root cause** (located, not inferred). Two different prune scopes are used for the same platform:

- `internal/cli/update_preview.go:223` — `previewPruneRoots("claude-code")` falls through to `default:`
  and returns `nil`. `isPruneEligible` (`pkg/adapter/manifest_diff.go:80`) treats an empty root list as
  "everything is eligible", so the **preview** lists all three stale rule files as `prune`.
- `pkg/adapter/claude/claude_update.go:75` — the **apply** path calls
  `adapter.BuildManifestDiff(oldManifest, newFiles, []string{".claude/skills/autopus"})`. Only
  `.claude/skills/autopus` is prune-eligible, so `.claude/rules/autopus/*` can never enter `diff.Prune`
  and `TransactionRemovesFromManifestDiff` receives nothing to remove.

The preview promises a prune the apply path is structurally incapable of performing.

**Why it does not self-heal**: the manifest is rewritten regardless, dropping the three old
`.claude/rules/autopus/` entries. A re-run of `update --local --plan` now lists **no** prune actions for
them, confirmed after the fact. The files are permanent orphans; only a manual `rm` or a clean reinstall
removes them.

**Also unwired**: `adapter.PruneManagedPaths` (`pkg/adapter/manifest_prune.go:11`) is fully implemented,
including backup-before-delete, and has zero callers anywhere in the repo, tests included.

**Not fixed here** — CR-T8 is evidence capture and was instructed to report a defect rather than repair
it. The three stale files were deliberately left in place so the defect stays reproducible.

## B1 Fix

**Fixed**: 2026-07-31, same session, after the evidence above.

### Design decision

One exported source of truth, `claude.PruneRoots()` in `pkg/adapter/claude/claude_update.go`, is now consumed
by both the apply path (`buildUpdateTransactionPlan`) and the preview path
(`internal/cli/update_preview.go::previewPruneRoots`, `case "claude-code"`). Symmetry is structural rather
than two lists that happen to agree, so the two cannot drift apart again.

Root set — `.claude/skills/autopus`, `rulecond.ClaudeRulesRelDir`, `rulecond.BodyRootRelPath`:

- The rule directory is required for the relocation this SPEC introduces.
- The conditional body root is included for the reverse direction: a rule reclassified from `hook-fired`
  back to `always` must have its stale relocated body deleted.
- The `rulecond` constants are referenced rather than restated so a path change cannot silently desync.
- The set was deliberately NOT widened to every managed directory. Pruning is already limited to paths
  recorded in the previous manifest, but a narrow, compiler-owned root set keeps the blast radius small.

`previewPruneRoots` returning `nil` for claude-code **was** part of the bug, not a harmless default:
`isPruneEligible` treats an empty root list as "everything is eligible", which is what made the preview
over-promise. Other platforms still take the `nil` default; that asymmetry is untouched here because their
adapters are outside this task's ownership, and it is recorded below as a follow-up.

### `adapter.PruneManagedPaths` — left untouched, deliberately

It is not wired, and it should not be: the transaction path already provides the safe-delete contract.
`transaction.removePath` (`pkg/adapter/transaction.go`) calls `tx.snapshot(rel, "remove")` before the
`os.Remove`, which copies the file to `.autopus/backup/<txid>/transaction/claude-code/<rel>` and journals it
for rollback. Wiring `PruneManagedPaths` in addition would double-delete, produce a second uncoordinated
backup, and bypass the transaction journal. It remains dead code pending a separate decision to delete it;
this task was instructed not to remove it.

### Regression tests

`pkg/adapter/claude/claude_update_prune_test.go` (new):

| Test | Asserts |
|------|---------|
| `TestUpdate_PrunesRelocatedBaselineRules` | end-to-end: a seeded pre-relocation manifest plus the 3 stale files on disk, then `Update` deletes all 3, keeps the relocated bodies, and leaves a backup |
| `TestBuildUpdateTransactionPlan_PruneSetCoversRelocation` | the computed prune set is exactly the 3 relocated rules; a still-generated rule is retained |
| `TestUpdate_NeverPrunesUnmanagedFiles` | a user file dropped into `.claude/rules/autopus/` and absent from the manifest survives |
| `TestPruneRoots_CoverBothSidesOfRelocation` | pins the root set, since dropping either directory reintroduces B1 in one direction |

`internal/cli/update_preview_prune_symmetry_test.go` (new):

| Test | Asserts |
|------|---------|
| `TestPreviewPruneRoots_MatchClaudeApplyPath` | `previewPruneRoots("claude-code")` equals `claude.PruneRoots()` |
| `TestPreviewAndApplyComputeSamePruneSet` | both root sets produce an identical `diff.Prune` through the shared builder, and a managed file outside the roots (`CLAUDE.md`) is never pruned |

The regression tests were confirmed to fail without the fix. Reverting only the apply-path root list produced:

```
--- FAIL: TestBuildUpdateTransactionPlan_PruneSetCoversRelocation
--- FAIL: TestUpdate_PrunesRelocatedBaselineRules
      file ".../.claude/rules/autopus/lore-commit.md" exists
        — lore-commit.md must not survive the upgrade in baseline context
      ... shell-portability.md, worktree-safety.md identical
      Should NOT be empty, but was []
        — a pruned managed file must be backed up before deletion
```

`TestUpdate_NeverPrunesUnmanagedFiles` passes in both states, which is correct: it is a safety invariant
that must hold before and after.

### Verification

```
go build ./...                                        # exit 0
go test -count=1 ./pkg/adapter/... ./internal/cli/
ok  github.com/insajin/autopus-adk/pkg/adapter          60.512s
ok  github.com/insajin/autopus-adk/pkg/adapter/claude    30.783s
ok  github.com/insajin/autopus-adk/pkg/adapter/codex     32.733s
ok  github.com/insajin/autopus-adk/pkg/adapter/gemini    53.262s
ok  github.com/insajin/autopus-adk/pkg/adapter/omp       22.564s
ok  github.com/insajin/autopus-adk/pkg/adapter/opencode  29.127s
ok  github.com/insajin/autopus-adk/internal/cli         166.592s
```

### End-to-end CLI demonstration

Run in a temp workspace (never the real root), seeded from the real pre-upgrade state with a
pre-CONDRULE manifest that still claims the three rule files:

```
=== BEFORE: baseline rule count ===   14
=== PLAN prune lines ===
- [generated_surface] prune .claude/rules/autopus/lore-commit.md (claude-code) — stale managed artifact would be pruned
- [generated_surface] prune .claude/rules/autopus/shell-portability.md (claude-code) — stale managed artifact would be pruned
- [generated_surface] prune .claude/rules/autopus/worktree-safety.md (claude-code) — stale managed artifact would be pruned
=== APPLY ===                         Update complete: 5 platform(s) updated
=== AFTER: baseline rule count ===    11
=== AFTER: are the 3 gone? ===        lore-commit.md pruned / shell-portability.md pruned / worktree-safety.md pruned
=== backups of pruned files ===
.autopus/backup/20260731T062159.630888000/transaction/claude-code/.claude/rules/autopus/lore-commit.md
.autopus/backup/20260731T062159.630888000/transaction/claude-code/.claude/rules/autopus/shell-portability.md
.autopus/backup/20260731T062159.630888000/transaction/claude-code/.claude/rules/autopus/worktree-safety.md
```

Preview and apply now agree: the plan announced exactly three prunes and the apply performed exactly
those three, 14 baseline rules down to 11. Each pruned file is recoverable from the transaction backup.
(The same backup directory also holds write-snapshots of other rule files; the transaction snapshots
before writes as well as before removes, so backup presence alone does not imply deletion.)

### Residual: already-orphaned workspaces do not self-heal

The fix prevents future orphaning; it does **not** retroactively clean a workspace the buggy version
already orphaned. Once the old manifest entry is gone, `BuildManifestDiff` has nothing to compare against
and never proposes the prune, regardless of the root set. The real root workspace
`/Users/bitgapnam/Documents/github/autopus-co` is in exactly that state after the CR-T8 evidence run: the
three stale files remain in `.claude/rules/autopus/` and no future `auto update` will remove them.

Closing that requires either a one-time manual removal of the three files, or a separate reconciliation
step that prunes compiler-owned directories by comparing against generated output rather than against the
previous manifest. That decision is out of scope for this fix and is left as a follow-up.

## B1 Residual Closure (2026-07-31, dispatcher)

The already-orphaned real root workspace was repaired by a one-time removal of the three stale files
(`lore-commit.md`, `shell-portability.md`, `worktree-safety.md`) from `.claude/rules/autopus/`.
Post-state verified: exactly 11 baseline rule files; the three relocated bodies plus
`conditional-rules.json` present under `.claude/hooks/autopus/conditional/`. Backups from the CR-T8
transaction remain under `.autopus/backup/`.

## Review-Barrier Remediation (2026-07-31)

Five security findings (H1 name-injection, H2 aggregate-cap bypass — both PoC-proven — M1 unbounded
manifest read, M2 CWD root resolution, L1 unbounded body read) were fixed in `pkg/rulecond` and
`internal/cli/rules.go` with extended oracles; the pre-fix H2 PoC reproduced 20037 bytes against the
8000-byte cap and the fixed assembly is bounded to `MaxContextBytes` including the truncation notice.
Dispatcher hook registration now derives from `CompiledClaude.Hooks`
(`pkg/content/hooks_conditional.go::ConditionalDispatcherHooks`); the hook shape has a single owner in
`pkg/rulecond` and an edit-scoped rule registers an `Edit|Write|MultiEdit` matcher (regression test).
Deterministic gate after remediation: `{"verdict":"pass","build_exit":0,"test_exit":0}`.

## S12 Live Firing Trace (2026-07-31)

Observed in a fresh headless Claude Code session (v2.1.220) from the workspace root, with the dev-built
`auto` (post-remediation tree) resolved via PATH override — the managed desktop launcher at
`~/.local/bin/auto` was not modified. The session executed `git gc --help` through the Bash tool; the
PreToolUse dispatcher fired and the model quoted the injected first line verbatim:

    ## Rule: worktree-safety

followed by `# Worktree Safety Rules`, matching the dispatcher output byte shape
(`hookSpecificOutput.additionalContext`, 2638 bytes for this payload). The model distinguished the
injection from CLAUDE.md baseline content (the baseline holds 11 rules and no worktree-safety), and the
tool call executed unblocked (fail-open contract held). Raw stream: session scratchpad
`live-session.jsonl`.

Environment follow-up (not SPEC completion debt): the user's managed desktop `auto` broker predates
`rules fire`; until the managed ADK slot is promoted to a build containing this SPEC, interactive
sessions resolve the old binary and the registered hook fails open with unknown-command (advisory,
non-blocking, no injection). Promotion goes through the desktop managed-ADK flow.
