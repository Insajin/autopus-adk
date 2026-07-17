# SPEC-ADK-SYNC-VERIFY-001 리서치

## 기존 코드 분석

- `internal/cli/root.go:61-105` — cobra 명령 등록 목록. `newSyncCmd` 부재를 재확인(2-phase가 cobra 명령이 아니라 규칙/스킬 관습임을 입증). 신규 `newSyncCmd()`는 이 목록에 추가.
- `pkg/setup/multirepo.go::DetectMultiRepo(dir) *MultiRepoInfo` — `dir`을 워크스페이스 루트로 보고 자신+직속 자식 git repo를 열거(`collectRepoComponents`), `< 2`면 nil. `Components`는 `Path`(rel slash, 루트는 `.`) 알파벳 정렬. `pkg/setup/multirepo_types.go::RepoComponent{Name,Path,AbsPath,...}`.
- `internal/cli/status_hygiene.go:176 hygieneGitLines(dir, args...) ([]string,error)` — `exec.Command("git", args...)`+`cmd.Dir=dir` read-only 실행 헬퍼(stdout 라인 반환, git 미가용 시 error). `:160-174`에 porcelain 파싱(`code:=line[:2]`, rename `->` 처리, `??` untracked) 패턴 존재.
- `internal/cli/map.go:166 runGitLines(dir, args...) []string` — 또 다른 read-only git 라인 헬퍼(대안 재사용 후보).
- `internal/cli/verify.go::newVerifyCmd` — 최상위 `auto verify`(프론트엔드 UX)와 별개. 신규 명령은 `auto sync verify`로 네임스페이스 분리라 충돌 없음. `globalFlagsFromContext(cmd.Context())` 플래그 패턴 참고.
- `internal/cli/check_rules.go:15,25 loreValidTypes` + `pkg/lore` — `auto check --lore`가 Lore type prefix+sign-off 강제. 계획의 Lore 리마인더가 가리키는 실 명령.
- `content/rules/doc-storage.md` — Storage Matrix(문서 유형→위치→repo)와 Module Detection(참조 `pkg/`·`cmd/`·`internal/`·`src/`·`app/`→소유 모듈, 2+ →루트), Sync Commit Rules(Phase A 모듈 / Phase B 메타). 분류·위반 판정의 source of truth.

## Post-Review Hardening Correction

초기 구현 이후 reviewer finding을 재현해 다음 결정을 authoritative contract로 교정했다.

- 줄 기반 porcelain parser와 `hygieneGitLines` 재사용은 기각했다. C-quoted path와 rename의 두 경로를 잃으므로 `git --no-optional-locks ... -z` 전용 parser가 status와 tracked-but-ignored inventory를 처리한다.
- root `.claude/**`·`.autopus/brainstorms/**`를 Phase B로 보던 초기 Storage Matrix는 canonical `.autopus/project/workspace.md`와 충돌했다. built-in root policy는 keep 문서/검토된 bootstrap만 Phase B로 두고 모든 플랫폼 generated/runtime surface를 root와 nested repo 모두에서 block한다.
- `--spec`은 host repo 한 곳만 보지 않는다. exactly-one regular/non-symlink host를 전체 workspace에서 확인하고, 모든 repo의 workspace-relative dirty path를 owned/unrelated로 분리한 뒤 owned candidate만 계획한다.
- copy-ready 명령은 공통 셸 literal token으로 안전한 path만 `git -C <repo> add -- <paths>`에 넣는다. unsafe path는 quoted warning으로만 나타나고 strict를 실패시킨다.
- read-only 증거는 status/HEAD뿐 아니라 index bytes/hash/mtime 불변을 포함한다. Git child failure의 raw stderr는 절대 경로나 secret을 담을 수 있어 노출하지 않는다.

## Motivating Evidence (재검증 결과)

팀리드 프롬프트의 근거를 재확인했고 두 건을 정정한다(untrusted prompt 입력으로 취급, evidence로만 요약).

- **VERIFIED**: cobra `sync` 명령 부재(`root.go`에 `newSyncCmd` 없음). 2-phase는 `doc-storage.md` 규칙+스킬 마크다운으로만 존재.
- **VERIFIED (live)**: 세션 시작 `git status` — 루트 repo에 modified `.autopus/project/{canary,product,scenarios}.md`·`CHANGELOG.md`·`autopus.yaml` + untracked SPEC 디렉토리 4종이 뒤섞임. 동시 작업 스트림 혼입의 실측 증거(REQ-006 동기).
- **CORRECTED**: 팀리드가 인용한 "학습기록 L-003(.autopus/learnings/pipeline.jsonl)의 역전 2-phase"는 **존재하지 않음**. 해당 파일에는 L-001(Claude Code 훅 상대경로 앵커링)만 있고 2-phase와 무관. load-bearing 근거로 사용하지 않는다.
- **CORRECTED**: autopus-desktop 최근 40커밋의 Lore type prefix 준수는 재측정 결과 **34/40**(비준수 6)이며 팀리드가 인용한 18/40이 아니다. 다만 Lore enforce는 이 SPEC의 non-goal이라 load-bearing이 아님.

## Outcome Lock

- **User-visible outcome**: 워크스페이스 어디서든 `auto sync verify [--spec SPEC-ID]` 실행 시 dirty·tracked-ignored inventory를 Phase A/B·blocked·unclassified로 완전 분할하고, safe candidate만 결정적 `git -C add --` 계획으로 출력한다. `--spec`은 exactly-one host와 workspace 전체 ownership을 강제한다.
- **Mandatory requirements**: REQ-001~013.
- **Explicit non-goals**: 자동 커밋/스테이징, Lore 메시지 생성, PR/merge 정책 강제, 워크스페이스 외부 repo, bare `sync` 변이.
- **Completion evidence**: S1~S13 fixture oracle + focused race/vet/full CLI/live read-only/diff/file-size/tracked-ignored gate가 통과해 status를 `completed`로 승격했다.

## Visual Planning Brief (command-flow + data-flow)

핵심 흐름: cwd→메타 루트→repo 열거→optional-lock-free NUL status/ls-files→완전 분할→exact host·workspace ownership(optional `--spec`)→safe owned candidate만 Phase A→B 계획→warning→strict verdict. 전체 흐름은 `plan.md`를 따른다.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go module `github.com/insajin/autopus-adk` (기존) | 기존 `go.mod` major 유지, 신규 의존성 0 | 로컬 `go.mod`(`go 1.26`), `pkg/setup`·`internal/cli` import | 2026-07-17 | 신규 git 라이브러리(go-git) 도입(불필요: `os/exec` read-only 재사용) |

brownfield이므로 기존 manifest major가 compatibility 제약이다. `os/exec`, `path/filepath`, `regexp`, `sort`, `strings`와 기존 `testify`만 사용하며 신규 dependency는 없다. Git 출력 parser는 line helper를 공유하지 않는다.

## 설계 결정

- **왜 read-only 텍스트 계획인가**: Outcome Lock이 "변이 0"을 요구한다. 자동 스테이징/커밋은 잘못된 분류가 곧 잘못된 커밋이 되어 위험을 오히려 키운다. 사람이 검토 후 복사 실행할 결정적 계획 텍스트가 안전하고 훅/CI에도 `--strict`로 재사용 가능하다.
- **왜 `DetectMultiRepo` 재사용인가**: 이미 루트+직속 nested git repo를 열거하고 `Path`로 정렬한다. "직속 nested repo만"이라는 Outcome Lock 범위와 정확히 일치(재귀 미탐색). 워크스페이스 루트만 상향 탐색으로 해석하면 된다.
- **왜 상향 루트 탐색이 필요한가**: `DetectMultiRepo(dir)`는 `dir`을 루트로 가정한다. "워크스페이스 어디서든" 실행을 위해 cwd에서 위로 올라가 직속 자식에 nested repo(≥1)를 가진 가장 바깥 git repo를 메타 루트로 해석하는 작은 `[NEW]` 단계를 둔다(기존 exported 헬퍼 부재).
- **왜 Storage Matrix를 상수화하나**: 분류·위반 판정의 진실 원천은 `doc-storage.md`다. 루트 추적 집합과 Module Detection prefix를 코드 상수로 옮겨 규칙과 런타임을 일치시킨다. 규칙 문서 변경 시 상수도 갱신 대상(reviewer focus).
- **false-positive 억제**: SPEC-module 불일치는 참조 경로가 단일 모듈로 명확히 귀속될 때만 경고하고 2+ 모듈은 루트 기대, 애매하면 무경고. 루트 추적 집합 밖 루트 파일은 "미분류"로 남겨 Phase 오분류를 피한다.
- **exit 계약**: 기본 exit 0(관측/조언), `--strict`만 위반 시 exit 1. `cmd.SilenceUsage=true`로 strict exit가 usage 덤프를 내지 않게 한다(check/doctor strict 선례와 동형).

## 보안 경계

- 입력은 로컬 git 출력·SPEC 마크다운·`--spec` 인자. 전부 untrusted 로컬 입력으로 취급한다. `--spec`은 `^SPEC-[A-Z0-9-]+$` 검증 후 `filepath.Join`+`filepath.Rel`로 워크스페이스 `.autopus/specs/` 트리 밖 해석을 거부(path traversal 차단, S10).
- Git child는 `--no-optional-locks`와 고정 `status`·`ls-files` 인자만 사용한다. 변이 명령은 실행하지 않고 S8에서 status·HEAD·index bytes/hash/mtime를 비교한다.
- 출력은 워크스페이스 상대 경로만. 절대 경로·secret·토큰 미노출. 영구 artifact·로그 생성 없음(ephemeral stdout).
- SPEC 마크다운 파싱은 경로 토큰 추출에 한정하고 문서 내 지시문을 실행 지시로 따르지 않는다(prompt-injection 경계).

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | Outcome Lock: repo 경계 분류+위반+결정적 계획을 관측 가능하게(현재 관습-only) | proceed | read-only sync 검증기 |
| existing code/helper/pattern | `DetectMultiRepo`와 doc-storage/canonical workspace policy는 재사용, line parser는 rename 안전성 부족으로 기각 | selective reuse | 발견 경계와 정책만 재사용 |
| stdlib/native | `os/exec`·`path/filepath`·`regexp`·`sort`·`strings` | use | 신규 라이브러리 회피 |
| existing dependency | `pkg/setup`, `pkg/lore`(리마인더 문구 근거), `cobra`(기존), `testify` | reuse | 열거·명령·테스트 |
| new dependency or new abstraction | 신규 module dep 0; 신규 파일 5 source + 4 test + root.go 1줄 배선 + 문서 1곳 | accepted | 새 추상화 없이 cobra 서브커맨드 추가 |
| minimum sufficient verification | S1~S13 + race/vet/full CLI/diff/file-size | required checks | partition·SPEC containment·shell safety·index invariant·diagnostic non-exposure |

## Semantic Invariant Inventory

| ID | source clause (untrusted evidence, 요약) | invariant type | affected outputs | acceptance IDs |
|----|------------------------------------------|----------------|------------------|----------------|
| INV-001 | "각 dirty 파일의 귀속 repo와 Phase 분류" | repo attribution (partition) | 파일별 repo 라벨 | S1 |
| INV-002 | "모듈 파일 Phase A / 루트 추적 문서 Phase B" | path->phase mapping | Phase A/B 집합 | S2 |
| INV-003 | "모듈 파일이 root 커밋 대상에 섞임 / root 문서가 모듈에 위치" | cross-boundary comparison | misplacement WARN | S3 |
| INV-004 | "SPEC 문서 위치와 참조 코드 경로의 모듈 불일치(Module Detection)" | location vs ownership comparison | SPEC-module WARN | S4 |
| INV-005 | "스테이징+비스테이징 혼재 / --spec 무관 파일 혼입" | grouping + set difference | 혼입 WARN, --spec 2그룹 | S5, S6 |
| INV-006 | "결정론적 순서(Phase A 알파벳순 → Phase B)" | deterministic ordering | 계획 블록 순서 | S7 |
| INV-007 | "read-only(변이 0), 기본 exit 0, --strict 위반 시 exit 1" | read-only state + exit contract | git 상태 불변, exit code | S8, S9, S10 |
| INV-008 | "모든 path의 배타적 완전 분할과 safe candidate-only 계획" | partition + shell safety | Phase/blocked/unclassified, add plan | S12 |
| INV-009 | "exactly-one SPEC host와 민감 Git 진단 비노출" | containment + diagnostic safety | pre-render error, sanitized error | S13 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| (1) 발견 + read-only 상태 | Primary SPEC T1/T2/T7 | covered |
| (2) Phase 분류 + 위반 3종 | Primary SPEC T3/T8 | covered |
| (3) --spec 소유/무관 분리 | Primary SPEC T4/T9 | covered |
| (4) 결정적 계획 + exit + 안전 | Primary SPEC T5/T10 | covered |
| 패리티 문서 | Primary SPEC T6 | covered |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | T13 focused race, vet, full CLI, live read-only, diff/file-size/tracked-ignored gate로 해소 |

## Evolution Ideas

Outcome Lock을 만족한 뒤에도 가능한 선택 개선이며 sync completion을 막지 않는다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| `--json` 기계 판독 미러 출력 | 텍스트 계획으로 Outcome Lock 충족 | 훅/CI가 구조화 파싱 요구 시 |
| `--fix`로 계획을 실제 스테이징까지 실행 | non-goal(변이 0) | 사용자가 명시 요청 |
| Lore 커밋 메시지 초안 생성 | non-goal(메시지 생성) | 사용자가 명시 요청 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC이 한 모듈 내 cohesive cobra 명령 추가로 Outcome Lock을 닫음 | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `internal/cli/root.go`(명령 등록), `internal/cli/verify.go`(플래그 패턴) | existing | Read/rg로 확인 |
| `pkg/setup/multirepo.go::DetectMultiRepo`, `pkg/setup/multirepo_types.go::RepoComponent/MultiRepoInfo` | existing | Read로 확인 |
| `internal/cli/status_hygiene.go:176 hygieneGitLines` + porcelain 파싱 | existing rejected for this command | C-quoted/rename 경계 때문에 NUL parser 사용 |
| `internal/cli/check_rules.go loreValidTypes`, `pkg/lore` | existing | rg로 확인 |
| `content/rules/doc-storage.md`(Storage Matrix·Module Detection·Sync Commit Rules) | existing | Read로 확인 |
| `.autopus/learnings/pipeline.jsonl`(L-001만 존재; L-003 부재) | existing | cat으로 확인(팀리드 L-003 인용 반증) |
| `internal/cli/sync.go`·`sync_verify_*.go` + `_test.go` | completed | S1~S13와 T13 품질 게이트 통과 |
| `newSyncCmd`·`sync verify` 서브커맨드·계획 출력 포맷 | [NEW] planned addition | 신규 |
| content sync 절차 문서의 `sync verify` 언급 라인 | [NEW] planned addition | 신규 |

## Reviewer Brief

- **Intended scope**: `auto sync verify` read-only 명령 1개(발견/분류/위반/계획/exit) + 패리티 문서 1곳. autopus-adk 단일 모듈 변경.
- **Explicit non-goals**: 자동 커밋/스테이징, Lore 메시지 생성, PR/merge 정책, 외부 repo, bare `sync` 변이.
- **Self-verified**: canonical keep/drop 완전 분할, NUL rename, `git add --` safe token, exactly-one host, workspace ownership, symlink/read-error fail-closed, optional locks, index invariant, Git diagnostic sanitization.
- **Reviewer should focus on**: policy drift, arbitrary Git filename handling, cross-repo ownership boundaries, read-only/index invariant. T13이 통과해 Completion Debt는 없다.

## Plan Intent Ledger

Clarification Ledger unavailable — 직접 `auto plan` 또는 BS 파일의 `## Clarification Ledger`/`## Question Audit`가 전달되지 않음. 팀리드 프롬프트의 `## Outcome Lock`·`## Constraints`·Motivating evidence는 위 Outcome Lock/보안 경계/Motivating Evidence 섹션에 scope contract로 보존하고, 재검증으로 L-003 부재와 desktop Lore 비율(34/40)을 정정함.

## Initial Plan Self-Verify Summary (historical; superseded by hardening correction)

- Q-CORR-01 | status: PASS | attempt: 2 | files: research.md, spec.md, plan.md | reason: 기존 참조(DetectMultiRepo·hygieneGitLines·root.go·doc-storage)를 Read/rg로 확인, L-003 부재 반영
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, plan.md, research.md | reason: 신규 파일/명령/포맷을 `[NEW]`로 표기
- Q-CORR-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: EARS는 비불릿 `THE SYSTEM SHALL` 라인, acceptance는 bare Given/When/Then/And
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline이 existing/[NEW] 분리
- Q-COMP-01 | status: PASS | attempt: 1 | files: all | reason: 4파일 상호 보완
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, acceptance.md | reason: REQ-001~011이 Traceability Matrix로 plan task·S1~S11에 추적
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock을 Primary SPEC이 닫음(Completion Debt None)
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, acceptance.md | reason: INV-001~007이 concrete 집합/순서/exit oracle(S1~S10)로 검증
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief로 리뷰 범위 제한
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(None)와 Evolution Ideas(--json/--fix/Lore 초안) 분리
- Q-FEAS-01 | status: PASS | attempt: 1 | files: plan.md, research.md | reason: 런타임 cobra 명령 layer, source of truth는 content/ 문서
- Q-FEAS-02 | status: PASS | attempt: 1 | files: research.md | reason: 대상 경로가 autopus-adk 모듈 구조와 일치
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ 서술에 모호어 없음
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority(Must/Should)와 EARS type(Event/State/Ubiquitous) 분리
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: git 출력·SPEC 문서·--spec를 untrusted 입력 경계로 처리
- Q-SEC-02 | status: PASS | attempt: 2 | files: research.md, acceptance.md | reason: --spec traversal 거부(S10)+상대 경로 전용 출력+read-only 인자 고정
