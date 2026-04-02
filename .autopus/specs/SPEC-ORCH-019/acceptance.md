# SPEC-ORCH-019 Acceptance Criteria

## Coverage Target

- New subprocess execution path: >= 80% line coverage
- All new files must have corresponding `_test.go` files
- Existing pane-mode tests must continue to pass without modification

---

## S1: Backend Interface and Selection

### S1.1: Default backend is PaneBackend (backward compatible)
- **Given**: `RunOrchestra()` is called with a terminal present and no `--subprocess` flag
- **When**: the function executes
- **Then**: `PaneBackend` is used (existing behavior preserved)

### S1.2: Headless environment uses SubprocessBackend
- **Given**: `RunOrchestra()` is called with `Terminal = nil` (CI/headless)
- **When**: the function executes
- **Then**: `SubprocessBackend` is used (pane mode not possible)

### S1.3: --subprocess flag selects SubprocessBackend
- **Given**: `RunOrchestra()` is called with `--subprocess` flag set
- **When**: the function executes
- **Then**: `SubprocessBackend` is used regardless of terminal capability

### S1.4: API signature unchanged
- **Given**: the `RunOrchestra()` function
- **When**: compared to the pre-refactor signature
- **Then**: the function signature `RunOrchestra(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error)` is identical

---

## S2: Subprocess Execution

### S2.1: Provider spawns as child process
- **Given**: a provider config for Claude with `binary: "claude"` and configured `args` from `autopus.yaml`
- **When**: `SubprocessBackend.Execute()` is called
- **Then**: a child process is spawned using the configured binary and args, with prompt piped via stdin

### S2.2: Process exit as completion signal
- **Given**: a running provider subprocess
- **When**: the process exits with code 0
- **Then**: the response is collected immediately (no polling delay)
- **And**: `ProviderResponse.TimedOut` is false

### S2.3: Timeout handling
- **Given**: a provider with timeout 120s
- **When**: the subprocess does not exit within 120s
- **Then**: the process is terminated
- **And**: `ProviderResponse.TimedOut` is true
- **And**: the pipeline continues with remaining providers

### S2.4: File-based input fallback
- **Given**: stdin pipe is not available for the provider
- **When**: `SubprocessBackend.Execute()` is called
- **Then**: the prompt is written to a temp file and passed via file input
- **And**: the temp file is cleaned up after execution

### S2.5: Token isolation
- **Given**: a subprocess invocation for any provider
- **When**: the provider command is constructed
- **Then**: the provider's configured isolation flags (from `autopus.yaml`) are included
- **And**: system token overhead is < 5K tokens

---

## S3: JSON Schema Enforcement

### S3.1: CLI-based schema via configured flag
- **Given**: a debater round 1 execution targeting a provider with `schema_flag` configured
- **When**: the command is constructed
- **Then**: the configured schema flag and a temp file path are appended to args
- **And**: the temp file contains valid JSON Schema for `DebaterR1Output`

### S3.2: Schema flag varies by provider config
- **Given**: different providers with different `schema_flag` values in `autopus.yaml`
- **When**: each provider's command is constructed
- **Then**: each uses its own configured schema flag name

### S3.3: Gemini prompt-embedded schema
- **Given**: a debater execution targeting Gemini
- **When**: the prompt is constructed
- **Then**: the JSON schema is embedded in the prompt text (no CLI flag)
- **And**: the output is validated post-hoc against the expected structure

### S3.4: Schema per role
- **Given**: roles `debater` (R1), `debater` (R2), `judge`, `reviewer`
- **When**: `SchemaBuilder.Generate(role)` is called for each
- **Then**: each returns a distinct valid JSON Schema with role-appropriate fields

---

## S4: Parallel Execution and Degradation

### S4.1: Parallel provider execution
- **Given**: 3 providers (Claude, Codex, Gemini) configured
- **When**: the independent round executes
- **Then**: all 3 providers run concurrently (not sequentially)
- **And**: total wall-clock time is approximately `max(individual times)`, not `sum`

### S4.2: Single provider failure — pipeline continues
- **Given**: 3 providers configured, Gemini fails with exit code 1
- **When**: the independent round completes
- **Then**: Claude and Codex results are used for the next phase
- **And**: Gemini is recorded in `FailedProviders`
- **And**: the pipeline continues to judge synthesis with 2 results

### S4.3: All providers fail — pipeline errors
- **Given**: 3 providers configured, all fail
- **When**: the independent round completes
- **Then**: `RunOrchestra()` returns a non-nil error
- **And**: all 3 are recorded in `FailedProviders`

### S4.4: Provider binary not installed — skip with warning
- **Given**: Gemini binary is not installed
- **When**: the pipeline initializes
- **Then**: Gemini is skipped with a warning log
- **And**: the pipeline runs with Claude and Codex only
- **And**: `FailedProviders` records Gemini with "binary not found" error

---

## S5: Prompt Building

### S5.1: Debater Round 1 prompt
- **Given**: topic "New authentication system" and project context
- **When**: `PromptBuilder.BuildDebaterR1()` is called
- **Then**: the prompt includes the topic, SCAMPER/HMW framing, and JSON output instructions
- **And**: the prompt does NOT include other providers' results

### S5.2: Debater Round 2 prompt with cross-pollination
- **Given**: round 1 results from 3 providers (anonymized)
- **When**: `PromptBuilder.BuildDebaterR2()` is called
- **Then**: the prompt includes prior results labeled "Debater A", "Debater B", "Debater C"
- **And**: provider real names do NOT appear in the prompt
- **And**: the prompt instructs acknowledgement -> integration -> risk analysis

### S5.3: Judge prompt — blind synthesis
- **Given**: final round results from 3 providers (anonymized)
- **When**: `PromptBuilder.BuildJudge()` is called
- **Then**: the prompt includes all results with anonymized identities
- **And**: includes consensus extraction and ICE scoring instructions
- **And**: provider real names do NOT appear in the prompt

---

## S6: Cross-Pollination

### S6.1: Identity anonymization
- **Given**: round 1 results with provider names "claude", "codex", "gemini"
- **When**: `CrossPollinateBuilder.Build()` is called
- **Then**: output prompts reference "Debater A", "Debater B", "Debater C"
- **And**: the identity mapping is preserved for de-anonymization

### S6.2: Content preservation
- **Given**: round 1 results with full analysis text
- **When**: cross-pollination prompts are generated
- **Then**: full content is preserved (no summarization or truncation)

### S6.3: ICE score removal
- **Given**: round 1 results containing ICE scores
- **When**: cross-pollination prompts are generated
- **Then**: ICE score sections are stripped from injected prior results

---

## S7: Pipeline Rounds

### S7.1: Fast mode (round 0)
- **Given**: `--rounds fast` flag
- **When**: the pipeline executes
- **Then**: providers run independently (no cross-pollination)
- **And**: judge synthesis runs on independent results
- **And**: total phases = 2 (independent + judge)

### S7.2: Standard mode (round 1)
- **Given**: `--rounds standard` flag (default)
- **When**: the pipeline executes
- **Then**: phase 1 = independent, phase 2 = cross-pollinate, phase 3 = judge
- **And**: total phases = 3

### S7.3: Deep mode (round 2)
- **Given**: `--rounds deep` flag
- **When**: the pipeline executes
- **Then**: phase 1 = independent, phase 2 = cross-pollinate, phase 3 = refine, phase 4 = judge
- **And**: total phases = 4

---

## S8: Output and Merge

### S8.1: Valid JSON parsing
- **Given**: a provider returns valid JSON matching the schema
- **When**: `OutputParser.Parse()` is called
- **Then**: the output is deserialized into the correct Go struct
- **And**: all required fields are populated

### S8.2: Malformed JSON recovery
- **Given**: a provider returns partially valid JSON (e.g., Gemini without schema enforcement)
- **When**: `OutputParser.Parse()` is called
- **Then**: the parser attempts structured extraction from the response
- **And**: logs a warning about schema non-compliance

### S8.3: Final merge document
- **Given**: judge verdict + identity mapping + all provider results
- **When**: `MergeSubprocessResults()` is called
- **Then**: output contains: judge synthesis section, individual provider summaries with real names, ICE scores
- **And**: output is valid markdown

### S8.4: ICE scoring in judge output
- **Given**: a judge synthesis output
- **When**: ICE scores are present
- **Then**: each idea's score is computed as Impact (1-10) x Confidence (1-10) x Ease (1-10) / 100
- **And**: ideas are ranked by ICE score in descending order

---

## S9: Configuration

### S9.1: autopus.yaml subprocess section
- **Given**: an `autopus.yaml` with `orchestra.providers.claude.subprocess.binary: "claude"`
- **When**: config is loaded
- **Then**: the provider's subprocess configuration is correctly parsed
- **And**: `binary`, `args`, `schema_flag`, `timeout` fields are available

### S9.2: Default configuration
- **Given**: no `subprocess` section in `autopus.yaml`
- **When**: config is loaded with subprocess mode enabled
- **Then**: sensible default provider configurations are used based on detected installed binaries

---

## S10: CLI Integration

### S10.1: auto orchestra run command
- **Given**: `auto orchestra run "new auth system" --strategy debate --rounds standard`
- **When**: the command is executed
- **Then**: the full subprocess pipeline runs
- **And**: the merged result is printed to stdout

### S10.2: Backward compatibility (pane is default)
- **Given**: `auto orchestra run "topic"` (no --subprocess flag)
- **When**: the command is executed in a cmux-capable terminal
- **Then**: the existing pane-based orchestration runs
- **And**: behavior is identical to pre-SPEC-ORCH-019

### S10.4: Subprocess opt-in
- **Given**: `auto orchestra run "topic" --subprocess`
- **When**: the command is executed
- **Then**: the subprocess backend is used instead of pane mode

### S10.3: Dry-run mode
- **Given**: `auto orchestra run "topic" --dry-run`
- **When**: the command is executed
- **Then**: prompts and schemas are written to files in the working directory
- **And**: no provider subprocesses are spawned
- **And**: file paths are printed to stdout

---

## S11: End-to-End Pipeline

### S11.1: Full subprocess pipeline E2E
- **Given**: Claude, Codex, and Gemini mock binaries that return valid JSON
- **When**: `auto orchestra run "design a caching layer" --strategy debate --rounds standard` is executed
- **Then**: phase 1 (independent) produces 3 debater outputs
- **And**: phase 2 (cross-pollinate) produces 3 refined outputs with anonymized prior results
- **And**: phase 3 (judge) produces a synthesis with ICE scores
- **And**: final output is a merged markdown document
- **And**: `OrchestraResult.FailedProviders` is empty
- **And**: total token count < 100K (measured from prompt sizes)

### S11.2: Headless/CI environment (auto-fallback to subprocess)
- **Given**: no terminal (Terminal = nil), running in CI
- **When**: `RunOrchestra()` is called with debate strategy
- **Then**: SubprocessBackend is automatically selected (pane mode not possible)
- **And**: the pipeline completes successfully
- **And**: no cmux/pane dependencies are required

---

## S12: Progress Display (P1)

### S12.1: TTY progress output
- **Given**: a TTY terminal and 3 providers executing
- **When**: the pipeline is running
- **Then**: each provider shows status (running/done/failed) and elapsed time
- **And**: the display updates in real-time

### S12.2: Non-TTY fallback
- **Given**: a non-TTY environment (CI, piped output)
- **When**: the pipeline is running
- **Then**: progress is logged as structured log lines (not ANSI/cursor control)
