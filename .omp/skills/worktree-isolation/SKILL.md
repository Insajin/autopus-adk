---
name: worktree-isolation
description: Worktree isolation for parallel agent execution in the pipeline
compatibility: omp
---

# OMP Native Task Isolation

This skill defines isolation policy for Autopus executor fan-out on OMP. The native `task` tool
owns workspace materialization, patch/branch integration, and cleanup.

## Activation

Use per-item isolation only when all conditions hold:

- the current dynamic `task` schema exposes `isolated`;
- OMP settings enable a non-`none` isolation mode;
- the project is a git repository;
- the item writes files;
- ownership is disjoint from every concurrent item.

Do not request isolation for `--solo`, read-only work, overlapping ownership, shared migration
numbering, or an environment where the field is absent. Missing required isolation is a blocker,
not permission to imitate it with shell commands.

## Native Dispatch Contract

Inspect the current dynamic schema before each wave. When it exposes batch mode, dispatch
independent isolated work in one batch with top-level `i` and shared `context`. Each item uses a
unique stable name, a discovered custom role when needed, a complete assignment, the discovered
isolation field enabled, the strict five-field receipt schema, and `schemaMode: strict`. When batch
mode is absent, dispatch the corresponding flat calls and reference one shared `local://` context.

The parent must check dynamic availability before adding `isolated` or `effort`. `isolated` does
not exist when `task.isolation.mode = none`, and `effort` does not exist when its setting is off.
Omit either unsupported field rather than sending it from a static example.

## Ownership Preflight

Before fan-out:

1. normalize every owned and forbidden path to a project-relative path;
2. reject absolute paths, traversal, symlinks, nested ownership, and ambiguous globs;
3. detect exact overlap and directory-prefix containment;
4. serialize tasks that share a generated artifact, migration directory, package manifest, lockfile,
   schema registry, or other mutable authority;
5. put cross-task interfaces in top-level `context` before dispatch.

A maximum of five Autopus executor items may be active in one wave. Queue overflow by task id.
This policy is stricter than OMP's session-wide semaphore and prevents excessive integration churn.

## Tool-Owned Lifecycle

For an isolated item, OMP:

1. captures the parent baseline;
2. creates the configured isolated workspace;
3. runs the child in that workspace;
4. captures a patch or commits a temporary task branch according to OMP settings;
5. applies or merges the result into the parent through the task lifecycle;
6. cleans the isolated workspace;
7. preserves output, transcript, and patch metadata through `agent://` and `history://`.

The Autopus parent must not run manual worktree creation, branch merge, cherry-pick, stash, reset,
or removal commands. It verifies the returned `changed_files`, ownership boundary, blockers, and
parent-tree result after OMP completes integration. Nested repositories are handled by OMP's nested
patch lifecycle and must not be merged manually.

## Sequential Work

A task is sequential when it depends on an earlier result, overlaps ownership, or shares a migration
numbering lane. Dispatch it only after the prerequisite result is visible in the parent workspace.
Do not use an isolated batch to hide a dependency edge.

## Failure and Conflict Handling

- A failed isolated run is terminal and not revivable because its workspace has been cleaned.
- Keep the transcript and patch metadata as evidence.
- If integration fails, stop the next wave and report the exact OMP lifecycle error.
- Never bypass a failed patch/branch integration with destructive git commands.
- A correction is a new explicitly named task with freshly declared ownership and context.
- Cancel still-running jobs through `hub` with top-level `i`; preserve user-owned changes and unrelated jobs.

## Receipt Verification

Every isolated worker returns exactly:

- `owned_paths`
- `changed_files`
- `verification`
- `blockers`
- `next_required_step`

The main session rejects missing fields, out-of-scope changes, body dumps, secret material, or claims
without observable verification. Integration is complete only after the parent sees the intended
changes and the next deterministic gate passes.

## OMP Coordination Contract

### Ownership gate

- Choose exactly one DAG owner with `--execution-owner omp|orca` before dispatch; omission selects owner `omp`.
- Owner `omp` is the default. The current OMP session is the sole DAG owner and uses its native `task`, `hub`, and `todo` tools.
- Owner `orca` is allowed only when `--execution-owner orca` is explicit. Before any Orca orchestration, run and read `orca skills get orchestration --full`.
- The single DAG owner invariant is mandatory: owner `orca` creates no OMP task DAG, and owner `omp` creates no Orca Run.

### Native field contracts

```json
{
  "i": "Dispatching bounded OMP work",
  "context": "Shared goal, constraints, owned-path boundaries, and cross-task contracts.",
  "tasks": [
    {
      "name": "Worker",
      "task": "Complete one self-contained assignment and return only the required receipt.",
      "outputSchema": {
        "type": "object",
        "additionalProperties": false,
        "required": ["owned_paths", "changed_files", "verification", "blockers", "next_required_step"],
        "properties": {
          "owned_paths": {"type": "array", "items": {"type": "string"}},
          "changed_files": {"type": "array", "items": {"type": "string"}},
          "verification": {"type": "array", "items": {"type": "string"}},
          "blockers": {"type": "array", "items": {"type": "string"}},
          "next_required_step": {"type": "string"}
        }
      },
      "schemaMode": "strict"
    }
  ]
}
```

- Inspect the current dynamic `task` schema before dispatch. Use the shown batch shape only when it exposes top-level `context` and `tasks`; otherwise use the discovered flat shape and place shared context in `local://`.
- Every model-authored `task`, `hub`, and `todo` call includes a concise top-level `i` while `tools.intentTracing` is enabled.
- Every `tasks` item uses `name` when a stable agent id is useful and carries per-item `task`, `outputSchema`, and `schemaMode`. Set `agent` only to select a custom agent type; omit it for OMP's default general worker.
- `isolated` and `effort` are conditional dynamic fields. Add `isolated` or `effort` only after the current schema exposes that exact field; otherwise omit it.
- `outputSchema` is the strict five-field receipt JSON Schema shown in the normalized batch: `owned_paths`, `changed_files`, `verification`, `blockers`, and `next_required_step`.
- Retain the agent id returned by `task`. For a non-isolated or otherwise revivable worker, every follow-up goes to that same id with `hub` send fields `{"i":"Following up with an existing worker","op":"send","to":"<same agent id>","message":"<follow-up>"}`; do not create a replacement merely to continue revivable work.
- An isolated worker is terminal after workspace cleanup and cannot be revived. A correction is a new explicitly named `task` item with freshly declared ownership and context, not a `hub` send to the terminal agent id.
- The parent OMP session owns progress. A `todo` call contains one top-level operation and intent: initialize with `{"i":"Updating parent-owned progress","op":"init","list":[{"phase":"Implementation","items":["..."]}]}`, advance with `{"i":"Updating parent-owned progress","op":"start","task":"<exact task content>"}`, complete with `{"i":"Updating parent-owned progress","op":"done","task":"<exact task content>"}`, and block with `{"i":"Updating parent-owned progress","op":"block","task":"<exact task content>","reason":"<reason>"}`.
