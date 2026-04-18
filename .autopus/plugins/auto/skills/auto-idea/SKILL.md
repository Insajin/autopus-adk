---
name: auto-idea
description: >
  아이디어 브레인스토밍 — 멀티 프로바이더 토론과 ICE 평가로 아이디어를 정리합니다
---

# auto-idea — Thin Alias Shim

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## Codex Invocation

Use this alias surface through any of these compatible forms:

- `@auto idea ...` — canonical router when the local Autopus plugin is installed
- `$auto-idea ...` — plugin-local direct alias shim
- `$auto idea ...` — direct router skill invocation

This file is not the detailed workflow definition.
Reinterpret the user's latest `$auto-idea ...` request as `@auto idea ...`, preserve flags exactly, and immediately load skill `auto`.

**프로젝트**: autopus-adk | **모드**: full

## Alias Shim Contract

- Treat this file as a thin alias shim only.
- Immediately load skill `auto` and use it as the canonical router.
- Preserve `--auto`, `--loop`, `--multi`, `--quality`, `--model`, `--variant`, `--team`, `--solo`, and subcommand-specific flags exactly as received.
- Let the router own `Context Load`, `SPEC Path Resolution`, branding, and hand-off to the detailed workflow.
- Do not execute workflow phases directly from this file when a detailed workflow exists.

## Canonical Reinterpretation

- Incoming alias: `$auto-idea <args>`
- Canonical router payload: `@auto idea <args>`
- Required next load: skill `auto`

## Detailed Workflow Source

After the router resolves the request, use these detailed sources:

- `.autopus/plugins/auto/skills/auto/SKILL.md` — plugin-local canonical router surface
- `.agents/skills/auto-idea/SKILL.md` — repository detailed workflow skill
- `.codex/prompts/auto-idea.md` — repository detailed prompt surface

The router must load the detailed workflow after context restoration and SPEC path resolution.

## Handoff Sequence

1. Reinterpret the alias payload as `@auto idea ...`.
2. Load skill `auto`.
3. Let the router perform `Context Load` and, if relevant, `SPEC Path Resolution`.
4. Let the router load the detailed `auto-idea` workflow before execution.
