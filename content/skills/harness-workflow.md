---
name: harness-workflow
description: Opt-in deterministic 4-phase Route A workflow skill for Claude Code only
triggers:
  - workflow
  - deterministic workflow
  - 워크플로우
  - 결정적 워크플로우
category: agentic
platforms:
  - claude
visibility: claude-only
level1_metadata: "Deterministic Route A, --workflow opt-in, 4-phase, exit-code gate, claude-only, fallback taxonomy"
---

# Harness Workflow Skill

## Overview

The harness workflow (`--workflow`) runs the `/auto go` Route A pipeline as a
**deterministic, opt-in Claude Code Workflow** instead of the default
LLM-sequenced subagent pipeline. Phase order, the build/test gate, and release
hygiene checks are fixed by a manifest, so the run is reproducible and its
adjudication does not depend on model judgment.

**Activation flag**: `/auto go SPEC-ID --workflow`

> **claude-code only.** This skill and the `--workflow` route are scoped to the
> claude-code platform. codex, antigravity-cli (gemini), and opencode render
> their own routers and never emit the workflow surface. The skill itself is
> compiled only into the claude surface (`visibility: claude-only`).

## Why claude-only

The deterministic route is built on Claude Code Workflows
(`.claude/workflows/route_a.workflow.js`) — a primitive only Claude Code
executes. The non-claude adapters produce **zero** `workflow*.js` files, **zero**
`--workflow` tokens, and no `harness-workflow` skill. Their Route A `/auto go`
behavior is unchanged (regression-0 guarantee). Adding `--workflow` to the claude
router template therefore cannot leak to other platforms.

## Capability Gate — `auto workflow doctor`

Before dispatching, the route runs `auto workflow doctor` to confirm the runtime
can execute the workflow. The gate distinguishes **required** primitives (a
missing one fails the gate) from **advisory** primitives (reported, non-fatal).

| Requirement | Value | Class |
|-------------|-------|-------|
| Claude Code version | **2.1.154** or later (`MinVersion`) | required |
| Platform | `claude` only | required |
| Workflow JS present | `.claude/workflows/route_a.workflow.js` | required |
| `auto workflow gate` bridge | resolvable on PATH | required |
| Resumable checkpoint store | available | advisory |

If `auto workflow doctor` reports `overall: fail` OR the platform is not claude,
the route emits a `fail-fast` fallback log line and enters Route A **without
executing any workflow**.

Inspect the planned run without executing it:

```
auto workflow render --dry-run
```

## Deterministic 4-Phase Flow

The workflow runs exactly four ordered phases. The phase-ids are authoritative
and match `content/workflows/route_a.schema.json`.

1. **planning** — produces the implementation plan and task breakdown; does not
   mutate the working tree beyond plan artifacts.
2. **implementation** — performs the repository-mutating work. Worktree
   creation, branch naming, the worktree slot cap, and worktree reclaim stay in
   the Go runtime (`pkg/pipeline`); the workflow JS owns sequencing only.
3. **gate_build_test** — the deterministic gate. Its verdict derives from build
   and test command **exit codes** (`verdict_source: exit_code`), not from an
   LLM verdict. The gate is executed outside the JS via the Go runtime (calling
   `auto workflow gate`), which runs build and test through an
   injectable `CommandRunner` seam and emits
   `{verdict, verdict_source, build_exit, test_exit}` JSON. A non-zero build or
   test exit code yields `verdict: fail`. The JS carries this phase as a sequencing
   marker with no shell-out primitives embedded.
4. **release_hygiene** — terminal phase that enforces release safety before
   sync:
   - **Generated-surface drift gate** — blocks when generated surfaces
     (`.claude` / `.codex` / `.gemini` / `.opencode` / `.autopus/plugins`) are
     staged without a matching source-of-truth change, and always blocks runtime
     artifacts such as `.autopus/txns` / `.autopus/orchestra`.
   - **300-line source limit** — `auto check --hygiene --arch --quiet --staged`.
   - **Lore commit format** — the commit-msg hook uses
     `auto check --lore --message <msgfile>` for the pending message boundary.

## Fallback Taxonomy

Every workflow failure maps to exactly one class — silent opt-out is forbidden.

| Failure kind | Class | Behavior |
|--------------|-------|----------|
| `non_claude_platform` | `fail-fast` | Abort immediately, fall back to Route A |
| `doctor_fail` | `fail-fast` | Abort immediately, fall back to Route A |
| `parity_drift` | `fail-closed` | Refuse to proceed and block |
| `execution_abort` | `resumable` | Resume from a recorded checkpoint |
| `api_unavailable` | `explicit` | Surface to the operator for a decision |

## Manifest is the Source of Truth

`content/workflows/route_a.md` and `content/workflows/route_a.schema.json` are
the human- and machine-authoritative manifest. The Workflow JS
(`templates/claude/workflows/route_a.workflow.js.tmpl` and the installed
`.claude/workflows/route_a.workflow.js`) is a **generated, edit-forbidden
surface** derived from the manifest by `auto generate-templates`. Do not edit the
JS by hand — edit the manifest and regenerate. A parity gate compares the
phase-id, retry, budget, and result-type sets across the markdown, the schema,
and the derived JS; any divergence fails generation closed and names the
diverging element.

## When to Use

| Scenario | Use `--workflow`? |
|----------|-------------------|
| Reproducible, gate-driven Route A run on Claude Code | Yes |
| Exit-code-based build/test adjudication required | Yes |
| Running on codex / gemini / opencode | No — flag is ignored, Route A runs as usual |
| Doctor reports `overall: fail` | No — auto fail-fast into Route A |

**Ref**: SPEC-HARNESS-WORKFLOW-001

---

## Team Workflow Substrate — route_team

> **claude-code only.** The team workflow substrate (`route_team`) is scoped to the claude-code platform by the same platform constraint that governs `route_a`. codex, antigravity-cli, and opencode never emit `route_team.workflow.js` and are unaffected (regression-0 guarantee).

### Overview

When `/auto go SPEC-ID --team` is invoked on claude-code and `auto workflow doctor` passes (and no disable flag is active), the pipeline is served by the **deterministic team Workflow substrate** instead of ad-hoc Agent Teams. The substrate is implemented as `.claude/workflows/route_team.workflow.js` — a **generated, edit-forbidden surface** derived from the manifest by `auto generate-templates`. Do not edit the JS by hand.

### Capability Gate

The team workflow substrate uses the **same `auto workflow doctor` gate** as `route_a`, including the same minimum version (`2.1.154`) and the same required vs advisory primitive classification. The parity gate now additionally covers model/effort/depth resolution (see Quality Binding below).

Disable paths (pre-route opt-out, not a taxonomy failure):
- `--no-workflow` flag
- `autopus.yaml` → `workflow.team_default=false`

Doctor-fail path: emit `[workflow] fallback-class=fail-fast reason=doctor_fail` and fall back to Route A **without** entering Agent Teams.

### 8 Ordered Phases

The `route_team` workflow runs exactly eight ordered phases. Phase IDs are authoritative and match `content/workflows/route_team.schema.json`.

| Phase | Type | Description |
|-------|------|-------------|
| **planning** | `agent()` | Produces the implementation plan and task breakdown; does not mutate the working tree beyond plan artifacts. |
| **test_scaffold** | `agent()` | Writes failing test skeletons for P0/P1 requirements (RED state). |
| **implementation** | `agent()` — task-threaded parallel executors | Runs executor agents concurrently via `parallel()` with `isolation: 'worktree'`, threading them over task assignments (`plan.tasks[i]`) produced by the planner. Each executor owns a **disjoint** file set; the planner groups inter-dependent files (impl + its test) into one task so isolated executors never recreate each other's files (overlap → merge conflict → skip). Fan-out count is dynamically bounded by `min(tasks.length, cap)` with `cap ≤ 5`. |
| **gate_build_test** | **deterministic Go gate** | Verdict derives from build/test **exit codes** (`verdict_source: exit_code`), not from an LLM verdict. Executed outside the JS via Go runtime (calling `auto workflow gate` execution bridge) which emits `{verdict, verdict_source, build_exit, test_exit}` JSON. A non-zero exit yields `verdict: fail`. |
| **annotation** | `agent()` | Applies `@AX` annotation tags to all files modified during implementation. |
| **testing** | `agent()` | Raises test coverage to 85%+; runs `go test -race -cover ./...` and affected QAMESH lanes. |
| **review** | `agent()` — dual-role verify-vote loop | Runs specialized `reviewer` (for verify-vote and optional synthesis) and `security-auditor` in parallel. Verify votes are bounded: **verify_votes ≤ 3**. |
| **release_hygiene** | **deterministic Go gate** | Executed outside the JS via Go runtime (calling `auto check --hygiene --arch --quiet --staged`) which enforces the 300-line source limit and generated-surface drift gate. Commit-msg hooks enforce Lore format via `auto check --lore --message <msgfile>`. |

### Manifest Source of Truth

| Artifact | Role |
|----------|------|
| `content/workflows/route_team.md` | Human-authoritative manifest |
| `content/workflows/route_team.schema.json` | Machine-authoritative manifest |
| `templates/claude/workflows/route_team.workflow.js.tmpl` | Template — generated surface, edit-forbidden |
| `.claude/workflows/route_team.workflow.js` | Installed generated surface — edit-forbidden |

Edit the manifest files and run `auto generate-templates` to regenerate the JS. The parity gate compares phase-id, retry, budget, and result-type sets across the markdown, schema, and JS; any divergence fails generation closed.

### Quality Binding

Per-run `--quality` resolves **three dimensions simultaneously** through the existing resolvers:

- **Model tier** — `ModelForAgent`: ultra → Opus for all agent phases; balanced → per-agent defaults.
- **Effort** — `ResolveEffort`: ultra → higher effort ceiling; balanced → per-agent defaults.
- **Orchestration depth** — `ResolveDepth`: ultra = 3-vote adversarial verify + synthesis in the review phase; balanced = single-vote. Hard caps: verify_votes ≤ 3, fan_out_cap ≤ 5, retry ≤ 3.

The resolved quality is delivered through the Workflow `args` input (`args.quality`). The workflow JS reads `RT.<phase>` from `args.quality` to override the schema baseline literal for each phase.

### Workflow input args Schema

The per-run context is passed via the `args` global with the following schema:
- `spec`: target SPEC ID (string)
- `workingDir`: absolute path to the workspace directory (string)
- `quality`: serialized per-phase quality binding (JSON object)
- `segment`: which segment to execute — `'A'` (planning through gate_build_test) or `'B'` (post-gate phases through release_hygiene); defaults to `'A'` when absent

### Segmented Dispatch Contract

The dispatcher (main session) launches the workflow in two segments separated by the deterministic gate. This is required because a single `workflow({scriptPath}, args)` launch runs all phases unconditionally — there is no re-entry point to block post-gate phases from within JS.

**Dispatcher sequence:**
1. Launch segment A: `workflow({scriptPath}, {spec, workingDir, quality, segment:'A'})`
   — executes planning, implementation, and the gate_build_test boundary marker.
   Executor agents run with `isolation: 'worktree'`; their changes are **uncommitted**
   working-tree edits stranded in separate worktrees under `.claude/worktrees/`.
   Segment A **returns `{ plan }`** (the planner's task assignment); capture it.
1b. Persist the returned plan to a temp JSON file, e.g.
   `<workingDir>/.claude/workflows/run-<runid>-plan.json` (the dispatcher can write
   files; the workflow JS cannot). This drives ownership enforcement in step 2.
2. Run `auto workflow merge --run <segment-A-runid> --ownership <plan.json>` (Go
   runtime, worktree consolidation). With `--ownership`, each worktree is assigned
   1:1 to the task it performed and **only files within that task's ownership are
   merged** — files an executor created outside its assigned set (overlap into
   another task's files) are reported in `skipped_out_of_scope` and never copied,
   giving a hard guarantee against executor overlap. Merge then copies the owned
   uncommitted changes into `workingDir`, stages them with `git add`, and removes
   the worktrees. This step is required before the gate: without it, `auto workflow
   gate` would build/test the unchanged main tree (vacuous pass). Any residual
   same-file conflict is reported in the JSON but is not a hard failure — the
   operator/gate decides. Exit non-zero only on a hard infrastructure error.
   (Without `--ownership`, merge falls back to the plain conflict-skip behavior.)
3. After merge, run `auto workflow gate` (Go runtime, exit-code verdict).
   — If `verdict != pass`: **abort. Do NOT launch segment B.**
4. Launch segment B: `workflow({scriptPath}, {spec, workingDir, quality, segment:'B'})`
   — executes annotation, testing, review, and the release_hygiene boundary marker.
5. After segment B returns, run `auto check --hygiene --arch --quiet --staged`.

The gate phases (`gate_build_test`, `release_hygiene`) are **segment-boundary markers** in the JS — they emit `phase(id)` + `log(...)` but contain no shell-out logic. All exit-code adjudication is performed by the dispatcher between segment launches (`verdict_source: exit_code` is preserved).

Inspect resolved per-phase model/effort/depth without executing agents:

```
auto workflow render --route team [--quality <mode>]
```

### Fallback Taxonomy

The `route_team` substrate shares the same fallback taxonomy as `route_a` — silent opt-out is forbidden.

| Failure kind | Class | Behavior |
|--------------|-------|----------|
| `non_claude_platform` | `fail-fast` | Abort immediately, fall back to Route A |
| `doctor_fail` | `fail-fast` | Abort immediately, fall back to Route A |
| `parity_drift` | `fail-closed` | Refuse to proceed and block |
| `execution_abort` | `resumable` | Resume from a recorded checkpoint |
| `api_unavailable` | `explicit` | Surface to the operator for a decision |

**Ref**: SPEC-HARNESS-WORKFLOW-TEAM-001
