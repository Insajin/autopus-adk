# SPEC-ADK-SYNC-VERIFY-001: Multi-Repo Sync 2-Phase Commit Plan Verifier

**Status**: completed
**Created**: 2026-07-17
**Domain**: SYNC-VERIFY
**Module**: autopus-adk

## 목적

멀티-repo 워크스페이스에서 `/auto sync`의 2-phase 커밋(모듈 Phase A / 메타 Phase B)은 cobra 명령이 아니라 `content/rules/doc-storage.md`의 Storage Matrix 규칙 + 스킬 마크다운으로만 존재한다(`internal/cli/root.go`에 `newSyncCmd` 부재로 재확인). 그래서 파일-귀속 실수가 런타임에서 관측되지 않고 에이전트 준수에만 의존한다. 실측 근거로, 세션 시작 시점 루트 repo에는 스테이징/미스테이징 `.autopus/project/*` 변경과 untracked SPEC 디렉토리 4종이 뒤섞여 있어(동시 작업 스트림 혼입) 사람이 커밋 대상을 수동 분리해야 한다.

이 SPEC은 dirty 파일을 repo 경계로 분류해 결정적 Phase A/B 커밋 계획을 출력하고 경계 위반을 경고하는 **read-only** 검증기 `auto sync verify [--spec SPEC-ID]`를 추가한다. 어떤 git 변이도 실행하지 않는다.

## Outcome Boundary

- **User-visible outcome**: 워크스페이스 어디서든 `auto sync verify`를 실행하면 (1) 루트+직속 nested repo의 dirty 파일을 열거하고, (2) 파일별 귀속 repo와 Phase(A 모듈 / B 메타)를 분류하며, (3) 위반 후보 3종(cross-boundary misplacement, SPEC 위치-코드경로 모듈 불일치, 무관 파일 혼입 의심)을 WARN으로 출력하고, (4) 사람이 그대로 실행할 수 있는 결정적 Phase A/B `git -C <repo> add <files>` + Lore 커밋 리마인더 계획을 제안한다. read-only(변이 0), 기본 exit 0, `--strict`면 위반 존재 시 exit 1.
- **Mandatory requirements**: REQ-001~REQ-011.
- **Explicit non-goals**: 자동 커밋/스테이징 실행(계획 출력만), Lore 커밋 메시지 생성, GitHub PR/merge 정책 강제, 워크스페이스 외부 sibling repo, `sync` 부모 명령의 다른 서브커맨드나 bare `sync` 변이 동작.
- **Completion evidence**: fixture 워크스페이스(temp 루트 git repo + 가짜 nested repo)에서 분류/위반/계획/strict-exit/read-only 각각 concrete 기대값 oracle 테스트 + 실 워크스페이스 라이브 실행 출력.

## Requirements

### REQ-001: 워크스페이스 루트 해석 + repo 경계 발견
WHEN `auto sync verify`가 워크스페이스 내부 임의 디렉토리에서 실행될 때, THE SYSTEM SHALL 상위로 올라가며 직속 자식에 nested git repo를 하나 이상 포함하는 가장 바깥 git repo를 메타 워크스페이스 루트로 해석하고, `setup.DetectMultiRepo`로 그 루트와 직속 nested git repo를 커밋 경계 repo로 열거해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 출력의 repo 목록(루트 `.` + nested 상대 경로)

### REQ-002: read-only dirty 상태 수집
WHEN 커밋 경계 repo가 열거될 때, THE SYSTEM SHALL 각 repo에서 read-only `git status --porcelain`(index/worktree XY 코드)로 dirty 파일을 수집하고, repository 상태를 변경하는 어떤 git 명령도 실행하지 않아야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: repo별 dirty 파일 목록 + 실행 후 git 상태 불변

### REQ-003: Phase 분류
WHEN dirty 파일이 repo에 귀속될 때, THE SYSTEM SHALL nested 모듈 repo 파일은 Phase A(모듈 커밋)로, 루트 repo 파일 중 doc-storage Storage Matrix의 루트 추적 집합(`ARCHITECTURE.md`, `.autopus/project/**`, 루트 `.autopus/specs/**`, 루트 `.autopus/brainstorms/**`, `CHANGELOG*.md`, `CLAUDE.md`, `.claude/**`, `autopus.yaml`)에 해당하는 파일은 Phase B(메타 커밋)로 분류해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 파일별 Phase 라벨

### REQ-004: cross-boundary misplacement 경고
WHEN 루트 repo dirty 문서가 참조 코드 경로로 단일 nested 모듈에 귀속되거나, nested 모듈 dirty 파일이 루트 범위 메타 문서일 때, THE SYSTEM SHALL 파일명, 현재 위치, 기대 위치를 명시한 cross-boundary misplacement 경고를 출력해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: WARN 라인(파일·현재위치·기대위치)

### REQ-005: SPEC 위치-코드경로 모듈 불일치 경고
WHEN 어떤 SPEC 디렉토리의 위치가 그 `spec.md`/`plan.md`가 참조하는 코드 경로에서 doc-storage Module Detection으로 도출한 소유 모듈과 다를 때, THE SYSTEM SHALL SPEC ID, 현재 위치, 감지된 소유 모듈을 명시한 location-mismatch 경고를 출력해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: SPEC ID를 포함한 WARN 라인

### REQ-006: 무관 파일 혼입 의심 경고
WHEN 한 repo에 스테이징된 파일과 미스테이징 파일이 공존하거나, `--spec`이 설정되고 스테이징된 파일이 대상 SPEC 소유가 아닌 경로를 포함할 때, THE SYSTEM SHALL 의심 외부 파일을 나열한 무관-혼입 경고를 출력해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 혼입 의심 파일을 나열한 WARN 라인

### REQ-007: --spec 소유/무관 분리
WHERE `--spec SPEC-ID`가 제공될 때, THE SYSTEM SHALL SPEC-ID를 `^SPEC-[A-Z0-9-]+$`로 검증하고 워크스페이스 `.autopus/specs/` 트리 내부로만 해석한 뒤, 대상 SPEC의 `plan.md`·`spec.md` 파일 소유 참조를 읽어 dirty 파일을 "이 SPEC 커밋 대상"과 "무관 dirty 파일"로 분리해야 한다.
- EARS type: State
- Priority: Must
- 관측 지점: 두 그룹으로 분리된 파일 목록

### REQ-008: 결정적 커밋 계획 출력
WHEN 분류가 완료될 때, THE SYSTEM SHALL Phase A 모듈 repo를 상대 경로 알파벳순으로 정렬한 뒤 Phase B 메타를 배치하고, 각 항목에 명시적 `git -C <repo> add <files>` 라인과 Lore 커밋 리마인더 1줄을 포함한 결정적 커밋 계획을 출력해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 순서가 고정된 계획 블록

### REQ-009: read-only + exit 계약
WHERE `--strict`가 설정되고 경고가 하나라도 출력됐을 때 THE SYSTEM SHALL exit code 1로 종료하고, 그 외 모든 경우 THE SYSTEM SHALL 어떤 변이도 없이 exit 0으로 종료해야 한다.
- EARS type: State
- Priority: Must
- 관측 지점: 프로세스 exit code

### REQ-010: 플랫폼 패리티 문서화
THE SYSTEM SHALL 4플랫폼(claude-code·codex·antigravity-cli·opencode) 모두에 유효한 content 스킬/규칙 참조 한 곳에 `auto sync verify`를 sync 절차의 커밋 전 단계로 문서화해야 한다.
- EARS type: Ubiquitous
- Priority: Should
- 관측 지점: `[NEW]` `sync verify`를 포함한 언급 라인

### REQ-011: 입력 안전 경계
THE SYSTEM SHALL SPEC 문서와 git 출력을 로컬 untrusted 입력으로 취급하고, SPEC-ID 패턴 실패나 워크스페이스 specs 트리 밖으로 해석되는 `--spec` 값을 거부하며, 절대 경로·비밀값 없이 워크스페이스 상대 경로만 출력해야 한다.
- EARS type: Ubiquitous
- Priority: Must
- 관측 지점: traversal 입력 거부 + 상대 경로 전용 출력

## 생성 파일 상세

- `[NEW] internal/cli/sync.go` — `newSyncCmd` 부모 + `verify` 서브커맨드 + 플래그(`--dir`·`--spec`·`--strict`) + 오케스트레이션. `internal/cli/root.go`에 `newSyncCmd()` 등록(REQ-001·007·009).
- `[NEW] internal/cli/sync_verify_discover.go` — 메타 루트 상향 탐색 + `setup.DetectMultiRepo` + repo별 read-only `git status --porcelain` 수집(`hygieneGitLines`/`status_hygiene` XY 파싱 패턴 재사용)(REQ-001·002).
- `[NEW] internal/cli/sync_verify_classify.go` — Storage Matrix Phase 분류 + 위반 3종 검출(misplacement·SPEC-module 불일치·혼입)(REQ-003·004·005·006).
- `[NEW] internal/cli/sync_verify_spec.go` — `--spec` plan.md/spec.md 소유 참조 읽기 + 소유/무관 분리 + SPEC-ID 검증(REQ-007·011).
- `[NEW] internal/cli/sync_verify_plan.go` — 결정적 계획 렌더(Phase A 알파벳 → B) + Lore 리마인더 + exit 계약(REQ-008·009).
- `[NEW]` 대응 `_test.go` + `[NEW]` content 스킬/규칙 sync 절차에 `sync verify` 언급 후 4플랫폼 템플릿 재생성(REQ-010).

## Related SPECs

None (Primary SPEC이 Outcome Lock을 단독으로 닫는다). doc-storage 규칙은 `content/rules/doc-storage.md`의 기존 문서이며 이 SPEC이 관측 가능하게 만드는 대상이지 의존 sibling이 아니다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 (Must) | T1, T2 | S1 | INV-001 |
| REQ-002 (Must) | T2, T7 | S1, S8 | INV-001, INV-007 |
| REQ-003 (Must) | T3, T7 | S2 | INV-002 |
| REQ-004 (Must) | T3, T8 | S3 | INV-003 |
| REQ-005 (Must) | T3, T8 | S4 | INV-004 |
| REQ-006 (Must) | T3, T8 | S5 | INV-005 |
| REQ-007 (Must) | T4, T9 | S6 | INV-005 |
| REQ-008 (Must) | T5, T9 | S7 | INV-006 |
| REQ-009 (Must) | T5, T9 | S8, S9 | INV-007 |
| REQ-010 (Should) | T6 | S11 | INV-002 |
| REQ-011 (Must) | T4, T10 | S10 | INV-007 |
