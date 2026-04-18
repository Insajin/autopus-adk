---
name: doc-storage
description: Document storage rules for correct SPEC, BS, and config file placement
category: structure
platform: gemini-cli
---

# Document Storage Rules

# Document Storage Rules

IMPORTANT: All documents MUST be stored in the correct location based on their scope. Misplaced documents cause sync failures and version control gaps.

## Storage Matrix

| Document Type | Location | Git Repo | Example |
|---------------|----------|----------|---------|
| Project context | Root | meta repo | `ARCHITECTURE.md`, `.autopus/project/*` |
| Harness config | Root | meta repo | `CLAUDE.md`, `.claude/`, `autopus.yaml` |
| Cross-module SPEC | Root `.autopus/specs/` | meta repo | SPECs affecting 2+ modules |
| Module-specific SPEC | `{module}/.autopus/specs/` | module repo | SPECs affecting a single module |
| Cross-module BS | Root `.autopus/brainstorms/` | meta repo | Ideas spanning 2+ modules |
| Module-specific BS | `{module}/.autopus/brainstorms/` | module repo | Ideas for a single module |
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
2. **Meta commit** (Phase B): Root-level documents (`ARCHITECTURE.md`, `.autopus/project/`, `CHANGELOG.md`) are committed to the meta repo

Both phases run in sequence. Phase B is skipped if no root files changed.

## Anti-Patterns

- Do NOT store module-specific SPECs at the root level
- Do NOT store cross-module SPECs inside a single module
- Do NOT create BS or SPEC IDs without checking global uniqueness first
- Do NOT commit root documents to a submodule repo (they are outside its git tree)
