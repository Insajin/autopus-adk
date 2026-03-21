# Lore Commit

IMPORTANT: All commits MUST use Lore format. Never use plain commit messages or Co-Authored-By trailers.

## Format

```
<type>(<scope>): <subject>

<body>

Why: <reason for this change>
Decision: <what was decided>
Alternatives: <other options considered>
Ref: <SPEC-ID or issue>

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

- Subject: 50 characters max, imperative mood
- Body: Focus on **why**, not what
- Why, Decision trailers: REQUIRED for all commits with design decisions
- Alternatives trailer: REQUIRED when alternatives were considered
- Ref trailer: Include when a SPEC or issue exists
- Sign with `🐙 Autopus <noreply@autopus.co>`
- NEVER add `Co-Authored-By` trailers
