# SPEC-ORCH-019 Implementation Plan

## Task List

### Phase A — Foundation (Parallel: T1, T2)

- [ ] **T1: ExecutionBackend interface + PaneBackend adapter** `[executor]`
  - Define `ExecutionBackend` interface in `pkg/orchestra/backend.go`
  - Define `ProviderRequest` struct (prompt, schema path, provider config, timeout)
  - Wrap existing `runProvider()` behind `PaneBackend` implementing `ExecutionBackend` (provider-level, not strategy-level)
  - Refactor `RunOrchestra()` to accept/select backend without changing public signature
  - Default backend remains pane mode; `--subprocess` flag opts into `SubprocessBackend`
  - **Files**: `backend.go` (new), `runner.go` (modify), `types.go` (modify)
  - **Tests**: Unit tests for backend selection logic, PaneBackend delegation
  - **Depends on**: none

- [ ] **T2: SchemaBuilder — JSON schema generation** `[executor]`
  - Define Go structs: `DebaterR1Output`, `DebaterR2Output`, `JudgeOutput`, `ReviewerOutput`
  - Implement `SchemaBuilder.Generate(role string) (string, error)` that marshals struct to JSON Schema
  - Write schema to temp file, return path
  - Handle Gemini fallback: `SchemaBuilder.EmbedInPrompt(role string) string` for prompt-level schema
  - **Files**: `schema_builder.go` (new)
  - **Tests**: Validate generated schemas against expected structure; Gemini prompt embedding test
  - **Depends on**: none

### Phase B — Core Engine (Parallel: T3, T4, T5)

- [ ] **T3: SubprocessBackend implementation** `[executor]`
  - Implement `SubprocessBackend.Execute()`: spawn provider CLI using config-driven args, pipe stdin, collect stdout JSON
  - File-based input fallback when stdin is not available
  - Process exit code as completion signal
  - Config-driven isolation flags for token reduction (from `autopus.yaml`)
  - Per-provider timeout via `context.WithTimeout`
  - JSON output validation against schema (post-hoc for Gemini)
  - **Files**: `subprocess_runner.go` (new)
  - **Tests**: Mock command execution; test stdin/file routing; timeout behavior; exit code handling
  - **Depends on**: T1 (backend interface), T2 (schema builder)

- [ ] **T4: PromptBuilder — role/round prompt generation** `[executor]`
  - `PromptBuilder.BuildDebaterR1(topic, context string) string` — independent divergence prompt with SCAMPER/HMW
  - `PromptBuilder.BuildDebaterR2(topic string, priorResults []AnonymizedResult) string` — cross-pollination prompt
  - `PromptBuilder.BuildJudge(topic string, allResults []AnonymizedResult) string` — blind judge synthesis
  - `PromptBuilder.BuildReviewer(topic string, judgeVerdict JudgeOutput) string` — PASS/REVISE/REJECT
  - **Files**: `prompt_builder.go` (new)
  - **Tests**: Snapshot tests for each prompt role/round combination
  - **Depends on**: T2 (schema structs for output types)

- [ ] **T5: OutputParser — JSON response parsing** `[executor]`
  - Parse provider JSON stdout into typed Go structs (`DebaterR1Output`, `JudgeOutput`, etc.)
  - Handle malformed JSON: extract structured data from partial/free-form responses
  - Validate parsed output against expected schema fields
  - Provider-specific parsing: Claude stream-json envelope extraction, Codex direct JSON, Gemini lenient parsing
  - **Files**: `output_parser.go` (new)
  - **Tests**: Parse valid JSON; handle malformed JSON; provider-specific edge cases
  - **Depends on**: T2 (output type definitions)

### Phase C — Orchestration Logic (Sequential, depends on Phase B)

- [ ] **T6: CrossPollinateBuilder** `[executor]`
  - `CrossPollinateBuilder.Build(round int, results []ProviderResult) []ProviderRequest`
  - Anonymize provider identities: real names -> "Debater A", "Debater B", "Debater C"
  - Remove ICE scores from prior outputs
  - Preserve full content (no summarization)
  - Maintain identity mapping for de-anonymization in final merge
  - **Files**: `crosspolinate.go` (new)
  - **Tests**: Verify anonymization; verify content preservation; verify ICE score removal
  - **Depends on**: T4 (prompt builder), T5 (output parser)

- [ ] **T7: JudgeBuilder + Output Merge** `[executor]`
  - `JudgeBuilder.Build(topic string, allResults []AnonymizedResult) ProviderRequest`
  - Generate blind judge prompt with consensus extraction + ICE scoring guidance
  - `MergeSubprocessResults(judgeVerdict JudgeOutput, identityMap map[string]string, results []ProviderResult) string`
  - Produce final markdown: judge synthesis + individual summaries + de-anonymized attribution
  - **Files**: `judge_builder.go` (new)
  - **Tests**: Judge prompt structure; merge output format; de-anonymization correctness
  - **Depends on**: T4 (prompt builder), T5 (output parser), T6 (anonymization)

- [ ] **T8: Pipeline Runner** `[executor]`
  - Implement `RunSubprocessPipeline(ctx, config) (*OrchestraResult, error)`
  - Full flow: prepare -> parallel independent -> cross-pollinate -> judge -> merge
  - `--rounds` preset handling: fast (0), standard (1), deep (2)
  - Concurrent execution with graceful degradation
  - Wire into existing `RunOrchestra()` via backend selection (T1)
  - **Files**: `pipeline.go` (new), `runner.go` (modify)
  - **Tests**: Pipeline flow tests with mock backend; round preset behavior; degradation scenarios
  - **Depends on**: T1, T3, T4, T5, T6, T7

### Phase D — Configuration + CLI (Parallel: T9, T10)

- [ ] **T9: autopus.yaml subprocess configuration** `[executor]`
  - Extend config parsing for `orchestra.providers.{name}.subprocess` section
  - Fields: `binary`, `args`, `schema_flag`, `stdin_mode`, `output_format`, `timeout`
  - Global: `orchestra.subprocess.enabled`, `max_concurrent`, `work_dir`, `rounds`
  - Provider binary auto-detection via `detect.IsInstalled()`
  - **Files**: config loader files (modify existing), `types.go` (modify)
  - **Tests**: Config parsing with/without subprocess section; binary detection
  - **Depends on**: T1 (for ProviderRequest types)

- [ ] **T10: CLI entry points** `[executor]`
  - `auto orchestra run "topic" --strategy debate --rounds standard --providers claude,codex,gemini`
  - `--subprocess` flag to opt into subprocess backend (pane mode remains default)
  - `--dry-run` outputs prompts to files without executing
  - Integrate with existing `auto orchestra brainstorm`, `auto orchestra review`
  - **Files**: CLI command files (modify existing)
  - **Tests**: Flag parsing; dry-run output; backward compat (default=pane); --subprocess opt-in
  - **Depends on**: T8 (pipeline runner), T9 (config)

### Phase E — P1 Enhancements (Parallel: T11, T12)

- [ ] **T11: Terminal progress display** `[executor]`
  - Spinner + status line per provider (running/done/failed, elapsed time)
  - Non-blocking: progress updates via goroutine channel
  - Graceful fallback for non-TTY environments (log-style output)
  - **Files**: `progress.go` (new)
  - **Tests**: Progress state transitions; non-TTY fallback
  - **Depends on**: T3 (subprocess runner events)

- [ ] **T12: Context summarizer** `[executor]`
  - Scan project files (ARCHITECTURE.md, key source files)
  - Compress to configurable `--max-tokens` budget
  - Inject as preamble into debater prompts
  - **Files**: `context_summarizer.go` (new)
  - **Tests**: Token budget enforcement; content relevance
  - **Depends on**: T4 (prompt builder integration point)

### Phase F — Integration Testing (Sequential, depends on all above)

- [ ] **T13: Integration tests** `[tester]`
  - E2E test: `auto orchestra run --subprocess` with mock provider binaries
  - Backend selection test: default -> PaneBackend, --subprocess -> SubprocessBackend, nil terminal -> SubprocessBackend
  - Degraded mode test: 1 provider failure + 2 success -> valid result
  - Round preset test: fast/standard/deep produce correct pipeline phases
  - Backward compat test: `--pane` flag produces identical behavior to pre-refactor
  - **Files**: `pipeline_integration_test.go` (new), `backend_test.go` (new)
  - **Tests**: All scenarios above
  - **Depends on**: T1-T12

- [ ] **T14: Regression tests for pane mode** `[tester]`
  - Verify existing pane-based tests continue to pass unchanged
  - Verify `RunOrchestra()` with Terminal != nil and no --subprocess flag uses PaneBackend (default)
  - Verify `RunOrchestra()` with Terminal != nil and --subprocess flag uses SubprocessBackend
  - **Files**: existing test files (verify, no modify)
  - **Tests**: Run existing test suite; add backend selection edge cases
  - **Depends on**: T1, T8

## Execution Strategy

```
Phase A: T1 ─────┐
         T2 ─────┘ (parallel)
                                  │
Phase B: T3 ─────┐               │
         T4 ─────┤ (parallel)    │ depends on A
         T5 ─────┘               │
                                  │
Phase C: T6 ──→ T7 ──→ T8       │ depends on B (sequential)
                                  │
Phase D: T9 ─────┐               │
         T10 ────┘ (parallel)    │ depends on C
                                  │
Phase E: T11 ────┐               │
         T12 ────┘ (parallel)    │ depends on D
                                  │
Phase F: T13 ──→ T14             │ depends on all (sequential)
```

## Risk Mitigations

| Risk | Mitigation |
|------|------------|
| `RunOrchestra()` signature change breaks 4 callers | T1 wraps backend selection inside existing signature |
| Gemini JSON schema non-enforcement | T2 provides prompt-embedded schema; T5 validates post-hoc |
| Large prompts exceed stdin pipe buffer | T3 uses file-based input for prompts > 4KB |
| Pane mode regression | T14 explicitly verifies pane behavior unchanged |
| File size limit (300 lines) | Each new file targets < 200 lines; split by concern |

## File Ownership Summary

| Task | New Files | Modified Files |
|------|-----------|----------------|
| T1 | `backend.go` | `runner.go`, `types.go` |
| T2 | `schema_builder.go` | — |
| T3 | `subprocess_runner.go` | — |
| T4 | `prompt_builder.go` | — |
| T5 | `output_parser.go` | — |
| T6 | `crosspolinate.go` | — |
| T7 | `judge_builder.go` | — |
| T8 | `pipeline.go` | `runner.go` |
| T9 | — | config loader, `types.go` |
| T10 | — | CLI command files |
| T11 | `progress.go` | — |
| T12 | `context_summarizer.go` | — |
| T13 | `pipeline_integration_test.go`, `backend_test.go` | — |
| T14 | — | existing test files (verify) |
