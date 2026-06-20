# SPEC-ADK-GENERATED-HYGIENE-001 구현 계획

## Phase 0: Inventory And Policy Snapshot

- [ ] T0.1: 현재 ADK repo 상태를 기록한다.
  - `git status --short --branch`
  - `git diff --name-status --cached`
  - `git diff --name-status`
- [ ] T0.2: tracked-but-ignored 후보를 기록한다.
  - `git ls-files -c -i --exclude-standard`
- [ ] T0.3: tracked generated/runtime 후보를 path family별로 분류한다.
  - `.codex/**`
  - `.claude/**`
  - `.gemini/**`
  - `.opencode/**`
  - `.agents/plugins/**`
  - `.autopus/*-manifest.json`
  - `.autopus/plugins/**`
  - `.autopus/context/signatures.md`
  - `.autopus/txns/**`
  - `config.toml`
- [ ] T0.4: source-of-truth owner mapping을 `auto check --hygiene` mapping과 비교한다.

## Phase 1: Observability Before Enforcement

- [ ] T1.1: `auto doctor` text output에 hygiene 관찰 섹션을 추가한다.
- [ ] T1.2: `auto doctor --json`에 `hygiene` data와 check rows를 추가한다.
- [ ] T1.3: `auto status` text output에 hygiene summary를 추가한다.
- [ ] T1.4: `auto status --json`에 `hygiene` data와 warning/check rows를 추가한다.
- [ ] T1.5: git command failure와 non-git directory는 hard fail이 아니라 diagnostic unavailable warning으로 처리한다.
- [ ] T1.6: focused temp git repo tests로 staged drift, tracked-but-ignored, runtime-unignored 표시를 검증한다.

## Phase 2: Cleanup Policy Contract

- [ ] T2.1: source-owned policy helper를 한 곳으로 모은다.
  - 후보: `internal/cli/check_rules_hygiene.go`의 path-family mapping과 `pkg/workflow/drift_gate.go`의 generated family를 중복 없이 공유하거나 명확히 연결한다.
- [ ] T2.2: runtime artifact family는 source-of-truth exception 없이 항상 blocker로 고정한다.
- [ ] T2.3: `.autopus/project/**`, `.autopus/specs/**`, sanitized `.autopus/learnings/pipeline.jsonl` allowlist를 테스트로 고정한다.
- [ ] T2.4: unrelated source 변경이 generated drift를 허용하지 않는 회귀 테스트를 유지한다.
- [ ] T2.5: `.gitignore` generated patterns를 root-anchored dogfood surface와 nested source fixture로 분리한다.
  - 예: root `.gemini/**`는 generated output이지만 `pkg/adapter/gemini/.gemini/settings.json` 같은 fixture는 source test data일 수 있다.
- [ ] T2.6: `.autopus/txns/` 누락을 init/gitignore policy에 반영하고 existing untracked runtime exposure를 관찰 패널에 표시한다.

## Phase 3: First Tracked Generated Cleanup Slice

- [ ] T3.1: dry-run inventory를 review한다.
- [ ] T3.2: `.autopus/txns/**`, `.autopus/brainstorms/**`, `.autopus/orchestra/**`, `.autopus/runtime/**` 같은 runtime-only tracked 후보부터 분리한다.
- [ ] T3.3: `git rm --cached` 또는 file removal은 path family 단위로 수행하고, source 파일은 건드리지 않는다.
- [ ] T3.4: `.gitignore`/init policy와 실제 ignored 상태를 맞춘다.
- [ ] T3.5: `git ls-files -c -i --exclude-standard`가 expected exception 외 비어 있는지 확인한다.

## Phase 3.5: SPEC Evidence Artifact Cleanup

- [ ] T3.5.1: `.autopus/specs/**/review.md`, `review-findings.json`, `.self-verify.log`를 canonical SPEC docs와 분리해 inventory한다.
- [ ] T3.5.2: historical review artifacts를 계속 tracking할지, generated review evidence로 untrack할지 정책 결정을 기록한다.
- [ ] T3.5.3: `spec.md`, `plan.md`, `acceptance.md`, `research.md`, `implementation.md`는 generated review artifact cleanup에 포함하지 않는다.

## Phase 4: Platform Generated Surface Cleanup

- [ ] T4.1: `.codex/**` generated surface를 source/template/adapter mapping과 비교한다.
- [ ] T4.2: `.claude/**`, `.gemini/**`, `.opencode/**`를 같은 방식으로 처리한다.
- [ ] T4.3: dogfood에 필요한 local generated surface는 regenerate command로 복원 가능해야 한다.
- [ ] T4.4: generated output을 tracking에서 제거한 뒤, source/template/adapter tests와 render dry-run으로 parity를 검증한다.

## Phase 5: CI Integration

- [ ] T5.1: CI hygiene job을 추가한다.
  - `go test ./pkg/workflow ./pkg/content ./internal/cli -count=1`
  - `go run ./cmd/auto check --hygiene --arch --quiet --staged`
  - `git ls-files -c -i --exclude-standard`
  - Lore gate command
- [ ] T5.2: generated/runtime drift와 tracked-but-ignored가 발견되면 job summary에 path family를 표시한다.
- [ ] T5.3: known dogfood exception이 필요하면 SPEC에 근거를 남기고 broad prefix exception은 금지한다.

## Verification Commands

```bash
go test ./pkg/workflow ./pkg/content ./internal/cli -count=1
go run ./cmd/auto check --hygiene --arch --quiet --staged
git diff --check
git status --short --branch
git ls-files -c -i --exclude-standard
```

## Worker Split

| Slice | Owner | Files | Notes |
|-------|-------|-------|-------|
| Observability | CLI worker | `internal/cli/doctor*.go`, `internal/cli/status*.go`, hygiene helper/tests | no hard fail |
| Policy contract | Workflow/CLI worker | `pkg/workflow/drift_gate.go`, `internal/cli/check_rules_hygiene.go`, tests | avoid broad source exception |
| Ignore anchoring | CLI/setup worker | `.gitignore`, `internal/cli/init.go`, init policy tests | protect nested fixtures |
| Runtime cleanup | Supervisor only | git index path family | no source edits |
| SPEC evidence cleanup | Supervisor only | `.autopus/specs/**/review*`, `.self-verify.log` | preserve canonical SPEC docs |
| Platform cleanup | Adapter/content worker | `content/**`, `templates/**`, `pkg/adapter/**` tests only if needed | regenerate before untrack |
| CI | DevOps worker | `.github/workflows/**` | after observability is stable |
