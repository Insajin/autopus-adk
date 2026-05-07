---
name: techstack-freshness
description: Greenfield technology stack choices require current stable version evidence
category: workflow
---

# Technology Stack Freshness

## Purpose

New project work must not rely on model memory, placeholder examples, or old
project defaults when choosing frameworks, runtimes, or dependency versions.

## Mode Classification

- **greenfield**: the user asks for a new project, scaffold, starter, or from-scratch implementation, even if the current working directory contains unrelated workspace manifests.
- **brownfield**: the task changes an existing project with dependency manifests that must remain compatible.

Use `pkg/techstack.InferMode()` as the source contract for this distinction when code needs to classify a request.

## Greenfield Requirements

Before SPEC/PRD text names a framework, runtime, package manager, or dependency version, the system shall create a `## Technology Stack Decision` section with:

- `mode=greenfield`
- selected technologies and resolved stable versions
- official source refs: official docs, release notes, or package registry refs
- `checked_at` date for each source
- rejected alternatives and reason when there is a meaningful choice
- prerelease status, if any

Greenfield choices must use current stable releases by default. Prerelease, beta, RC, canary, preview, snapshot, or `next` versions require an explicit user/product constraint recorded as `allow_prerelease=true`.

## Brownfield Requirements

Brownfield work shall preserve existing manifest major versions unless migration is explicitly in scope. Existing versions are compatibility constraints, not freshness evidence. If a migration is proposed, record the same source refs and checked-at date required for greenfield work.

## Evidence Sources

Prefer sources in this order:

1. Official documentation or release notes
2. Package registries (`npm`, PyPI, crates.io, pkg.go.dev/module tags) when they expose the current stable version
3. Context7 documentation metadata when it includes a resolved version
4. Targeted web search limited to official sources when the primary source is unavailable

## Required SPEC/Research Text

`research.md` or `prd.md` must include:

```markdown
## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
```

Do not leave placeholder examples in this table. Do not cite unversioned "latest" as the version; resolve it to a concrete stable version or record a blocker.

## Anti-Patterns

- Selecting React, Next.js, Tailwind, Vite, Python, Go, or other stack versions from prompt examples alone
- Treating latest documentation as proof that the dependency version was resolved
- Copying old brownfield versions into a greenfield project without source refs
- Silently accepting prerelease versions as "latest"
