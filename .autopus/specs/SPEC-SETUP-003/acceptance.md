# SPEC-SETUP-003 Acceptance: Preview-First Bootstrap and Onboarding Truth Sync

## Test Scenarios

### AC-001: update preview performs no writes

Given a project with pending generated surface updates  
When `auto update --plan` is executed  
Then the command shows which files would change  
And no files are modified on disk  
And the output distinguishes tracked docs from generated/runtime files

### AC-002: setup preview explains target documents

Given a project where `auto setup generate` would refresh project documents  
When preview mode is used  
Then the command lists target files and why each file would change  
And no writes occur until apply is explicitly requested

### AC-003: connect onboarding text matches implementation

Given the current release only supports the implemented `auto connect` state machine  
When the user views CLI help, `README.md`, or `docs/README.ko.md` onboarding guidance  
Then the wording matches the implemented steps  
And `auto connect status` is documented as the deterministic verify surface  
And unsupported detect/configure/verify claims are not presented as already available

### AC-004: meta workspace hint shows source-of-truth context

Given the user runs bootstrap commands from a meta workspace with nested repos  
When preview mode is shown  
Then the command surfaces the owning repo or source-of-truth hint before apply

## Edge Cases

### AC-005: preview/apply drift is detected

Given the filesystem changes between preview and apply  
When the user attempts apply  
Then the system revalidates the change set  
And it returns `ErrStaleChangePlan` or recomputes before writing files

### AC-006: non-interactive preview in CI

Given preview mode is executed without a TTY  
When the command runs in CI or an agent session  
Then it still prints deterministic no-write preview output without requiring TUI interaction

### AC-007: README/help truth-sync regression test

Given onboarding copy changes in CLI help or README  
When unit tests run  
Then `internal/cli/connect_truth_sync_test.go` validates the shared state-machine copy  
And the test fails if `auto connect status` guidance disappears or legacy detect/configure/verify wording returns

## Definition of Done

- [x] preview mode guarantees no writes
- [x] file classification distinguishes tracked/generated/runtime targets
- [x] onboarding docs/help align with actual implementation
- [x] repo-aware hints work in meta workspace contexts
- [x] preview/apply drift and CI preview flows are regression-tested
- [x] README/help truth-sync is protected by regression tests
