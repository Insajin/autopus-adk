---
spec: SPEC-ORCH-018
type: tasks
---

# SPEC-ORCH-018 Task Details

## T1: sendPromptWithRetry retry-first logic

**Priority**: P0
**File**: `pkg/orchestra/surface_manager.go`
**Requirement**: R1

### Current Behavior (lines 151-180)

```go
func sendPromptWithRetry(...) {
    SendLongText(existing pane) -> FAIL
    -> recreatePane() immediately    // BUG: destroys context
    -> retry 3x on NEW pane
}
```

### Target Behavior

```go
func sendPromptWithRetry(...) {
    SendLongText(existing pane) -> FAIL
    -> pollUntilPrompt(30s) on same pane
    -> retry SendLongText on SAME pane (attempt 2, wait 2s)
    -> retry SendLongText on SAME pane (attempt 3, wait 4s)
    -> if all fail: recreatePane() as LAST RESORT
    -> waitForSessionReady on new pane
    -> retry SendLongText on new pane (final attempt)
}
```

### Implementation Notes

- Keep the existing `recreatePane()` function unchanged -- only change the call order
- Add log messages to distinguish same-pane retries from recreation retries
- The exponential backoff (2s, 4s) gives cmux surfaces time to stabilize after transient failures

---

## T2: pollUntilPrompt timeout increase

**Priority**: P0
**File**: `pkg/orchestra/interactive_debate.go`
**Requirement**: R2

### Change

Line 230: Change `pollUntilPrompt(ctx, cfg.Terminal, pi.paneID, patterns, 10*time.Second)` to `pollUntilPrompt(ctx, cfg.Terminal, pi.paneID, patterns, 30*time.Second)`.

### Implementation Notes

- Single constant change
- Consider extracting the timeout as a named constant: `const round2PollTimeout = 30 * time.Second`
- Add a log line if the poll takes longer than 20s (warning threshold)

---

## T3: ReadScreen scrollback depth + config field

**Priority**: P0
**Files**: `pkg/orchestra/interactive.go`, `pkg/orchestra/types.go`
**Requirement**: R3

### types.go Change

Add field to `OrchestraConfig`:

```go
ScrollbackLines int // R3: ReadScreen scrollback depth (default 500, 0 = use terminal default)
```

### interactive.go Change

In `waitAndCollectResults()` (line 273), pass scrollback options:

```go
// Before:
screen, _ := cfg.Terminal.ReadScreen(readCtx, pi.paneID, terminal.ReadScreenOpts{Scrollback: true})

// After:
scrollback := cfg.ScrollbackLines
if scrollback == 0 {
    scrollback = 500 // default
}
screen, _ := cfg.Terminal.ReadScreen(readCtx, pi.paneID, terminal.ReadScreenOpts{
    Scrollback:     true,
    ScrollbackLines: scrollback,
})
```

### Implementation Notes

- Check if `terminal.ReadScreenOpts` already has a `ScrollbackLines` field. If not, add it to `pkg/terminal/` types.
- The default of 500 lines is sufficient for SCAMPER (7 lenses x ~10 lines) + ICE table + HMW questions.

---

## T4: --no-judge flag + skip judge logic

**Priority**: P0
**Files**: `internal/cli/orchestra_brainstorm.go`, `pkg/orchestra/interactive_debate_helpers.go`, `pkg/orchestra/types.go`
**Requirement**: R4

### types.go Change

Add field to `OrchestraConfig`:

```go
NoJudge bool // R4: skip judge phase when true
```

### CLI Change (orchestra_brainstorm.go)

Add flag:

```go
cmd.Flags().Bool("no-judge", false, "judge 단계를 건너뛰고 raw 응답만 출력")
```

Wire to config:

```go
cfg.NoJudge, _ = cmd.Flags().GetBool("no-judge")
```

### Judge Skip Logic (interactive_debate_helpers.go)

In `runJudgeRound()` caller (in `interactive_debate.go` or wherever the judge is invoked), add guard:

```go
if cfg.NoJudge {
    // Skip judge, return raw responses
    return result, nil
}
```

---

## T5: --yield-rounds mode + JSON output

**Priority**: P0
**Files**: `internal/cli/orchestra_brainstorm.go`, `pkg/orchestra/yield.go` (new), `pkg/orchestra/types.go`
**Requirements**: R5, R9

### types.go Change

Add field:

```go
YieldRounds bool // R5: yield after round 1 with JSON output
```

### New File: yield.go

Create `pkg/orchestra/yield.go` with:

- `YieldOutput` struct matching the R9 JSON schema
- `YieldRoundResponse` struct for per-response data
- `WriteYieldOutput(w io.Writer, output YieldOutput) error` function

### CLI Change

Add flag:

```go
cmd.Flags().Bool("yield-rounds", false, "Round 1 후 JSON 출력 및 pane 유지")
```

### Debate Flow Change

In `interactive_debate.go`, after Round 1 collection and before Round 2:

```go
if cfg.YieldRounds {
    output := buildYieldOutput(cfg, panes, responses, sessionID)
    WriteYieldOutput(os.Stdout, output)
    // Do NOT call cleanupInteractivePanes
    return result, nil
}
```

---

## T6: auto orchestra collect command

**Priority**: P0
**Files**: `internal/cli/orchestra_collect.go` (new), `internal/cli/orchestra.go`
**Requirement**: R6

### Command Structure

```
auto orchestra collect --session-id <ID> --round <N>
```

### Implementation

1. Load session file from `/tmp/autopus-orch-session-{ID}.json`
2. For each pane in the session, call `ReadScreen` with scrollback
3. Build JSON output with provider responses
4. Write JSON to stdout
5. Do NOT clean up panes

### Registration

Add to `newOrchestraCmd()` in `orchestra.go`:

```go
cmd.AddCommand(newOrchestraCollectCmd())
```

---

## T7: auto orchestra cleanup command

**Priority**: P0
**Files**: `internal/cli/orchestra_cleanup.go` (new), `internal/cli/orchestra.go`
**Requirement**: R7

### Command Structure

```
auto orchestra cleanup --session-id <ID>
```

### Implementation

1. Load session file from `/tmp/autopus-orch-session-{ID}.json`
2. For each pane, call terminal kill-pane
3. Remove session file
4. Print confirmation to stderr

### Registration

Add to `newOrchestraCmd()`:

```go
cmd.AddCommand(newOrchestraCleanupCmd())
```

---

## T8: Session persistence file

**Priority**: P2 (promoted to P0 in plan due to T5/T6/T7 dependency)
**File**: `pkg/orchestra/session.go` (new)
**Requirement**: R12

### Structures

```go
type OrchestraSession struct {
    ID         string                 `json:"id"`
    Panes      map[string]string      `json:"panes"`       // provider -> pane ID
    Providers  []ProviderConfig       `json:"providers"`
    Rounds     [][]ProviderResponse   `json:"rounds"`
    CreatedAt  time.Time              `json:"created_at"`
}
```

### Functions

- `NewSessionID() string` -- generates `orch-{timestamp}-{random}`
- `SaveSession(session OrchestraSession) error` -- writes to `/tmp/autopus-orch-session-{ID}.json` with 0600 perms
- `LoadSession(id string) (*OrchestraSession, error)` -- reads and parses session file
- `RemoveSession(id string) error` -- deletes session file

---

## T9: --context flag + context-aware prompt

**Priority**: P1
**Files**: `internal/cli/orchestra_brainstorm.go`, `pkg/orchestra/debate.go`
**Requirement**: R8

### CLI Change

Add flag:

```go
cmd.Flags().Bool("context", false, "프로젝트 컨텍스트를 브레인스토밍 프롬프트에 포함")
```

### Context Loading

In `orchestra_brainstorm.go`, load and concatenate project files:

```go
contextFiles := []string{
    "ARCHITECTURE.md",
    ".autopus/project/product.md",
    ".autopus/project/structure.md",
}
```

Skip missing files with a warning log.

### debate.go Change

Add a conditional in the debate round execution:

```go
var isolation string
if cfg.ContextMode {
    isolation = contextAwareInstruction + projectContext
} else {
    isolation = topicIsolationInstruction
}
```

Add new constant:

```go
const contextAwareInstruction = "Use the project context below to ground your ideas in the actual codebase. Focus on the given topic.\n\n"
```

---

## T10: Structured JSON output for --no-judge

**Priority**: P1
**File**: `pkg/orchestra/yield.go`
**Requirement**: R9

### Implementation

Extend the `YieldOutput` struct (created in T5) to handle the full --no-judge output format. This is a refinement of T5's JSON output to match the exact R9 schema including `round_history` with per-round responses.

---

## T11: idea.md skill update

**Priority**: P1
**File**: `.claude/skills/autopus/idea.md`
**Requirement**: R10

### Changes

Update the skill's Step 3-4 flow:

- Step 3: Replace direct orchestra call with `auto orchestra brainstorm --no-judge --yield-rounds`
- Add Steps 3.5-3.8 for the main-session judge workflow
- Step 4: Keep ICE scoring but ensure it runs in the main session

---

## T12: auto orchestra inject command

**Priority**: P2
**Files**: `internal/cli/orchestra_inject.go` (new), `internal/cli/orchestra.go`
**Requirement**: R11

### Command Structure

```
auto orchestra inject --session-id <ID> --provider <name> "prompt text"
```

### Implementation

1. Load session from persistence file
2. Find pane ID for the specified provider
3. Use `SendLongText` (which internally uses cmux set-buffer/paste-buffer) to inject prompt
4. Output confirmation to stderr

---

## T13: Unit tests for stability fixes (T1-T4)

**Priority**: P0
**Files**: `pkg/orchestra/surface_manager_test.go`, `pkg/orchestra/interactive_debate_test.go`

### Test Cases

- T1: Test sendPromptWithRetry retries on same pane before recreation
- T1: Test sendPromptWithRetry falls back to recreation after 2 same-pane failures
- T1: Test successful send on first try (no retry)
- T2: Verify pollUntilPrompt timeout constant is 30s
- T3: Verify default scrollback lines is 500
- T4: Test judge skip when NoJudge is true
- T4: Test judge runs when NoJudge is false (default)

---

## T14: Unit tests for yield/collect/cleanup (T5-T8)

**Priority**: P0
**Files**: `pkg/orchestra/yield_test.go`, `pkg/orchestra/session_test.go`, `internal/cli/orchestra_collect_test.go`, `internal/cli/orchestra_cleanup_test.go`

### Test Cases

- T5: Test YieldOutput JSON serialization matches schema
- T6: Test collect reads session and returns JSON
- T7: Test cleanup removes panes and session file
- T8: Test session save/load/remove lifecycle
- T8: Test session file permissions are 0600
- T8: Test NewSessionID generates unique IDs
