# SPEC-ADK-GENERATED-HYGIENE-001 수락 기준

## 시나리오

### S1: Inventory First

- Given: `autopus-adk`에 generated/runtime tracked 후보가 존재한다.
- When: cleanup을 시작한다.
- Then: 먼저 `git status --short --branch`, `git diff --name-status --cached`, `git diff --name-status`, `git ls-files -c -i --exclude-standard` 결과가 기록된다.

### S2: Human-Managed Docs Are Allowed

- Given: `.autopus/project/workspace.md` 또는 `.autopus/specs/SPEC-X/spec.md`가 staged 상태다.
- When: hygiene check가 실행된다.
- Then: 해당 path는 generated/runtime drift로 분류되지 않는다.

### S3: Runtime Artifacts Are Always Blocked

- Given: `.autopus/txns/txn/journal.json` 또는 `.autopus/runtime/file`이 staged 상태다.
- When: hygiene check가 실행된다.
- Then: source-of-truth 동반 변경 여부와 관계없이 runtime artifact blocker로 표시된다.

### S4: Generated Drift Requires Matching Source Family

- Given: `.codex/agents/reviewer.toml`이 staged 상태다.
- When: `templates/codex/agents/reviewer.toml.tmpl` 또는 `pkg/adapter/codex/codex_agents.go` 같은 matching source family가 함께 staged되어 있다.
- Then: generated drift는 source-matched로 표시된다.

### S5: Unrelated Source Does Not Permit Generated Drift

- Given: `.codex/agents/reviewer.toml`과 unrelated `content/workflows/route_a.md`가 staged 상태다.
- When: hygiene check가 실행된다.
- Then: generated drift는 여전히 warning 또는 blocker로 남는다.

### S6: Doctor Text Hygiene Panel

- Given: git worktree에 generated/runtime drift, tracked-but-ignored, runtime-unignored 후보가 있다.
- When: `auto doctor --dir <repo>`가 실행된다.
- Then: text output에 hygiene 섹션이 표시되고 세 항목의 count와 대표 path가 출력된다.

### S7: Doctor JSON Hygiene Payload

- Given: git worktree에 hygiene 후보가 있다.
- When: `auto doctor --dir <repo> --json`이 실행된다.
- Then: JSON data에 hygiene payload가 포함되고 checks에 `doctor.hygiene.*` 항목이 포함된다.

### S8: Status Text Hygiene Summary

- Given: git worktree에 hygiene 후보가 있다.
- When: `auto status --dir <repo>`가 실행된다.
- Then: SPEC dashboard 아래에 hygiene summary가 표시된다.

### S9: Status JSON Hygiene Payload

- Given: git worktree에 hygiene 후보가 있다.
- When: `auto status --dir <repo> --json`이 실행된다.
- Then: JSON data에 hygiene payload가 포함되고 warnings/checks가 hard error 없이 표시된다.

### S10: Non-Git Directory Is Diagnostic Only

- Given: `.git`이 없는 temp directory다.
- When: `auto doctor` 또는 `auto status`가 실행된다.
- Then: hygiene diagnostic unavailable warning이 표시되며 command 자체는 hygiene 때문에 실패하지 않는다.

### S11: Recursive SPEC Scan

- Given: nested repo 또는 nested module 아래 `.autopus/specs/SPEC-NESTED/spec.md`가 있다.
- When: `auto status --dir <workspace>`가 실행된다.
- Then: depth-1 전용 scan 때문에 SPEC이 누락되지 않는다.

### S12: No Bulk Delete

- Given: cleanup SPEC이 draft 또는 approved 상태다.
- When: first implementation slice가 실행된다.
- Then: generated/runtime path family가 대량 삭제되지 않고, inventory와 path-family 단위 plan이 먼저 review된다.

### S13: Nested Fixture Is Not Bulk-Untracked

- Given: `.gitignore` contains a generated-surface pattern such as `.gemini/`.
- And: a tracked fixture path exists under a source directory, such as `pkg/adapter/gemini/.gemini/settings.json`.
- When: cleanup inventory is produced.
- Then: the fixture is classified as ambiguous/source-fixture candidate and is not removed by a root generated-surface bulk command.

### S14: SPEC Review Artifacts Are Separate From SPEC Docs

- Given: `.autopus/specs/SPEC-X/review.md` is tracked-but-ignored.
- And: `.autopus/specs/SPEC-X/spec.md` is tracked and not ignored.
- When: cleanup inventory is produced.
- Then: `review.md` is classified as generated review evidence while `spec.md` remains canonical SPEC documentation.

## 품질 게이트

- `go test ./internal/cli -run 'Test.*Hygiene|TestBuildStatusJSONPayload|TestRunStatusJSON' -count=1`
- `go test ./pkg/workflow ./pkg/content ./internal/cli -count=1`
- `go run ./cmd/auto check --hygiene --arch --quiet --staged`
- `git diff --check`
- `git status --short --branch`
- `git ls-files -c -i --exclude-standard`
