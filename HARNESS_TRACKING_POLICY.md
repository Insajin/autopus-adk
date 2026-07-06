# Harness Tracking Policy

Autopus-ADK is the source of truth for harness templates, content, adapters, and tests. Generated platform surfaces in this repository are dogfood outputs only. They may be regenerated locally by `auto update`, but they must not be committed as canonical source.

## Track

- `content/**`
- `templates/**`
- `pkg/**`
- `internal/**`
- `cmd/**`
- `configs/**`
- project documentation such as `README.md`, `ARCHITECTURE.md`, `CHANGELOG.md`, and `.autopus/specs/**`
- root bootstrap configuration that is intentionally reviewed, such as `autopus.yaml`

## Do Not Track

- `.claude/**`
- `.codex/**`
- `.gemini/**`
- `.opencode/**`
- `.agents/commands/**`
- `.agents/skills/**`
- `.agents/plugins/**`
- `.agents/hooks.json`
- `.autopus/plugins/**`
- `.autopus/*-manifest.json`
- `.autopus/context/signatures.md`
- `.autopus/orchestra/**`
- `.autopus/brainstorms/**`
- `.autopus/txns/**`
- `.autopus/runtime/**`
- raw QA/runtime outputs under `.autopus/qa/{runs,cache,gui,feedback,evidence,releases}/**`
- root-local runtime config such as `.mcp.json` and `config.toml`

## Cleanup

If generated surfaces are already tracked, remove them from the index without deleting local files:

```bash
git ls-files -c -i --exclude-standard
git ls-files -c -i --exclude-standard -z | xargs -0 git rm --cached --ignore-unmatch --
git status --short --untracked-files=all
```

After cleanup, source template changes should appear under `content/**`, `templates/**`, `pkg/**`, or `internal/**`. Generated platform changes should be ignored and reproducible via `auto update`.
