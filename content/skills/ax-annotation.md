---
name: ax-annotation
description: "@AX code annotation workflow skill for agent-driven tag application"
triggers:
  - ax
  - annotation
  - 어노테이션
  - 태그
category: quality
level1_metadata: "@AX tags, NOTE/WARN/ANCHOR/TODO, per-file limits, [AUTO] prefix"
---

# @AX Annotation Skill

Reference: `pkg/content/ax.go:GenerateAXInstruction()` is the canonical source for all @AX rules.
This skill provides actionable guidance for WHEN and HOW agents apply @AX tags.

## Canonical Source

All tag type definitions, trigger conditions, lifecycle rules, and per-file limits are defined in
`pkg/content/ax.go:GenerateAXInstruction()`. Do NOT redefine rules here — consult that function
as the authoritative source before applying any tag.

## When to Apply @AX Tags

### NOTE Triggers

Apply `@AX:NOTE` when you encounter:
- A magic constant with no explanation
- An exported function over 100 lines that has no godoc comment
- A business rule that is not self-evident from the code

### WARN Triggers

Apply `@AX:WARN` (with `@AX:REASON`) when you detect:
- A goroutine or channel launched without a `context.Context`
- Cyclomatic complexity >= 15 (check with `gocyclo` or manual count)
- Mutation of a package-level or global variable
- A function with 8 or more `if` branches

### ANCHOR Triggers

Apply `@AX:ANCHOR` (with `@AX:REASON`) when:
- A function has fan_in >= 3 callers (heuristic: `grep -r "FuncName(" . | wc -l`)
- Removing or renaming the symbol would break multiple consumers

### TODO Triggers

Apply `@AX:TODO` when:
- A public function has no corresponding test file
- A SPEC requirement is referenced but not yet implemented
- An error is returned without handling (silent discard)

## Application Workflow

Execute after the GREEN or REFACTOR phase of TDD:

1. **Scan** — list all files modified in this task
2. **Detect triggers** — for each file, check NOTE / WARN / ANCHOR / TODO conditions above
3. **Draft tags** — prefix every agent-generated tag with `[AUTO]`
4. **Attach REASON** — add `@AX:REASON` immediately after every WARN and ANCHOR tag
5. **Count per-file** — verify ANCHOR <= 3 and WARN <= 5 per file
6. **Handle overflow** — apply overflow strategy (see below)
7. **Commit** — include tags in the same commit as the code change

## Per-File Limits and Overflow

| Tag | Limit | Overflow Strategy |
|-----|-------|-------------------|
| ANCHOR | 3 per file | Downgrade the entry with the lowest fan_in count to NOTE |
| WARN | 5 per file | Retain the 5 highest-priority (oldest / most severe); drop new candidates |

When a downgrade occurs, add a comment: `// @AX:NOTE: [downgraded from ANCHOR — fan_in < threshold]`

## [AUTO] Prefix Rule

Every tag inserted by an agent MUST begin with `[AUTO]`:

```go
// @AX:NOTE [AUTO]: Magic constant — see payment SLA documentation
const retryLimit = 3
```

Human-authored tags omit `[AUTO]`. Never remove an existing `[AUTO]` prefix.

## @AX:CYCLE Tracking

`@AX:TODO` tags that survive 3 or more TDD cycles without resolution must be escalated to
`@AX:WARN`. Cycle count is tracked via a `sync` comment on the tag line:

```go
// @AX:TODO [AUTO] @AX:CYCLE:2: Add input validation — SPEC-AUTH-001
```

When CYCLE reaches 3, replace the TODO with a WARN and add `@AX:REASON`.

## Language-Specific Comment Syntax

| Language | Prefix |
|----------|--------|
| Go, Java, TypeScript, Rust | `//` |
| Python, Ruby | `#` |
| Haskell | `--` |
