# Document Storage Rules

IMPORTANT: All documents MUST be stored in the correct location based on their scope. Misplaced documents cause sync failures and version control gaps.

## Storage Matrix

| Document Type | Location | Git Repo | Example |
|---------------|----------|----------|---------|
| Project context | Root | meta repo | `ARCHITECTURE.md`, `.autopus/project/*` |
| Harness bootstrap config | Root | meta repo | `CLAUDE.md`, `autopus.yaml`, `opencode.json`, `.mcp.json`, `.autopus/context/constraints.yaml` |
| Generated harness/runtime surface | Local working copy only | Do not commit | `.claude/`, `.codex/`, `.gemini/`, `.opencode/`, `.autopus/plugins/`, `.autopus/*-manifest.json` |
| Cross-module SPEC | Root `.autopus/specs/` | meta repo | SPECs affecting 2+ modules |
| Module-specific SPEC | `{module}/.autopus/specs/` | module repo | SPECs affecting a single module |
| Brainstorm/runtime output | Local working copy only | Do not commit | `.autopus/brainstorms/`, `.autopus/orchestra/`, `.autopus/runtime/` |
| CHANGELOG | Root | meta repo | `CHANGELOG.md` |
| Module CHANGELOG | `{module}/CHANGELOG.md` | module repo | Module-specific changes |

## Module Detection

WHEN creating a SPEC or BS, determine the target module by:

1. Check which `pkg/`, `cmd/`, `internal/`, `src/`, `app/` paths are referenced
2. Match those paths to the submodule that contains them
3. If paths span 2+ modules → cross-module → root
4. If no code paths → use the module closest to the described feature

## ID Uniqueness

SPEC IDs and BS IDs MUST be globally unique across the entire workspace.

- Before creating a new ID, scan ALL locations: `.autopus/specs/SPEC-*` AND `*/.autopus/specs/SPEC-*`
- Same for BS: `.autopus/brainstorms/BS-*` AND `*/.autopus/brainstorms/BS-*`
- ID collision is a hard error — never create a duplicate

## Sync Commit Rules

WHEN `/auto sync` runs:

1. **Module commit** (Phase A): SPEC files within `{TARGET_MODULE}` are committed to the module's git repo
2. **Meta commit** (Phase B): Canonical root documents and reviewed bootstrap config (`AGENTS.md`, `ARCHITECTURE.md`, `CLAUDE.md`, `autopus.yaml`, `opencode.json`, `.mcp.json`, `.autopus/context/constraints.yaml`, `.autopus/project/`, `.autopus/specs/`, `.autopus/learnings/pipeline.jsonl`, and human-maintained root Markdown) are committed to the meta repo

Both phases run in sequence. Phase B is skipped if no root files changed.

Before committing, run `auto sync verify` (read-only, zero git mutations). It inventories NUL-delimited Git status plus tracked-but-ignored files with optional locks disabled, partitions every path into a Phase A/B candidate, blocked generated/runtime path, or unclassified path, and renders only shell-safe candidates as `git -C <repo> add -- <paths>`. Generated/runtime, tracked-but-ignored, unsafe, and unclassified paths never enter a copy-ready command.

Use `auto sync verify --spec SPEC-ID` to locate exactly one regular, non-symlink SPEC host across the whole workspace, plan only workspace-relative dirty paths owned by that SPEC, and report every unrelated path. Use `--strict` in hooks or CI to exit non-zero for any boundary, ownership, blocked, or unclassified warning.

## Context Document Rotation

IMPORTANT: The session-load context documents MUST stay compact current-state maps, not append-only ledgers. WHEN `/auto sync` updates them, per-SPEC completion history rotates into per-document archive files instead of accumulating. Unbounded history makes every `/auto` session load truncate at the context cap.

### Session-Load Set

Seven documents load into the model context at every `/auto` session start. Each has a per-document byte cap; the caps sum to 100000 bytes.

| Document | Per-doc cap (bytes) |
|----------|---------------------|
| `.autopus/project/product.md` | 18000 |
| `ARCHITECTURE.md` | 16000 |
| `.autopus/project/scenarios.md` | 20000 |
| `.autopus/project/workspace.md` | 12000 |
| `.autopus/project/tech.md` | 10000 |
| `.autopus/project/structure.md` | 18000 |
| `.autopus/project/canary.md` | 6000 |

### Keep vs Move

Keep only current-state facts in the load-set document. Move completion history and over-detail into that document's archive — losslessly (move, never summarize, rewrite, or delete).

| Keep (current fact, load-set) | Move (to archive, lossless) |
|-------------------------------|------------------------------|
| Latest-state description of what a capability does now | Completion dates (`completed YYYY-MM-DD`) |
| Active boundaries, ownership, command routing | Module commit hashes (`@abc1234`), per-SPEC completion attribution |
| Package-level structure map, active scenario index | Full directory trees, stale or verbose scenario bodies |
| Active canary configuration | Verification narrative (follow-up verification, review-loop hardening) |

### Rotation Rules

WHEN `/auto sync` updates a session-load document:

1. Retain only current-state facts in the document. Append the removed history and over-detail to that document's own archive file at `.autopus/project/archive/<doc>-history-<year>H<half>.md`, where `<doc>` is the document base name without extension (for example, `product` for `product.md`).
2. Choose the half-year bucket from the record's `completed|implemented YYYY-MM-DD` date tag: H1 covers January–June, H2 covers July–December. An undated record inherits the half-year of the nearest dated record above it.
3. Prepend one `Archived-From: <doc>@<date>` header line to every moved record, keeping the original text intact — for example, `Archived-From: product.md@2026-06-15`.
4. Rotation is idempotent: a record already carrying an `Archived-From:` header is never moved or duplicated again.
5. Add a history pointer line to each compacted document that references its own archive file — for example, `History: .autopus/project/archive/product-history-2026H1.md`.

WHEN `/auto sync` records a changelog entry:

1. Keep only the most recent half-year in `CHANGELOG.md`. Move older entries into `CHANGELOG-<year>H<half>.md` without discarding any entry.
2. Take each entry's half-year from its heading ISO date (`completed|implemented|in progress YYYY-MM-DD`); an undated entry inherits the half-year of the nearest dated entry above it.

### Weight Guard

`auto doctor` sums the seven session-load documents and emits a non-blocking warning when the combined size exceeds 120000 bytes or any single document exceeds 20000 bytes. The warning is advisory — it never fails harness health — and signals that rotation is overdue.

### Drift Guard

`auto doctor` also runs an advisory drift gate that reports installed-surface content drift, orphan platform manifests, and — in the ADK source repo — un-regenerated templates and a stale binary commit. Like the weight guard it is non-blocking and only hints at `auto update`, `rm`, or `generate-templates` rather than repairing anything.

### Evidence Freshness Guard

`auto doctor` runs an advisory evidence freshness guard that reports the age of learnings, canary, and memindex loops. Like the other guards, it is non-blocking and warns when the age exceeds 30 days, suggesting `auto learn record`, `auto canary`, or `auto mem rebuild` to refresh the evidence.
Additionally, the `--spec` query filter can be used with `auto learn query` to restrict results strictly to entries matching a specific SPEC ID.

## Anti-Patterns

- Do NOT store module-specific SPECs at the root level
- Do NOT store cross-module SPECs inside a single module
- Do NOT create BS or SPEC IDs without checking global uniqueness first
- Do NOT commit root documents to a submodule repo (they are outside its git tree)
- Do NOT append per-SPEC completion history to a session-load document; rotate it into the document's archive file
- Do NOT summarize or delete rotated history; move it verbatim so record counts are conserved
