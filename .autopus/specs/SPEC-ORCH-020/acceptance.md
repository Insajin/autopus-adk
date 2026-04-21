# SPEC-ORCH-020 Acceptance: Orchestra Reliability Kit

## Test Scenarios

### AC-001: wrong `cwd` is blocked before round execution

Given pane-mode orchestra is configured with a working directory different from the provider session's effective directory  
When the run starts  
Then the system emits a preflight failure receipt for that provider  
And the provider does not enter round execution  
And the final result marks the run as degraded or failed with the exact failing preflight check

### AC-002: prompt transport truncation is detected deterministically

Given a provider transport path mutates or truncates the injected prompt  
When orchestra records the prompt transport receipt  
Then the system detects the length/hash mismatch before completion waiting  
And the provider is downgraded or failed with a prompt transport integrity error

### AC-003: hook timeout produces structured failure evidence

Given hook-mode completion never signals within the configured timeout  
When the round deadline expires  
Then the system emits a structured timeout event with `run_id`, `round_id`, and `provider_id`  
And a failure bundle is written  
And the configured fallback or termination path is explicit in the final summary

### AC-004: healthy providers continue under degraded mode

Given one provider fails preflight and two providers pass  
When the run can still satisfy quorum with the healthy providers  
Then the system continues with the healthy providers  
And the final output explicitly reports degraded execution and the skipped provider

### AC-005: secret-bearing prompt and launch metadata are redacted

Given a provider prompt or launch command contains token-like or credential-bearing content  
When orchestra persists a transport receipt or failure bundle  
Then raw secret values are omitted or masked  
And the persisted artifact keeps only hashes, byte counts, or safe previews needed for diagnosis

### AC-006: artifact permissions and retention are enforced

Given a run emits failure bundles or replay ledgers  
When those artifacts are persisted  
Then they are written under the documented runtime directory with user-scoped permissions  
And the artifact schema version is recorded  
And configured retention or rotation rules are applied

## Edge Cases

### AC-007: receipt write fails but execution can continue

Given a transient filesystem error occurs while writing a non-critical collection receipt  
When provider execution otherwise succeeds  
Then the system records the receipt failure in logs  
And preserves the provider result  
And marks the bundle as partial instead of silently dropping context

### AC-008: capability mismatch forces subprocess downgrade

Given pane-mode is requested but the provider preflight reports that required launch or collection capabilities are unavailable  
When the run starts  
Then the system downgrades to the configured subprocess path or exits with a deterministic error  
And does not attempt best-effort pane execution

## Sync Verification

| Criterion | Sync Status | Evidence |
|-----------|-------------|----------|
| AC-001 | partial | `TestPreflightReceipt_UsesRequestedWorkingDir`, `buildInteractiveLaunchCmdWithCWD`로 requested `cwd` binding과 receipt 기록은 반영되었지만 shell-observed mismatch 차단은 아직 없다. |
| AC-002 | partial | `promptReceipt`가 hash/byte length/safe preview를 기록하지만 transport mutation mismatch를 재검증해 fail시키는 경로는 아직 없다. |
| AC-003 | verified | `TestCollectRoundHookResults_TimeoutWritesStructuredEvidence`, `TestRunPaneDebate_HookMode` |
| AC-004 | verified | degraded `OrchestraResult`, `FailedProvider.NextRemediation`, CLI artifact summary |
| AC-005 | verified | `TestSanitizeArtifact_RedactsSecrets`, `sanitizeArtifact`, `redactSensitiveText` |
| AC-006 | verified | `reliabilityRuntimeRoot`, `pruneReliabilityArtifacts`, `TestPruneReliabilityArtifacts_RemovesOldRuns` |
| AC-007 | partial | missing-result partial receipt는 `TestCollectRoundHookResults_MissingResultWritesPartialReceipt`로 검증되지만, receipt write 자체의 filesystem failure surfacing은 별도 follow-up이다. |
| AC-008 | partial | `ReliabilityFallbackMode` enum과 timeout/failure summary는 반영되었지만 capability-driven launch downgrade orchestration은 아직 additive 수준이다. |

## Definition of Done

- [ ] Wrong-`cwd` regression is reproducible in test and blocked before round execution
- [ ] Prompt transport mismatch is covered by deterministic tests
- [x] Hook timeout emits structured failure evidence
- [x] Degraded-run summary includes provider-level failure reasons
- [x] Sanitization prevents raw secrets from entering receipts and bundles
- [x] Artifact permissions and retention are enforced
- [ ] Failure bundle contains enough data to reconstruct the run without terminal history
