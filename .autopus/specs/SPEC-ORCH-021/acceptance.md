# SPEC-ORCH-021 수락 기준

> **Contract correction (2026-07-17)**: pane transport는 `SplitPane`이 비어 있지 않은 pane ID를 반환한 순간 commit된다. subprocess fallback은 이 commit 전 provisioning 실패에만 허용되며, commit 후 launch/ready/prompt/completion/collection/cleanup 실패에서는 금지된다.

## Backend selection truth table (oracle reference)

수락 시나리오는 아래 진실표의 각 행을 검증한다. 표는 공유 술어 `paneCapable(term, subprocessMode)` (= `!subprocessMode && term != nil && term.Name() != "plain"`)로 키잉되며, R3(non-nil `"plain"`)은 술어 false로 subprocess가 되어야 한다(F-001). `executed backend`는 실제로 응답을 생산한 backend 식별자(`pane` 또는 `subprocess`)다. pane transport commit 후 실행/완료/수집 실패는 `pane` 경로의 failed/timed-out 결과로 남고, cleanup-only 실패는 pane-path warning/receipt가 될 수 있다. 어느 경우에도 subprocess 응답을 생산하지 않는다.

| Row | terminal | SubprocessMode | pane provisioning / execution outcome | expected executed backend |
|-----|----------|----------------|-----------------------------------------|---------------------------|
| R1 | cmux | false | ok | pane |
| R2 | tmux | false | ok | pane |
| R3 | plain | false | n/a | subprocess |
| R4 | nil | false | n/a | subprocess |
| R5 | cmux | true | n/a | subprocess |
| R6 | cmux | false | `SplitPane` error or empty pane ID (not committed) | subprocess (fallback) |
| R7 | cmux | false | non-empty pane ID (even with an accompanying error), then any later stage fails (committed) | pane-attributed failure/timeout or cleanup warning/receipt; subprocess calls = 0 |

## 시나리오

### S1: brainstorm가 cmux에서 pane 기본값으로 실행 (R1, REQ-001/REQ-007)
Given a cmux-capable terminal is detected and the user runs `orchestra brainstorm` without `--subprocess`
When the brainstorm command resolves its backend via the shared selection rule
Then the resolved backend name is `pane`
And the provider argv does not contain `-p`.

### S2: spec review structured가 cmux에서 pane backend 선택 (R1, REQ-002/REQ-006/REQ-007)
Given a cmux-capable terminal and a structured spec review config with `SubprocessMode` unset
When `SelectBackend(cfg)` is evaluated with `cfg.Terminal` populated by terminal detection
Then the returned backend name is `pane`
And the structured review path uses that backend rather than a hardcoded subprocess backend.

### S3: spec review structured가 plain 터미널에서 subprocess (R3, REQ-002/REQ-006)
Given a plain terminal (`Name() == "plain"`) and a structured spec review config with `SubprocessMode` unset
When `SelectBackend(cfg)` is evaluated
Then the returned backend name is `subprocess`.

### S4: orchestra run structured가 cmux에서 pane backend 사용 (R1, REQ-003)
Given a cmux-capable terminal and `auto orchestra run` invoked without `--subprocess`
When the run pipeline builds its `SubprocessPipelineConfig.Backend` from the shared selection rule with `cfg.Terminal` populated
Then `SubprocessPipelineConfig.Backend.Name()` equals `pane`.

### S5: orchestra run structured가 nil 터미널에서 subprocess (R4, REQ-003)
Given no terminal is detected (`cfg.Terminal == nil`) and `auto orchestra run` invoked without `--subprocess`
When the run pipeline builds its backend from the shared selection rule
Then `SubprocessPipelineConfig.Backend.Name()` equals `subprocess`.

### S6: pane provisioning 실패 시에만 subprocess fallback이 성공하고 경로 기록 (R6, REQ-005)
Given a cmux-capable terminal with `SubprocessMode` unset, so the interactive pane backend is selected
And `SplitPane` returns an error or an empty pane ID before any provider pane transport is committed
And `-p`/stdin subprocess execution IS available (the user has API access)
When the pane backend `Execute` handles that request
Then the response is produced by falling back to `-p`/stdin subprocess execution as best-effort
And the recorded executed-backend identifier for that response equals `subprocess`
And the response output is non-empty parseable structured output.

### S6b: pane commit 후 실패와 cleanup warning 모두 pane attribution을 유지 (R7, REQ-005)
Given a cmux/tmux terminal whose `SplitPane` call returned a non-empty pane ID for a provider request, with or without an accompanying error
And the pane transport is therefore committed
When any later pane stage fails or times out — provider launch command construction, launch send/enter, session ready, prompt send/enter, completion detection, response collection, or cleanup
Then execution/completion/collection failures remain a pane-path failure or bounded timeout
And a cleanup-only failure may instead be recorded as a pane-path warning/receipt without changing an otherwise usable provider result
And close retry remains bounded, and a successfully closed pane is untracked
And the recorded backend/receipt identifies the pane path in every case
And the subprocess executor call count is exactly zero, including completion, collection, and cleanup fault-marker cases.

### S6c: multi-pane partial provisioning 실패는 transactional cleanup 후에만 fallback (REQ-005)
Given a multi-provider pane run is provisioning all provider panes
And one or more `SplitPane` calls return non-empty pane IDs but a later `SplitPane` call fails before any provider CLI is launched
When the provisioning transaction aborts
Then every partially created pane is cleaned up
And no provider CLI launch or prompt send occurred in any partial pane
And subprocess fallback may start only after that cleanup completes
And the event order proves partial-pane close precedes every fallback subprocess invocation.

### S7: SubprocessMode 강제가 cmux에서도 subprocess (R5, REQ-007)
Given a cmux-capable terminal and `SubprocessMode == true` (user forced `--subprocess=true`)
When `SelectBackend(cfg)` is evaluated
Then the returned backend name is `subprocess`
And no interactive pane is opened.

### S8a: pane backend가 sanitize된 parseable reviewer JSON을 반환 (REQ-004/REQ-008/INV-003/INV-006)
Given an interactive pane whose scrollback screen contains ANSI escape sequences, a CLI banner line, and a reviewer JSON object `{"verdict":"PASS","summary":"ok","findings":[]}`
When the pane backend reads and sanitizes the screen and returns a `ProviderResponse`
Then the response output contains no ANSI escape or banner lines
And `OutputParser.ParseReviewer(response.Output)` returns a result with verdict `PASS` and no error.

### S8b: pane backend가 parseable debater_r1 JSON을 반환 (REQ-004/INV-003)
Given an interactive pane whose scrollback screen contains terminal noise and a debater Round 1 JSON object with at least one idea (e.g. `{"ideas":[{"title":"x","detail":"y"}]}`)
When the pane backend reads and sanitizes the screen and returns a `ProviderResponse`
And `OutputParser.ParseDebaterR1(response.Output)` is invoked
Then the parse returns at least one idea and no error.

### S8c: pane backend가 parseable judge JSON을 반환 (REQ-004/INV-003)
Given an interactive pane whose scrollback screen contains terminal noise and a judge JSON object with a non-empty recommendation (e.g. `{"recommendation":"adopt idea x"}`)
When the pane backend reads and sanitizes the screen and returns a `ProviderResponse`
And `OutputParser.ParseJudge(response.Output)` is invoked
Then the parse returns a non-empty recommendation and no error.

### S9: legacy guard와 SelectBackend가 동일 술어 결과에 합의 (REQ-007/INV-001)
Given the shared predicate `paneCapable(term, subprocessMode)` and the two consumers of it — the legacy `RunOrchestra` guard (`runner.go`, the brainstorm path) and `SelectBackend` (the structured spec review and orchestra run paths)
When both consumers are evaluated with identical inputs across the truth-table rows R1–R7
Then for every row the legacy guard's pane-vs-subprocess decision equals `paneCapable(term, subprocessMode)`
And `SelectBackend` returns the pane backend exactly when `paneCapable(term, subprocessMode)` is true and the subprocess backend exactly when it is false
And specifically: with (terminal cmux, `SubprocessMode` false) both resolve to pane-mode, and with (terminal plain, `SubprocessMode` false) AND with (terminal nil, `SubprocessMode` false) AND with (terminal cmux, `SubprocessMode` true) both resolve to subprocess-mode.

### S10: 세션 ready 전 prompt 미전송, ready 후 전송 (REQ-009/INV-008)
Given the pane backend launches a provider session and the screen does not yet match a `SessionReadyPatterns` prompt
When `Execute` drives the session toward sending the prompt
Then no prompt-send occurs while `isSessionReady` is false
And once the screen matches a session-ready pattern within `startupTimeoutFor(provider)`, the prompt is sent exactly once afterward
And if the ready timeout expires after pane commit, the result is a pane-path failure and the subprocess executor call count is exactly zero.

### S11: 완료 감지가 monitor 우선 후 poll로 timeout 내 전환 (REQ-010/INV-009)
Given `MonitorEnabled` is true and the event-driven completion detector does not fire within `MonitorTimeout`
When `Execute` waits for completion via the reused `waitForCompletion` path
Then the system transitions to `ScreenPollDetector` polling before the overall per-provider timeout
And completion is detected via polling if the screen later shows a completion pattern, without introducing any new detection mechanism
And this detector transition stays inside the committed pane transport and does not invoke subprocess execution.

### S12: 완료 미발생 시 bounded timeout 내 결정적 실패 반환 (no hang) (REQ-011/INV-010)
Given an interactive pane where completion never occurs (neither hook/monitor nor poll ever matches)
When `Execute` is invoked with a bounded per-provider timeout
Then `Execute` returns within that bounded timeout
And the returned `ProviderResponse` has `TimedOut` (or failed) set with any partial sanitized output
And the call does not block indefinitely and does not return unsanitized garbage
And the subprocess executor call count is exactly zero.

### S13: hook 미설치 시 monitor/poll로 degrade (REQ-012/INV-011)
Given completion hooks are not installed (`HookMode` is false, no `HookSession`)
When `Execute` resolves its completion detector via `resolveCompletionDetector`
Then the resolved detector is a non-hook detector (monitor or `ScreenPollDetector`), not `FileIPCDetector`
And execution still proceeds to completion or to the deterministic timeout result without erroring on the missing hook
And the in-pane detector degrade does not switch the execution backend or invoke subprocess execution.

### S14: pane provisioning + -p 동시 실패 시 actionable 에러 (구독 현실 비은폐) (REQ-005/REQ-013/INV-012)
Given a run where pane provisioning fails before transport commit AND `-p`/stdin subprocess fallback is unavailable (e.g. the user has only a subscription session, so `-p` cannot authenticate to the provider API)
When `Execute` (and its fallback) exhaust both paths
Then the surfaced error string names both causes: that interactive pane provisioning failed and that the `-p` subprocess fallback was unavailable
And the error string contains an actionable recovery instruction to ensure a logged-in cmux/tmux CLI session
And the result is not a raw provider/API error
And the recorded execution path indicates that neither backend succeeded
And this combined-error branch cannot be reached after a non-empty pane ID has committed the pane transport.

### S15: gemini subprocess argv가 프롬프트를 --print 값으로 포함하고 무값 --print를 만들지 않음 (REQ-014/INV-013)
Given the gemini (`agy`) provider configured for subprocess execution and a prompt string
When the subprocess argv is constructed for gemini
Then the argv passes the prompt as the value of `--print`/`-p` (the prompt token appears in the `--print` value position)
And the argv does not contain a bare `--print` with no following value
And the argv would not produce the `flag needs an argument: -print` failure.

### S16: codex subprocess argv가 exec로 시작하고 --output-schema를 포함 (REQ-014/INV-015)
Given the codex provider configured for subprocess execution with a structured role and a schema file path supplied
When the subprocess argv is constructed for codex
Then the argv begins with the `exec` subcommand
And the argv contains `--output-schema` followed by the schema file path.

### S17: gemini pane argv가 --print를 포함하지 않음 (대화형) (REQ-015/INV-014)
Given the gemini (`agy`) provider configured for the interactive pane path
When the pane launch argv is constructed for gemini
Then the pane argv does not contain `--print`
And the pane argv launches an interactive session (bare `agy` or `-i`/`--prompt-interactive`).

### S18: codex가 기본 review provider 집합에 포함되고 그 structured argv가 --output-schema를 포함 (REQ-014/REQ-016/INV-015/INV-016)
Given the default configuration's structured spec review provider set resolved from `review_gate.providers`
When the review provider set is resolved and codex's structured argv is constructed
Then codex is present in the resolved review provider set
And codex's structured argv contains `--output-schema`.

### S19: codex pane argv가 exec로 시작하지 않음 (대화형 TUI) (REQ-015/INV-014)
Given the codex provider configured for the interactive pane path
When the pane launch argv is constructed for codex
Then the pane argv does not begin with the `exec` non-interactive subcommand.

### S20: review provider 집합이 orchestra 커맨드 집합과 일관 (codex 누락 없음) (REQ-016/INV-016)
Given the default configuration where orchestra commands use providers `[claude, codex, gemini]`
When the structured review provider set is resolved
Then no provider configured for orchestra commands is silently absent from the review set
And specifically codex is not dropped from structured review.

### S21: judge도 pane-first이며 자체 provisioning 실패에서만 subprocess 허용 (REQ-005)
Given a pane-capable terminal and a debate/pipeline execution that reaches the judge without explicit `SubprocessMode`
When judge execution begins
Then the judge attempts `SplitPane` before any subprocess invocation
And if `SplitPane` returns a non-empty judge pane ID, every later judge failure remains on the pane path and the subprocess executor call count is exactly zero
And a non-empty judge pane ID commits transport even if `SplitPane` also reports an error
And only an empty pane ID or error without a pane ID from the judge's own `SplitPane` provisioning may permit subprocess fallback
And an empty-ID split failure is oracle-verified to execute the permitted subprocess fallback.
