# SPEC-STICKYRULE-001: Sticky Rule Re-Attach For Long Sessions

**Status**: completed
**Created**: 2026-07-30
**Domain**: STICKYRULE
**Priority**: MEDIUM
**Source**: direct `@auto plan` request (track 1, user-confirmed sibling)
**Depends On**: `SPEC-CONDRULE-001`
**Related**: `SPEC-CONDRULE-001` (primary)
**Target module**: `autopus-adk/`
**Module ownership**: `autopus-adk/.autopus/specs/SPEC-STICKYRULE-001/**`

## Purpose

Rules loaded at session start sit at the far end of a long conversation's context. In a session that runs for dozens of turns, the instructions that govern every single response, namely the language policy and the objective-reasoning discipline, are the ones most likely to be diluted, because they were stated once and never restated.

This SPEC re-attaches a small set of designated rules periodically during a session, so a rule that governs every response is present near the end of the conversation as well as the beginning.

## Background

The omp rule contract expresses this with `alwaysApply: true`. `SPEC-CONDRULE-001` introduces the frontmatter parser and the field but deliberately does not act on it.

Claude Code has no sticky-rule primitive. The deterministic equivalent is the `UserPromptSubmit` hook: it fires before each user turn and can return structured output to prepend text to that turn. Verified against official Claude Code hooks documentation on 2026-07-30, that structured output is an object whose `hookSpecificOutput` carries both `hookEventName` set to `UserPromptSubmit` and `additionalContext`; a bare `additionalContext` without the event name is not recognized as structured hook output.

Three concerns differ from the primary SPEC. First, state: sticky re-attach must know how many prompts have passed, which means per-session counter state derived from an untrusted `session_id`. Second, blast radius: exit code 2 on `UserPromptSubmit` erases the user's prompt, and an unrecovered Go panic terminates a process with exactly that status, so the safety property needs an enforced barrier rather than an assertion. Third, platform reach: `pkg/content/hooks.go::generateCLIHooks` is shared by every adapter whose `SupportsHooks()` returns true, which is claude, codex, gemini, and opencode, so a Claude-only event must be guarded the way `generateCC21Hooks` already guards its own.

This runtime reads rule bodies from disk and injects them into model context, the same boundary `SPEC-CONDRULE-001` constrains for its own body root. The same containment contract applies here.

## Outcome Boundary

- Outcome Lock: in a long Claude Code session, the designated sticky rules are re-stated on a fixed prompt cadence instead of appearing only at session start; the runtime never reads a file outside its designated body root, never terminates with an exit status that erases the user's prompt, and leaves non-Claude platforms untouched.
- Mandatory requirements: sticky classification from `alwaysApply`, rejection of the unrepresentable sticky combination, claude-confined manifest and `UserPromptSubmit` hook with a fixed timeout and a removal path, the `auto rules sticky` runtime with cadence and per-session counter, structured hook output, project-root resolution, safe state-filename derivation, read-side containment with a per-body cap, a total error partition, an enforced process-exit barrier, bounded state, two-rule mapping, and live evidence.
- Explicit non-goals: conditional rule firing and the frontmatter schema itself (owned by `SPEC-CONDRULE-001`), `.agents/rules/*.md` emission (SPEC-OMP-001), adaptive or model-driven cadence, context-window measurement, compaction hooks, and re-attaching rules inside subagent sessions.
- Completion evidence: focused Go unit tests for cadence, session isolation, state-path derivation, containment, the error partition, the exit barrier, and hook lifecycle; regenerated-surface assertions that the `SPEC-CONDRULE-001` baseline set is unchanged and that non-Claude settings gain no entry; a parity oracle; and one observed re-injection in a live Claude Code session.

## Definitions

- Sticky rule: a rule whose frontmatter declares `alwaysApply: true`.
- Cadence `N`: the prompt interval between re-injections, defaulting to 8.
- Prompt index: the count of `UserPromptSubmit` events observed for one `session_id`, starting at 1.
- State key: the lowercase hex SHA-256 digest of the raw `session_id`, used as the state filename.
- Sticky body root: the directory `.claude/rules/autopus/`, which holds the body of every rule whose `SPEC-CONDRULE-001` classification is `always` or `paths-scoped`.
- Injectable body: a rule body with its YAML frontmatter removed, which is the only form this SPEC injects and the only form its byte accounting counts.
- Project root: the nearest ancestor of the hook process working directory that contains a `.claude` directory.
- Structured hook output: a JSON object whose `hookSpecificOutput` carries `hookEventName` equal to `UserPromptSubmit` together with `additionalContext`.
- Violation reason code: one of `absolute_path`, `path_escape`, `symlink_escape`, `not_regular_file`, `bad_extension`, `body_too_large`.

## Requirements

### Scope And Source Ownership

**REQ-STICKYRULE-SCOPE-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL make all changes inside `autopus-adk/` source-of-truth paths, and SHALL NOT directly edit the generated `.claude/`, `.codex/`, `.gemini/`, or `.opencode/` surfaces.

### Sticky Classification

**REQ-STICKYRULE-SCHEMA-01**
Priority: Must
Type: Event-driven
WHEN a rule declares `alwaysApply: true` and its `SPEC-CONDRULE-001` classification is `always` or `paths-scoped`, THE SYSTEM SHALL mark it sticky as an orthogonal flag, SHALL NOT introduce a fourth classification value, and SHALL leave the rule's baseline load placement unchanged.

**REQ-STICKYRULE-SCHEMA-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL preserve `alwaysApply` verbatim in the frontmatter emitted to every platform that already preserves rule frontmatter, which is codex, opencode, and gemini.

**REQ-STICKYRULE-SCHEMA-03**
Priority: Must
Type: Unwanted
IF a rule declares `alwaysApply: true` while its classification is `hook-fired`, THEN THE SYSTEM SHALL reject the rule during validation with an error naming the rule and both attributes, because `SPEC-CONDRULE-001` relocates a hook-fired body out of the sticky body root and this SPEC resolves sticky bodies from that single root only.

### Compilation

**REQ-STICKYRULE-COMPILE-01**
Priority: Must
Type: Event-driven
WHEN at least one rule is sticky and the target platform is claude-code, THE SYSTEM SHALL record the sticky rule set in the compiled manifest and SHALL register exactly one `UserPromptSubmit` hook entry with an explicit timeout of 5 seconds.

**REQ-STICKYRULE-COMPILE-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL treat `UserPromptSubmit` as an autopus-managed event whenever it manages hooks for claude-code, so that a settings file carrying a previously installed sticky entry has that entry removed once no rule is sticky, and SHALL leave user-defined event keys it does not manage untouched.

**REQ-STICKYRULE-COMPILE-03**
Priority: Must
Type: Unwanted
IF the target platform is not claude-code, THEN THE SYSTEM SHALL emit no `UserPromptSubmit` hook entry and no sticky manifest, because the event is a Claude Code contract and `generateCLIHooks` is shared by every adapter whose `SupportsHooks()` returns true.

**REQ-STICKYRULE-COMPILE-04**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL record sticky body locations as paths relative to the sticky body root, SHALL verify at compile time that each is a regular `.md` file inside that root and at or under the per-body cap, SHALL verify that the injectable bodies of the whole sticky set fit the aggregate cap, and SHALL fail generation with an error naming the rule rather than emitting an entry the runtime would refuse or silently drop.

### Runtime Re-Attach

**REQ-STICKYRULE-FIRE-01**
Priority: Must
Type: Event-driven
WHEN the sticky runtime observes prompt index 1, or an index where the index minus 1 is an exact multiple of the cadence `N`, THE SYSTEM SHALL write structured hook output carrying `hookEventName` equal to `UserPromptSubmit` and an `additionalContext` holding the sticky rule names and injectable bodies, and SHALL write nothing at every other index.

**REQ-STICKYRULE-FIRE-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL keep one independent prompt counter per `session_id`, and SHALL derive the state filename as the lowercase hex SHA-256 digest of the raw `session_id`, so no untrusted input reaches a filesystem path component.

**REQ-STICKYRULE-FIRE-03**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL exit zero on every path, and SHALL NOT emit exit code 2 or a `decision` value of `block`, because either erases the user's prompt.

**REQ-STICKYRULE-FIRE-04**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL emit only sticky rule names and injectable bodies, and SHALL NOT write, log, or echo the submitted prompt text, the raw `session_id`, the transcript path, or the working directory.

**REQ-STICKYRULE-FIRE-05**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL cap injected context at 6000 bytes measured over the injectable bodies plus any truncation notice and excluding JSON framing, and SHALL drop whole rules in reverse rule-name order once the cap is reached, which admits the shipped pair whose injectable bodies measure 4628 bytes or less.

**REQ-STICKYRULE-FIRE-06**
Priority: Must
Type: Unwanted
IF a sticky body location is absolute, contains a `..` component, resolves after symlink resolution to a location outside the sticky body root, names anything other than a regular file, or does not end in `.md`, THEN THE SYSTEM SHALL skip that rule, SHALL NOT open or inject its contents, and SHALL record the matching violation reason code.

**REQ-STICKYRULE-FIRE-07**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL cap a single injectable body at 4000 bytes, SHALL skip a body exceeding that cap under reason code `body_too_large` without injecting any part of it, and SHALL apply this per-body cap before the aggregate cap of REQ-STICKYRULE-FIRE-05.

**REQ-STICKYRULE-FIRE-08**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL assign every runtime condition to exactly one of four cases and SHALL NOT leave a condition unassigned or multiply assigned. An unresolvable project root writes no stdout and one stderr line. Whole-run benign absence, meaning a missing or malformed manifest, an unparseable stdin payload, an unusable state directory, or an off-cadence index, writes no stdout and no stderr. Per-rule benign absence, meaning a body file that does not exist, skips only that rule with no stderr while every other sticky rule still injects. Per-rule fail-closed, meaning a containment or size violation under REQ-STICKYRULE-FIRE-06 or REQ-STICKYRULE-FIRE-07, suppresses only the offending rule and writes its reason code to stderr while every other contained sticky rule still injects.

**REQ-STICKYRULE-FIRE-09**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL report a fail-closed suppression on stderr as the rule name and the violation reason code only, SHALL NOT write the resolved target path or any byte of the target file to stderr or stdout, and SHALL resolve the manifest at a fixed location relative to the project root rather than from an environment variable, a command flag, or the hook payload.

**REQ-STICKYRULE-FIRE-10**
Priority: Must
Type: Event-driven
WHEN the aggregate cap drops one or more rules, THE SYSTEM SHALL append a single-line truncation notice naming the dropped rules to `additionalContext`, SHALL count that notice against the aggregate cap, and SHALL include no prompt text in it.

**REQ-STICKYRULE-FIRE-11**
Priority: Must
Type: Unwanted
IF any unrecovered runtime fault occurs, including a panic, THEN a top-level deferred recover SHALL suppress it, discard partial stdout, and terminate the process with exit code zero, because an unrecovered Go panic otherwise terminates the process with exit status 2 and erases the user's prompt.

**REQ-STICKYRULE-FIRE-12**
Priority: Must
Type: Event-driven
WHEN the runtime starts, THE SYSTEM SHALL resolve the project root by walking up from the process working directory to the nearest ancestor containing a `.claude` directory, and WHEN no such ancestor exists THE SYSTEM SHALL write one stderr line reading `sticky project_root_unresolved` and exit zero, so a misconfigured working directory is observable rather than silently inert.

### State Bounds

**REQ-STICKYRULE-STATE-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL store counter state under the gitignored `.autopus/runtime/` tree, SHALL delete state entries whose modification time is older than 7 days, and SHALL cap the state directory at 200 entries by removing the oldest first.

### Rule Mapping And Configuration

**REQ-STICKYRULE-MAP-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL mark `language-policy` and `objective-reasoning` sticky, SHALL leave every other rule non-sticky, and SHALL resolve the cadence `N` from `autopus.yaml` as the configured integer when it is positive and as 8 when the key is absent, zero, or negative. A non-integer scalar is rejected by the existing typed `yaml.Unmarshal` in `pkg/config/loader.go` before the cadence is read, so it is a config load error rather than a fallback case.

### Observability And Verification

**REQ-STICKYRULE-OBS-01**
Priority: Should
Type: Event-driven
WHEN an operator runs `auto rules list`, THE SYSTEM SHALL show the sticky flag and the effective cadence alongside the existing classification columns.

**REQ-STICKYRULE-VERIFY-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL keep the `SPEC-CONDRULE-001` baseline rule set unchanged and SHALL record one observed live re-injection as completion evidence.

**REQ-STICKYRULE-VERIFY-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL keep `pkg/adapter/parity_test.go` reporting Codex rules parity at or above the 95% gate after the sticky frontmatter change, with the per-platform rule counts unchanged from the `SPEC-CONDRULE-001` post-relocation baseline.

## Generated And Changed Surfaces

| Surface | Role | Change |
|---------|------|--------|
| `content/rules/language-policy.md`, `content/rules/objective-reasoning.md` | rule source of truth | `alwaysApply: true` added |
| `templates/gemini/rules/autopus/` | gemini rule templates | regenerated so the new key propagates |
| `[NEW] pkg/rulecond/sticky.go` | cadence, state, injection payload | new file in the package created by `SPEC-CONDRULE-001` |
| `pkg/content/hooks.go` | hook config assembly | claude-guarded `UserPromptSubmit` entry when sticky rules exist |
| `pkg/adapter/claude/claude_settings.go` | settings writer | `UserPromptSubmit` added to the managed-event set |
| `pkg/config/schema.go` | harness config | cadence field on the hooks configuration |
| `[NEW] internal/cli/rules_sticky.go` | `auto rules sticky` subcommand | new file under the `auto rules` namespace |
| `[NEW] .autopus/runtime/sticky-rules/<sha256>` | per-session counter | generated, gitignored |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| sibling of the primary | Independent user-visible outcome, user-confirmed. Different hook event, different trigger model, and per-session state that conditional firing does not need. | `SPEC-CONDRULE-001` (primary) |

This SPEC creates no sibling of its own. Recursive siblings are prohibited.

## Related SPECs

- `SPEC-CONDRULE-001` — primary and approved; supplies the frontmatter parser, the `alwaysApply` field, the manifest format, the containment helper, and the `auto rules` CLI namespace. This SPEC consumes them and reopens none of its requirements. Where the shared containment helper needs a root parameter, this SPEC owns that non-breaking change and preserves the primary's call-site behavior.
- `SPEC-OMP-001` — separate track; owns `.agents/rules/*.md` emission and is a non-goal here.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-STICKYRULE-SCOPE-01 | T1, T5 | S7 | INV-104 |
| REQ-STICKYRULE-SCHEMA-01 | T1 | S7 | INV-104 |
| REQ-STICKYRULE-SCHEMA-02 | T6 | S8 | INV-104 |
| REQ-STICKYRULE-SCHEMA-03 | T1 | S7 | INV-104 |
| REQ-STICKYRULE-COMPILE-01 | T2 | S16 | INV-110 |
| REQ-STICKYRULE-COMPILE-02 | T2 | S16 | INV-110 |
| REQ-STICKYRULE-COMPILE-03 | T2 | S16 | INV-111 |
| REQ-STICKYRULE-COMPILE-04 | T9 | S12 | INV-108 |
| REQ-STICKYRULE-FIRE-01 | T3 | S1 | INV-101 |
| REQ-STICKYRULE-FIRE-02 | T3, T4 | S2, S3 | INV-102, INV-105 |
| REQ-STICKYRULE-FIRE-03 | T3 | S4, S13 | INV-112 |
| REQ-STICKYRULE-FIRE-04 | T3 | S5 | INV-103 |
| REQ-STICKYRULE-FIRE-05 | T3 | S6 | INV-106 |
| REQ-STICKYRULE-FIRE-06 | T9 | S12 | INV-108 |
| REQ-STICKYRULE-FIRE-07 | T9 | S12 | INV-108 |
| REQ-STICKYRULE-FIRE-08 | T3, T9 | S13 | INV-109 |
| REQ-STICKYRULE-FIRE-09 | T9 | S12, S13 | INV-109 |
| REQ-STICKYRULE-FIRE-10 | T3 | S6 | INV-106 |
| REQ-STICKYRULE-FIRE-11 | T3 | S13 | INV-112 |
| REQ-STICKYRULE-FIRE-12 | T3 | S13 | INV-109 |
| REQ-STICKYRULE-STATE-01 | T4 | S9 | INV-107 |
| REQ-STICKYRULE-MAP-01 | T5 | S1, S14 | INV-101 |
| REQ-STICKYRULE-OBS-01 | T7 | S11 | INV-104 |
| REQ-STICKYRULE-VERIFY-01 | T8 | S7, S10 | INV-101 |
| REQ-STICKYRULE-VERIFY-02 | T6 | S15 | INV-111 |
