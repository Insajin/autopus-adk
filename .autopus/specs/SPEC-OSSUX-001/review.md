# Review: SPEC-OSSUX-001

**Verdict**: PASS
**Revision**: 0
**Date**: 2026-04-02 17:10:36

## Findings

| Provider | Severity | Description |
|----------|----------|-------------|
| gemini | major | Contradiction regarding `auto config set` command existence** |
| gemini | major | Missing technical design for REQ-011 (Deferring Orchestra Provider Detection)** |
| gemini | minor | Missing implementation detail for non-interactive (`--yes`) profile assignment** |
| gemini | suggestion | Potential race condition in global `state.json` updates** |

## Provider Responses

### Response 1

MCP issues detected. Run /mcp list for status.### 1. VERDICT
**REVISE**

### 2. FINDINGS

- **FINDING: [major] Contradiction regarding `auto config set` command existence**
  REQ-007 correctly states that the `auto config set` subcommand shall be implemented as a *new* Cobra command in `internal/cli/config_cmd.go`. However, the design document (`design.md` §6) contradicts this by assuming it already exists ("기존 `auto config set` 커맨드의 키 매핑에..."). Since `auto config` does not exist anywhere in the current codebase (as verified in `internal/cli/root.go`), the design document must be updated to outline the creation of the `config` parent command and the `set` subcommand, rather than just adding a key mapping to a non-existent command.

- **FINDING: [major] Missing technical design for REQ-011 (Deferring Orchestra Provider Detection)**
  REQ-011 requires deferring Worker/Orchestra provider detection until *after* the user selects the "Developer" profile to save latency. However, `design.md` §4 specifies that `InitWizardOpts` still accepts a pre-computed `Providers []string` slice. In the current `internal/cli/init.go` flow, `detect.DetectOrchestraProviders()` is called *before* initializing the TUI wizard. If the provider list must be passed into the wizard initialization, the detection cannot be deferred. The design must propose a mechanism to delay this (e.g., passing a callback function `ProviderDetector func() []string` to the wizard, or moving the detection logic explicitly into the step evaluation phase).

- **FINDING: [minor] Missing implementation detail for non-interactive (`--yes`) profile assignment**
  REQ-010 requires defaulting the `usage_profile` to `developer` when the `--yes` flag is used. The design document (`design.md` §4) addresses this by modifying `defaultResult(opts)` inside the `tui` package. However, `internal/cli/init.go` currently bypasses the `tui.RunInitWizard` entirely when `--yes` is provided. To satisfy REQ-010 and ensure `usage_profile: developer` is explicitly written to `autopus.yaml` (REQ-003), `init.go` must explicitly set `cfg.UsageProfile = config.ProfileDeveloper` in the non-interactive fallback block.

- **FINDING: [suggestion] Potential race condition in global `state.json` updates**
  `design.md` §5 outlines `StateStore.Load` and `StateStore.Save` reading and writing the entire `~/.autopus/state.json` file. If a user has multiple terminals running `auto go` in different projects and they complete concurrently, a race condition could overwrite one project's state. While this is low-risk for a hint feature (and is marked as `// best-effort` in the design), consider applying simple file-level locking (e.g., `fslock`) during the Load-Increment-Save cycle in `RecordSuccess` and `CheckAndShow` to ensure atomic updates.

### 3. REASONING
The SPEC and its accompanying design outline a well-thought-out approach to profile branching and non-intrusive hints. Specifically, using a global state file (`~/.autopus/state.json`) mapped by a hash of the project path is an elegant way to track cross-project hint occurrences without polluting local project workspaces with uncommitted user state. 

However, a **REVISE** verdict is required because the design document contradicts the SPEC regarding the existence of the `auto config` command (which must be built from scratch) and fails to provide a viable architectural path for deferring the expensive platform detection step inside the init wizard. Once the design is updated to properly introduce the new Cobra command and handle the delayed execution of `DetectOrchestraProviders()`, this SPEC will be ready for implementation.


