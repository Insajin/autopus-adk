# SPEC-ADK-GENERATED-HYGIENE-001: ADK Generated Surface Tracking Boundary Cleanup

**Status**: draft
**Created**: 2026-06-20
**Domain**: HYGIENE
**Module**: autopus-adk

## 목적

`autopus-adk`는 하네스 source of truth이면서 동시에 자기 자신에게 하네스를 설치해 dogfood하는 repo다. 이 때문에 `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, `.autopus/plugins/**`, manifest 파일, runtime 파일처럼 ADK가 생성한 platform surface와 사람이 관리하는 source/template/adapter 코드가 Git index 안에 섞여 있다.

이 SPEC은 대량 삭제를 즉시 수행하지 않고, 먼저 source-of-truth와 dogfood generated output의 경계를 문서화하고 staged/working-tree hygiene 관찰 지표를 안정화한 뒤, tracked generated surface를 작은 slice로 untrack 또는 정책 예외화하는 계획을 정의한다.

## 범위

### In Scope

- `autopus-adk` 내부에서 source of truth로 유지해야 하는 파일군과 dogfood generated output으로 취급할 파일군을 명시한다.
- tracked generated/runtime 후보를 탐지하는 deterministic 명령과 분류 기준을 정의한다.
- `auto check --hygiene`, `auto doctor`, `auto status`, CI job이 같은 path-family 정책을 사용하도록 수렴한다.
- generated surface를 source/template/adapter 변경 없이 커밋하는 경로를 차단하거나 관찰 가능하게 한다.
- cleanup migration은 dry-run inventory -> policy update -> focused untrack/delete -> regenerate/diff verification 순서로 나눈다.

### Out of Scope

- 이 SPEC에서 `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, `.autopus/plugins/**`를 즉시 대량 삭제하지 않는다.
- root `autopus-co` generated/runtime surface는 이 SPEC의 구현 대상이 아니다. root 정책은 `.autopus/project/workspace.md`를 따른다.
- 하네스 생성 결과의 내용 parity 리팩터링은 `auto-router.md.tmpl` 등 giant prompt/template 모듈화 후속 SPEC으로 다룬다.
- 제품 repo `Autopus/`, `autopus-desktop/`의 generated tracking 정책은 각 owning repo 정책을 따른다.

## Source Of Truth Boundary

### Source Of Truth

- `content/**`: agent/rule/skill/workflow/hook의 canonical content.
- `templates/**`: platform별 generated surface 템플릿.
- `pkg/adapter/**`: platform renderer, compile policy, path mapping.
- `pkg/content/**`: content catalog와 generator bridge.
- `pkg/workflow/**`: drift/hygiene gate, workflow schema, deterministic phases.
- `internal/cli/**`: CLI command, doctor/status/check/CI integration.
- `autopus.yaml`, `opencode.json`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `DESIGN.md`: 사람이 검토한 bootstrap/config/doc surface. 단, 파일별 정책은 별도 명시한다.
- `.autopus/specs/**`, `.autopus/project/**`, sanitized `.autopus/learnings/pipeline.jsonl`: 사람이 관리하거나 redacted learning contract를 따르는 project knowledge.

### Dogfood Generated Output

- `.codex/**`
- `.claude/**`
- `.gemini/**`
- `.opencode/**`
- `.agents/plugins/marketplace.json`
- `.autopus/*-manifest.json`
- `.autopus/context/signatures.md`
- `.autopus/plugins/**`
- `.autopus/txns/**`
- `.autopus/brainstorms/**`
- `.autopus/orchestra/**`
- `.autopus/runtime/**`
- `config.toml`

Dogfood generated output은 local execution을 위해 존재할 수 있지만, source/template/adapter 변경의 검증 산출물이다. 커밋 대상이 되려면 source-of-truth change와 regenerated output의 관계가 문서화되어야 하며, runtime artifact는 예외 없이 커밋하지 않는다.

## 요구사항

### P0 - Must Have

**R1: Inventory First**
WHEN cleanup을 시작할 때, THE SYSTEM SHALL tracked generated/runtime 후보를 먼저 inventory로 출력하고, 사람이 검토할 수 있는 분류 표를 생성해야 한다.

**R2: Source Boundary Documentation**
WHEN generated path family가 발견될 때, THE SYSTEM SHALL 그 path family의 canonical source owner(`content/**`, `templates/**`, `pkg/adapter/**`, `pkg/content/**`, `pkg/workflow/**`, `internal/cli/**`)를 문서화해야 한다.

**R3: No Bulk Delete Without Slice**
WHEN generated/runtime tracked 파일을 제거해야 할 때, THE SYSTEM SHALL path family별 slice로 나누고, 각 slice마다 `git status`, inventory diff, regenerate/diff verification을 기록해야 한다.

**R4: Runtime Artifact Refusal**
WHEN runtime artifact path family(`.autopus/txns/**`, `.autopus/brainstorms/**`, `.autopus/orchestra/**`, `.autopus/runtime/**`)가 staged 또는 tracked 상태로 발견될 때, THE SYSTEM SHALL source-of-truth 예외 없이 commit blocker로 분류해야 한다.

**R5: Human-Managed Docs Allowlist**
WHEN `.autopus/project/**`, `.autopus/specs/**`, sanitized `.autopus/learnings/pipeline.jsonl`이 변경될 때, THE SYSTEM SHALL generated/runtime drift로 오분류하지 않아야 한다.

**R6: Source-Matched Generated Drift**
WHEN generated platform surface가 staged될 때, THE SYSTEM SHALL broad prefix 예외가 아니라 generated path family별 source mapping으로 source-of-truth 동반 변경 여부를 판단해야 한다.

**R7: Ignore Pattern Anchoring**
WHEN `.gitignore` generated-surface patterns are evaluated, THE SYSTEM SHALL distinguish root dogfood surface from nested source fixtures. Non-anchored patterns such as `.gemini/` must not cause `pkg/adapter/gemini/.gemini/settings.json`-style fixtures to be untracked by bulk cleanup.

### P1 - Should Have

**R8: Doctor/Status Observability**
WHEN `auto doctor` 또는 `auto status`가 실행될 때, THE SYSTEM SHALL generated/runtime drift, tracked-but-ignored, runtime-unignored 상태를 hard fail 없이 관찰 패널로 보여줘야 한다.

**R9: CI Hygiene Job**
WHEN CI가 실행될 때, THE SYSTEM SHALL `auto check --hygiene --arch --quiet --staged`, tracked-but-ignored, source line limit, Lore gate를 한 job에서 실행해야 한다.

**R10: Recursive SPEC Status Resolver**
WHEN `auto status`가 SPEC을 스캔할 때, THE SYSTEM SHALL canonical recursive resolver 정책과 일치하게 nested `.autopus/specs/**`를 놓치지 않아야 한다.

### P2 - Could Have

**R11: Dogfood Generated Surface Mode**
WHEN ADK repo가 자기 자신에게 하네스를 설치할 때, THE SYSTEM SHOULD dogfood generated surface를 tracked source와 분리하는 explicit mode를 제공해야 한다.

**R12: Cleanup Report Publication**
WHEN tracked generated surface cleanup이 끝날 때, THE SYSTEM SHOULD path-family별 removed/kept/exception 목록과 verification command를 cleanup report로 남겨야 한다.

**R13: SPEC Evidence Policy**
WHEN `.autopus/specs/**/review.md`, `.autopus/specs/**/review-findings.json`, or `.autopus/specs/**/.self-verify.log` are tracked-but-ignored, THE SYSTEM SHOULD classify them separately from canonical SPEC documents (`spec.md`, `plan.md`, `acceptance.md`, `research.md`, `implementation.md`) before any untrack action.

## 위험 및 완화

| 위험 | 영향 | 완화 |
|------|------|------|
| 실제 source 파일을 generated로 오분류 | source 손실 | inventory first, path family owner mapping, no bulk delete |
| dogfood 실행에 필요한 generated surface가 사라짐 | local harness 실행 실패 | source regenerate command와 dry-run diff를 slice마다 실행 |
| root workspace 정책과 ADK repo 정책 혼동 | 잘못된 repo commit | root `.autopus/project/workspace.md`와 ADK SPEC을 분리 |
| 기존 dirty/generated drift를 새 작업 결과로 오인 | review noise | touched file 범위와 pre/post status를 기록 |

## 완료 기준

- cleanup 정책 문서가 source-of-truth와 dogfood generated output 경계를 명확히 설명한다.
- `auto doctor`/`auto status`에서 hygiene 관찰 정보가 표시된다.
- tracked generated/runtime inventory 명령과 분류 기준이 문서화된다.
- 첫 cleanup implementation slice 전에 대량 삭제가 발생하지 않는다.
- final verification에는 `git status`, `git ls-files -c -i --exclude-standard`, focused tests, relevant package tests가 포함된다.
