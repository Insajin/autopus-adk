# SPEC-CONDRULE-001: Conditional Rules — Trigger Metadata And Per-Platform Compilation

**Status**: completed
**Created**: 2026-07-30
**Domain**: CONDRULE
**Priority**: HIGH
**Source**: direct `@auto plan` request (track 1)
**Related**: `SPEC-STICKYRULE-001` (sibling)
**Target module**: `autopus-adk/`
**Module ownership**: `autopus-adk/.autopus/specs/SPEC-CONDRULE-001/**`

## Purpose

Every file under `.claude/rules/autopus/` is loaded into context at the start of every Claude Code session. All 14 ADK rules therefore cost context tokens in every session, including rules that only matter at one narrow moment. `lore-commit` matters when a commit is being written, `worktree-safety` matters when `git gc` is about to run, and `shell-portability` matters when a `timeout` prefix appears in a command.

This SPEC adds trigger metadata to the rule frontmatter schema and compiles that metadata per platform, so a triggered rule costs zero baseline context and is injected at the exact moment it applies.

## Background

Two independent firing mechanisms exist, and this SPEC uses both rather than inventing one.

1. Claude Code natively supports path-scoped rules. A `.claude/rules/*.md` file carrying a `paths:` frontmatter list loads only when Claude reads a matching file, while rules without `paths` load unconditionally at launch. This is the native equivalent of the omp `globs` field and needs no ADK runtime machinery.
2. Claude Code has no native equivalent of the omp TTSR interrupt, which matches a regex against what the agent is about to do. The deterministic equivalent is a `PreToolUse` hook. It receives `tool_name` and `tool_input` on stdin and can return `hookSpecificOutput.additionalContext` to inject text before the tool runs.

The ADK already owns the surfaces both mechanisms need. `content/rules/*.md` is the rule source of truth, `pkg/content/hooks.go` builds `[]adapter.HookConfig`, and `pkg/adapter/claude/claude_settings.go` writes those configs into the nested `.claude/settings.json` hook schema.

Frontmatter field names follow the omp rule contract (`condition`, `scope`, `globs`, `alwaysApply`, `interruptMode`, `astCondition`) so that when SPEC-OMP-001 emits `.agents/rules/*.md` on a separate track, the same metadata passes through unchanged.

The dispatcher reads rule bodies from a compiled on-disk manifest and injects their contents into model context. That read path is a trust boundary: a cloned repository can ship its own manifest, so the manifest and the body files it names are untrusted repo-supplied input, not trusted local configuration. Requirements REQ-CONDRULE-FIRE-07 through REQ-CONDRULE-FIRE-10 constrain that boundary.

## Outcome Boundary

- Outcome Lock: a Claude Code user on an unmodified ADK install pays zero baseline context for triggered rules, and sees `lore-commit`, `shell-portability`, and `worktree-safety` text injected at the tool call that matches their trigger, without the dispatcher ever becoming a path for reading files outside its designated body directory. Untriggered rules behave exactly as before.
- Mandatory requirements: frontmatter trigger schema, rule classification, native `paths:` compilation, hook-fired relocation plus dispatcher, the `auto rules fire` runtime, read-side body containment with a per-body size cap and an explicit fail-closed class, trigger mapping for the four natural-fit rules, cross-platform schema preservation, parity-count reconciliation, and live firing evidence.
- Explicit non-goals: emitting `.agents/rules/*.md` for omp (owned by SPEC-OMP-001), sticky re-attach (owned by sibling `SPEC-STICKYRULE-001`), advisor or model-routing absorption, implementing TTSR-style mid-token interruption inside Claude Code, conditionalizing the remaining ten rules, and evaluating `astCondition` inside the ADK.
- Completion evidence: focused Go unit tests for the classifier, compiler, dispatcher, and containment checks; regenerated-surface assertions on `.claude/rules/autopus/`, `.claude/settings.json`, and the conditional manifest; `pkg/adapter/parity_test.go` green at the 95% Codex rules gate with the reconciled counting rule; and one observed Claude Code firing trace.

## Definitions

- Trigger field: any of `condition`, `scope`, `globs`, `alwaysApply`, `interruptMode`, `astCondition` in rule frontmatter.
- Classification: exactly one of `always` (no trigger field), `paths-scoped` (`globs` present and `condition` absent), or `hook-fired` (`condition` present with a tool `scope`).
- Condition subject: the single string a rule's `condition` regexes are matched against for a given tool call.
- Dispatcher: the `auto rules fire` command registered as one `PreToolUse` hook entry per distinct matcher.
- Baseline context: the set of files Claude Code loads at session start with no tool call, which for rules is every `.claude/rules/autopus/*.md` lacking a `paths:` field.
- Conditional body root: the single directory `.claude/hooks/autopus/conditional/` that holds every relocated hook-fired rule body.
- Violation reason code: one of `absolute_path`, `path_escape`, `symlink_escape`, `not_regular_file`, `bad_extension`, `body_too_large`.

## Requirements

### Scope And Source Ownership

**REQ-CONDRULE-SCOPE-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL make all rule and compiler changes inside `autopus-adk/` source-of-truth paths, and SHALL NOT directly edit the generated `.claude/`, `.codex/`, `.gemini/`, or `.opencode/` surfaces.

### Frontmatter Trigger Schema

**REQ-CONDRULE-SCHEMA-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL accept `condition`, `scope`, `globs`, `alwaysApply`, `interruptMode`, and `astCondition` in rule frontmatter alongside the existing `name`, `description`, and `category` fields.

**REQ-CONDRULE-SCHEMA-02**
Priority: Must
Type: Event-driven
WHEN a rule carries no trigger field, THE SYSTEM SHALL classify it as `always` and SHALL emit byte-identical content to the pre-change generated surface for every platform.

**REQ-CONDRULE-SCHEMA-03**
Priority: Must
Type: Unwanted
IF a `condition` value parses as a file glob rather than a regex, THEN THE SYSTEM SHALL reject the rule during validation and SHALL name the offending rule and field, because the omp parser coerces glob-shaped `condition` values into `tool:edit(...)` and `tool:write(...)` scopes and silently changes the rule meaning across tracks.

**REQ-CONDRULE-SCHEMA-04**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL preserve `interruptMode` and `astCondition` verbatim in emitted rule frontmatter without interpreting them, so downstream omp consumption stays lossless.

### Per-Platform Compilation

**REQ-CONDRULE-COMPILE-01**
Priority: Must
Type: Event-driven
WHEN compiling a `paths-scoped` rule for claude-code, THE SYSTEM SHALL translate `globs` into a native `paths:` frontmatter list on `.claude/rules/autopus/<name>.md` and SHALL NOT register any hook entry for that rule.

**REQ-CONDRULE-COMPILE-02**
Priority: Must
Type: Event-driven
WHEN compiling a `hook-fired` rule for claude-code, THE SYSTEM SHALL write the rule body into the conditional body root, SHALL record the rule in the conditional manifest, and SHALL register the dispatcher hook, so the rule body is absent from baseline context.

**REQ-CONDRULE-COMPILE-03**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL register exactly one dispatcher hook entry per distinct hook event and matcher pair, regardless of how many rules compile to that pair.

**REQ-CONDRULE-COMPILE-04**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL write the conditional manifest with rules ordered by rule name and with stable key ordering, so regeneration produces no spurious diff.

**REQ-CONDRULE-COMPILE-05**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL preserve every trigger field in the frontmatter emitted to `.codex/rules/autopus/` and `.opencode/rules/autopus/`, and SHALL leave the existing gemini rule frontmatter handling unchanged.

**REQ-CONDRULE-COMPILE-06**
Priority: Must
Type: Event-driven
WHEN writing a manifest entry, THE SYSTEM SHALL verify at compile time that the body is a regular `.md` file inside the conditional body root and is at or under the per-body size cap, and SHALL fail generation with an error naming the rule rather than emitting an entry the dispatcher would later refuse, so a rule never stops firing silently.

**REQ-CONDRULE-COMPILE-07**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL record manifest body locations as paths relative to the conditional body root, and SHALL NOT store absolute paths in the manifest.

### Runtime Firing

**REQ-CONDRULE-FIRE-01**
Priority: Must
Type: Event-driven
WHEN the dispatcher receives a hook stdin payload whose condition subject matches at least one manifest rule regex, THE SYSTEM SHALL emit a `hookSpecificOutput.additionalContext` payload containing the matched rule names and rule bodies.

**REQ-CONDRULE-FIRE-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL derive the condition subject as `tool_input.command` for the `Bash` tool and as `tool_input.file_path` for the `Edit`, `Write`, and `MultiEdit` tools, and SHALL treat every other tool as a non-match.

**REQ-CONDRULE-FIRE-03**
Priority: Must
Type: Unwanted
IF the manifest is missing, unreadable, or malformed, or a stored regex fails to compile, or the stdin payload is not valid JSON, THEN THE SYSTEM SHALL exit zero with empty output and SHALL NOT block, deny, or delay the tool call.

**REQ-CONDRULE-FIRE-04**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL emit only rule names and rule bodies, and SHALL NOT include the matched command text, file contents, regex capture groups, or any other tool input in its output, so credentials present on a command line are never re-injected into context.

**REQ-CONDRULE-FIRE-05**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL cap total injected context at 8000 bytes, SHALL drop whole rules in reverse rule-name order once the cap is reached, and SHALL evaluate conditions with the Go `regexp` RE2 engine so match time stays linear in subject length.

**REQ-CONDRULE-FIRE-06**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL keep the dispatcher advisory, and SHALL NOT emit a `permissionDecision` value of `deny` or `ask`, a `continue: false` field, or exit code 2.

**REQ-CONDRULE-FIRE-07**
Priority: Must
Type: Unwanted
IF a manifest body location is absolute, contains a `..` component, resolves after symlink resolution to a location outside the conditional body root, names anything other than a regular file, or does not end in `.md`, THEN THE SYSTEM SHALL skip that rule, SHALL NOT open or inject its contents, and SHALL record the matching violation reason code.

**REQ-CONDRULE-FIRE-08**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL cap a single rule body at 4000 bytes, SHALL skip a body exceeding that cap under reason code `body_too_large` without injecting any part of it, and SHALL apply this per-body cap before the 8000-byte aggregate cap of REQ-CONDRULE-FIRE-05.

**REQ-CONDRULE-FIRE-09**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL sort every dispatcher error into exactly one of two classes and SHALL NOT collapse them. Benign absence is fail-open: a missing or malformed manifest, an unparseable stdin payload, or no condition match produces empty output for the whole run, while a body file that does not exist skips only that rule, writes no stderr diagnostic, and leaves every other matched rule injected. A containment or size violation under REQ-CONDRULE-FIRE-07 or REQ-CONDRULE-FIRE-08 is fail-closed: it suppresses only the offending rule, records the violation reason code, and leaves every other matched and contained rule injected. Both classes exit zero.

**REQ-CONDRULE-FIRE-10**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL report a fail-closed suppression on stderr as the rule name and the violation reason code only, SHALL NOT write the resolved target path or any byte of the target file to stderr or stdout, and SHALL NOT place the diagnostic in model context.

**REQ-CONDRULE-FIRE-11**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL resolve the manifest at a fixed location relative to the project root, and SHALL NOT accept a manifest location from an environment variable, a command flag, or the hook stdin payload.

### Rule Mapping

**REQ-CONDRULE-MAP-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL classify `shell-portability`, `lore-commit`, and `worktree-safety` as `hook-fired` on the `Bash` tool, SHALL classify `file-size-limit` as `paths-scoped` over source-code globs, and SHALL leave the remaining ten rules classified as `always`.

**REQ-CONDRULE-MAP-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL add complete frontmatter to `content/rules/worktree-safety.md`, which currently carries none, before assigning it a trigger.

### Observability And Verification

**REQ-CONDRULE-OBS-01**
Priority: Should
Type: Event-driven
WHEN an operator runs `auto rules list`, THE SYSTEM SHALL print every rule with its classification, its trigger summary, and its compiled claude-code destination.

**REQ-CONDRULE-VERIFY-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL keep `pkg/adapter/parity_test.go` at or above its existing 95% Codex rules parity gate, SHALL keep the `.claude/rules/` drift and hygiene mappings consistent with the new generated paths, and SHALL record one observed Claude Code firing trace as completion evidence.

**REQ-CONDRULE-VERIFY-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL extend `classifyFile` in `pkg/adapter/parity_test.go` to count a FileMapping whose target path lies under the conditional body root as a rule, so relocation conserves the counted claude-code rule total at 14, and SHALL leave the compiled manifest file itself uncounted.

## Generated And Changed Surfaces

| Surface | Role | Change |
|---------|------|--------|
| `content/rules/*.md` | rule source of truth | trigger frontmatter added to 4 rules |
| `[NEW] pkg/rulecond/` | schema, classifier, compiler, containment, matcher | new package |
| `pkg/content/hooks.go` | hook config assembly | dispatcher entries appended |
| `pkg/adapter/claude/claude_prepare_files.go` | claude file mapping | classification-aware rule routing |
| `pkg/adapter/parity_test.go` | parity gate | `classifyFile` counts conditional bodies as rules |
| `[NEW] internal/cli/rules.go` | `auto rules` namespace | `fire` and `list` subcommands |
| `[NEW] .claude/hooks/autopus/conditional-rules.json` | compiled manifest | generated artifact |
| `[NEW] .claude/hooks/autopus/conditional/<name>.md` | relocated rule bodies | generated artifact |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| one sibling | Sticky re-attach is an independent user-visible outcome with its own hook event, its own state model, and separately testable acceptance. The user confirmed the split explicitly. | `SPEC-STICKYRULE-001` |

Recursive siblings are prohibited. `SPEC-STICKYRULE-001` depends on the `alwaysApply` field and the frontmatter parser delivered here, and owns nothing inside this SPEC Outcome Lock.

## Related SPECs

- `SPEC-STICKYRULE-001` — sibling; consumes the `alwaysApply` field, the `pkg/rulecond` parser, and the same read-side containment contract applied to its own body root.
- `SPEC-OMP-001` — separate track; owns `.agents/rules/*.md` emission and is a non-goal here.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-CONDRULE-SCOPE-01 | T1, T7 | S8 | INV-005 |
| REQ-CONDRULE-SCHEMA-01 | T1 | S7 | INV-005 |
| REQ-CONDRULE-SCHEMA-02 | T2 | S8 | INV-002 |
| REQ-CONDRULE-SCHEMA-03 | T2 | S6 | INV-008 |
| REQ-CONDRULE-SCHEMA-04 | T1, T8 | S7 | INV-005 |
| REQ-CONDRULE-COMPILE-01 | T3 | S13 | INV-002 |
| REQ-CONDRULE-COMPILE-02 | T4 | S3 | INV-003 |
| REQ-CONDRULE-COMPILE-03 | T4 | S10 | INV-002 |
| REQ-CONDRULE-COMPILE-04 | T4 | S9 | INV-004 |
| REQ-CONDRULE-COMPILE-05 | T8 | S7 | INV-005 |
| REQ-CONDRULE-COMPILE-06 | T11 | S14 | INV-009 |
| REQ-CONDRULE-COMPILE-07 | T11 | S14 | INV-009 |
| REQ-CONDRULE-FIRE-01 | T5 | S1, S2 | INV-001 |
| REQ-CONDRULE-FIRE-02 | T5 | S2 | INV-007 |
| REQ-CONDRULE-FIRE-03 | T5 | S4 | INV-006 |
| REQ-CONDRULE-FIRE-04 | T5 | S5 | INV-006 |
| REQ-CONDRULE-FIRE-05 | T5 | S9 | INV-004 |
| REQ-CONDRULE-FIRE-06 | T5 | S4 | INV-006 |
| REQ-CONDRULE-FIRE-07 | T11 | S14 | INV-009 |
| REQ-CONDRULE-FIRE-08 | T11 | S14 | INV-009 |
| REQ-CONDRULE-FIRE-09 | T11 | S15 | INV-010 |
| REQ-CONDRULE-FIRE-10 | T11 | S15 | INV-010 |
| REQ-CONDRULE-FIRE-11 | T11 | S14 | INV-009 |
| REQ-CONDRULE-MAP-01 | T7 | S2, S3 | INV-002 |
| REQ-CONDRULE-MAP-02 | T7 | S3 | INV-002 |
| REQ-CONDRULE-OBS-01 | T6 | S11 | INV-002 |
| REQ-CONDRULE-VERIFY-01 | T9, T10 | S12 | INV-001 |
| REQ-CONDRULE-VERIFY-02 | T9 | S16 | INV-011 |
