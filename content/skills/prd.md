---
name: prd
description: PRD(Product Requirements Document) 작성 스킬
triggers:
  - prd
  - PRD
  - product requirements
  - 기획서
category: workflow
level1_metadata: "PRD generation, Standard/Minimal modes, quality validation"
---

# PRD Skill

Skill for creating Product Requirements Documents (PRD) that provide top-level context for the Planning → SPEC pipeline.

## PRD Writing Process

### Step 1: Request Analysis

Identify the four core dimensions from the user's request:

- **What**: What product or feature are we building?
- **Why**: What problem does it solve? What is the business motivation?
- **Who**: Who are the primary users or stakeholders?
- **When**: What is the target release or deadline?

Clarify any missing dimensions before proceeding.

### Step 2: Codebase Context Collection

Gather relevant context to ground the PRD in the current state of the system:

- **Related files**: Identify existing modules, packages, or services affected
- **Existing patterns**: Review coding conventions, API patterns, data models
- **Prior SPECs**: Check for related SPEC documents across top-level and submodules

```
ls .autopus/specs/ */.autopus/specs/ 2>/dev/null   # list existing SPECs (top-level + submodules)
cat .autopus/specs/SPEC-*/prd.md */.autopus/specs/SPEC-*/prd.md 2>/dev/null  # review related PRDs
```

Use this context to ensure the PRD aligns with existing architecture and avoids conflicts.

### Step 3: PRD Section Authoring

Choose the appropriate mode based on scope:

#### Mode Selection

| Mode | When to Use | Sections |
|------|-------------|----------|
| **Standard** | New features, cross-team work, public APIs | 10 sections |
| **Minimal** | Small changes, internal tools, hotfixes | 5 sections |

#### Standard Mode (10 sections)

Reference: `templates/shared/prd-standard.md.tmpl`

1. **Problem & Context** — Current situation, problem statement, business impact
2. **Goals & Success Metrics** — SMART goals with quantitative success criteria
3. **Target Users** — User groups, roles, usage frequency, key expectations
4. **User Stories** — As a / I want / so that + Given-When-Then acceptance criteria
5. **Functional Requirements** — MoSCoW prioritized (P0=Must, P1=Should, P2=Could)
6. **Non-Functional Requirements** — Performance, security, scalability, compliance
7. **Technical Constraints** — Stack constraints, external dependencies, compatibility
8. **Out of Scope** — Explicit exclusions to prevent scope creep
9. **Risks & Open Questions** — Risk severity/mitigation + unresolved questions
10. **Practitioner Q&A** — Key implementation questions with answers or TBD

#### Minimal Mode (5 sections)

Reference: `templates/shared/prd-minimal.md.tmpl`

1. **Problem** — Core problem in 1-2 sentences
2. **Requirements** — P0 items only, EARS format
3. **Technical Notes** — Constraints, dependencies, impact on existing code
4. **Out of Scope** — At least one explicit exclusion
5. **Key Q&A** — 3-5 blocking questions with answers or TBD

### Step 4: Quality Validation

Run the following checklist before finalizing the PRD:

```markdown
## PRD Quality Checklist

### Structure (Standard mode)
- [ ] All 10 sections present and non-empty
- [ ] Overview is ≤ 3 sentences

### Structure (Minimal mode)
- [ ] All 5 sections present and non-empty

### Goals
- [ ] At least 1 measurable success metric defined
  - Good: "p99 latency < 200ms", "DAU increase by 10%"
  - Bad: "improve performance", "make users happy"

### Requirements
- [ ] At least 1 P0 (Must Have) requirement listed
- [ ] Requirements written in EARS format

### Scope
- [ ] At least 1 Out of Scope item explicitly listed

### Consistency
- [ ] No conflicts with existing SPECs (check both `.autopus/specs/` and `*/.autopus/specs/`)
- [ ] Terminology matches codebase conventions
```

Flag any checklist failures to the user before saving.

### Step 5: File Save

Save the completed PRD to the target module's SPEC directory:

```
{target-module}/.autopus/specs/SPEC-{ID}/prd.md
```

Where `{ID}` is the next available SPEC identifier (e.g., `SPEC-AUTH-001`), unique across the entire project. The target module is determined by the spec-writer's module detection logic.

If the directory does not exist, create it:

```bash
mkdir -p {target-module}/.autopus/specs/SPEC-{ID}
```

## Relationship to Other Skills

PRD sits at the top of the planning pipeline:

```
PRD (this skill)
  └─> Planning (planning.md) — EARS requirements, MoSCoW prioritization
        └─> SPEC — Formal specification with implementation tasks
```

- The **Goals** and **Requirements** sections of a PRD feed directly into `planning.md`'s requirements analysis step.
- EARS format and MoSCoW priorities defined in a PRD carry forward unchanged into the Planning and SPEC phases.
- When creating a Planning document, reference the PRD for top-level context and constraints.

## Output Example

```markdown
# PRD: Async Job Queue

**SPEC-ID**: SPEC-QUEUE-001
**Mode**: Standard
**Date**: 2026-03-23

## 1. Problem & Context
API endpoints take 500ms+ due to slow background tasks running synchronously in the request path.
This causes timeouts and poor user experience under load.

## 2. Goals & Success Metrics
| Goal | Success Metric | Target |
|------|---------------|--------|
| Reduce latency | p99 API response time | < 200ms |
| Improve reliability | Job failure rate | < 0.1% |

## 8. Out of Scope
- Priority queues (deferred to SPEC-QUEUE-002)
- Cross-region job routing

## 5. Functional Requirements
### P0 — Must Have
| ID | Requirement |
|----|-------------|
| FR-01 | WHEN a job is enqueued, THE SYSTEM SHALL persist it durably before acknowledging |
| FR-02 | WHEN a job fails, THE SYSTEM SHALL retry up to 3 times with exponential backoff |

### P1 — Should Have
| ID | Requirement |
|----|-------------|
| FR-10 | WHILE a job is running, THE SYSTEM SHALL emit telemetry events |
```
