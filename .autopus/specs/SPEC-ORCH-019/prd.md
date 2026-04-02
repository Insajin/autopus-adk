# PRD: Subprocess-Based Multi-Provider Orchestration Engine

**SPEC-ID**: SPEC-ORCH-019
**Module**: autopus-adk
**Package**: `pkg/orchestra/`
**Status**: draft
**Created**: 2026-04-01
**Origin**: BS-019

---

## 1. Problem & Context

The current `--multi` orchestration engine relies on cmux-based screen scraping to drive multiple AI providers (Claude, Codex, Gemini) in terminal panes. This architecture has accumulated significant accidental complexity:

- **89 files** in `pkg/orchestra/`, with approximately 33 (~37%) dedicated to screen scraping, TUI noise removal, Hook IPC, completion detection via polling, and pane lifecycle management.
- **cmux dependency** prevents `--multi` usage in plain terminals (SSH sessions, CI/CD, headless environments).
- **~50K tokens wasted per provider turn** due to system context loading (CLAUDE.md, MCP plugins, etc.) in each pane session.
- **Non-deterministic completion detection**: screen polling with idle thresholds causes both false positives (premature completion) and false negatives (timeout on slow providers). This is the #1 source of orchestra regressions (SPEC-ORCH-001 through SPEC-ORCH-018 trace).
- **Token cost**: A standard 3-provider debate pipeline consumes ~368K tokens, of which ~270K are wasted on repeated system context.

All three major providers now support structured non-interactive execution:
- Claude: `claude --bare -p` with `--json-schema`
- Codex: `codex exec` with `--output-schema`
- Gemini: `gemini -p` with `--output-format json`

This creates a viable alternative: replace screen scraping with structured subprocess execution and JSON-schema-enforced output, using process exit as deterministic completion signal.

### Prior Art

- SPEC-ORCH-001 through SPEC-ORCH-018: Incremental fixes for pane-based orchestra (completion detection, surface management, hook IPC, noise filtering).
- BS-018: A2A+MCP Hybrid Worker — related subprocess execution for bridge removal.
- Multi-Agent Debate research (arxiv 2507.05981): Round 0 (independent + judge) provides best cost-quality tradeoff.

## 2. Goals & Success Metrics

### Goals

| ID | Goal |
|----|------|
| G1 | Enable `--multi` orchestration without cmux dependency |
| G2 | Reduce token consumption per pipeline by 70%+ |
| G3 | Achieve deterministic completion detection (process exit vs screen polling) |
| G4 | Simplify codebase by decoupling from TUI/screen scraping layer |
| G5 | Maintain backward compatibility with existing pane mode |

### Success Metrics

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| Token usage per 3-provider debate pipeline | ~368K | < 100K | Sum of input+output tokens across all subprocess invocations |
| Completion detection false positive/negative rate | ~15% (estimated from regression frequency) | 0% | Process exit code = deterministic |
| `--multi` works in plain terminal | No (requires cmux) | Yes | Manual test in bare SSH session |
| Orchestra source files (non-test) | ~45 | < 20 (core + subprocess) | `ls pkg/orchestra/*.go \| grep -v _test \| wc -l` |
| Time-to-first-result (3 providers, fast mode) | ~90s (pane startup + polling) | < 60s | Wall clock from command start to merged output |

## 3. Target Users

| User | Context | Pain Point |
|------|---------|------------|
| **CLI power users** | Run `auto idea --multi`, `auto go --multi` daily | cmux required, frequent completion detection failures |
| **CI/CD pipelines** | Want multi-provider review in automated workflows | Cannot use `--multi` in headless environments |
| **SSH/remote developers** | Develop on remote machines via SSH | cmux unavailable or impractical in SSH sessions |
| **Cost-conscious teams** | Monitor token spend across providers | ~368K tokens per pipeline is prohibitive for frequent use |

## 4. User Stories / Job Stories

### US-1: Plain Terminal Multi-Provider
**When** I run `auto idea --multi "new auth system"` in a plain terminal,
**I want** Claude, Codex, and Gemini to execute in parallel subprocesses and return a merged result,
**So that** I get multi-provider brainstorming without needing cmux installed.

### US-2: Deterministic Completion
**When** a provider subprocess completes,
**I want** the system to detect completion via process exit (not screen polling),
**So that** I never experience false completion or unnecessary timeouts.

### US-3: Token-Efficient Execution
**When** running a multi-provider pipeline,
**I want** each provider to receive only the scoped prompt (not full system context),
**So that** token usage stays under 100K for a standard debate.

### US-4: Adaptive Round Control
**When** running `auto idea --multi --rounds fast`,
**I want** independent parallel execution followed by a single judge synthesis (no cross-pollination),
**So that** I get quick results with optimal cost-quality tradeoff.

### US-5: Structured JSON Output
**When** providers return their analysis,
**I want** output enforced by JSON schema,
**So that** cross-pollination and judge synthesis operate on structured data (not free-form text with TUI noise).

### US-6: Progress Visibility
**When** subprocesses are running in parallel,
**I want** a terminal progress display showing each provider's status,
**So that** I know which providers are still working and approximate time remaining.

### US-7: Backward Compatibility
**When** I run `auto idea --multi --pane`,
**I want** the existing cmux-based pane mode to work as before,
**So that** I can fall back to the visual pane mode when I prefer it.

### US-8: Provider CLI Configuration
**When** I add a new AI provider to my `autopus.yaml`,
**I want** to configure its subprocess CLI syntax declaratively,
**So that** new providers can be added without code changes.

## 5. Functional Requirements

### P0 — Must Have

| ID | Requirement |
|----|-------------|
| FR-01 | **ExecutionBackend interface**: Define an `ExecutionBackend` interface with `Execute(ctx, prompt, schema) -> ProviderResponse` method. Implement `SubprocessBackend` (new) and refactor existing pane logic behind `PaneBackend`. |
| FR-02 | **Subprocess execution**: `SubprocessBackend` spawns provider CLIs as child processes (`claude --bare -p`, `codex exec`, `gemini -p`), passes prompt via stdin, collects JSON output from stdout, and uses process exit as completion signal. |
| FR-03 | **JSON schema enforcement**: Each provider invocation includes a JSON schema parameter (`--json-schema`, `--output-schema`, `--output-format json`) to enforce structured output. Schemas are generated per role (debater, judge, reviewer). |
| FR-04 | **Parallel subprocess execution**: Multiple providers execute concurrently using `errgroup`. Individual provider failure does not abort the entire pipeline — failed providers are recorded in `FailedProviders`. |
| FR-05 | **Prompt builder**: `auto orchestra prompt` subcommand generates role-specific, round-specific structured prompts. Accepts `--role`, `--round`, `--topic`, `--context` flags. |
| FR-06 | **Schema builder**: `auto orchestra schema` subcommand generates JSON schemas per role. Accepts `--role` flag with values `debater`, `judge`, `reviewer`. |
| FR-07 | **Cross-pollination engine**: `auto orchestra crosspolinate` subcommand takes round N results, anonymizes provider identities, and produces round N+1 prompts with prior analysis injected. |
| FR-08 | **Judge synthesis**: `auto orchestra judge-prompt` subcommand generates a blind judge prompt from all provider results, with anonymized identities (Provider A, B, C). |
| FR-09 | **Merge output**: `auto orchestra merge` subcommand takes judge verdict + provider identity mapping and produces the final markdown result document. |
| FR-10 | **Pipeline runner**: `auto orchestra run` subcommand executes the full pipeline (prepare -> independent -> cross-pollinate -> judge -> merge) with `--strategy`, `--rounds`, `--providers` flags. |
| FR-11 | **Backend selection**: `RunOrchestra()` selects `SubprocessBackend` when terminal is nil or "plain", `PaneBackend` when a pane-capable terminal is configured and `--pane` flag is set. Default changes from pane to subprocess. |
| FR-12 | **Token isolation**: Subprocess invocations use `--bare` mode (or equivalent 4-layer isolation) to prevent system context loading, targeting < 5K system tokens per turn. |

### P1 — Should Have

| ID | Requirement |
|----|-------------|
| FR-13 | **Adaptive rounds**: Support `--rounds` flag with presets: `fast` (round 0: independent + judge), `standard` (round 1: independent + cross-pollinate + judge), `deep` (round 2: independent + cross-pollinate + refine + judge). |
| FR-14 | **Provider CLI abstraction**: Provider subprocess CLI syntax is configured via `autopus.yaml` `providers.subprocess` section, not hardcoded. Support fields: `binary`, `args`, `schema_flag`, `stdin_mode`, `output_format`. |
| FR-15 | **Terminal progress display**: Show real-time progress for each provider subprocess (running/done/failed status, elapsed time). Parse streaming JSON events where available (Claude `stream-json`, Codex `--json`). |
| FR-16 | **Context summarizer**: `auto orchestra context` subcommand generates a compressed project context summary for prompt injection, respecting a `--max-tokens` budget. |
| FR-17 | **Timeout per provider**: Configurable per-provider timeout with defaults: Claude 120s, Codex 120s, Gemini 180s (accounting for slow init). |
| FR-18 | **Graceful degradation**: When a provider binary is not installed, skip it with a warning rather than failing the pipeline, provided at least 2 providers remain. |

### P2 — Could Have

| ID | Requirement |
|----|-------------|
| FR-19 | **Streaming progress parsing**: Parse provider-specific streaming formats (Claude `stream-json`, Codex JSONL) to show incremental thinking status in the progress display. |
| FR-20 | **Rich mode for Claude**: `--rich` flag enables Claude-specific skill loading (`orchestra-respond` skill) for enhanced analysis, while other providers use bare mode. |
| FR-21 | **Session resume**: Support `--resume` flag to resume an interrupted orchestration session by replaying from the last completed phase. |
| FR-22 | **cmux hybrid mode**: `--visual` flag runs subprocesses inside cmux panes for visual feedback, but collects results via JSON files (not screen scraping). |

## 6. Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR-01 | **Latency**: Full 3-provider fast-mode pipeline completes in < 60 seconds on standard hardware. |
| NFR-02 | **Memory**: Peak memory usage < 100MB for 3 concurrent provider subprocesses. |
| NFR-03 | **Token efficiency**: Total token consumption for a standard debate < 100K (77% reduction from current ~368K). |
| NFR-04 | **Reliability**: Zero false completion detections (process exit is binary). |
| NFR-05 | **File size**: All new source files MUST be under 300 lines (target < 200). |
| NFR-06 | **Test coverage**: New subprocess execution path achieves >= 80% line coverage. |
| NFR-07 | **Backward compatibility**: Existing `--multi` pane mode remains functional via `--pane` flag with zero behavior changes. |
| NFR-08 | **Portability**: Subprocess mode works on macOS, Linux, and Windows (WSL). No platform-specific dependencies. |

## 7. Technical Constraints

| Constraint | Detail |
|------------|--------|
| Language | Go 1.26, consistent with autopus-adk codebase |
| Package location | `pkg/orchestra/` — extends existing package, does not create new top-level packages |
| Provider CLIs | Must handle: `claude` (Anthropic), `codex` (OpenAI), `gemini` (Google). Additional providers via config. |
| Gemini init latency | Gemini CLI has ~60s initialization time. Must not block other providers. |
| JSON schema support | Gemini supports `--output-format json` but not schema enforcement. Fallback: prompt-level schema instruction. |
| stdin pipe limits | Long prompts (>4KB) must use file-based input to avoid PTY truncation. |
| Config source | Provider subprocess config in `autopus.yaml` alongside existing pane config |
| Interface constraint | `RunOrchestra()` public API signature must not change (4 callers as noted in `@AX:ANCHOR`) |

## 8. Out of Scope

| Item | Reason |
|------|--------|
| Removing cmux legacy code (~33 files) | Separate future SPEC — coexistence required during transition |
| Modifying idea/review skill pipelines | Skills call `RunOrchestra()` — backend selection is transparent |
| Modifying autopus-bridge | Bridge is a separate component (BS-018 scope) |
| New provider integrations (beyond claude/codex/gemini) | Config-driven extensibility enables this, but specific integrations are separate SPECs |
| Cost tracking / billing integration | Orthogonal concern — separate SPEC |
| A2A protocol integration | BS-018 scope, not this SPEC |

## 9. Risks & Open Questions

### Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Gemini 60s init latency degrades UX | HIGH | MEDIUM | Start Gemini subprocess first; display progress; document expected wait time |
| JSON schema not enforced by all providers equally | MEDIUM | MEDIUM | Validate output against schema post-hoc; fall back to prompt-level instruction |
| Large prompt exceeds stdin pipe buffer | LOW | HIGH | Use file-based input for prompts > 4KB; test with 10K+ token prompts |
| Provider CLI breaking changes across versions | MEDIUM | HIGH | Pin expected CLI versions in config; add version detection |
| `--bare` mode removes useful Claude capabilities | LOW | MEDIUM | `--rich` flag (FR-20) restores skill loading for Claude when needed |

### Open Questions

| # | Question | Impact | Proposed Resolution |
|---|----------|--------|---------------------|
| OQ-1 | Should subprocess mode become the default for `--multi`, or require explicit `--subprocess` flag? | UX | Default to subprocess, offer `--pane` for legacy. Phase in over 2 releases. |
| OQ-2 | How to handle Gemini's lack of JSON schema enforcement? | Output quality | Embed schema in prompt text + post-hoc validation. Accept slightly lower structure compliance. |
| OQ-3 | Should judge always be Claude (strongest model), or configurable? | Quality vs flexibility | Default to Claude as judge; `--judge-provider` flag for override. |
| OQ-4 | What is the minimum viable progress display without cmux? | UX | Spinner + status line per provider. Streaming parse is P2. |
| OQ-5 | Should cross-pollination anonymize provider identities? | Debate quality | Yes — research shows anonymization reduces position bias (arxiv 2507.05981). |

## 10. Pre-mortem

*Imagining it is 3 months post-launch and this feature has failed. What went wrong?*

| Failure Scenario | Root Cause | Prevention |
|------------------|-----------|------------|
| "Nobody uses subprocess mode — they prefer seeing panes" | Underestimated UX value of visual pane feedback | Invest in progress display (FR-15); offer `--visual` hybrid (FR-22); user research before deprecating panes |
| "JSON output quality is worse than free-form pane output" | Schema constraints limit provider expressiveness | Start with loose schemas (few required fields); iterate based on output quality comparison |
| "Gemini results always arrive 60s late, degrading experience" | Gemini init time not adequately communicated | Start Gemini first; show ETA; consider `--exclude gemini` shortcut for fast runs |
| "New provider CLIs keep breaking our subprocess calls" | Provider CLI APIs are unstable | Version pin in config; integration tests against each provider; degrade gracefully |
| "Token reduction didn't materialize because prompts grew" | Context summarizer produces large summaries | Hard token cap on context injection; measure per-release |
| "Pane mode regressed because nobody tested it after refactoring" | Backend interface refactoring introduced pane bugs | Maintain pane integration tests; do not modify pane code paths in this SPEC |

## 11. Practitioner Q&A

**Q: Why not just fix the existing pane-based orchestra instead of building a new backend?**
A: The pane architecture has a fundamental impedance mismatch: it communicates via screen text (send keystrokes, read screen pixels) while we need structured data. 18 SPECs of incremental fixes (SPEC-ORCH-001 through 018) demonstrate this is a structural problem, not a bug density problem. Subprocess + JSON is the architecturally correct approach.

**Q: Why keep pane mode at all?**
A: Some users value the visual feedback of seeing providers work in real-time. The `--visual` hybrid (FR-22) will eventually provide this without screen scraping, but until then, `--pane` ensures no capability regression.

**Q: Why use `--bare` instead of 4-layer token isolation?**
A: `--bare` achieves the same token reduction (~50K -> ~2K system tokens) with a single flag instead of managing 4 separate isolation layers. It's the simplest correct approach. 4-layer isolation remains available for fine-grained control if needed.

**Q: How does this relate to BS-018 (A2A+MCP Hybrid Worker)?**
A: BS-019 is a subset/precursor. The subprocess runner built here (`subprocess_runner.go`) is reusable by BS-018's worker architecture. BS-019 provides standalone value (improved `--multi`) independent of BS-018's bridge removal timeline.

**Q: What about providers that don't support JSON schema?**
A: Gemini accepts `--output-format json` but doesn't enforce a schema. Our mitigation: (1) embed the expected schema in the prompt text, (2) validate output post-hoc, (3) if validation fails, extract structured data from free-form JSON response. Research shows prompt-level schema instruction achieves ~90% compliance even without enforcement.

**Q: Won't subprocess spawning be slower than reusing pane sessions?**
A: For individual turns, yes (subprocess startup ~2-5s vs pane already running). But total pipeline time is faster because: (1) no 20s initial polling delay, (2) no idle threshold waiting, (3) deterministic completion means no wasted polling cycles. Net effect: faster overall pipeline.

---

*Ref: BS-019, SPEC-ORCH-001~018*
