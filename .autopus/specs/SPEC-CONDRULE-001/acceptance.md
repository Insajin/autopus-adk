# SPEC-CONDRULE-001 Acceptance Criteria

T7 assigns these condition regexes in `content/rules/`: `shell-portability` uses `(^|[;&|]\s)\s*g?timeout\s+[0-9]`, `lore-commit` uses `\bgit\s+commit\b`, and `worktree-safety` uses `\bgit\s+(gc|prune|repack)\b`. The conditional body root is `.claude/hooks/autopus/conditional/`.

## Test Scenarios

### S1: AC-CONDRULE-001 - Dispatcher fires lore-commit on a commit command
Priority: Must
Given the manifest contains `lore-commit` with condition `\bgit\s+commit\b` and matcher `Bash`
And the body file `lore-commit.md` sits in the conditional body root
When `auto rules fire --event PreToolUse` receives stdin `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit -m \"feat(x): y\""}}`
Then stdout parses as JSON with `hookSpecificOutput.hookEventName` equal to `PreToolUse`
And `additionalContext` contains the literal `🐙 Autopus <noreply@autopus.co>` from the `lore-commit` body, names the rule `lore-commit`, and the exit code is 0.

### S2: AC-CONDRULE-002 - Heterogeneous tool inputs produce the exact fired rule set
Priority: Must
Given the manifest holds `lore-commit`, `shell-portability`, and `worktree-safety` on matcher `Bash`
And the manifest holds a fixture rule `fixture-go-edit` with scope `tool:edit(*.go)` and condition `\.go$`
When the dispatcher is invoked once per row below
Then the set of rule names in `hookSpecificOutput.additionalContext` equals the expected set exactly

| tool_name | condition subject | expected fired rule names |
|-----------|-------------------|---------------------------|
| Bash | `git commit -m "fix: y"` | lore-commit |
| Bash | `timeout 540 auto spec review` | shell-portability |
| Bash | `gtimeout 30 go test ./...` | shell-portability |
| Bash | `git gc --prune=now` | worktree-safety |
| Bash | `git repack -ad && git commit -m x` | lore-commit, worktree-safety |
| Bash | `ls -la` | (empty) |
| Bash | `npm run gc` | (empty) |
| Bash | `echo "git commit is required"` | lore-commit |
| Edit | `/repo/pkg/rulecond/fire.go` | fixture-go-edit |
| Edit | `/repo/README.md` | (empty) |
| Read | `/repo/pkg/rulecond/fire.go` | (empty) |

And the `echo` row is an accepted false positive, because injection is advisory and costs context rather than blocking an action
And the `Read` row confirms a tool outside the Bash, Edit, Write, MultiEdit set yields no condition subject.

### S3: AC-CONDRULE-003 - Hook-fired rule bodies leave baseline context
Priority: Must
Given `content/rules/` holds 14 rules
And T7 classified `shell-portability`, `lore-commit`, and `worktree-safety` as hook-fired
When the claude-code adapter regenerates its surfaces
Then `.claude/rules/autopus/` contains exactly these 11 files: `branding.md`, `context7-docs.md`, `deferred-tools.md`, `doc-storage.md`, `file-size-limit.md`, `language-policy.md`, `objective-reasoning.md`, `project-identity.md`, `spec-quality.md`, `subagent-delegation.md`, `techstack-freshness.md`
And `.claude/rules/autopus/lore-commit.md`, `shell-portability.md`, and `worktree-safety.md` do not exist
And the conditional body root contains those three files, each byte-identical to the `content/rules/` body after frontmatter removal.

### S4: AC-CONDRULE-004 - Benign absence fails open
Priority: Must
Given the dispatcher is invoked once per condition below, each against a manifest holding only the rule under test
When each condition is present

| benign condition | expected exit code | expected stdout |
|------------------|--------------------|-----------------|
| manifest file absent | 0 | empty |
| manifest contains `{not json` | 0 | empty |
| manifest rule regex is `a(` | 0 | empty |
| stdin is empty | 0 | empty |
| stdin is `not json` | 0 | empty |
| named body file does not exist, single-rule manifest | 0 | empty |

Then stdout is empty and the exit code is 0 for every row
And no output contains `permissionDecision` or `continue`, and exit code 2 is never produced.

### S5: AC-CONDRULE-005 - Tool input never re-enters context
Priority: Must
Given the manifest holds `lore-commit` with condition `\bgit\s+commit\b`
When the dispatcher receives stdin whose `tool_input.command` is `export GH_TOKEN=ghp_EXAMPLEFAKETOKEN && git commit -m x`
Then `hookSpecificOutput.additionalContext` contains the `lore-commit` rule body
And `additionalContext` does not contain the substring `ghp_EXAMPLEFAKETOKEN`, the substring `export GH_TOKEN`, or the substring `git commit -m x`.

### S6: AC-CONDRULE-006 - Glob-shaped condition values are rejected
Priority: Must
Given a rule file declares `condition: "**/*.go"` in its frontmatter
When rule validation runs
Then validation returns an error naming the rule file and the field `condition`
And the error text states that a file glob belongs in `globs`, not `condition`
And the same rule with `condition: "\\.go$"` and `globs: ["**/*.go"]` validates without error.

### S7: AC-CONDRULE-007 - Trigger fields survive codex and opencode emission
Priority: Must
Given `content/rules/lore-commit.md` frontmatter declares `condition`, `scope`, `interruptMode`, and `astCondition`
When the codex and opencode adapters prepare their rule mappings
Then `.codex/rules/autopus/lore-commit.md` frontmatter contains keys `condition`, `scope`, `interruptMode`, `astCondition`, and `platform: codex`
And `.opencode/rules/autopus/lore-commit.md` frontmatter contains keys `condition`, `scope`, `interruptMode`, and `astCondition`, each byte-identical to the source value
And the gemini adapter output for the same rule is byte-identical to its pre-change output.

### S8: AC-CONDRULE-008 - Unconditional rules stay byte-identical
Priority: Must
Given a golden checksum is captured for each of the ten untouched rules on each platform before the change
When all four platform adapters regenerate after the change
Then the emitted content checksum for each of the ten rules equals its golden checksum on each platform, and no hook entry references any of those ten rules.

### S9: AC-CONDRULE-009 - Manifest is deterministic and the aggregate cap truncates predictably
Priority: Must
Given three hook-fired rules named `lore-commit`, `shell-portability`, and `worktree-safety`
When the manifest is generated twice from the same source
Then both manifest files are byte-identical and the `rules` array is ordered `lore-commit`, `shell-portability`, `worktree-safety`
And given instead a synthetic manifest whose three contained bodies are 3500, 3500, and 3000 bytes and all match one subject
When the dispatcher fires under the 8000-byte aggregate cap
Then `additionalContext` contains the first two rules by name order, omits the third, and is at most 8000 bytes
And the omitted rule is named in a one-line truncation notice that contains no tool input.

### S10: AC-CONDRULE-010 - Dispatcher entries are deduplicated per matcher
Priority: Must
Given three rules compile to hook event `PreToolUse` with matcher `Bash`
When `.claude/settings.json` is generated
Then `hooks.PreToolUse` contains exactly one entry whose `matcher` is `Bash`, whose command invokes `auto rules fire`, and which holds exactly one hook object
And pre-existing autopus hook entries for other matchers and user-defined unmanaged event keys are still present.

### S11: AC-CONDRULE-011 - Rule classification is inspectable
Priority: Should
Given all 14 rules are present with their T7 trigger assignments
When an operator runs `auto rules list`
Then the output has one row per rule, and `lore-commit` shows classification `hook-fired`, trigger `tool:bash`, and a destination under the conditional body root
And `file-size-limit` shows `paths-scoped` with destination `.claude/rules/autopus/file-size-limit.md`, and `branding` shows `always`
And the classification counts are 3 hook-fired, 1 paths-scoped, and 10 always.

### S12: AC-CONDRULE-012 - Firing is observed in a live Claude Code session
Priority: Must
Given `auto update` has regenerated the claude-code surfaces on this workspace
And a fresh Claude Code session has started with no `worktree-safety` text in its baseline context
When the session issues a Bash tool call whose command is `git gc --prune=now`
Then the `worktree-safety` rule text is present in the model context for that turn
And the tool call is not blocked, and the observed trace is recorded in the completion evidence.

### S13: AC-CONDRULE-013 - Path-scoped rules compile to native frontmatter
Priority: Must
Given `content/rules/file-size-limit.md` declares source-code `globs` and no `condition`
When the claude-code adapter regenerates
Then `.claude/rules/autopus/file-size-limit.md` frontmatter contains a `paths:` list derived from `globs`
And that file remains under `.claude/rules/autopus/`, with no manifest entry and no hook entry referencing `file-size-limit`
And the dynamic threshold rendering from the existing file-size-limit render path is still present in the body.

### S14: AC-CONDRULE-014 - Body reads are confined to the conditional body root
Priority: Must
Given a hostile manifest supplies one entry per row below, each with a condition matching the subject `git commit -m x`
And a control entry `lore-commit` names the legitimate contained body `lore-commit.md`
And a decoy file at the repository root named `secrets.md` contains the literal string `EXAMPLEFAKESECRET`
When the dispatcher fires with `tool_name` `Bash` and that subject

| manifest body value | expected reason code | injected? |
|---------------------|----------------------|-----------|
| `lore-commit.md` (control) | none | yes |
| `/etc/passwd` | absolute_path | no |
| `../../../secrets.md` | path_escape | no |
| `escape.md` symlinked to `../../../secrets.md` | symlink_escape | no |
| `subdir` (a directory) | not_regular_file | no |
| `notes.txt` inside the root | bad_extension | no |
| `huge.md` inside the root at 4001 bytes | body_too_large | no |
| `../conditional/lore-commit.md` | path_escape | no |

Then `additionalContext` contains the `lore-commit` body and nothing else
And `additionalContext` does not contain `EXAMPLEFAKESECRET`, any byte of `/etc/passwd`, or any byte of the 4001-byte body
And stderr contains each expected reason code exactly once
And stderr does not contain `/etc/passwd`, `secrets.md`, or `EXAMPLEFAKESECRET`
And the exit code is 0, and compile-time generation of the same entries fails with an error naming each offending rule.

### S15: AC-CONDRULE-015 - Fail-open and fail-closed stay distinct
Priority: Must
Given a manifest holds contained rule `lore-commit`, contained rule `worktree-safety`, and hostile entry `evil` whose body value is `/etc/passwd`
And all three conditions match the subject `git gc --prune=now && git commit -m x`
When the dispatcher fires once per row below

| error class | trigger | rules injected | stdout | stderr |
|-------------|---------|----------------|--------|--------|
| fail-open | manifest absent | none | empty | empty |
| fail-open | stdin unparseable | none | empty | empty |
| fail-open | no condition match | none | empty | empty |
| fail-open | `worktree-safety` body file deleted | lore-commit | non-empty | empty |
| fail-closed | `evil` body is `/etc/passwd` | lore-commit, worktree-safety | non-empty | `evil absolute_path` |

Then each row produces exactly the listed injected rule set
And a fail-closed violation suppresses only the offending rule while every other matched contained rule still injects
And a fail-open condition never writes a stderr diagnostic
And every row exits 0 and no row produces exit code 2, `permissionDecision`, or `continue`.

### S16: AC-CONDRULE-016 - Relocation conserves the counted rule total
Priority: Must
Given the pre-change parity report records claude 14, codex 14, gemini 14 rules at 100% Codex rules parity
And relocation moves 3 rule bodies out of `.claude/rules/autopus/`
When `classifyFile` gains a case returning `rules` for a target path containing `hooks/autopus/conditional/`
And `go test ./pkg/adapter -run TestParity` runs after the change
Then the parity report records claude 14, codex 14, and gemini 14 rules, Codex rules parity is 100.0%, and the 95% assertion passes
And `classifyFile` returns `rules` for `.claude/hooks/autopus/conditional/lore-commit.md`
And `classifyFile` returns the empty string for `.claude/hooks/autopus/conditional-rules.json`
And without the `classifyFile` change the same run reports claude 11 and Codex rules parity 78.6%.

## Oracle Acceptance Notes

Every Must scenario is oracle-first: each names concrete inputs and exact expected output rather than asserting that a section, a file, or a zero exit code exists.

- S2 is the pattern-matching oracle (INV-001, INV-007): eleven heterogeneous rows fixing the expected fired rule name set, including two non-matching commands, an accepted false positive, and a tool outside the condition-subject set.
- S3 fixes the exact 11-file baseline set and the 3 relocated bodies by enumeration (INV-003); S4 enumerates six benign-absence conditions with expected exit code and stdout per row; S5 asserts named absent substrings including a fake credential; S9 fixes manifest byte-identity across two generations and the exact rules kept and dropped under the aggregate cap (INV-004).
- S14 is the read-side containment oracle (INV-009): eight heterogeneous manifest entries spanning absolute path, `..` traversal, symlink escape, directory, wrong extension, oversize body, and a normalized re-entry attempt, each with a concrete reason code and injected-or-not verdict, plus named absent substrings proving decoy content reaches neither context nor stderr.
- S15 is the fail-open versus fail-closed partition oracle (INV-010): five rows fixing the exact injected rule set per error class.
- S16 is the parity oracle (INV-011): concrete per-platform counts before and after, including the 78.6% figure the change must avoid. S12 is the only scenario requiring live observation, because every other scenario can pass against a correctly compiled but unwired build.

Tolerance: byte caps are exact upper bounds, not approximations. Regex behavior is the RE2 semantics of the listed patterns. Path containment is evaluated after symlink resolution, so a symlink whose target stays inside the root remains admissible.
