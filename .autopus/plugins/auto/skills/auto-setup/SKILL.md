---
name: auto-setup
description: >
  프로젝트 컨텍스트 생성 — 코드베이스를 분석하고 ARCHITECTURE.md 및 .autopus/project 문서를 생성합니다
---

# auto-setup — Thin Alias Shim

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## Codex Invocation

Use this alias surface through any of these compatible forms:

- `@auto setup ...` — canonical router when the local Autopus plugin is installed
- `$auto-setup ...` — plugin-local direct alias shim
- `$auto setup ...` — direct router skill invocation

This file is not the detailed workflow definition.
Reinterpret the user's latest `$auto-setup ...` request as `@auto setup ...`, preserve flags exactly, and immediately load skill `auto`.

**프로젝트**: autopus-adk | **모드**: full

## Alias Shim Contract

- Treat this file as a thin alias shim only.
- Immediately load skill `auto` and use it as the canonical router.
- Preserve `--auto`, `--loop`, `--multi`, `--quality`, `--model`, `--variant`, `--team`, `--solo`, and subcommand-specific flags exactly as received.
- Let the router own `Context Load`, `SPEC Path Resolution`, branding, and hand-off to the detailed workflow.
- Do not execute workflow phases directly from this file when a detailed workflow exists.

## Canonical Reinterpretation

- Incoming alias: `$auto-setup <args>`
- Canonical router payload: `@auto setup <args>`
- Required next load: skill `auto`

## Detailed Workflow Source

After the router resolves the request, use these detailed sources:

- `.autopus/plugins/auto/skills/auto/SKILL.md` — plugin-local canonical router surface
- `.agents/skills/auto-setup/SKILL.md` — repository detailed workflow skill
- `.codex/prompts/auto-setup.md` — repository detailed prompt surface

The router must load the detailed workflow after context restoration and SPEC path resolution.

## Handoff Sequence

1. Reinterpret the alias payload as `@auto setup ...`.
2. Load skill `auto`.
3. Let the router perform `Context Load` and, if relevant, `SPEC Path Resolution`.
4. Let the router load the detailed `auto-setup` workflow before execution.
