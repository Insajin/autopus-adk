# SPEC-LEARN-001: Pipeline Learning Infrastructure

**Status**: draft
**Created**: 2026-04-03
**Updated**: 2026-04-03
**Domain**: LEARN

## 목적

파이프라인 실패 패턴을 자동 기록하고, 기존 워크플로우(go, fix, sync, 세션 시작)에 녹아들어 사용자가 별도 커맨드를 실행하지 않아도 학습이 자동으로 동작하는 인프라.

**핵심 원칙**: 독립 커맨드 0개. 모든 학습은 기존 플로우에 자동 삽입.

gstack의 `/learn`에서 영감을 받되, "사용자가 능동적으로 실행하는 관리 커맨드는 결국 안 쓰인다"는 피드백을 반영하여 완전 자동 인프라로 설계.

SPEC-RETRO-001(자동 회고)의 핵심 기능을 sync 자동 요약으로 흡수.

## 요구사항

### R1: Learning Store

WHEN the `auto` binary initializes a project (`auto init` or `auto setup`), THE SYSTEM SHALL create `.autopus/learnings/` directory with `pipeline.jsonl`.

### R2: Gate Failure Auto-Recording

WHEN Gate 2 (Validation) returns FAIL, THE SYSTEM SHALL automatically extract the failure reason and resolution from the retry cycle, then append a learning entry of type `gate_fail` to `pipeline.jsonl`. No user action required.

### R3: Coverage Gap Auto-Recording

WHEN Gate 3 (Coverage) reports coverage below the threshold, THE SYSTEM SHALL automatically record the uncovered packages and gap delta as a learning entry of type `coverage_gap`.

### R4: Review Issue Auto-Recording

WHEN Phase 4 (Review) returns REQUEST_CHANGES, THE SYSTEM SHALL automatically parse the reviewer's change requests and record each distinct issue as a learning entry of type `review_issue`.

### R5: Executor Error Auto-Recording

WHEN an executor agent fails consecutively (2+ times on the same task), THE SYSTEM SHALL automatically record the failure cause and workaround (if retry succeeded) as a learning entry of type `executor_error`.

### R6: Auto-Injection at Planning (`/auto go` Phase 1)

WHEN `/auto go` enters Phase 1 (Planning), THE SYSTEM SHALL automatically query `pipeline.jsonl` for entries matching the current SPEC's file paths, packages, or domain keywords, and inject the top-N most relevant entries (max 5, max 2000 tokens) into the planner prompt. Display one-line notice:

```
💡 관련 학습 패턴 {N}개 주입됨
```

### R7: Auto-Injection at Fix (`/auto fix`)

WHEN `/auto fix` runs, THE SYSTEM SHALL automatically query learnings for entries matching the error context (file path, package, error pattern), and inject matching entries into the debugging prompt. Display one-line notice if entries found.

### R8: Auto-Summary at Sync (`/auto sync` — retro 흡수)

WHEN `/auto sync` completes all sync targets, THE SYSTEM SHALL automatically display a "이번 사이클 학습 요약" section BEFORE the completion bar:

```
🐙 학습 요약 ────────────────────────
  신규 기록: {N}개 (gate_fail: {n}, review_issue: {n}, ...)
  반복 패턴 Top 3:
    1. "{pattern}" — {reuse_count}회 주입됨
    2. "{pattern}" — {reuse_count}회
    3. "{pattern}" — {reuse_count}회
  개선 영역: {이전 sync 대비 줄어든 실패 유형}
```

WHERE no new learning entries exist since last sync, display: `학습 요약: 신규 항목 없음 ✓`

### R9: Auto-Prune at Sync

WHEN `/auto sync` runs, THE SYSTEM SHALL automatically prune learning entries older than 90 days. Display one-line notice only if entries were pruned:

```
정리: {N}개 학습 항목 만료 삭제 (90일 초과)
```

### R10: Session Start Notification

WHEN a new session starts AND `.autopus/learnings/pipeline.jsonl` contains 5+ entries, THE SYSTEM SHALL display a non-intrusive notification in the Context Load phase:

```
💡 학습 패턴 {N}개 — 다음 파이프라인에 자동 반영됩니다
```

This integrates with the existing Tier 1 branding session start display.

### R11: Fix Completion Auto-Capture

WHEN `/auto fix` successfully resolves a bug, THE SYSTEM SHALL automatically record the fix pattern (error → root cause → resolution) as a learning entry of type `fix_pattern` in `pipeline.jsonl`. Display:

```
💡 수정 패턴 학습됨: "{one-line pattern summary}"
```

### R12: Learning Entry Schema

EACH learning entry SHALL conform to the JSON schema:
- `id` (string, auto-generated, format: L-{NNN})
- `timestamp` (RFC3339)
- `type` (enum: gate_fail, coverage_gap, review_issue, executor_error, fix_pattern)
- `phase` (string: gate2, gate3, phase4, phase2, fix)
- `spec_id` (string, optional)
- `files` (string array)
- `packages` (string array)
- `pattern` (string, human-readable failure/fix description)
- `resolution` (string, how it was resolved)
- `severity` (enum: high, medium, low)
- `reuse_count` (int, incremented each time this entry is injected)

### R13: Relevance Matching

WHEN querying learnings for injection, THE SYSTEM SHALL score entries by:
1. Exact file path match (highest weight)
2. Package prefix match
3. Domain keyword match (from SPEC title/domain)
4. Recency (newer entries preferred over older)

WHERE no entries score above the minimum threshold, inject nothing.

### R14: Reuse Tracking

WHEN a learning entry is injected into a prompt, THE SYSTEM SHALL increment its `reuse_count`.

## 자동 트리거 맵

| 트리거 시점 | 동작 | 사용자 행동 |
|------------|------|------------|
| Gate 2/3 FAIL | 실패 패턴 자동 기록 (R2, R3) | 없음 |
| Phase 4 REQUEST_CHANGES | 리뷰 이슈 자동 기록 (R4) | 없음 |
| Executor 연속 실패 | 에러 패턴 자동 기록 (R5) | 없음 |
| `/auto go` Phase 1 | 관련 learnings 자동 주입 (R6) | 없음 |
| `/auto fix` 시작 | 관련 learnings 자동 참조 (R7) | 없음 |
| `/auto fix` 완료 | 수정 패턴 자동 기록 (R11) | 없음 |
| `/auto sync` 완료 | 학습 요약 자동 표시 + 자동 prune (R8, R9) | 읽기만 |
| 세션 시작 | 학습 알림 (R10) | 읽기만 |

## 생성 파일 상세

### Go 바이너리 (pkg/learn/)

| 파일 | 역할 |
|------|------|
| `pkg/learn/types.go` | LearningEntry 구조체, 타입 상수, 스키마 정의 |
| `pkg/learn/store.go` | JSONL 파일 읽기/쓰기/append, ID 자동 생성 |
| `pkg/learn/query.go` | 관련성 매칭, 점수 계산, 필터링, 토큰 제한 |
| `pkg/learn/prune.go` | 시간 기반 자동 정리 (90일) |
| `pkg/learn/summary.go` | sync용 학습 요약 생성 (반복 패턴 Top N, 개선 영역) |

### 스킬 레벨 변경

| 파일 | 변경 |
|------|------|
| `templates/claude/commands/auto-router.md.tmpl` | go Phase 1 주입, fix 참조, sync 요약 표시, 세션 알림 |
| `templates/shared/agent-pipeline.md.tmpl` (해당 시) | Phase 1에 learnings 주입 단계 |

## 노출되는 CLI 커맨드: 없음

모든 기능은 기존 워크플로우(go, fix, sync, 세션 시작)에 자동 삽입. `auto learn` 서브커맨드는 생성하지 않음.
