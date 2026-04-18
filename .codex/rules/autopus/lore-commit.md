---
name: lore-commit
description: Lore commit format rules for structured, traceable commit messages
category: workflow
platform: codex
---

# Lore Commit

IMPORTANT: All commits MUST use Lore format. Never use plain commit messages or Co-Authored-By trailers.

## Format

```
<type>(<scope>): <subject>

<body>

Constraint: <invariant or design boundary>
Confidence: <low|medium|high>
Scope-risk: <local|module|system>
Reversibility: <trivial|moderate|difficult>
Directive: <follow-up guidance>
Tested: <what was verified>
Not-tested: <what remains unverified>
Related: <SPEC-ID, issue, or related change>

🐙 Autopus <noreply@autopus.co>
```

## Types

| Type | Description |
|------|-------------|
| feat | New feature |
| fix | Bug fix |
| refactor | Code improvement without behavior change |
| test | Add or modify tests |
| docs | Documentation |
| chore | Build, config changes |
| perf | Performance improvement |

## Rules

- `auto check --lore` currently enforces a valid Lore type prefix and the Autopus sign-off.
- Structured Lore trailers use the `Constraint` / `Rejected` / `Confidence` / `Scope-risk` / `Reversibility` / `Directive` / `Tested` / `Not-tested` / `Related` protocol.
- Default `autopus.yaml` requires `Constraint` when Lore trailer validation is enabled.
- `Why` / `Decision` / `Alternatives` trailers are legacy guidance and are no longer the source of truth.
- Sign with `🐙 Autopus <noreply@autopus.co>`
- NEVER add `Co-Authored-By` trailers
- When committing from Codex, build the full Lore message first and use `git commit -F <message-file>` so trailers and sign-off are preserved exactly.
