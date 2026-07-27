# SPEC-CONTEXT-ENGINEERING-001 Acceptance

## Test Scenarios

### S1: Claude thin router selects one generated detail
Priority: Must
Given a full-mode Claude scratch root is generated
When `.claude/skills/auto/SKILL.md` and generated route details are inspected
Then expected router reference count for `.claude/skills/autopus/auto-go.md` is exactly `1`
And expected generated `auto-go.md` contains one `## Context Profile`
And expected installed file count for generation-only `auto-workflows.md` is `0`
And expected effective router and detail contain no unconditional all-document preload instruction

### S2: Four-platform exact command/document matrix
Priority: Must
Given scratch full-mode projects are generated for Claude, Codex, OpenCode, and Gemini/Antigravity
When the effective `plan`, `test`, `canary`, and `go` detail paths are normalized
Then expected `plan` set is required `core,architecture,relevant_spec`, optional `signature,learning`, excluded `test,canary`
And expected `test` set is required `core,test`, optional `signature,learning`, excluded `canary`
And expected `canary` set is required `core,canary`, optional `learning`, excluded `test,signature`
And expected `go` supervisor set is required `core,resolved_spec,plan,acceptance,available_architecture`, worker optional `signature,learning,task_declared_extra`, excluded `test,canary`
And each platform expected normalized set is exactly equal

### S3: Canonical five-field condensed return parity
Priority: Must
Given canonical worker contract owners and each platform's effective agent-pipeline skill exist
When their worker return fields are normalized
Then expected canonical field set is exactly `owned_paths,changed_files,verification,blockers,next_required_step`
And expected platform field set is exactly equal to the canonical set
And expected condensed return upper bound is `2,000` estimated tokens
And expected short correct returns are accepted without padding

### S4: Safe project-relative JIT guidance
Priority: Must
Given each platform's effective agent-pipeline skill and Gemini native/plugin mirrors are generated
When optional-retrieval guidance is normalized
Then expected guidance requires stable project-relative source refs, selected refs and hashes, and omitted count
And expected guidance rejects absolute paths, `..` traversal, symlinks, and non-regular files
And expected guidance requires sanitization and redaction while preserving injection evidence
And expected guidance forbids raw tool, provider, required-body, and repeated-artifact replay
And expected Gemini native and Antigravity plugin normalized semantics are exactly equal

### S5: Scenario executable wire is not archived
Priority: Must
Given document rotation guidance covers `scenarios.md`
When keep-versus-archive rules are inspected
Then expected keep fields include top-level `Build` and runnable scenario `Command`, `Verify`, `Status`
And expected text forbids replacing runnable scenario bodies with index-only entries

### S6: Doctor terminology and compatibility stay advisory
Priority: Must
Given the context catalog exceeds total or per-document soft caps
When text and JSON doctor checks run
Then expected output names the context catalog and measured bytes
And expected overall doctor health remains unchanged by this advisory warning
And expected exported symbol name is exactly `ContextLoadSet`
And expected JSON IDs are exactly `doctor.context_weight.total` and `doctor.context_weight.doc.<name>`

### S7: Complete required-document integrity is preserved
Priority: Must
Given a GPT/Codex go or review context delivery is built from complete required documents
When existing promptlayer and review delivery tests run
Then expected required bodies, source hashes, prompt hashes, and snapshot verification remain unchanged
And available architecture auto-inclusion behavior remains unchanged

### S8: Generated surface and WIP hygiene
Priority: Must
Given the implementation is complete in `autopus-adk`
When scratch generation, focused tests, status, and tracked-but-ignored checks run
Then expected generated roots are temporary verification artifacts only
And expected unrelated dirty files are neither modified nor staged

## Edge Cases

### Edge Case 1: Optional document absent
Priority: Must
Given an optional catalog document does not exist
When a command profile does not select it
Then expected supervisor required set is unchanged
And expected worker optional set omits the absent reference without inventing or requiring it

### Edge Case 2: Required document integrity failure
Priority: Must
Given a required document is missing, stale, or hash-mismatched
When verified context delivery runs
Then expected dispatch remains fail-closed under the existing integrity contract
And unsafe absolute, traversal, symlink, or non-regular required refs return a non-nil error

### Edge Case 3: Short worker result
Priority: Must
Given a correct worker result fits well below 1,000 tokens
When the condensed return contract is applied
Then expected result is accepted without padding
And 2,000 estimated tokens remains the upper bound

## Oracle Acceptance Notes

- `[NEW] pkg/adapter/context_engineering_acceptance_test.go` generates all adapters and compares exact normalized sets, paths, one-detail counts, five fields, security tokens, and mirror semantics.
- `[NEW] pkg/adapter/context_engineering_test_helpers_test.go` resolves generated pipeline references inside each scratch root and applies restrictive-clause polarity checks plus adversarial inverted fixtures.
- `[NEW] internal/cli/context_engineering_contract_test.go` locks doctor human text, `ContextLoadSet`, exact JSON IDs, advisory health, and scenario keep/forbid tokens.
- Generated surface oracle은 whole-file byte equality가 아니라 exact required/worker-optional/excluded semantic set을 비교한다.
- Antigravity mirror oracle은 provider path만 정규화하고 context/security/return semantics는 그대로 비교한다.
- Doctor oracle은 경고 문자열, symbol/ID identity, overall health의 비변경을 함께 확인한다.
- Required-document compatibility oracle은 기존 promptlayer tests를 재실행해 full body와 hash behavior를 확인한다.
- Scenario oracle은 `Command`, `Verify`, `Status`, `Build` literal과 index-only 금지 문구를 확인한다.

## Definition of Done

- [x] S1-S8 and Edge Case 1-3 PASS
- [x] focused tests and race tests PASS
- [x] vet, build, strict SPEC, architecture, diff checks PASS
- [x] reviewer/security findings resolved
- [x] Completion Debt none
