<div align="center">

# 🐙 Autopus-ADK

### The Operating System for AI-Powered Development

**Your AI agents don't just autocomplete — they plan, debate, implement, test, review, and ship.**

[![GitHub Stars](https://img.shields.io/github/stars/Insajin/autopus-adk?style=social)](https://github.com/Insajin/autopus-adk/stargazers)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![Platforms](https://img.shields.io/badge/Platforms-5-orange)](#-one-config-five-platforms)
[![Agents](https://img.shields.io/badge/Agents-15-blueviolet)](#-15-specialized-agents)
[![Skills](https://img.shields.io/badge/Skills-35-ff69b4)](#-all-commands)

[Quick Start](#-30-second-install) · [Why Autopus](#-the-problem) · [Features](#-what-makes-autopus-different) · [Pipeline](#-the-pipeline) · [Docs](#-all-commands)

[🇰🇷 한국어](docs/README.ko.md)

</div>

---

<!-- TODO: Replace with actual demo GIF when available -->
<!-- <p align="center"><img src="docs/assets/demo.gif" width="720" alt="Autopus pipeline demo" /></p> -->

## 🎬 See It In Action

```bash
# You describe what you want.
/auto plan "Add OAuth2 with Google and GitHub providers"

# 15 agents handle the rest — planning, testing, implementing, reviewing.
/auto go SPEC-AUTH-001 --auto --loop
```

```
🐙 Pipeline ─────────────────────────────────────────────
  ✓ Phase 1:   Planning         planner decomposed 5 tasks
  ✓ Phase 1.5: Test Scaffold    12 failing tests created (RED)
  ✓ Phase 2:   Implementation   3 executors in parallel worktrees
  ✓ Phase 2.5: Annotation       @AX tags applied to 8 files
  ✓ Phase 3:   Testing          coverage: 62% → 91%
  ✓ Phase 4:   Review           TRUST 5: APPROVE | Security: PASS
  ───────────────────────────────────────────────────────
  ✅ 5/5 tasks │ 91% coverage │ 0 security issues │ 4m 32s
```

> 💡 One slash command. Production-ready code with tests, security audit, documentation, and decision history.

---

## 😤 The Problem

You're using AI coding tools. They're powerful. But...

- 🔄 **Platform lock-in** — Switch from Claude to Codex? Rewrite all your rules and prompts from scratch.
- 🎲 **Hope-driven development** — "Add auth" → AI writes code, skips tests, ignores security, forgets docs. *Maybe* it works.
- 🧠 **Amnesia** — Next session, the AI forgets every decision. "Why did we use this pattern?" → silence.
- 👤 **Solo agent** — One model, one context, one shot. Multi-file refactoring? Good luck.

---

## 🔥 What Makes Autopus Different

### 🤖 AI Agents That Form a Team, Not a Chatbot

Autopus doesn't give you one AI assistant — it gives you a **software engineering team of 15 specialized agents** with defined roles, quality gates, and retry logic.

```
🧠 Planner        →  Decomposes requirements into tasks
⚡ Executor ×N    →  Implements code in parallel worktrees
🧪 Tester         →  Writes tests BEFORE code (TDD enforced)
✅ Validator       →  Checks build, lint, vet
🔍 Reviewer       →  TRUST 5 code review
🛡️ Security       →  OWASP Top 10 audit
📝 Annotator      →  Documents code with @AX tags
🏗️ Architect      →  System design decisions
... and 7 more
```

### ⚔️ AI Models That Debate Each Other

Not one model reviewing your code — **multiple models arguing about it.**

```bash
auto orchestra review --strategy debate
```

Claude, Codex, and Gemini independently review your code, then **debate each other's findings** in a structured 2-phase argument. A judge renders the final verdict.

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Claude     │    │   Codex     │    │   Gemini    │
│  🔍 Review   │    │  🔍 Review   │    │  🔍 Review   │
└──────┬──────┘    └──────┬──────┘    └──────┬──────┘
       │                  │                  │
       └──────────┬───────┴──────────────────┘
                  ▼
         ⚔️  Debate Phase
         (rebuttals & counter-arguments)
                  │
                  ▼
          🏛️  Judge Verdict
```

4 strategies: **Consensus** · **Debate** · **Pipeline** · **Fastest**

### 🔁 Self-Healing Pipeline (RALF Loop)

Quality gates don't just fail — they **fix themselves and retry.**

```bash
/auto go SPEC-AUTH-001 --auto --loop
```

```
🐙 RALF [Gate 2] ──────────────────
  Iteration: 1/5 │ Issues: 3
  → spawning executor to fix golangci-lint warnings...

🐙 RALF [Gate 2] ──────────────────
  Iteration: 2/5 │ Issues: 3 → 0
  Status: PASS ✅
```

**RALF = RED → GREEN → REFACTOR → LOOP** — TDD principles applied to the pipeline itself. Built-in circuit breaker prevents infinite loops.

### 🌳 Parallel Agents in Isolated Worktrees

Multiple executors work **simultaneously** — each in its own git worktree. No conflicts. No corruption.

```
Phase 2: Implementation
  ├── ⚡ Executor 1 (worktree/T1) → pkg/auth/provider.go     ✓
  ├── ⚡ Executor 2 (worktree/T2) → pkg/auth/handler.go      ✓
  └── ⚡ Executor 3 (worktree/T3) → pkg/auth/middleware.go    ✓

Phase 2.1: Merge (task-ID order)
  ✓ T1 merged → T2 merged → T3 merged → working branch
```

File ownership prevents conflicts. GC suppression prevents corruption. Up to **5 concurrent worktrees.**

### 📜 Lore: Your Codebase Never Forgets

Every commit captures the **why**, not just the what. Queryable forever.

```
feat(auth): add OAuth2 provider abstraction

Why: Need Google + GitHub support, extensible for future providers
Decision: Interface-based abstraction over direct SDK usage
Alternatives: Direct SDK calls (rejected: too coupled)
Ref: SPEC-AUTH-001

🐙 Autopus <noreply@autopus.co>
```

9 structured trailers. Query with `auto lore query "why interface?"`. Stale decisions auto-detected after 90 days.

### 🌐 One Config, Five Platforms

```bash
auto init   # auto-detects all installed AI coding CLIs
```

One `autopus.yaml` generates **native configuration** for every detected platform.

| Platform | What Gets Generated |
|----------|-------------------|
| **Claude Code** | `.claude/rules/`, `.claude/skills/`, `.claude/agents/`, `CLAUDE.md` |
| **Codex** | `.codex/`, `AGENTS.md` |
| **Gemini CLI** | `.gemini/`, `GEMINI.md` |
| **Cursor** | `.cursor/rules/`, `.cursorrules` |
| **OpenCode** | `.opencode/`, `agents.json` |

Same 15 agents. Same 35 skills. Same rules. **Everywhere.**

---

## 📦 30-Second Install

```bash
git clone https://github.com/Insajin/autopus-adk.git
cd autopus-adk && make build && make install
auto --version
```

Then, in any project:

```bash
auto init       # Detect platforms, generate harness
auto setup      # Generate project context docs
```

---

## 🤖 The Pipeline

### 7-Phase Multi-Agent Pipeline

Every `/auto go` runs this:

```
Phase 1    │ 🧠 Planner         │ SPEC → task decomposition + agent assignment
Phase 1.5  │ 🧪 Tester          │ Scaffold failing tests (RED)
Phase 2    │ ⚡ Executor ×N      │ TDD implementation (parallel worktrees)
Phase 2.5  │ 📝 Annotator       │ @AX tag lifecycle management
Gate  2    │ ✅ Validator        │ Build + lint + vet + file size
Phase 3    │ 🧪 Tester          │ Coverage boost → 85%+
Phase 4    │ 🔍 Reviewer + 🛡️    │ TRUST 5 review + OWASP security audit
```

### 15 Specialized Agents

| Agent | Role | When |
|-------|------|------|
| **Planner** | SPEC decomposition, task assignment, complexity assessment | Phase 1 |
| **Spec Writer** | Generate spec.md, plan.md, acceptance.md, research.md | `/auto plan` |
| **Tester** | Test scaffold (RED) + coverage boost (GREEN) | Phase 1.5, 3 |
| **Executor** | TDD implementation in parallel worktrees | Phase 2 |
| **Annotator** | @AX tag lifecycle management | Phase 2.5 |
| **Validator** | Build, vet, lint, file size checks | Gate 2 |
| **Reviewer** | TRUST 5 code review | Phase 4 |
| **Security Auditor** | OWASP Top 10 vulnerability scan | Phase 4 |
| **Architect** | System design, architecture decisions | on-demand |
| **Debugger** | Reproduction-first bug fixing | `/auto fix` |
| **DevOps** | CI/CD, Docker, infrastructure | on-demand |
| **Frontend Specialist** | Playwright E2E + VLM visual regression | Phase 3.5 |
| **UX Validator** | Frontend component visual validation | Phase 3.5 |
| **Perf Engineer** | Benchmark, pprof, regression detection | on-demand |
| **Explorer** | Codebase structure analysis | `/auto map` |

### Quality Modes

```bash
/auto go SPEC-ID --quality ultra      # All agents on Opus — max quality
/auto go SPEC-ID --quality balanced   # Adaptive: Opus/Sonnet/Haiku by task complexity
```

| Mode | Planner | Executor | Validator | Cost |
|------|---------|----------|-----------|------|
| **Ultra** | Opus | Opus | Opus | $$$ |
| **Balanced** | Opus | Adaptive* | Haiku | $ |

\* HIGH complexity → Opus · MEDIUM → Sonnet · LOW → Haiku

### Execution Modes

| Flag | Mode | Description |
|------|------|-------------|
| *(default)* | Subagent pipeline | Main session orchestrates Agent() calls |
| `--team` | Agent Teams | Lead / Builder / Guardian role-based teams |
| `--solo` | Single session | No subagents, direct TDD |
| `--auto --loop` | Full autonomy | RALF self-healing, no human gates |
| `--multi` | Multi-provider | Debate/consensus review with multiple models |

---

## 📐 SPEC-Driven Development

Every feature follows **plan → go → sync**:

```
/auto plan "Add webhook delivery with retry and dead letter queue"
         │
         ▼
  ┌─────────────────────────────────────────────────────┐
  │  .autopus/specs/SPEC-HOOK-001/                      │
  │  ├── prd.md          PRD (10 or 5 sections)         │
  │  ├── spec.md         EARS requirements              │
  │  ├── plan.md         Task breakdown + assignments   │
  │  ├── acceptance.md   Given-When-Then criteria       │
  │  └── research.md     Technical research + risks     │
  └─────────────────────────────────────────────────────┘
         │
         ▼
/auto go SPEC-HOOK-001 --auto --loop    # 15 agents execute
         │
         ▼
/auto sync SPEC-HOOK-001               # docs + changelog updated
```

---

## 🎯 TRUST 5 Code Review

Every review scores across 5 dimensions:

| | Dimension | What It Checks |
|---|-----------|----------------|
| **T** | Tested | 85%+ coverage, edge cases, `go test -race` |
| **R** | Readable | Clear naming, single responsibility, ≤ 300 LOC |
| **U** | Unified | gofmt, goimports, golangci-lint, consistent patterns |
| **S** | Secured | OWASP Top 10, no injection, no hardcoded secrets |
| **T** | Trackable | Meaningful logs, error context, SPEC/Lore references |

---

## 📊 Multi-Model Orchestration

| Strategy | How It Works | Best For |
|----------|-------------|----------|
| **🤝 Consensus** | Independent answers merged by key agreement | Planning, code review |
| **⚔️ Debate** | 2-phase adversarial review + judge verdict | Critical decisions, security |
| **🔗 Pipeline** | Provider N's output → Provider N+1's input | Iterative refinement |
| **⚡ Fastest** | First completed response wins | Quick queries |

Providers: **Claude** · **Codex** · **Gemini** — with graceful degradation.

---

## 📖 All Commands

<details>
<summary><strong>CLI Commands</strong> (19 root commands, 52 total with subcommands)</summary>

| Command | Description |
|---------|-------------|
| `auto init` | Initialize harness — detect platforms, generate files |
| `auto update` | Update harness (preserves user edits via markers) |
| `auto doctor` | Health diagnostics |
| `auto platform` | Manage platforms (list / add / remove) |
| `auto arch` | Architecture analysis (generate / lint) |
| `auto spec` | SPEC management (new / validate / review) |
| `auto lore` | Decision tracking (context / commit / validate / stale) |
| `auto orchestra` | Multi-model orchestration (review / plan / secure) |
| `auto setup` | Project context documents (generate / update / validate) |
| `auto status` | SPEC dashboard (done / in-progress / draft) |
| `auto telemetry` | Pipeline telemetry (record / summary / cost / compare) |
| `auto skill` | Skill management (list / info) |
| `auto search` | Knowledge search (Exa) |
| `auto docs` | Library documentation lookup (Context7) |
| `auto lsp` | LSP integration (diagnostics / refs / rename / symbols) |
| `auto verify` | Harness state and rule verification |
| `auto check` | Harness rule checks (anti-pattern scanning) |
| `auto hash` | File hashing (xxhash) |

</details>

<details>
<summary><strong>Slash Commands</strong> (inside AI Coding CLI)</summary>

| Command | Description |
|---------|-------------|
| `/auto plan "description"` | Create a SPEC for a new feature |
| `/auto go SPEC-ID` | Implement with full pipeline |
| `/auto go SPEC-ID --auto --loop` | Fully autonomous + self-healing |
| `/auto go SPEC-ID --team` | Agent Teams (Lead/Builder/Guardian) |
| `/auto go SPEC-ID --multi` | Multi-provider orchestration |
| `/auto fix "bug"` | Reproduction-first bug fix |
| `/auto review` | TRUST 5 code review |
| `/auto secure` | OWASP Top 10 security audit |
| `/auto map` | Codebase structure analysis |
| `/auto sync SPEC-ID` | Sync docs after implementation |
| `/auto dev "description"` | One-shot: plan → go → sync |
| `/auto setup` | Generate/update project context docs |
| `/auto stale` | Detect stale decisions and patterns |
| `/auto why "question"` | Query decision rationale |

</details>

---

## ⚙️ Configuration

<details>
<summary><strong><code>autopus.yaml</code></strong> — single config for everything</summary>

```yaml
mode: full                    # full or lite
project_name: my-project
platforms:
  - claude-code

architecture:
  auto_generate: true
  enforce: true

lore:
  enabled: true
  required_trailers: [Why, Decision]
  stale_threshold_days: 90

spec:
  review_gate:
    enabled: true
    strategy: debate
    providers: [claude, gemini]
    judge: claude

methodology:
  mode: tdd
  enforce: true

orchestra:
  enabled: true
  default_strategy: consensus
  providers:
    claude:
      binary: claude
    codex:
      binary: codex
    gemini:
      binary: gemini
```

</details>

---

## 🏗️ Architecture

```
autopus-adk/
├── cmd/auto/           # Entry point
├── internal/cli/       # 19 Cobra commands (52 with subcommands)
├── pkg/
│   ├── adapter/        # 5 platform adapters (Claude, Codex, Gemini, Cursor, OpenCode)
│   ├── orchestra/      # Multi-model orchestration (4 strategies)
│   ├── spec/           # SPEC engine (EARS format)
│   ├── lore/           # Decision tracking (9-trailer protocol)
│   ├── content/        # Agent/skill/hook generation + skill activator
│   ├── arch/           # Architecture analysis + rule enforcement
│   ├── sigmap/         # go/ast API signature extraction
│   ├── constraint/     # Anti-pattern scanning
│   ├── telemetry/      # Pipeline telemetry + cost estimation
│   ├── cost/           # Token-based cost estimator
│   ├── setup/          # Project doc generation
│   ├── lsp/            # LSP integration
│   ├── search/         # Knowledge search (Context7/Exa)
│   └── ...             # template, detect, config, version
├── templates/          # Platform-specific templates
├── content/            # Embedded content (15 agents, 36 skills)
└── configs/            # Default configuration
```

---

## 🤝 Contributing

Autopus-ADK is open source under the MIT license. PRs welcome!

```bash
make test       # Run tests with race detection
make lint       # Run go vet
make coverage   # Generate coverage report
```

---

<div align="center">

**🐙 Autopus** — Your AI agents deserve a team, not a chatbox.

</div>
