---
spec: SPEC-ORCH-018
type: acceptance
---

# SPEC-ORCH-018 Acceptance Criteria

## P0 -- Must Have

### R1: Retry-first on SendLongText failure

- [ ] AC-1.1: When `SendLongText` fails on an existing pane, the system retries on the SAME pane before attempting recreation
- [ ] AC-1.2: Same-pane retries use exponential backoff: 2s for first retry, 4s for second retry
- [ ] AC-1.3: Maximum 2 same-pane retries before falling back to pane recreation
- [ ] AC-1.4: Pane recreation only occurs after all same-pane retries are exhausted
- [ ] AC-1.5: After pane recreation, the system waits for session readiness before retrying `SendLongText`
- [ ] AC-1.6: Log messages clearly indicate whether retry is on same pane or after recreation

### R2: pollUntilPrompt timeout increase

- [ ] AC-2.1: `pollUntilPrompt` uses 30s timeout for Round 2+ (was 10s)
- [ ] AC-2.2: Polling interval is 3s
- [ ] AC-2.3: Round 1 behavior is unchanged (no polling needed since provider just launched)
- [ ] AC-2.4: Timeout is logged when exceeded

### R3: ReadScreen scrollback depth

- [ ] AC-3.1: `ReadScreen` requests scrollback with minimum 500 lines
- [ ] AC-3.2: `OrchestraConfig.ScrollbackLines` field exists with default value 500
- [ ] AC-3.3: Custom scrollback depth is respected when set via config
- [ ] AC-3.4: Gemini SCAMPER + ICE table output (observed at ~200 lines) is fully captured

### R4: --no-judge flag

- [ ] AC-4.1: `--no-judge` flag is available on `auto orchestra brainstorm`
- [ ] AC-4.2: When `--no-judge` is set, `runJudgeRound()` is not called
- [ ] AC-4.3: Raw provider responses are output to stdout
- [ ] AC-4.4: `OrchestraConfig.NoJudge` field is set correctly from CLI flag
- [ ] AC-4.5: Default behavior (without flag) is unchanged -- judge runs normally

### R5: --yield-rounds mode

- [ ] AC-5.1: `--yield-rounds` flag is available on `auto orchestra brainstorm`
- [ ] AC-5.2: When set, the system runs Round 1 and collects results
- [ ] AC-5.3: JSON output to stdout contains: round number, pane IDs, provider responses, session ID
- [ ] AC-5.4: Panes remain alive after command exits
- [ ] AC-5.5: Exit code is 0 on success
- [ ] AC-5.6: Session metadata is persisted for use by `collect`/`cleanup` commands

### R6: auto orchestra collect command

- [ ] AC-6.1: `auto orchestra collect --session-id <ID> --round <N>` command exists
- [ ] AC-6.2: Command reads screens of existing panes identified by session ID
- [ ] AC-6.3: Output is JSON to stdout with provider responses
- [ ] AC-6.4: Panes are NOT cleaned up after collection
- [ ] AC-6.5: Returns error if session ID not found

### R7: auto orchestra cleanup command

- [ ] AC-7.1: `auto orchestra cleanup --session-id <ID>` command exists
- [ ] AC-7.2: All panes for the given session are killed
- [ ] AC-7.3: Session persistence file is removed
- [ ] AC-7.4: Cleanup confirmation is output to stderr
- [ ] AC-7.5: Idempotent -- running cleanup on already-cleaned session does not error

## P1 -- Should Have

### R8: brainstorm --context flag

- [ ] AC-8.1: `--context` flag is available on `auto orchestra brainstorm`
- [ ] AC-8.2: When set, `ARCHITECTURE.md`, `.autopus/project/product.md`, `.autopus/project/structure.md` are loaded
- [ ] AC-8.3: Project context is injected into the brainstorm prompt before SCAMPER section
- [ ] AC-8.4: `topicIsolationInstruction` is replaced with context-aware instruction
- [ ] AC-8.5: Missing context files are skipped with a warning (not a hard error)

### R9: Structured JSON output for --no-judge

- [ ] AC-9.1: When `--no-judge` is active, output matches the specified JSON schema
- [ ] AC-9.2: JSON contains `strategy`, `rounds`, `round_history`, `panes`, `session_id` fields
- [ ] AC-9.3: Each response includes `provider`, `output`, `duration_ms`, `timed_out` fields
- [ ] AC-9.4: Output is valid JSON parseable by `jq`

### R10: idea.md skill update

- [ ] AC-10.1: `/auto idea` Step 3 calls `auto orchestra brainstorm --no-judge --yield-rounds`
- [ ] AC-10.2: Step 3.5 reviews Round 1 results with project context
- [ ] AC-10.3: Step 3.6 sends rebuttal prompt to each pane
- [ ] AC-10.4: Step 3.7 calls `auto orchestra collect` for Round 2 results
- [ ] AC-10.5: Step 3.8 calls `auto orchestra cleanup`
- [ ] AC-10.6: Step 4 performs ICE scoring in the main session

## P2 -- Could Have

### R11: auto orchestra inject command

- [ ] AC-11.1: `auto orchestra inject --session-id <ID> --provider <name> "prompt"` command exists
- [ ] AC-11.2: Prompt is sent to the specified provider's pane
- [ ] AC-11.3: Uses cmux `set-buffer`/`paste-buffer` for long prompts
- [ ] AC-11.4: Returns error if provider/session not found

### R12: Session persistence file

- [ ] AC-12.1: Session file is written to `/tmp/autopus-orch-session-{ID}.json`
- [ ] AC-12.2: Contains pane IDs, provider configs, and round history
- [ ] AC-12.3: File is valid JSON
- [ ] AC-12.4: File is removed by `cleanup` command
- [ ] AC-12.5: File permissions are 0600 (user-only read/write)
