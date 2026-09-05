---
name: techstack-freshness
description: Greenfield technology stack choices require current stable version evidence
category: workflow
skillScoped: true
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

## Upstream Resolution and Managed Tool Strategies

Check the current upstream stable release against an official source. Discovery
may use a moving label, but every build manifest, lock, setup record, or release
receipt must resolve it to a concrete immutable version, SHA, or digest. Never
persist `latest`, an unqualified branch, or another moving reference as build
identity.

For each managed runtime, compiler, package manager, browser, helper, or
application tool, select and record exactly one strategy:

- **`latest-stable`**: resolve the official current stable release, then pin its concrete immutable identity.
- **`repo-compatible-pin`**: preserve the repository-declared compatible version or lock and record why compatibility takes priority over upstream freshness.
- **`source-exact`**: build or select the artifact produced from the owning repository's exact intended source state and bind it to that source identity.

These strategies are not interchangeable. Record the selected strategy,
official or repository source ref, resolved identity, and `checked_at` value.

## Native and Live QA Source Binding

Native or live QA may claim **current-source PASS** only when the owning
repository's current intended source state and the artifact receipts for every
tested runtime component are an exact match.

Capture the start and end source SHA, tree digest, and diff digest. Bind those
values to digests of the actual app binary or bundle, helper executable, and
browser binary or bundle used by the run. Artifact receipts must identify the
artifact path, immutable digest, build or install origin, capture time, and the
source identity it claims to represent. If the source state changes during the
run, rebuild or reinstall as required and repeat the affected QA against a new
receipt.

An installed older app, a previously built helper, or web-only evidence cannot
substitute for current native PASS. Such evidence may be reported only with its
actual scope and provenance; it must not be promoted to current-source,
native-release, or end-to-end proof.

## Failure and Offline Behavior

Stale evidence, a missing version or digest, an artifact/source mismatch, or an
official-source lookup failure must fail closed before release proof or native
proof is claimed. Do not infer freshness from caches, model memory, installed
tool output, or a successful web-only run.

Only explicitly offline development may continue in degraded mode. Mark it
`DEGRADED_OFFLINE`, list the unresolved lookups and stale or missing receipts,
and prohibit release, native, and current-source PASS claims until the evidence
is refreshed and exact matching succeeds.

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

Do not leave illustrative example rows in this table. Do not cite unversioned
"latest" as the version; resolve it to a concrete stable version or record a
blocker. When managed tooling or native/live QA is in scope, also record the
selected strategy and the source/artifact receipt described above.

## Anti-Patterns

- Selecting React, Next.js, Tailwind, Vite, Python, Go, or other stack versions from prompt examples alone
- Treating latest documentation as proof that the dependency version was resolved
- Copying old brownfield versions into a greenfield project without source refs
- Silently accepting prerelease versions as "latest"
- Treating an installed app version or web-only success as proof of the current native source
- Claiming release readiness while source or artifact identity evidence is stale, missing, or unresolved
