---
name: deferred-tools
description: Load deferred tool schemas via ToolSearch before use to keep interactive UI and scheduling tools functional
category: harness
---

# Deferred Tools Loading

IMPORTANT: Claude Code loads most tool schemas lazily as "deferred tools". Calling them without first loading their schema silently downgrades UI (e.g., `AskUserQuestion` renders as plain text) or raises `InputValidationError` (e.g., `TaskCreate`, `SendMessage`).

This rule applies to Claude Code platforms. Gemini CLI, Codex, and OpenCode do not expose a deferred-tool mechanism — platform adapters may safely ignore or transform the specific tool references below.

## Detection

Deferred tools appear by name in a `<system-reminder>` at the start of the session but their schemas are NOT in the prompt. Look for the reminder:

> The following deferred tools are now available via ToolSearch. Their schemas are NOT loaded — calling them directly will fail with InputValidationError.

Common deferred tools in this workspace:

- `AskUserQuestion` — interactive option picker (Triage flow, Gate 1 approval)
- `TaskCreate`, `TaskUpdate`, `TaskList`, `TaskGet`, `TaskOutput`, `TaskStop` — background/long-running task management
- `TeamCreate`, `TeamDelete`, `SendMessage` — Agent Teams lifecycle
- `CronCreate`, `CronList`, `CronDelete`, `RemoteTrigger` — scheduling
- `WebFetch`, `WebSearch` — web access (Phase 1.8 fallback)
- `EnterPlanMode`, `ExitPlanMode`, `EnterWorktree`, `ExitWorktree`, `Monitor`, `PushNotification`

## Rule

WHEN a `/auto` subcommand or skill instructs the use of a deferred tool, THE SYSTEM SHALL call `ToolSearch` to load its schema BEFORE the first invocation in the session.

```
ToolSearch(query = "select:AskUserQuestion")
```

Batch multiple tools in one call when they will all be used in the same flow:

```
ToolSearch(query = "select:TeamCreate,SendMessage,TeamDelete")
```

WHERE the schema is already loaded in the current session, skip — do not re-load.

## Trigger Points (Proactive Loading)

Load schemas proactively at these points to avoid mid-flow failures:

| Trigger | Preload |
|---------|---------|
| `/auto` Triage (natural language input) | `AskUserQuestion` |
| `/auto go` Gate 1 Approval (non-`--auto`) | `AskUserQuestion` |
| `/auto go --team` (Route B) | `TeamCreate`, `SendMessage`, `TeamDelete` |
| `/auto` long-running operations | `TaskCreate`, `TaskUpdate`, `TaskList` |
| Phase 1.8 Doc Fetch web fallback | `WebSearch`, `WebFetch` |
| `/auto schedule` or `/loop` | `CronCreate`, `CronList`, `CronDelete` |

## Degraded UI Detection

IF a planned `AskUserQuestion` call is rendered as a numbered text list in your output → the schema was NOT loaded. Recover:

1. Call `ToolSearch(query="select:AskUserQuestion")`
2. Re-issue the question via the real `AskUserQuestion` tool
3. Do NOT ask the user to manually reply by typing a number

## Anti-Patterns

- Do NOT ignore the deferred tools `<system-reminder>` and attempt direct invocation
- Do NOT fall back to text-based prompts when interactive UI is available — load the schema instead
- Do NOT load every deferred tool preemptively — only load what the current flow will actually use
- Do NOT re-run `ToolSearch` for already-loaded schemas in the same session
