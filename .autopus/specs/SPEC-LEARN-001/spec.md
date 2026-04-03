# SPEC-LEARN-001: 파이프라인 실패 자동 학습 시스템

**Status**: draft
**Created**: 2026-04-03
**Domain**: LEARN

## 목적

Lore 커밋이 "성공한 의사결정"을 기록하는 반면, 파이프라인 실패 패턴은 현재 휘발성이다. Gate 실패, executor 에러, reviewer REQUEST_CHANGES 등의 실패 패턴을 `.autopus/learnings/`에 자동 기록하고, 다음 파이프라인 Planning 단계에서 planner 프롬프트에 주입하여 동일 실수 반복을 방지한다.

gstack의 `/learn` 스킬에서 영감을 받았으나, autopus-adk의 기존 아키텍처(Go 바이너리 + 스킬 레벨 프롬프트)에 맞게 이원화 설계: 기록은 Go 바이너리, 주입은 스킬 프롬프트.

## 요구사항

### R1: Learning Store

WHEN the `auto` binary initializes a project, THE SYSTEM SHALL create `.autopus/learnings/` directory with two JSONL files: `pipeline.jsonl` (파이프라인 실패) and `patterns.jsonl` (코드 패턴).

### R2: Gate Failure Recording

WHEN Gate 2 (Validation) returns FAIL, THE SYSTEM SHALL extract the failure reason and resolution from the retry cycle, then append a learning entry of type `gate_fail` to `pipeline.jsonl`.

### R3: Coverage Gap Recording

WHEN Gate 3 (Coverage) reports coverage below the threshold, THE SYSTEM SHALL record the uncovered packages and gap delta as a learning entry of type `coverage_gap` to `pipeline.jsonl`.

### R4: Review Issue Recording

WHEN Phase 4 (Review) returns REQUEST_CHANGES, THE SYSTEM SHALL parse the reviewer's change requests and record each distinct issue as a learning entry of type `review_issue` to `pipeline.jsonl`.

### R5: Executor Error Recording

WHEN an executor agent fails consecutively (2+ times on the same task), THE SYSTEM SHALL record the failure cause and workaround (if retry succeeded) as a learning entry of type `executor_error` to `pipeline.jsonl`.

### R6: Learning Injection at Planning

WHEN `/auto go` enters Phase 1 (Planning), THE SYSTEM SHALL query `.autopus/learnings/pipeline.jsonl` for entries matching the current SPEC's file paths, packages, or domain keywords, and inject the top-N most relevant entries (max 5, max 2000 tokens) into the planner prompt.

### R7: Learning Injection at Fix

WHEN `/auto fix` runs, THE SYSTEM SHALL query `.autopus/learnings/` for entries matching the error context (file path, package, error pattern), and inject matching entries into the debugging prompt.

### R8: CLI Subcommands

THE SYSTEM SHALL provide the following CLI subcommands:
- `auto learn list` — display learning entries with optional filters (--type, --spec, --since)
- `auto learn add "..."` — manually add a learning entry of type `manual`
- `auto learn prune` — remove entries older than a configurable threshold (default 90 days)

### R9: Learning Entry Schema

EACH learning entry SHALL conform to the JSON schema:
- `id` (string, auto-generated, format: L-{NNN})
- `timestamp` (RFC3339)
- `type` (enum: gate_fail, coverage_gap, review_issue, executor_error, manual)
- `phase` (string: gate2, gate3, phase4, phase2)
- `spec_id` (string, optional)
- `files` (string array)
- `packages` (string array)
- `pattern` (string, human-readable failure description)
- `resolution` (string, how it was resolved)
- `severity` (enum: high, medium, low)
- `reuse_count` (int, incremented each time this entry is injected)

### R10: Relevance Matching

WHEN querying learnings for injection, THE SYSTEM SHALL score entries by:
1. Exact file path match (highest weight)
2. Package prefix match
3. Domain keyword match (from SPEC title/domain)
4. Recency (newer entries preferred over older)

WHERE no entries score above the minimum threshold, THE SYSTEM SHALL inject nothing rather than irrelevant entries.

### R11: Reuse Tracking

WHEN a learning entry is injected into a prompt, THE SYSTEM SHALL increment its `reuse_count` field in the JSONL file.

## 생성 파일 상세

### Go 바이너리 (pkg/learn/)

| 파일 | 역할 |
|------|------|
| `pkg/learn/types.go` | LearningEntry 구조체, 타입 상수, 스키마 정의 |
| `pkg/learn/store.go` | JSONL 파일 읽기/쓰기/append, ID 자동 생성 |
| `pkg/learn/query.go` | 관련성 매칭, 점수 계산, 필터링, 토큰 제한 |
| `pkg/learn/prune.go` | 오래된 항목 정리 로직 |

### CLI (internal/cli/)

| 파일 | 역할 |
|------|------|
| `internal/cli/learn.go` | `auto learn` 서브커맨드 (list, add, prune) |

### 스킬 레벨 변경

| 파일 | 변경 |
|------|------|
| `content/skills/agent-pipeline.md` | Phase 1에 learnings 주입 단계 추가 |
| `content/skills/debugging.md` | Step 2에 learnings 참조 추가 |
| `templates/claude/commands/auto-router.md.tmpl` | fix 서브커맨드에 learnings 참조 추가 |
