---
name: auto-doctor
description: >
  상태 진단 — 하네스 설치 상태와 플랫폼 wiring을 점검합니다
---

# auto-doctor — Thin Alias Shim

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## Codex Invocation

Use this alias surface through any of these compatible forms:

- `@auto doctor ...` — canonical router when the local Autopus plugin is installed
- `$auto-doctor ...` — plugin-local direct alias shim
- `$auto doctor ...` — direct router skill invocation

This file is not the detailed workflow definition.
Reinterpret the user's latest `$auto-doctor ...` request as `@auto doctor ...`, preserve flags exactly, and immediately load skill `auto`.

**프로젝트**: autopus-adk | **모드**: full

## Alias Shim Contract

- Treat this file as a thin alias shim only.
- Immediately load skill `auto` and use it as the canonical router.
- Preserve `--auto`, `--loop`, `--multi`, `--quality`, `--model`, `--variant`, `--team`, `--solo`, and subcommand-specific flags exactly as received.
- Let the router own `Context Load`, `SPEC Path Resolution`, branding, and hand-off to the detailed workflow.
- Do not execute workflow phases directly from this file when a detailed workflow exists.

## Canonical Reinterpretation

- Incoming alias: `$auto-doctor <args>`
- Canonical router payload: `@auto doctor <args>`
- Required next load: skill `auto`

## Detailed Workflow Source

After the router resolves the request, use these detailed sources:

- `.autopus/plugins/auto/skills/auto/SKILL.md` — plugin-local canonical router surface
- `.agents/skills/auto-doctor/SKILL.md` — repository detailed workflow skill
- `.codex/prompts/auto-doctor.md` — repository detailed prompt surface

The router must load the detailed workflow after context restoration and SPEC path resolution.

## Handoff Sequence

1. Reinterpret the alias payload as `@auto doctor ...`.
2. Load skill `auto`.
3. Let the router perform `Context Load` and, if relevant, `SPEC Path Resolution`.
4. Let the router load the detailed `auto-doctor` workflow before execution.
