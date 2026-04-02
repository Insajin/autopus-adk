# SPEC-ORCH-019: Subprocess-Based Multi-Provider Orchestration Engine

**Status**: completed
**Created**: 2026-04-01
**Domain**: ORCH
**Module**: autopus-adk
**Package**: `pkg/orchestra/`
**Origin**: BS-019

## Purpose

The current `--multi` orchestration engine depends on cmux-based screen scraping for driving
multiple AI providers in terminal panes. After 18 incremental SPECs (SPEC-ORCH-001 through
SPEC-ORCH-018), the pane architecture has proven to be a structural impedance mismatch:
it communicates via screen text while the system needs structured data.

All major providers now support non-interactive subprocess execution with JSON output
(e.g., Claude, Codex, Gemini, OpenCode). Provider-specific CLI flags and arguments are
configured via `autopus.yaml` rather than hardcoded, enabling new providers (including
opencode and future CLIs) to be added without code changes.

This SPEC introduces a `SubprocessBackend` that replaces screen scraping with structured
subprocess execution, using process exit as the deterministic completion signal and JSON
schema enforcement for structured output.

**Key outcomes:**
- `--multi` works in plain terminals (SSH, CI/CD, headless)
- ~77% token reduction (368K -> ~84K per pipeline)
- Zero false completion detections (process exit is binary)
- Backward-compatible: pane mode remains default, `--subprocess` flag opts into new backend

## Requirements

### P0 — Must Have

**R01 — ExecutionBackend Interface**
WHEN the orchestra engine initializes,
THE SYSTEM SHALL provide an `ExecutionBackend` interface with method
`Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error)` and `Name() string`,
with two implementations: `SubprocessBackend` (new) and `PaneBackend` (wrapping existing pane logic).
`ExecutionBackend` operates at the **provider-invocation level** (single provider call).
Strategy code (consensus.go, debate.go) calls `backend.Execute()` for each provider individually.
`PaneBackend` wraps the existing `runProvider()` function (not `RunPaneOrchestra()`).
`SubprocessBackend` replaces `runProvider()` with subprocess execution.
This interface composes with the existing `command` interface in `command.go`.

**R02 — Subprocess Execution**
WHEN a provider is executed via `SubprocessBackend`,
THE SYSTEM SHALL spawn the provider CLI using the configured subprocess arguments
from `autopus.yaml` `providers.{name}.subprocess` section,
pass the prompt via stdin pipe (default) or via a temp file when stdin is not available,
collect JSON output from stdout,
and use process exit code as the deterministic completion signal.
> _Non-normative note:_ Example provider commands include `claude -p`, `codex exec --quiet`,
> `gemini -p`. Provider-specific CLI flags are configured in `autopus.yaml` and validated at runtime.

**R03 — JSON Schema Enforcement**
WHEN a subprocess provider invocation is prepared,
THE SYSTEM SHALL enforce structured output using the provider's configured schema mechanism
(CLI flag or prompt-embedded schema) to produce structured output per role (debater, judge, reviewer).
The schema flag name and delivery method are read from `autopus.yaml` `providers.{name}.subprocess.schema_flag`.
> _Non-normative note:_ CLI-based schema flags (e.g., `--json-schema`, `--output-schema`) are passed
> as temp file paths. For providers without CLI schema support (e.g., Gemini), the schema is embedded
> in the prompt text and output is validated post-hoc.

**R04 — Parallel Execution with Graceful Degradation**
WHEN multiple providers execute concurrently via `SubprocessBackend`,
THE SYSTEM SHALL execute providers concurrently with graceful degradation,
record individual provider failures in `FailedProviders`,
and continue the pipeline provided at least 1 provider succeeds.
The concurrency mechanism (e.g., errgroup, WaitGroup) is an implementation detail.

**R05 — PromptBuilder (Template-Based)**
WHEN a debate pipeline phase requires a prompt,
THE SYSTEM SHALL generate role-specific and round-specific structured prompts via a `PromptBuilder`
that renders Go templates from `templates/shared/orchestra-*.md.tmpl`.
Templates receive `OrchestraPromptData` containing project overview (Layer 1), relevant paths
auto-detected from topic keywords (Layer 2), and tool usage limits (Layer 3).
The main session pre-computes `RelevantPaths` via Grep before spawning subprocesses,
so providers focus on reading and analyzing rather than searching.

**R06 — SchemaBuilder**
WHEN JSON schema enforcement is needed for a provider invocation,
THE SYSTEM SHALL generate JSON schemas from Go struct definitions per role
(`DebaterR1Schema`, `DebaterR2Schema`, `JudgeSchema`, `ReviewerSchema`)
via a `SchemaBuilder`, writing the schema to a temp file for CLI consumption.

**R07 — Cross-Pollination Engine**
WHEN round N of a debate completes,
THE SYSTEM SHALL anonymize provider identities (Provider A, B, C),
remove ICE scores from prior outputs,
preserve full content without summarization,
and inject prior analysis into round N+1 prompts via `CrossPollinateBuilder`.

**R08 — Judge Synthesis**
WHEN all debate rounds complete,
THE SYSTEM SHALL generate a blind judge prompt with anonymized provider identities,
consensus extraction guidance, and ICE scoring instructions via a `JudgeBuilder`.

**R09 — Output Merge**
WHEN the judge verdict and provider identity mapping are available,
THE SYSTEM SHALL produce a final markdown result document that includes
the judge's synthesis, individual provider summaries, and de-anonymized attribution.

> **ICE Scoring**: Impact (1-10) x Confidence (1-10) x Ease (1-10) / 100.
> Used in brainstorm judge output to rank ideas by weighted desirability.

**R10 — Pipeline Runner**
WHEN `auto orchestra run` is invoked with `--strategy`, `--rounds`, and `--providers` flags,
THE SYSTEM SHALL execute the full subprocess pipeline
(prepare -> independent -> cross-pollinate -> judge -> merge) using `SubprocessBackend`.

**R11 — Backend Selection**
WHEN `RunOrchestra()` is called,
THE SYSTEM SHALL preserve the current pane-based behavior as the DEFAULT backend.
`SubprocessBackend` is selected only when `--subprocess` flag is explicitly set,
OR when terminal is nil (headless/CI environment where pane mode is not possible).
The `RunOrchestra()` public API signature SHALL NOT change.
A future SPEC (after subprocess mode is validated in production) may flip the default.

**R12 — Token Isolation with Code Access**
WHEN a subprocess invocation is prepared,
THE SYSTEM SHALL use `--bare` mode (Claude) or equivalent isolation to skip hooks, skills,
plugins, MCP servers, auto memory, and CLAUDE.md loading — targeting < 5K system tokens per turn.
Built-in tools (Read, Grep, Glob) remain available in bare mode, enabling providers to read
project code on demand. Tool auto-approval is configured via `--allowedTools "Read,Grep,Glob"`
(Claude), `--sandbox read-only` (Codex), or `--yolo` (Gemini).

> **Verified**: Claude `--bare` skips context loading but retains Bash, Read, Edit tools.
> Codex `exec` reads files within the Git repo by default.
> Gemini `-p --yolo` has read_file, search_file_content, list_directory tools available.

### P1 — Should Have

**R13 — Adaptive Rounds**
WHEN the `--rounds` flag is provided to `auto orchestra run`,
THE SYSTEM SHALL support presets: `fast` (round 0: independent + judge only),
`standard` (round 1: independent + cross-pollinate + judge),
`deep` (round 2: independent + 2x cross-pollinate + judge).

**R14 — Provider CLI Abstraction**
WHEN a provider's subprocess CLI syntax is configured,
THE SYSTEM SHALL read configuration from `autopus.yaml` `providers.{name}.subprocess` section
with fields: `binary`, `args`, `schema_flag`, `stdin_mode`, `output_format`, `timeout`.

**R15 — Terminal Progress Display**
WHILE subprocess providers are executing in parallel,
THE SYSTEM SHALL display a terminal progress indicator showing each provider's status
(running/done/failed) and elapsed time.

**R16 — Context Summarizer**
WHEN a subprocess prompt includes project context,
THE SYSTEM SHALL generate a compressed project context summary respecting a configurable
`--max-tokens` budget to minimize token consumption.

**R17 — Per-Provider Timeout**
WHEN a provider subprocess is spawned,
THE SYSTEM SHALL apply a configurable per-provider timeout
(defaults: Claude 120s, Codex 120s, Gemini 180s).

**R18 — Graceful Provider Skip**
WHEN a provider binary is not installed on the host,
THE SYSTEM SHALL skip it with a warning rather than failing the pipeline,
provided at least 1 provider remains available.

### P2 — Could Have

**R19 — Streaming Progress Parsing**
WHILE a subprocess provider produces streaming output,
THE SYSTEM SHALL parse provider-specific streaming formats
(Claude `stream-json`, Codex JSONL) to show incremental thinking status.

**R20 — Rich Mode for Claude**
WHEN the `--rich` flag is set,
THE SYSTEM SHALL enable Claude-specific skill loading (`orchestra-respond` skill)
for enhanced analysis while other providers use bare mode.

**R21 — Session Resume**
WHEN the `--resume` flag is set,
THE SYSTEM SHALL resume an interrupted orchestration session by replaying from the last completed phase.

**R22 — cmux Hybrid Mode**
WHEN the `--visual` flag is set,
THE SYSTEM SHALL run subprocesses inside cmux panes for visual feedback
but collect results via JSON files (not screen scraping).

## File Map

| File | Purpose |
|------|---------|
| `pkg/orchestra/backend.go` | `ExecutionBackend` interface + `PaneBackend` adapter |
| `pkg/orchestra/subprocess_runner.go` | `SubprocessBackend` implementation |
| `pkg/orchestra/prompt_builder.go` | Role/round-specific prompt generation |
| `pkg/orchestra/schema_builder.go` | JSON schema generation from Go structs |
| `pkg/orchestra/crosspolinate.go` | Cross-pollination engine (anonymization + injection) |
| `pkg/orchestra/judge_builder.go` | Judge prompt synthesis |
| `pkg/orchestra/output_parser.go` | JSON output parsing + validation |
| `pkg/orchestra/pipeline.go` | Full subprocess pipeline runner |
| `pkg/orchestra/progress.go` | Terminal progress display (P1) |
| `pkg/orchestra/relevant_paths.go` | Topic-based relevant path detection (Grep pre-scan) |

### Prompt Templates

| Template | Purpose |
|----------|---------|
| `templates/shared/orchestra-context.md.tmpl` | 3-layer context block (Must/Should/May Read) — partial, included by all |
| `templates/shared/orchestra-debater-r1.md.tmpl` | Round 1 independent divergence |
| `templates/shared/orchestra-debater-r2.md.tmpl` | Round 2 cross-pollination |
| `templates/shared/orchestra-judge.md.tmpl` | Blind judge synthesis |
| `templates/shared/orchestra-reviewer.md.tmpl` | SPEC/code review verdict |
| `templates/shared/orchestra-consensus.md.tmpl` | Consensus strategy (single round) |

### 3-Layer Context Injection Strategy

Providers in `--bare` mode have no CLAUDE.md, skills, or MCP. The prompt itself must
provide all project understanding through a 3-layer reading guide:

- **Layer 1 (Must Read)**: `ARCHITECTURE.md`, `product.md`, `go.mod` — always required
- **Layer 2 (Should Read)**: topic-relevant paths auto-detected by `relevant_paths.go`
  via keyword extraction + Grep pre-scan in the main session before subprocess spawn
- **Layer 3 (May Read)**: provider-autonomous exploration with `Glob`/`Grep` within `MaxTurns` budget

The main session pre-computes Layer 2 paths to avoid each provider redundantly
scanning the codebase. Providers focus token budget on reading and analyzing,
not searching.

## Key Type Definitions

```
ProviderRequest {
  Provider string        // provider name (e.g., "claude")
  Prompt   string        // full prompt text
  Schema   []byte        // JSON schema (optional)
  Role     string        // "debater", "judge", "reviewer"
  Round    int           // round number
  Timeout  time.Duration // per-provider timeout
}
```

## Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR-01 | Full 3-provider fast-mode pipeline completes in < 180 seconds. Excluding Gemini initialization, Claude+Codex complete in < 60 seconds |
| NFR-02 | Peak memory usage < 100MB for 3 concurrent subprocesses |
| NFR-03 | Total token consumption for standard debate < 100K |
| NFR-04 | Zero false completion detections (process exit is binary) |
| NFR-05 | All new source files under 300 lines (target < 200) |
| NFR-06 | New subprocess execution path >= 80% line coverage |
| NFR-07 | Pane mode remains the default; existing behavior unchanged without `--subprocess` flag |

## References

- BS-019: Subprocess-Based Multi-Provider Orchestration
- SPEC-ORCH-001 through SPEC-ORCH-018: Pane-based orchestra incremental fixes
- Multi-Agent Debate research (arxiv 2507.05981)
