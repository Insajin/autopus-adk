# SPEC-STICKYRULE-001 Acceptance Criteria

T5 marks two rules sticky: `language-policy` and `objective-reasoning`, whose source files measure 1189 and 3303 bytes with frontmatter. Injection uses the injectable body, meaning the body with frontmatter removed, so 4492 bytes is the upper bound on the pair. The aggregate cap is 6000 bytes, the per-body cap is 4000 bytes, the default cadence is `N = 8`, and the sticky body root is `.claude/rules/autopus/`.

## Test Scenarios

### S1: AC-STICKYRULE-001 - Re-attach follows the exact prompt cadence
Priority: Must
Given the sticky set is `language-policy` and `objective-reasoning` and the cadence `N` is 8
And session `S-alpha` has no prior state
When `auto rules sticky --event UserPromptSubmit` is invoked 17 times in sequence for session `S-alpha`
Then injection occurs at prompt indexes 1, 9, and 17 only

And indexes 1, 9, and 17 each write structured output naming both rules, while indexes 2, 8, 10, and 16 each write empty stdout
And each injecting invocation writes JSON whose `hookSpecificOutput.hookEventName` equals `UserPromptSubmit` and whose `hookSpecificOutput.additionalContext` names both rules
And every invocation exits 0.

### S2: AC-STICKYRULE-002 - Sessions keep independent counters
Priority: Must
Given the cadence `N` is 8 and sessions `S-alpha` and `S-beta` have no prior state
When the runtime is invoked in exactly this 12-step order: alpha, beta, alpha, alpha, beta, alpha, alpha, beta, alpha, alpha, alpha, alpha
Then alpha has been invoked 9 times and beta 3 times
And alpha injects on its own prompt indexes 1 and 9, which are steps 1 and 12
And beta injects on its own prompt index 1 only, which is step 2, and its indexes 2 and 3 produce empty output
And the alpha counter value after the run is 9 and the beta counter value is 3.

### S3: AC-STICKYRULE-003 - A hostile session identifier cannot escape the state directory
Priority: Must
Given the state directory is `.autopus/runtime/sticky-rules/`
When the runtime receives each `session_id` below, namely `abc123`, `../../etc/passwd`, `/absolute/path`, the empty string, `..%2F..%2Fetc`, and a 100000-character string
Then every state filename matches `^[0-9a-f]{64}$` and no file is created or modified outside `.autopus/runtime/sticky-rules/`
And no state file content contains the raw `session_id`, and every invocation exits 0.

### S4: AC-STICKYRULE-004 - Whole-run benign absence stays silent
Priority: Must
Given the runtime is invoked once per condition below, each against a manifest holding only the rule under test
When each condition is present, namely stdin empty, stdin `not json`, manifest absent, manifest containing `{not json`, state file containing `not-a-number`, and the single sticky body file deleted
Then stdout is empty and the exit code is 0 for all six
And no output contains `decision` or `block`, and exit code 2 is never produced, because on `UserPromptSubmit` it would erase the user's prompt
And the deleted-body row is the single-rule case of the per-rule benign skip that S13 exercises alongside a healthy rule.

### S5: AC-STICKYRULE-005 - Prompt text and session identity never persist or return
Priority: Must
Given the runtime receives stdin whose `prompt` is `deploy with AWS_SECRET_ACCESS_KEY=EXAMPLEFAKESECRET`, whose `session_id` is `S-alpha`, whose `transcript_path` is `/Users/example/.claude/projects/x/transcript.jsonl`, and whose `cwd` is `/Users/example/private-repo`
When the runtime injects at prompt index 1
Then stdout contains the `language-policy` and `objective-reasoning` injectable bodies
And stdout does not contain the substrings `EXAMPLEFAKESECRET`, `deploy with`, `S-alpha`, or `/Users/example`
And the written state file contains none of those substrings.

### S6: AC-STICKYRULE-006 - Caps measure injectable bodies and truncate deterministically
Priority: Must
Given the shipped sticky pair whose injectable bodies total at most 4492 bytes
When the runtime injects under the 6000-byte aggregate cap
Then `additionalContext` contains both injectable bodies in full with no truncation notice
And `additionalContext` contains no YAML frontmatter, so no line of it is `name: language-policy`, `category: workflow`, or `alwaysApply: true`
And given instead a synthetic sticky set named `aaa-rule`, `mmm-rule`, and `zzz-rule` whose injectable bodies are 3000, 2000, and 2000 bytes
When the runtime injects
Then `additionalContext` contains `aaa-rule` and `mmm-rule`, omits the `zzz-rule` body, and is at most 6000 bytes measured over injectable bodies plus the notice and excluding JSON framing
And a single-line truncation notice naming `zzz-rule` is present inside `additionalContext`, counts toward the 6000 bytes, and contains no prompt text
And repeating the run produces a byte-identical payload.

### S7: AC-STICKYRULE-007 - Sticky is orthogonal, bounded, and refuses the unrepresentable case
Priority: Must
Given `language-policy` and `objective-reasoning` are classified `always` by `SPEC-CONDRULE-001` and now also carry `alwaysApply: true`
When the claude-code adapter regenerates
Then `.claude/rules/autopus/` still contains exactly the 11 files asserted by `SPEC-CONDRULE-001` scenario S3, including `language-policy.md` and `objective-reasoning.md`
And neither file gains a `paths:` frontmatter field
And the compiled manifest lists exactly `language-policy` and `objective-reasoning` as sticky
And given instead that `lore-commit`, which `SPEC-CONDRULE-001` classifies `hook-fired`, declares `alwaysApply: true`
When validation runs
Then validation returns an error naming `lore-commit`, the value `hook-fired`, and the value `alwaysApply`
And generation does not emit a sticky manifest entry for it.

### S8: AC-STICKYRULE-008 - The alwaysApply field propagates to every frontmatter-preserving platform
Priority: Must
Given `content/rules/language-policy.md` declares `alwaysApply: true`
And the installed gemini rule today carries the source `name`, `description`, and `category` keys plus `platform: antigravity-cli`, so gemini rule emission is frontmatter pass-through and is not invariant under this change
When the codex, opencode, and gemini adapters prepare their rule mappings after template regeneration
Then the codex mapping whose target path ends in `language-policy.md` contains `alwaysApply: true` and `platform: codex`, whichever of the two shapes `ruleFilePath` selects
And the opencode mapping for the same rule contains `alwaysApply: true`
And the gemini mapping for the same rule contains `alwaysApply: true` and retains `platform: antigravity-cli`
And each preserved value is byte-identical to the source, and no platform's emitted body text changes apart from the added frontmatter key.

### S9: AC-STICKYRULE-009 - State stays bounded
Priority: Must
Given the state directory holds 250 entries and 40 of them have a modification time 8 days in the past
When the runtime next writes state
Then the 40 entries older than 7 days are deleted and the directory holds at most 200 entries afterward
And the entries removed are the oldest by modification time, the current session entry survives, and the invocation exits 0.

### S10: AC-STICKYRULE-010 - Re-attach is observed in a live Claude Code session
Priority: Must
Given `auto update` has regenerated the claude-code surfaces and the cadence `N` is set to 2 for the test
When a fresh Claude Code session submits 3 user prompts
Then the `language-policy` rule text is present in the context of prompt 1 and prompt 3 and is not re-injected at prompt 2
And no prompt is erased or blocked, and the observed trace is recorded in the completion evidence.

### S11: AC-STICKYRULE-011 - Sticky state is inspectable
Priority: Should
Given `language-policy` and `objective-reasoning` are sticky and the cadence is 8
When an operator runs `auto rules list`
Then the output shows a sticky column with `language-policy` and `objective-reasoning` true, the effective cadence 8, and every other rule false.

### S12: AC-STICKYRULE-012 - Body reads are confined to the sticky body root
Priority: Must
Given a hostile manifest supplies one sticky entry per row below
And a control entry `language-policy` names the legitimate contained body `language-policy.md`
And a decoy file at the repository root named `secrets.md` contains the literal string `EXAMPLEFAKESECRET`
When the runtime injects at prompt index 1

| manifest body value | expected reason code | injected? |
|---------------------|----------------------|-----------|
| `language-policy.md` (control) | none | yes |
| `/etc/passwd` | absolute_path | no |
| `../../../secrets.md` | path_escape | no |
| `escape.md` symlinked to `../../../secrets.md` | symlink_escape | no |
| `autopus` (a directory) | not_regular_file | no |
| `notes.txt` inside the root | bad_extension | no |
| `huge.md` inside the root whose injectable body is 4001 bytes | body_too_large | no |

Then `additionalContext` contains the `language-policy` injectable body and nothing else, and does not contain `EXAMPLEFAKESECRET`, any byte of `/etc/passwd`, or any byte of the 4001-byte body
And stderr contains each expected reason code exactly once
And stderr does not contain `/etc/passwd`, `secrets.md`, or `EXAMPLEFAKESECRET`
And the exit code is 0, and compile-time generation of the same entries fails with an error naming each offending rule.

### S13: AC-STICKYRULE-013 - Every runtime condition lands in exactly one case
Priority: Must
Given the manifest holds contained `language-policy`, contained `objective-reasoning`, and hostile entry `evil` whose body value is `/etc/passwd`
And the prompt index is an injecting index unless the row says otherwise
When the runtime runs once per row below

| case | trigger | rules injected | stdout | stderr | exit |
|------|---------|----------------|--------|--------|------|
| project root unresolvable | no ancestor holds `.claude` | none | empty | `sticky project_root_unresolved` | 0 |
| whole-run benign | manifest absent | none | empty | empty | 0 |
| whole-run benign | stdin unparseable | none | empty | empty | 0 |
| whole-run benign | state directory unusable | none | empty | empty | 0 |
| whole-run benign | index is off-cadence | none | empty | empty | 0 |
| per-rule benign | `objective-reasoning` body file deleted | language-policy | non-empty | empty | 0 |
| per-rule fail-closed | `evil` body is `/etc/passwd` | language-policy, objective-reasoning | non-empty | `evil absolute_path` | 0 |
| exit barrier | an injected panic during truncation | none | empty | empty | 0 |

Then each row produces exactly the listed injected rule set, stdout, stderr, and exit code
And the per-rule cases suppress only the offending rule while every other sticky rule still injects
And the panic row proves the deferred recover discards partial stdout, since an unrecovered Go panic would otherwise exit 2
And no row produces exit code 2, `decision`, or `block`.

### S14: AC-STICKYRULE-014 - Cadence resolution is observable
Priority: Must
Given the runtime resolves the cadence from `autopus.yaml`
When the configured value is each of the cases below, namely an absent key, `0`, `-3`, and `2`
Then an absent key, `0`, and `-3` each resolve to N=8 and inject at indexes 1 and 9 within 1..9
And `2` resolves to N=2 and injects at indexes 1, 3, 5, 7, and 9
And `auto rules list` reports the same effective cadence
And a non-integer scalar such as `fast` is rejected by `pkg/config/loader.go` at config load with a type error, so it never reaches cadence resolution.

### S15: AC-STICKYRULE-015 - The sticky change is parity-neutral
Priority: Must
Given the `SPEC-CONDRULE-001` post-relocation parity baseline of claude 14, codex 14, gemini 14 rules at 100% Codex rules parity
When `alwaysApply: true` is added to two rules and `go test ./pkg/adapter -run TestParity` runs
Then the parity report still records claude 14, codex 14, and gemini 14 rules
And Codex rules parity is 100.0% and the 95% assertion passes
And no rule changes classification or emission directory as a result of the sticky flag.

### S16: AC-STICKYRULE-016 - The hook is created, confined, and removed
Priority: Must
Given a workspace configured for claude-code, codex, gemini, and opencode
When each transition below occurs, namely two rules sticky, then sticky flags removed while other hooks remain, then sticky flags removed with no hooks at all
Then the sticky transition yields exactly one claude `UserPromptSubmit` entry invoking `auto rules sticky` with timeout 5
And both removal transitions leave no `UserPromptSubmit` key in `.claude/settings.json`
And codex, gemini, and opencode settings carry no `UserPromptSubmit` entry in any of the three transitions
And the removal rows prove `UserPromptSubmit` is treated as an autopus-managed event, since the pre-change `prepareSettingsMapping` builds its managed set only from the hooks being installed and would otherwise preserve a stale entry as a user key
And a user-defined event key that autopus does not manage survives every transition unchanged.

## Oracle Acceptance Notes

Every Must scenario is oracle-first: each pairs concrete inputs with a concrete expected output, and where a bound is involved it states an explicit tolerance, rather than asserting that a section or a file exists.

- S1 is the cadence oracle (INV-101), fixing emit-or-empty for eight specific indexes across a 17-prompt run plus the `hookEventName` field, so an off-by-one or a bare `additionalContext` fails the table.
- S2 fixes an explicit 12-step invocation order and both final counter values, so INV-102 is checked by value with no ambiguous alternation.
- S3 enumerates six hostile or degenerate identifiers and fixes the state filename shape as `^[0-9a-f]{64}$` (INV-105).
- S6 fixes the real measured pair against the 6000-byte aggregate cap, asserts frontmatter is absent from the payload, and pins the truncation notice contract, so INV-106 covers accounting as well as ordering.
- S8 encodes verified adapter behavior: gemini rule emission is frontmatter pass-through, so `alwaysApply` must appear there rather than the file being unchanged, and the codex assertion is path-shape agnostic.
- S12 is the containment oracle (INV-108) with seven heterogeneous entries and named absent substrings.
- S13 is the totality oracle (INV-109, INV-112): eight rows covering all four cases plus the panic barrier, fixing injected set, stdout, stderr, and exit code for each.
- S14 fixes cadence resolution per configured value (INV-101), S15 fixes the parity counts (INV-111), and S16 fixes hook creation, platform confinement, and removal (INV-110, INV-111).
- S10 is the only scenario requiring live observation, because every other scenario can pass against a correctly compiled but unwired build.

Tolerance: the explicit tolerance on every byte cap is zero, meaning caps are exact upper bounds over injectable bodies plus any truncation notice and excluding JSON framing; S13 fixes the expected stdout, expected stderr, and expected exit code per row. The cadence predicate is exact integer arithmetic. Path containment is evaluated after symlink resolution, so a symlink whose target stays inside the root remains admissible.
