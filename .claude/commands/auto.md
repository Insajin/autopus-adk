# /auto — Autopus-ADK

> 🐙 Autopus v0.4 | autopus-adk | full

At the start of every response, output the following banner:

```
🐙 Autopus ─────────────────────────
```

Then continue with the task output. End each subcommand response with `🐙`.

ARGUMENTS: $ARGUMENTS

## Context Load (on every session start)

Before processing any subcommand, load project context by reading these files if they exist:

1. `ARCHITECTURE.md` — architecture map (domains, layers, dependencies)
2. `.autopus/project/product.md` — project overview, core features
3. `.autopus/project/structure.md` — directory structure, package roles
4. `.autopus/project/tech.md` — tech stack, build, testing
5. `.autopus/project/scenarios.md` — E2E test scenarios (if exists)
6. `.autopus/context/signatures.md` — exported API signatures (if exists)

If none of these files exist, display:

```
No project context documents found. Run `/auto setup` to initialize.
```

## Subcommand Routing

**Strip global flags first** from $ARGUMENTS, then determine the subcommand from the first remaining word.

### Global Flags

| Flag | Description |
|------|-------------|
| `--think` | Activate Sequential Thinking MCP for deep analysis mode. Load the `mcp__sequential-thinking__sequentialthinking` tool via ToolSearch, then perform step-by-step reasoning. |
| `--ultrathink` | Same as `--think` but with higher reasoning effort. |

### Routing Rules

1. **Strip global flags**: Separate `--think`, `--ultrathink`, and other global flags from $ARGUMENTS
2. **Match subcommand**: Use the first remaining word to determine the subcommand
3. **Triage-based Smart Routing (natural language → difficulty-aware flow)**:
   - If the first word does not match a known subcommand and the remaining text has 2+ words:
   - Execute the **Triage Process** (see below) to assess difficulty and recommend a flow
   - Route to the recommended flow after user confirmation (or auto-proceed with `--auto`)
4. **Empty arguments**: Show the subcommand list

### Triage Process

WHEN a natural language request is detected (rule 3 above), THE SYSTEM SHALL assess task difficulty before routing.

#### Step 1: Signal Extraction

Analyze the request text for the following signals:

| Signal | LOW indicators | MEDIUM indicators | HIGH indicators |
|--------|---------------|-------------------|-----------------|
| **Change type** | fix, 수정, 고쳐, typo, rename, 오타, 삭제, 로그추가, config | 리팩토링, refactor, 개선, improve, 옵션추가, 확장, extend | 새로운, new, feature, 기능, 모듈, 시스템, 아키텍처, API설계 |
| **Scope** | specific file/function mentioned | package/module mentioned | "전체", "모든", multi-domain, cross-cutting |
| **Verb intensity** | change, update, move, remove | add, enhance, restructure | design, build, create, implement (large scope) |

#### Step 2: Impact Scan (fast, ≤ 5 seconds)

Run a quick codebase scan to validate the heuristic:

1. **Grep** for keywords from the request to estimate affected files
2. **Glob** for patterns matching the described scope
3. Count: affected files, affected packages

This step is OPTIONAL if the text-based signals are unambiguous. Skip if `--auto` flag is set.

#### Step 3: Difficulty Classification

| Difficulty | Criteria (ANY of) | Recommended Flow |
|-----------|-------------------|------------------|
| **LOW** | Single file change; bug fix with known location; config/doc edit; ≤ 1 package affected | `/auto fix "{desc}"` — direct fix, no SPEC |
| **MEDIUM** | 2-3 files; existing feature extension; single-package refactor; test additions | `/auto plan "{desc}" --skip-prd` — SPEC only, no PRD |
| **HIGH** | 3+ files across packages; new feature/module; API design; architecture change; multi-domain impact | `/auto plan "{desc}"` — full PRD + SPEC pipeline |
| **CRITICAL** | Security vulnerability; urgent production fix | `/auto fix "{desc}"` + recommend `/auto secure` after |

#### Step 4: Triage Display and Confirmation

WHEN `--auto` is NOT set:

1. Display the triage analysis as text:

```
🐙 Triage ───────────────────────────
  요청: "{natural language request}"

  분석:
  - 변경 유형: {bug fix | enhancement | new feature | refactor | config}
  - 예상 영향: ~{N}개 파일 / {N}개 패키지
  - 난이도: {LOW | MEDIUM | HIGH | CRITICAL}

  추천: {recommended flow description}
```

2. Use the `AskUserQuestion` tool to present the selection (do NOT use text-based numbered options):

```
AskUserQuestion(
  questions = [{
    question: "어떤 플로우로 진행할까요?",
    header: "Triage",
    multiSelect: false,
    options: [
      { label: "바로 수정 (Recommended)", description: "/auto fix — SPEC 없이 직접 수정" },
      { label: "SPEC만 생성", description: "/auto plan --skip-prd — PRD 생략, SPEC만 작성" },
      { label: "PRD + SPEC 전체", description: "/auto plan — PRD 작성 후 SPEC 생성" }
    ]
  }]
)
```

Adjust the recommended option (add "(Recommended)" suffix) based on the difficulty classification. Place the recommended option FIRST in the options list.

WHEN `--auto` IS set, display triage result and auto-proceed:

```
🐙 Triage ───────────────────────────
  요청: "{request}"
  난이도: {level} → {flow} (자동 진행)
```

#### Triage Override

The user can always force a specific flow by using the subcommand directly:
- `/auto fix "..."` — skip triage, go directly to fix
- `/auto plan "..."` — skip triage, go directly to plan
- `/auto plan "..." --skip-prd` — skip triage, plan without PRD

Triage ONLY activates for bare natural language input without a recognized subcommand.

### Subcommand List

| Subcommand | Description | Mode |
|-----------|-------------|------|
| setup | Generate/update project context documents | all |
| plan | Write a SPEC | all |
| go | Implement a SPEC | full |
| fix | Fix a bug | all |
| map | Analyze codebase structure | all |
| review | Code review | full |
| spec review | SPEC multi-provider review | full |
| secure | Security audit | full |
| stale | Detect stale decisions | all |
| sync | Synchronize documentation | all |
| why | Query decision rationale | all |
| init | Initialize harness | all |
| update | Update harness | all |
| doctor | Health diagnostics | all |
| platform | Platform management | all |

---

## setup — Generate/Update Project Context Documents

Analyze the project's architecture, structure, and tech stack to produce context documents.
Agents are re-initialized on every session, so these documents provide continuity across sessions.

### Pipeline

1. **Analyze codebase**: Use Explore agent to analyze the full project structure
2. **Generate/update ARCHITECTURE.md**: Run `auto arch generate` or have the agent analyze and write it directly
3. **Generate/update project docs**: Create 4 files under `.autopus/project/`
4. **Generate/update scenarios.md**: Analyze codebase to extract user-facing E2E test scenarios (SPEC-E2E-001 R1)

### Output Files

| File | Content |
|------|---------|
| `ARCHITECTURE.md` | Domains, layers, dependency map, rule violations |
| `.autopus/project/product.md` | Project name, description, core features, use cases, mode |
| `.autopus/project/structure.md` | Directory structure, package roles, entry points, file stats |
| `.autopus/project/tech.md` | Tech stack, build, testing, architecture patterns |
| `.autopus/project/scenarios.md` | User-facing E2E test scenarios extracted from the codebase |

### Scenario Generation (Step 4)

Analyze the project codebase to generate `.autopus/project/scenarios.md`.

#### Step 4.1: Detect Project Type

Scan the project root to determine the primary stack. Multiple types can coexist (e.g., Go CLI + API).

| Project Type | Detection Signals | Extraction Target |
|---|---|---|
| **Go CLI** | `go.mod` + Cobra imports (`github.com/spf13/cobra`) | Command tree: subcommands, flags, args |
| **Go API** | `go.mod` + HTTP handler patterns (`http.HandleFunc`, `gin.Engine`, `chi.Router`, `echo.New`) | Routes: method, path, handler name |
| **Python CLI** | `setup.py`/`pyproject.toml` + `argparse`/`click`/`typer` imports | CLI commands, arguments, options |
| **Python API** | `requirements.txt`/`pyproject.toml` + `flask`/`fastapi`/`django` imports | Routes: method, path, handler |
| **Node.js API** | `package.json` + `express`/`fastify`/`nestjs`/`hono` imports | Routes: method, path, handler |
| **Frontend** | `package.json` + `react`/`vue`/`svelte`/`next`/`nuxt` deps | Pages/routes, key user interactions |
| **Library** | No CLI/API/Frontend signals detected | Exported public API functions |

If no signals match, default to **Library** type with minimal scenarios.

#### Template Selection

| Project Type | Template |
|---|---|
| Go CLI, Python CLI, Node CLI | `templates/shared/scenarios-cli.md.tmpl` |
| Go API, Python API, Node.js API | `templates/shared/scenarios-api.md.tmpl` |
| Frontend | `templates/shared/scenarios-frontend.md.tmpl` |
| Library | `templates/shared/scenarios-library.md.tmpl` |

For mixed types (e.g., CLI + API), use the primary type's template and append scenarios from the secondary type below.

#### Step 4.2: Extract Scenarios

For each detected project type, the agent analyzes the codebase directly to extract user-facing scenarios.

**Go CLI** (has Go extractor: `pkg/e2e/extract_cobra.go` → `ExtractCobra()`):
- Parse Cobra command tree to extract subcommands, flags, and expected behavior
- Generate one scenario per subcommand (happy path + error case)

**All other types** (agent-driven extraction):
- Read entry point files (routes, handlers, pages, CLI definitions)
- For each entry point, generate a scenario with:
  - **Command/Action**: What the user does (CLI command, HTTP request, page navigation)
  - **Precondition**: Required state, env vars, auth tokens, seed data
  - **Expected result**: Status code, output, page state
  - **Verify**: Appropriate verification primitives

**Verification primitives by type:**

| Type | Primitives |
|---|---|
| CLI | `exit_code(N)`, `stdout_contains(str)`, `stderr_empty()`, `file_exists(path)`, `file_contains(path, str)` |
| API | `status_code(N)`, `json_path(path, value)`, `header_contains(key, str)`, `response_time_ms(N)` |
| Frontend | `page_title(str)`, `element_visible(selector)`, `element_text(selector, str)`, `url_matches(pattern)` |

#### Step 4.3: Render scenarios.md

Use the structured format:

```markdown
# E2E Scenarios — {ProjectName}

> Auto-generated by `/auto setup`. Updated by `/auto sync`.

## Project Type: {CLI | API | Frontend | Library | CLI+API | ...}
## Binary: {binary name or N/A}
## Build: {build command or N/A}

### S1: {id} — {description}
- **Command**: {command or action}
- **Precondition**: {required state}
- **Env**: {KEY=value or N/A}
- **Expect**: {expected result}
- **Verify**: {primitive1}, {primitive2}
- **Depends**: {S{N} or N/A}
- **Status**: active
```

Reference implementation: `pkg/setup/scenarios.go` → `generateScenarios()`, `pkg/e2e/scenario.go` → `RenderScenarios()`

### Execution

The agent explores and analyzes the codebase directly:

1. Read and analyze project manifest (`go.mod`, `package.json`, `pyproject.toml`, etc.) and source files
2. Detect project type(s) using Step 4.1 signals
3. Overwrite existing files to reflect the current code state
4. Auto-create `.autopus/project/` directory if it does not exist
5. Extract E2E scenarios per detected project type (Step 4.2) and write `scenarios.md` (Step 4.3)

Usage: `/auto setup`

### Completion Message

```
Project context documents have been generated/updated.
- ARCHITECTURE.md
- .autopus/project/product.md
- .autopus/project/structure.md
- .autopus/project/tech.md
- .autopus/project/scenarios.md (E2E test scenarios)
Context will be loaded automatically on the next session.
```

---

## plan — Write a SPEC

@.claude/skills/autopus/planning.md

Write a SPEC document. Delegate to the `spec-writer` agent to analyze the codebase and produce 4 SPEC files.

### Flags

| Flag | Description |
|------|-------------|
| `--multi` | Enable multi-provider review. Automatically runs multi-provider review after SPEC generation. |
| `--strategy` | Multi-provider strategy: consensus (default), debate, pipeline, fastest (requires `--multi`) |
| `--skip-prd` | Skip PRD phase and proceed directly to SPEC generation. |
| `--prd-mode` | PRD writing mode: `standard` (10 sections, default) or `minimal` (5 sections). |
| `--auto` | Autonomous mode: skip user confirmation gates including PRD review. |

Usage: `/auto plan "feature description"`, `/auto plan "feature description" --multi`, `/auto plan "feature description" --multi --strategy debate`, `/auto plan "feature description" --skip-prd`, `/auto plan "feature description" --prd-mode minimal`

### Pipeline (execute in this exact order)

#### [REQUIRED] Step 1: Parse Flags

Extract flags from $ARGUMENTS:
- Check for `--multi` → `MULTI_FLAG = true/false`
- Extract `--strategy <value>` → `STRATEGY = value` (default: consensus)
- Check for `--skip-prd` → `SKIP_PRD = true/false`
- Extract `--prd-mode <value>` → `PRD_MODE = value` (default: standard)
- Check for `--auto` → `AUTO_MODE = true/false`
- Remaining text → `FEATURE_DESC`

#### Step 1.5: PRD Generation

@.claude/skills/autopus/prd.md

WHEN `SKIP_PRD = false` (default), generate a PRD before SPEC creation.

##### Step 1.5.1: Generate SPEC-ID

Determine the SPEC-ID (DOMAIN + NUMBER) before PRD generation:
- Analyze the feature description to determine appropriate DOMAIN keyword
- Check existing `.autopus/specs/` to avoid NUMBER collision
- Create the directory: `.autopus/specs/SPEC-{DOMAIN}-{NUMBER}/`

##### Step 1.5.1.5: Multi-Provider Brainstorm (Conditional)

WHEN `MULTI_FLAG = true` AND `SKIP_PRD = false`:

Run multi-provider brainstorming via the Orchestra engine:

```bash
auto orchestra brainstorm "{FEATURE_DESC}" --strategy debate
```

Save the result to `.autopus/specs/SPEC-{DOMAIN}-{NUMBER}/brainstorm.md`.

WHEN `MULTI_FLAG = false` → [INTENDED SKIP: Step 1.5.1.5]

##### Step 1.5.2: Select Template

- If `PRD_MODE = standard` → reference `templates/shared/prd-standard.md.tmpl` (10 sections)
- If `PRD_MODE = minimal` → reference `templates/shared/prd-minimal.md.tmpl` (5 sections)

##### Step 1.5.3: Generate PRD

Generate the PRD by analyzing the feature description and codebase context:
- Scan relevant source files for existing patterns and constraints
- Fill in the template sections based on the analysis
- WHEN `brainstorm.md` exists: Read `.autopus/specs/SPEC-{DOMAIN}-{NUMBER}/brainstorm.md` and use top-ranked ideas as PRD input
- Save to `.autopus/specs/SPEC-{DOMAIN}-{NUMBER}/prd.md`

##### Step 1.5.4: Quality Validation (R10)

Validate the generated PRD:
- All required sections present (Standard: 10, Minimal: 5)
- Goals contain at least 1 measurable success metric
- Requirements contain at least 1 P0 priority item
- Out of Scope contains at least 1 item

If validation fails, display warnings but continue (do not block).

##### Step 1.5.5: User Confirmation Gate (R9)

WHEN `AUTO_MODE = false`:
1. Display PRD summary (section headings + key points)
2. Ask user: approve or request revision
3. If revision requested: accept revision text, regenerate PRD, re-validate (max 2 iterations)
4. If approved: proceed to Step 2

WHEN `AUTO_MODE = true`:
- Skip confirmation gate, proceed directly to Step 2

WHEN `SKIP_PRD = true`:
- Skip this entire step, proceed to Step 2 (no prd.md generated)

#### [REQUIRED] Step 2: Spawn spec-writer Agent

Spawn a `spec-writer` agent to generate the SPEC:

```
Agent(
  subagent_type = "spec-writer",
  prompt = """
    Feature description: {FEATURE_DESC}
    Project directory: {current directory}
    PRD file: .autopus/specs/SPEC-{DOMAIN}-{NUMBER}/prd.md  ← (if exists, otherwise omit this line)
    Brainstorm file: .autopus/specs/SPEC-{DOMAIN}-{NUMBER}/brainstorm.md  ← (if exists, use top-ranked ideas as additional context)
  """
)
```

When a PRD file exists, the spec-writer agent MUST read it first and use its Goals, Requirements, and Out of Scope sections as the authoritative top-level context for the generated SPEC.

Extract the **SPEC-ID** (e.g., SPEC-UX-001) from the agent's output.

> **⏭ POST-AGENT**: spec-writer returned. NEXT REQUIRED STEP: [CONDITIONAL] Step 3: Review Gate Decision. Do NOT skip to Completion.

#### [CONDITIONAL] Step 3: Review Gate Decision

```
┌─────────────────────────────────────────────┐
│ MULTI_FLAG == true                          │
│   OR autopus.yaml → review_gate.enabled?    │
├──────────┬──────────────────────────────────┤
│ TRUE     │ FALSE                            │
│ ↓        │ ↓                                │
│ Step 4   │ Step 5 [INTENDED SKIP: Step 4]   │
└──────────┴──────────────────────────────────┘
```

**How to check:**
- Run `cat autopus.yaml | grep -A2 "review_gate"` via Bash to check the enabled value
- Or Read `autopus.yaml` directly

#### [CONDITIONAL] Step 4: Run Multi-Provider Review

Run the `auto spec review` CLI command via the Bash tool:

```bash
auto spec review {SPEC-ID} --strategy {STRATEGY}
```

**Handling results:**
- **PASS** → Update Status in `.autopus/specs/{SPEC-ID}/spec.md` to `approved` (use Edit tool)
- **REVISE** → Print finding list, apply fixes, and re-run review (up to 2 iterations)
- **REJECT** → Print finding list and guide the user to redesign

**Fallback:**
- If `auto spec review` fails (provider not installed, etc.):
  - Print warning: "Multi-provider review could not be executed. Please check your provider configuration."
  - Keep Status as `draft`
  - Proceed to Step 5

#### [REQUIRED] Pre-Completion Verification

Before displaying completion output, verify ALL steps were evaluated:

- [ ] Step 1: Parse Flags — completed
- [ ] Step 2: Spawn spec-writer Agent — completed, SPEC-ID extracted
- [ ] Step 3: Review Gate Decision — evaluated (result: Step 4 or INTENDED SKIP)
- [ ] Step 4: Multi-Provider Review — completed OR [INTENDED SKIP]

IF any item is unchecked → return to that step. Do NOT display Completion Guidance.

#### [REQUIRED] Step 5: Completion Guidance

Guide the next step based on review outcome:

- If Status is `approved`:
  ```
  ✓ SPEC {SPEC-ID} approved
  Next step: /auto go {SPEC-ID}
  ```
- If Status is `draft` (review skipped or not run):
  ```
  SPEC {SPEC-ID} created (status: draft)
  Next step: /auto go {SPEC-ID}
  Run review: /auto spec review {SPEC-ID}
  ```

---

## go — Implement a SPEC

@.claude/skills/autopus/tdd.md
@.claude/skills/autopus/agent-pipeline.md
@.claude/skills/autopus/worktree-isolation.md

Implement code based on a SPEC document. Follows TDD methodology.

### Step 0: Parse Flags (REQUIRED — do this FIRST)

IMPORTANT: You MUST parse these flags from $ARGUMENTS before any other action.

Extract the following flags:
- `--team` → `TEAMS_MODE = true/false` (Agent Teams)
- `--solo` → `SOLO_MODE = true/false` (single session, no subagents)
- `--auto` → `AUTO_MODE = true/false`
- `--multi` → `MULTI_MODE = true/false`
- `--quality <value>` → `QUALITY = value` (ultra | balanced)
- `--strategy <value>` → `STRATEGY = value`
- `--continue` → `CONTINUE_MODE = true/false`
- `--loop` → `LOOP_MODE = true/false`
- `--skip-scaffold` → `SKIP_SCAFFOLD = true/false`
- Remaining text (non-flag word) → `SPEC_ID`

Determine execution mode:
- `TEAMS_MODE = true` → Agent Teams mode
- `SOLO_MODE = true` → Single session mode
- Otherwise → **Subagent pipeline** (default)

After parsing, display the detected configuration:

```
Mode: {subagent | teams | teams+multi | solo}
SPEC: {SPEC_ID}
Quality: {ultra | balanced | pending selection}
```

### Step 1: Load SPEC

Load `.autopus/specs/SPEC-{SPEC_ID}/spec.md` and check status.

**Draft Guard**: If status is "draft" and review_gate is enabled, refuse implementation and prompt:
```
SPEC {SPEC_ID} is in draft status. Run review first: /auto spec review {SPEC_ID}
```

### Step 2: Route to Pipeline

Route based on the execution mode determined in Step 0.

#### Route A: Subagent Pipeline (default, no --team, no --solo)

IMPORTANT: This is the DEFAULT route. It REQUIRES spawning subagents using the Agent tool. Each Phase below MUST use an Agent() call — do NOT implement these phases yourself in the main session.

@.claude/skills/autopus/worktree-isolation.md

For parallel tasks, include `isolation: "worktree"` in Agent() calls to enable worktree isolation.

#### Route B: Agent Teams (`--team`)

IMPORTANT: This route uses Claude Code Agent Teams (experimental). Requires `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`.

If `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` is not set, abort with:
```
Agent Teams requires CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1. Use default mode (subagent) instead.
```

Use TeamCreate to create a team, then spawn specialized teammates using `Agent(subagent_type=..., team_name=..., name=...)`. Each teammate loads its agent definition from `.claude/agents/autopus/`, inheriting tools, skills, model, and domain expertise. Teammates communicate directly via SendMessage.

Team composition — each member maps 1:1 to a specialized agent definition:

**Core members (always spawned):**
| Name | subagent_type | Agent Definition | Phase |
|------|---------------|------------------|-------|
| `lead` | `planner` | planner.md | Phase 1, coordination |
| `builder-1` | `executor` | executor.md | Phase 2 |
| `tester` | `tester` | tester.md | Phase 1.5, Phase 3 |
| `guardian` | `validator` | validator.md | Gate 2 |

**Phase-dependent members (spawned when needed):**
| Name | subagent_type | Agent Definition | Condition |
|------|---------------|------------------|-----------|
| `builder-2` | `executor` | executor.md | 2+ parallel tasks |
| `annotator` | `annotator` | annotator.md | Phase 2.5 |
| `auditor` | `security-auditor` | security-auditor.md | Phase 4 |
| `reviewer` | `reviewer` | reviewer.md | Phase 4 |

Quality Mode applies to model selection: Ultra → all `model: "opus"`, Balanced → omit (use frontmatter defaults). Adaptive Quality overrides builder model per task complexity (HIGH→opus, MEDIUM→sonnet, LOW→haiku).

Reference: `.claude/skills/autopus/agent-teams.md`

The pipeline phases are the same as Route A, but orchestrated through Agent Teams instead of individual Agent() calls. Worktree safety rules R1-R5 apply equally — each builder teammate works in an independent worktree. See @.claude/skills/autopus/worktree-isolation.md and `.claude/rules/autopus/worktree-safety.md`.

#### Route C: Single Session (`--solo`)

1. **TDD Implementation**: Execute RED → GREEN → REFACTOR based on tasks in plan.md
2. **Post-Implementation**: Validate against acceptance.md criteria and update status to done

#### Subagent Pipeline Phases (Route A)

```
Phase 1: Planning        → planner     (mode: plan)
Phase 1.5: Test Scaffold → tester      (mode: bypassPermissions) — skip if --skip-scaffold
Gate 1:  Approval        → skip if AUTO_MODE
Phase 2: Implementation  → executor×N  (mode: bypassPermissions, parallelizable)
Phase 2.1: Worktree Merge → main session (merge worktree branches into working branch)
Gate 2:  Validation      → validator   (mode: plan)  — retry up to 3× on FAIL
Phase 2.5: Annotation    → annotator   (mode: bypassPermissions) — @AX tags on modified files
Phase 3: Testing         → tester      (mode: bypassPermissions)
Gate 3:  Coverage        → verify 85%+ coverage
Phase 4: Review          → reviewer + security-auditor (parallel, mode: plan)  — retry up to 2× on REQUEST_CHANGES
```

### Retry Limits Reference

| Gate / Phase | Max Retries | With --loop | On Exceeded |
|-------------|-------------|-------------|-------------|
| Gate 1: Approval | skip if AUTO_MODE | skip if AUTO_MODE | N/A |
| Gate 2: Validation | 3 | 5 | Abort, guide: `--continue` |
| Gate 3: Coverage | re-spawn tester | 3 retries | Until 85%+ / circuit break |
| Phase 4: Review | 2 | 3 | Abort, request user intervention |

### RALF Loop Protocol

WHEN `LOOP_MODE = true`, the pipeline applies the RALF (RED-GREEN-REFACTOR-LOOP) cycle at each quality gate:

```
RALF Loop (per Phase):
  ┌─→ RED    : Phase 실행 (구현/테스트/리뷰)
  │   GREEN  : Gate 검증 (PASS/FAIL 판정)
  │   REFACTOR: FAIL 시 이슈 수정 에이전트 스폰
  └── LOOP   : Gate PASS까지 반복 (circuit breaker 적용)
```

#### Iteration Limits

| Gate / Phase | Without --loop | With --loop |
|-------------|---------------|-------------|
| Gate 2 (Validation) | 3 retries | 5 retries |
| Gate 3 (Coverage) | 1 retry | 3 retries |
| Phase 4 (Review) | 2 retries | 3 retries |

WHERE `LOOP_MODE = false`, existing retry limits apply with no behavioral change (R8 backward compatibility).

#### Circuit Breaker

WHILE `LOOP_MODE = true`, THE SYSTEM SHALL track consecutive no-progress iterations per Phase:
- Progress is measured by: issue count, coverage %, review issue count
- WHERE 3 consecutive iterations produce no new commits or file changes, THE SYSTEM SHALL break the loop
- On circuit break: log stuck state, display remaining issues, suggest recovery options

#### Progress Detection

WHEN each RALF iteration completes, THE SYSTEM SHALL compare current state against previous iteration:
- Issue count: decreased = progress
- Coverage %: increased = progress
- Review issues: decreased = progress
- WHERE no measurable improvement for 3 consecutive iterations → circuit breaker triggers

#### Escalation on Circuit Break

WHEN circuit breaker triggers, THE SYSTEM SHALL display:

```
🐙 RALF [CIRCUIT BREAK] ──────────────
  Phase: {phase} | Iteration: {N}/{max}
  Stuck: 3 consecutive iterations with no progress
  Remaining Issues:
  - {issue list}
  복구 옵션:
  1. /auto go {SPEC-ID} --continue  (수정 후 재개)
  2. /auto fix "{specific issue}"   (개별 이슈 수정)
```

#### Flag Combination Matrix

| Flags | Gate 1 (Approval) | Gates 2-4 |
|-------|-------------------|-----------|
| (none) | User approval | Retry with default limits, stop |
| --auto | Skip | Retry with default limits, stop |
| --loop | User approval | RALF loop with extended limits |
| --auto --loop | Skip | RALF loop with extended limits (완전 자율) |

#### RALF Iteration Display

WHEN `LOOP_MODE = true` AND a retry iteration occurs, THE SYSTEM SHALL display:

```
🐙 RALF [{phase}] ──────────────────
  Iteration: {N}/{max} | Issues: {before} → {after}
  Progress: {description}
  Status: {RETRY | PASS | CIRCUIT_BREAK}
```

**Step 2.1: Determine Quality Mode**

Priority order:
1. If `QUALITY` is set → use it. If invalid value, print error and stop.
2. If `AUTO_MODE = true` and `QUALITY` is not set → auto-select "balanced"
3. Otherwise → use the `AskUserQuestion` tool for interactive selection (do NOT use text-based numbered options):

```
AskUserQuestion(
  questions = [{
    question: "품질 모드를 선택하세요.",
    header: "Quality",
    multiSelect: false,
    options: [
      {
        label: "Balanced (Recommended)",
        description: "핵심 분석은 Opus, 구현은 Sonnet, 검증은 Haiku. Adaptive Quality 적용. (~$0.45/100K tokens)",
        preview: "planner=opus\nexecutor=sonnet (adaptive)\nvalidator=haiku\ntester=sonnet\nreviewer=sonnet"
      },
      {
        label: "Ultra",
        description: "모든 에이전트를 Opus로 실행. 최고 품질. (~$2.25/100K tokens)",
        preview: "planner=opus\nexecutor=opus\nvalidator=opus\ntester=opus\nreviewer=opus"
      }
    ]
  }]
)
```

When telemetry is enabled (`autopus.yaml → telemetry.enabled: true`), run `auto telemetry cost` output via Bash to get actual historical cost data. If previous runs exist, display historical average instead of the static table above.

Quality mode determines the `model` parameter in Agent() calls:
- **Ultra**: add `model: "opus"` to ALL Agent() calls
- **Balanced**: omit `model` parameter (use each agent's default)

Display after determination:
```
품질 모드: {Ultra | Balanced} — {description}
```

**Step 2.1.1: Initialize Telemetry (R8)**

WHEN telemetry is enabled (`autopus.yaml → telemetry.enabled: true`), THE SYSTEM SHALL record telemetry at each pipeline phase boundary. Run via Bash before Phase 1:

```bash
auto telemetry record --spec-id {SPEC_ID} --action start --quality-mode {QUALITY_MODE}
```

WHEN a phase starts, record:
```bash
auto telemetry record --spec-id {SPEC_ID} --action start --phase "{PHASE_NAME}"
```

WHEN an agent completes, record:
```bash
auto telemetry record --spec-id {SPEC_ID} --action agent --agent "{AGENT_NAME}" --phase "{PHASE_NAME}" --status {PASS|FAIL} --files {N} --tokens {estimated}
```

Token estimation: count the agent's output text length, divide by 4, multiply by 4 (input:output = 3:1).

WHEN a phase ends, record:
```bash
auto telemetry record --spec-id {SPEC_ID} --action end --phase "{PHASE_NAME}" --status {PASS|FAIL}
```

If telemetry is disabled or `auto telemetry record` fails, silently skip — telemetry must never block the pipeline.

**[REQUIRED] Phase 1 — Planning (MUST call Agent tool)**

```
Agent(
  subagent_type = "planner",
  model = "opus",          ← Ultra only; omit for Balanced
  mode = "plan",
  prompt = """
    SPEC file: .autopus/specs/SPEC-{SPEC_ID}/spec.md
    Plan file: .autopus/specs/SPEC-{SPEC_ID}/plan.md

    Decompose the SPEC into implementation tasks.
    Return an agent assignment table:
    | Task ID | Description | Agent    | Mode       | File Ownership  |
    |---------|-------------|----------|------------|-----------------|
    | T1      | ...         | executor | parallel   | pkg/foo/*.go    |
    | T2      | ...         | executor | sequential | pkg/foo/*.go    |
  """
)
```

Display progress after completion:
```
🐙 Pipeline ─────────────────────────
  ✓ Phase 1: Planning
  → Phase 2: Implementation [0/N tasks]
  ○ Phase 3: Testing
  ○ Phase 4: Review
```

> **⏭ POST-AGENT**: planner returned. NEXT REQUIRED STEP: Phase 1.5: Test Scaffold. Do NOT skip to Completion.

**[REQUIRED] Phase 1.5 — Test Scaffold (MUST call Agent tool)**

WHEN `SKIP_SCAFFOLD = false`:
```
Agent(
  subagent_type = "tester",
  mode = "bypassPermissions",
  prompt = """
    Phase: Test Scaffold (Phase 1.5)
    SPEC file: .autopus/specs/SPEC-{SPEC_ID}/spec.md
    Create failing test skeletons for P0/P1 requirements.
    All tests MUST FAIL. Report any passing tests.
  """
)
```

WHEN `SKIP_SCAFFOLD = true` → [INTENDED SKIP: Phase 1.5]

> **⏭ POST-AGENT**: tester (scaffold) returned. NEXT REQUIRED STEP: Gate 1: Approval. Do NOT skip to Completion.

**[GATE] Gate 1: Approval** — If `AUTO_MODE = false`, show the planner's assignment table and ask user to approve before proceeding. If `AUTO_MODE = true` → [INTENDED SKIP: Gate 1]

**[REQUIRED] Phase 2 — Implementation (MUST call Agent tool)**

For each task in the planner's assignment table, spawn executor agents:

**GC Suppression**: During parallel worktree execution, all git commands MUST use `git -c gc.auto=0` to prevent garbage collection interference. See `.claude/rules/autopus/worktree-safety.md`.

Parallel tasks (Mode = "parallel" AND no file ownership conflict):
```
# Call MULTIPLE Agent() in a SINGLE message for parallel execution
# Each parallel executor runs in an isolated git worktree (R1)
# Use git -c gc.auto=0 for all git commands during parallel execution (R5)
Agent(
  subagent_type = "executor",
  model = "opus",          ← Ultra only; omit for Balanced
  mode = "bypassPermissions",
  isolation = "worktree",  ← R1: worktree isolation for parallel tasks
  prompt = "Implement T1: {task description}. Files: {file ownership}. SPEC requirements: {relevant requirements}"
)
Agent(
  subagent_type = "executor",
  model = "opus",          ← Ultra only; omit for Balanced
  mode = "bypassPermissions",
  isolation = "worktree",  ← R1: worktree isolation for parallel tasks
  prompt = "Implement T2: {task description}. Files: {file ownership}. SPEC requirements: {relevant requirements}"
)
```

Collect `worktree_path` and `branch` from each agent return value for Step 2.3.1 merge.

Max concurrent worktrees: **5**. Queue overflow tasks and spawn as slots free.

Sequential tasks (Mode = "sequential" OR file ownership conflict) do NOT use worktree isolation:
```
# Call Agent() one at a time, passing previous result to next
result_t1 = Agent(subagent_type = "executor", prompt = "Implement T1: ...")
# If T1 used a worktree branch, merge it immediately before spawning T2 (R3):
#   git -c gc.auto=0 merge <branch_T1> && git worktree remove <path_T1>
Agent(subagent_type = "executor", prompt = "Implement T2 (depends on T1). T1 result: {result_t1}. ...")
```

> **⏭ POST-AGENT**: executor(s) returned. NEXT REQUIRED STEP: [REQUIRED] Phase 2.1: Worktree Merge. Do NOT skip to Completion.

**[REQUIRED] Phase 2.1 — Worktree Merge (parallel tasks only)**

WHEN all parallel executors complete, merge their worktree branches into the working branch before proceeding to Gate 2 (R3).

Merge in ascending task-ID order:
```bash
# Phase 2.1: batch merge — repeat for each parallel task branch in task-ID order
git -c gc.auto=0 merge <branch_T1>   # R5: gc.auto=0 suppresses auto GC
git worktree remove <path_T1>
git -c gc.auto=0 merge <branch_T2>
git worktree remove <path_T2>
# ... continue for all collected parallel branches
```

On merge conflict (R4):
1. `git merge --abort` — restore clean state
2. Log conflicting files and task IDs
3. Abort pipeline — do NOT auto-resolve even in `AUTO_MODE`:
   ```
   ✗ [MERGE ERROR] T{id} merge failed: ownership validation gap detected. Files: {list}
     복구 옵션:
     1. Resolve conflicts manually and re-run: /auto go {SPEC-ID} --continue
     2. Review file ownership with planner: /auto plan {SPEC-ID}
   ```

On lock error during merge (R5 — shared resource lock retry):
- Retry with exponential backoff: 3s → 6s → 12s (max 3 retries, 4 total attempts)
- On final failure: abort affected agent, log error, continue other merges

**Worktree merge failure recovery:**
```
✗ Worktree 머지 실패: {file list}
  원인: 파일 소유권 검증 갭 (ownership validation gap)
  복구 옵션:
  1. /auto go {SPEC-ID} --continue  (수정 후 재개)
  2. git worktree list              (워크트리 상태 확인)
  3. git worktree remove --force    (수동 정리)
```

Display merge progress after each branch:
```
🐙 worktree merge ───────────────────
  ✓ T{id}: merged {branch} → working branch
  워크트리 정리: {path} 제거됨
```

**[CHECKPOINT] Phase 2.1 Complete** — All worktree branches merged. Proceed to Gate 2.

After each executor, display:
```
🐙 executor ─────────────────────────
  파일: {N}개 수정 | 테스트: {N}개 추가 | 커버리지: {N}%
  다음: Phase 2.1 Worktree Merge (병합 필수)
```

**[GATE] Gate 2 — Validation (MUST call Agent tool)**

```
Agent(
  subagent_type = "validator",
  mode = "plan",
  prompt = """
    Validate the implementation. Check:
    - go build ./... passes
    - go vet ./... passes
    - golangci-lint run passes
    - No file exceeds 300 lines

    Return format:
    Verdict: PASS | FAIL
    Issues: <list>
    Recommended Agent: executor | tester | planner
  """
)
```

- **PASS** → proceed to Phase 3
- **FAIL** → spawn the Recommended Agent to fix issues, then re-validate (up to 3 retries)

WHEN `LOOP_MODE = true`: extend retry limit to 5. Display RALF iteration status after each retry. Apply circuit breaker if 3 consecutive no-progress iterations.

**Validation failure recovery (Gate 2):**
```
✗ Validation 실패: {N}개 이슈 발견
  복구 옵션:
  1. /auto go {SPEC-ID} --continue  (중단점에서 재개)
  2. /auto fix "{specific issue}"   (개별 이슈 수정)
```

After validation, display:
```
🐙 validator ────────────────────────
  Verdict: {PASS|FAIL} | 검사 항목: {N}개 | 이슈: {N}개
  다음: {pass → 테스트 단계 | fail → 수정 후 재검증}
```

> **⏭ POST-AGENT**: validator returned. NEXT REQUIRED STEP: Phase 2.5: Annotation. Do NOT skip to Completion.

**[REQUIRED] Phase 2.5 — Annotation (MUST call Agent tool)**

```
Agent(
  subagent_type = "annotator",
  mode = "bypassPermissions",
  prompt = """
    Apply @AX tags to all files modified during Phase 2.
    Follow ax-annotation skill rules.
    Validate per-file limits: ANCHOR ≤ 3, WARN ≤ 5.
  """
)
```

> **⏭ POST-AGENT**: annotator returned. NEXT REQUIRED STEP: Phase 3: Testing. Do NOT skip to Completion.

**[REQUIRED] Phase 3 — Testing (MUST call Agent tool)**

```
Agent(
  subagent_type = "tester",
  mode = "bypassPermissions",
  prompt = """
    Raise test coverage to 85%+.
    Add missing edge case tests.
    Run: go test -race -cover ./...
    Ensure all tests pass.
  """
)
```

After tester, display:
```
🐙 tester ───────────────────────────
  테스트: {N}개 추가 | 커버리지: {before}% → {after}% | 통과: {N}/{N}
  다음: 리뷰 단계로 진행
```

> **⏭ POST-AGENT**: tester returned. NEXT REQUIRED STEP: Gate 3: Coverage Check. Do NOT skip to Completion.

**[GATE] Gate 3: Coverage** — verify 85%+ coverage from tester output. If below, re-spawn tester. For harness-only tasks → [INTENDED SKIP: Gate 3]

WHEN `LOOP_MODE = true`: extend coverage retry limit to 3 (default: 1). Display RALF iteration status after each retry.

**[REQUIRED] Phase 4 — Review (MUST call Agent tool)**

Run reviewer and security-auditor in **parallel**:

```
# Call BOTH in a SINGLE message for parallel execution
Agent(
  subagent_type = "reviewer",
  mode = "plan",
  prompt = """
    Review all changes using TRUST 5 criteria.
    Return format:
    Verdict: APPROVE | REQUEST_CHANGES
    Issues: <list with file:line references>
  """
)
Agent(
  subagent_type = "security-auditor",
  mode = "plan",
  prompt = """
    Audit all changed files for security vulnerabilities.
    Return format:
    Verdict: PASS | FAIL
    Issues: <list with file:line references and severity>
  """
)
```

Both must return APPROVE/PASS for the gate to pass. If results conflict, Lead consolidates issue lists with priority: security issues > code quality issues. Consolidated issue list sent to executor for remediation.

- **Both APPROVE/PASS** → pipeline complete
- **Either REQUEST_CHANGES/FAIL** → spawn executor to fix consolidated issues, then re-review (up to 2 retries)

WHEN `LOOP_MODE = true`: extend review retry limit to 3. Display RALF iteration status after each retry. Apply circuit breaker if 3 consecutive identical issues.

**Review failure recovery (retries exceeded):**
```
✗ Review 실패: {N}개 변경 요청 (재시도 초과)
  복구 옵션:
  1. /auto go {SPEC-ID} --continue  (수정 후 재개)
  2. /auto review                   (변경사항 직접 검토)
```

After reviewer, display:
```
🐙 reviewer ─────────────────────────
  Verdict: {APPROVE|REQUEST_CHANGES} | Issues: {N}개 | 심각도: {low|medium|high}
  다음: {verdict-based guidance}
```

> **⏭ POST-AGENT**: reviewer returned. NEXT REQUIRED STEP: Gate 4: Review Result Handling. Do NOT skip to Completion.

**[GATE] Gate 4: Review Result Handling**

- **APPROVE** → pipeline complete, proceed to Pre-Completion Verification
- **REQUEST_CHANGES** → spawn executor to fix issues, then re-review (max 2 retries, see **Retry Limits Reference**)

**Review failure recovery (retries exceeded):**
```
✗ Review 실패: {N}개 변경 요청 (재시도 초과)
  복구 옵션:
  1. /auto go {SPEC-ID} --continue  (수정 후 재개)
  2. /auto review                   (변경사항 직접 검토)
```

**Parallel execution rules:**
- Only parallelize tasks whose Mode is "parallel" in the planner's assignment table
- File ownership conflict (same path pattern) → force sequential
- Pass SPEC requirements, task description, and relevant file context to each subagent

**Retry limits:** See **Retry Limits Reference** table above.

**Fallback:** If a subagent fails 2 consecutive times, handle in the main session directly.

**Subagent failure recovery:**
```
✗ {agent-name} 실패: {error description}
  복구 옵션:
  1. /auto go {SPEC-ID} --continue  (파이프라인 재개)
  2. {manual fallback instruction}  (수동 처리)
```

### Pipeline (--multi mode: multi-provider)

When `MULTI_MODE = true`, activate multi-provider orchestration:

- Use the `auto orchestra review` engine to review with multiple AI providers (claude, gemini, codex)
- Validate with multi-provider consensus at the review gate
- Can be combined with `--strategy`: consensus (default), debate, pipeline, fastest

`--team --multi` combination: Agent Teams + multi-provider review activated in the Review Phase.

Usage: `/auto go SPEC-ID` (subagent pipeline), `/auto go SPEC-ID --team` (Agent Teams), `/auto go SPEC-ID --solo` (single session), `/auto go SPEC-ID --quality ultra`, `/auto go SPEC-ID --auto --loop`

### Flags

| Flag | Description |
|------|-------------|
| `--auto` | Autonomous mode: run the full pipeline without user confirmation |
| `--loop` | RALF loop mode: auto-retry failed gates with extended iteration limits until PASS or circuit break |
| `--continue` | Resume an interrupted implementation from a previous session |
| `--team` | Agent Teams mode: use Claude Code Agent Teams (experimental) for multi-agent collaboration |
| `--solo` | Single session mode: no subagents, main session implements directly |
| `--multi` | Multi-provider mode: activate orchestra engine-based review |
| `--strategy` | Multi-provider strategy: consensus, debate, pipeline, fastest (requires --multi) |
| `--quality` | Quality mode: ultra (all agents Opus), balanced (mixed) |
| `--skip-scaffold` | Skip Phase 1.5 Test Scaffold |

### Harness-Only Tasks

If all tasks modify only `.md` files:
- Skip Go build/test validation
- Validator performs format-only checks (frontmatter validity, 300-line limit)

### [REQUIRED] Pre-Completion Verification

Before displaying completion output, verify ALL phases were evaluated:

- [ ] Phase 1: Planning — completed
- [ ] Phase 1.5: Test Scaffold — completed OR skipped (--skip-scaffold)
- [ ] Gate 1: Approval — completed OR skipped (AUTO_MODE)
- [ ] Phase 2: Implementation — all tasks completed
- [ ] Phase 2.1: Worktree Merge — completed (if parallel tasks existed) OR N/A
- [ ] Gate 2: Validation — PASS
- [ ] Phase 2.5: Annotation — completed
- [ ] Phase 3: Testing — completed (or skipped for harness-only)
- [ ] Gate 3: Coverage — 85%+ verified (or N/A for harness-only)
- [ ] Phase 4: Review — APPROVE

IF any item is unchecked → return to that phase/gate. Do NOT display Completion Guidance.

### Completion Guidance

After go completes:

1. **Record pipeline end** (R8): `auto telemetry record --spec-id {SPEC_ID} --action end --status PASS`
2. **Show cost** (R6): When telemetry is enabled, run `auto telemetry cost --spec-id {SPEC_ID}` via Bash and include the one-line cost summary in the completion output:
   ```
   소요: {duration} | 추정 비용: ${amount} ({Quality Mode})
   ```
3. **Guide next step**:
   ```
   Next step: /auto sync {SPEC-ID}
   ```


---

## fix — Fix a Bug

@.claude/skills/autopus/debugging.md

Reproduce the bug and fix it with minimal changes. Uses a Reproduction-First approach.

Usage: `/auto fix "bug description"`, `/auto fix --file path/to/file.go`

---

## map — Analyze Codebase Structure

Analyze and visualize the codebase structure, dependencies, and key patterns.

Analysis includes:
- Directory structure and package hierarchy
- Core entry points (main, handlers)
- Dependency flow (inter-module imports)
- @AX ANCHOR points (high fan-in functions)
- Complexity hotspots

Usage: `/auto map`, `/auto map path/to/package`

---

## review — Code Review

@.claude/skills/autopus/review.md

Review changed code against TRUST 5 criteria (Tested, Readable, Unified, Secured, Trackable).

Usage: `/auto review`, `/auto review HEAD~3..HEAD`, `/auto review --pr 123`


---

## spec review — SPEC Multi-Provider Review

@.claude/skills/autopus/spec-review.md

Review a SPEC document using multiple providers (claude, gemini, etc.) to validate quality. Delivers a PASS/REVISE/REJECT verdict via the Orchestra engine.

### Pipeline

1. **Load SPEC**: Load `.autopus/specs/SPEC-{ID}/spec.md` and extract requirements
2. **Collect code context**: If `auto_collect_context: true`, gather relevant source code
3. **Run multi-provider review**: Execute independent reviews with configured providers
4. **Merge verdicts**: Determine final verdict according to strategy (debate, consensus)
5. **Save results**: Write `review.md` to the SPEC directory

### Verdict Criteria

- **PASS**: SPEC approved, update status to "approved"
- **REVISE**: Revision needed, return with finding list
- **REJECT**: Fundamental redesign required

Usage: `/auto spec review SPEC-ID`, `/auto spec review SPEC-ID --strategy debate --timeout 180`


---

## secure — Security Audit

@.claude/skills/autopus/security-audit.md

Detect security vulnerabilities in the codebase and suggest fixes. Includes OWASP Top 10 coverage.

Usage: `/auto secure`, `/auto secure --owasp`, `/auto secure --secrets`


---

## stale — Detect Stale Decisions

Detect outdated decisions, expired TODOs, and obsolete architecture patterns.

Detects:
- Lore entries older than 90 days
- Stale @AX:TODO tags
- Unused interfaces and redundant abstractions

Usage: `/auto stale`, `/auto stale --lore`, `/auto stale --ax`

---

## sync — Synchronize Documentation

Synchronize code and documentation after implementation is complete. Updates SPEC status and related documents.

Sync targets:
- Update SPEC status (completed)
- Update project context documents (ARCHITECTURE.md + `.autopus/project/*`)
- Update API documentation
- Manage @AX tag lifecycle (CYCLE tracking, TODO escalation, ANCHOR verification)
- Capture Lore records
- Update CHANGELOG.md

### Project Document Update

On sync, detect structural changes in the codebase and update:
- `ARCHITECTURE.md` — reflect new packages/dependencies
- `.autopus/project/structure.md` — reflect directory/file changes
- `.autopus/project/tech.md` — reflect dependency/build changes
- `.autopus/project/product.md` — reflect new features/commands

Usage: `/auto sync SPEC-ID`, `/auto sync`

### @AX Lifecycle Management

On sync, manage @AX tag lifecycle across the codebase:

#### TODO Cycle Tracking
1. Find all `@AX:TODO` tags in source files
2. Increment `@AX:CYCLE:N` counter (or add `@AX:CYCLE:1` if absent)
3. Escalate to `@AX:WARN` if CYCLE >= 3, adding `@AX:REASON: escalated from TODO after N cycles`

#### ANCHOR Fan-In Verification
1. Find all `@AX:ANCHOR` tags
2. Grep for function name references to count callers
3. If caller count < 3, downgrade to `@AX:NOTE` with updated reason

#### NOTE Cleanup
1. Find all `@AX:NOTE` tags referencing specific functions
2. Verify the referenced function still exists in the codebase
3. Remove orphaned NOTE tags

### Lore Commit

After all sync targets are complete, commit the changed files in Lore format.

**Determining files to commit:**
1. Check changed/added files with `git status`
2. Stage SPEC-related files (`.autopus/specs/`) and project docs (`ARCHITECTURE.md`, `.autopus/project/*`)
3. If implementation code changes are included, stage those files as well

**Commit message templates:**

With SPEC-ID:
```
docs(<scope>): sync SPEC-{ID} 문서 및 상태 갱신

SPEC 구현 완료에 따른 문서 동기화.
상태를 completed로 갱신하고 프로젝트 컨텍스트 문서를 업데이트.

Why: SPEC 구현 완료 후 코드와 문서의 일관성 유지
Decision: sync 워크플로우에서 커밋까지 자동 수행
Ref: SPEC-{ID}

🐙 Autopus <noreply@autopus.co>
```

Without SPEC-ID:
```
docs(sync): 프로젝트 문서 동기화

코드베이스 변경사항을 프로젝트 컨텍스트 문서에 반영.

Why: 문서와 코드의 일관성 유지
Decision: 변경 감지된 문서만 선별 갱신

🐙 Autopus <noreply@autopus.co>
```

**Commit type rules:**
- If implementation code (`.go`, etc.) changes are included: use the primary type from the SPEC (feat, fix, refactor, etc.)
- If only docs/SPEC files changed: use `docs` type

**Skip commit when:**
- No changes reported by `git status` — skip the commit and print a notice only

### Completion Guidance

After sync completes, display the workflow lifecycle bar:

```
🐙 Workflow: {SPEC-ID}
  ✓ plan  →  ✓ go  →  ● sync
```

Then display the milestone celebration ResultBox for a completed full cycle:

```
╭────────────────────────────────────╮
│ 🐙 파이프라인 완료!                 │
│ {SPEC-ID}: {title}                 │
│ 태스크: N/N | 커버리지: N%          │
│ 리뷰: {verdict}                    │
╰────────────────────────────────────╯
```

**Field mappings:**
- `{SPEC-ID}` — e.g., `SPEC-UX-001`
- `{title}` — SPEC title from spec.md
- `N/N` — completed tasks / total tasks from plan.md
- `N%` — final test coverage (use `N/A` if not applicable or harness-only)
- `{verdict}` — reviewer verdict (`APPROVE` or `N/A` if review was skipped)

Then show state-aware next step guidance:

```
다음 단계: 다음 기능을 시작하려면 /auto plan "기능 설명" 을 실행하세요.
```

---

## why — Query Decision Rationale

Look up the rationale behind code or architecture decisions.

Query methods:
- Keyword search across Lore entries
- Extract decision context from commit history
- Look up NOTE/ANCHOR in @AX tags
- Link to related SPEC documents

Usage: `/auto why "why was this pattern used?"`, `/auto why path/to/file.go`

---

## ADK Management — init, update, doctor, platform

Manage the Autopus-ADK harness itself.

### Current Configuration

- **Project**: autopus-adk
- **Mode**: full
- **Platform**: claude-code

### Usage

- `/auto init` — initialize harness
- `/auto update` — update harness files
- `/auto doctor` — run health diagnostics
- `/auto platform list` — list platforms
- `/auto platform add <name>` — add a platform
- `/auto platform remove <name>` — remove a platform
