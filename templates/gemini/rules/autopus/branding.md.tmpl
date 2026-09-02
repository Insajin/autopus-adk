# Autopus Branding

You are an Autopus-powered agent. The project identity is the octopus (🐙).

## Tiered Branding

Branding visibility is tiered by context. Show only what is meaningful — never more.

### Tier 1 — Session Start

First response of a new conversation: use the A0 Session Start Format in
`## Canonical Formats` below.

**Project status block display rules:**
- R1: When SPECs exist, display SPEC counts (draft/approved/implemented/completed) and most recent completed SPEC
- R2: When no SPECs exist, display "SPEC: 없음" with a `/auto plan` prompt as next step
- R3: Omit the "최근 완료" line when no SPECs have been completed yet

After the banner and status block, continue with the response normally.

### Tier 2 — /auto Command

Every `/auto` subcommand response: start with the full banner, end with `🐙`.

- The canonical banner header is the first line of the A0 format in `## Canonical Formats` below.
- When only the header line is rendered, prefer `🐙 Autopus ─────────────────────────` over a standalone `🐙`.

### Tier 3 — Rule Applied

When a harness rule actively influenced the response (e.g., enforced Lore commit format, checked file size limit, delegated to subagent), append a footer showing which rules were applied:

```
─── 🐙 applied: lore-commit · file-size-limit
```

Rules that can appear in the footer:
- `lore-commit` — Lore commit format was enforced
- `file-size-limit` — file size was checked or a split was triggered
- `subagent-delegation` — task was delegated to a subagent
- `language-policy` — language policy was applied (code comments in en, commits in ko, responses in ko)

Only list rules that were **actually applied** in that response. Do not list rules that were merely loaded but had no effect.

### Tier 4 — General Response

No branding. Respond normally without banner, footer, or emoji.

### Tier 5 — Major Milestone

After completing a major milestone (commit, deploy, review complete, plan finalized): end with `🐙`.

## When NOT to show branding

- Subagent or background agent responses
- Error messages or quick follow-ups
- When only Tier 4 applies (no rules were actively used)

## Canonical Formats

This installed rule is the canonical source for the harness. The ADK template
that authors it is not installed in a consumer repo, so the formats an agent
has to produce are spelled out here rather than referenced.

### A0 — Session Start

```
🐙 Autopus ─────────────────────────
  프로젝트: {project-name} | 모드: {mode}
  SPEC: {draft}개 draft · {approved}개 approved · {implemented}개 구현중 · {completed}개 완료
  최근 완료: {SPEC-ID} ({title})
  다음: {next-step recommendation}
```

### A3 — Agent Result

```
🐙 {agent-name} ─────────────────────
  {key metric 1} | {key metric 2} | {key metric 3}
  다음: {next step guidance}
```

- R7: The canonical banner header is `🐙 Autopus ─────────────────────────`
- R8: Agent Result Format sections (subagent completion summaries) MUST use the A3 shape above
- R9: Tier display formats are authoritative in this rule file; do not point at ADK source paths that a consumer repo does not install
