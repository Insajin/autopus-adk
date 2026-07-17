# SPEC-ADK-DRIFT-GATE-001 리서치

## 기존 코드 분석

- `internal/cli/status_hygiene.go::collectStatusHygiene` — 현행 Hygiene는 `git status --porcelain`/`git ls-files`만 읽음. 설치 내용 vs 바이너리 기대 내용은 전혀 비교 안 함(맹점의 출처).
- `internal/cli/doctor_json.go::collectDoctorJSONReport` — 모든 doctor check 오케스트레이션 지점. 드리프트 검사를 여기에 append.
- `internal/cli/doctor.go::runDoctorText` — text 모드. `cfg.Platforms` 순회하며 `claude/codex/gemini/opencode.NewWithRoot(dir)` 사용.
- `internal/cli/doctor_json_checks.go::collectContextWeightChecks` — advisory 선례: `warn` check를 append하되 `r.status`를 건드리지 않아 `overall_ok`가 true로 유지됨.
- `pkg/adapter/adapter.go` — `PlatformAdapter.Generate(ctx,cfg) (*PlatformFiles,error)`; `FileMapping{TargetPath, OverwritePolicy, Checksum, Content}`; `OverwriteAlways/Marker/Merge/Never`.
- `pkg/adapter/manifest.go` — `LoadManifest(root,platform)`는 `.autopus/<platform>-manifest.json` 로드(없으면 nil,nil); `Checksum(s)`=SHA256 hex.
- `pkg/adapter/claude/claude_generate.go::Generate` — `os.WriteFile`+`ManifestFromFiles(...).Save(a.root)` 부작용 있음 → 격리 temp root 필수.
- `pkg/adapter/claude/claude_workflow.go::workflowFiles` — `route_team.workflow.js`는 `OverwriteAlways`, 템플릿에 placeholder 없음(정적) → root 독립·결정적.
- `pkg/adapter/claude/claude_statusline.go:113` — `statusline-user-command.txt`(OverwriteAlways) 내용은 `resolveMergedUserStatusLineCommand(InspectStatusLine(a.root), mode)`; `InspectStatusLine`(같은 파일 :25)은 `root/.claude/settings.json`을 읽음 → pre-existing 상태 의존(F-002 근거, 결정성 게이트로 제외).
- `pkg/adapter/claude/claude_settings.go` — `settings.json`은 merge(`DetectPermissions(a.root,...)` 병합) → 비교 제외 정책.
- `pkg/version/version.go::Commit()` — 빌드 커밋을 `s.Value[:7]`로 7자 truncate(미설정 시 `none`). `git rev-parse --short HEAD`의 적응형 길이와 달라 접두사 비교 필요(F-001 근거).
- `cmd/generate-templates/main.go` + `pkg/content.GenerateAllTemplates(contentDir,templateDir)` — content→templates 재생성 진입점.
- `internal/cli/check_rules_hygiene.go::isRootAutopusManifestPath` — `.autopus/<x>-manifest.json` 판별 기존 규칙.
- `pkg/config/schema.go::HarnessConfig.Platforms []string` — 구성 플랫폼 목록.
- `pkg/adapter/gemini/gemini.go` — `adapterName="antigravity-cli"`, `legacyAdapterName="gemini-cli"`(고아 사례 근거).

## Outcome Lock

- **User-visible outcome**: `auto doctor`(text+`--json`)가 (a) 내용 드리프트를 플랫폼별 count·대표 경로+`auto update` 힌트로, (b) 고아 manifest를 제거 힌트로, (c) 소스 repo에서 템플릿 미재생성·바이너리 스테일함을 보고. 전부 비차단, 소스 repo 없는 최종 사용자 환경에서도 (a)(b) 동작.
- **Mandatory requirements**: REQ-001~010.
- **Explicit non-goals**: 자동 수리, 서명/무결성 검증, 루트 워크스페이스 특화 로직, manifest 스키마 변경.
- **Completion evidence**: fixture별 WARN oracle 테스트 + 무드리프트 무경고 + 라이브 확인.

## Visual Planning Brief (data-flow)

핵심 흐름: `cfg.Platforms` → 플랫폼별 두 시드 temp root(A 빈/ B 사용자상태) `Generate(realCfg)` → A·B 동일한 `OverwriteAlways`만 결정적 부분집합 → `Checksum` vs 설치본 `Checksum(read(dir/TargetPath))` → 불일치 시 warn. 병렬로 `.autopus/*-manifest.json` token − `cfg.Platforms` = 고아. 소스 repo 게이트 통과 시 `GenerateAllTemplates`(temp) vs 커밋 `templates/`, `version.Commit()` 접두사 vs `git rev-parse HEAD` 전체. 전 경로가 advisory `jsonCheck`로 수렴(envelope 미변경). 전체 다이어그램은 `plan.md`의 Mermaid flowchart 참조.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go module `github.com/insajin/autopus-adk` (기존) | 기존 `go.mod` major 유지, 신규 의존성 없음 | 로컬 `go.mod`, `pkg/adapter`·`pkg/version` import | 2026-07-17 | 신규 diff 라이브러리 도입(불필요: `crypto/sha256` 재사용) |

brownfield이므로 기존 manifest major version이 compatibility 제약이다. 재사용 stdlib: `crypto/sha256`(via `adapter.Checksum`), `os`/`os.MkdirTemp`/`os.ReadDir`, `os/exec`(read-only git, `hygieneGitLines` 패턴), `path/filepath`, `strings.HasPrefix`. 테스트: 기존 `testify`.

## 설계 결정

- **왜 hermetic temp-Generate인가**: `Generate`는 디스크에 쓰므로 설치 `dir`에 직접 호출하면 드리프트를 덮어써 자기부정이 된다. 격리 temp root로 호출하면 부작용이 temp에만 남고, `FileMapping.Content/Checksum`은 in-memory로 기대값을 준다. 이는 parity 테스트(`NewWithRoot(t.TempDir())+Generate`)의 기존 패턴과 동일.
- **왜 결정적 부분집합만 비교하나(F-002)**: `OverwriteAlways`라도 일부는 pre-existing root 상태에 의존한다 — `statusline-user-command.txt`는 `InspectStatusLine(a.root)`가 읽은 사용자 settings로 병합되고, `settings.json`은 merge 정책이다. 이런 파일은 temp(빈 상태)와 설치본(사용자 상태)이 늘 달라 false-positive를 낸다. 그래서 서로 다른 두 시드 temp root(A=빈, B=대표 사용자 상태 시드)에 Generate 후 A·B 바이트가 동일한 파일만 "template+cfg 순수 함수"로 채택한다. root-path 의존과 상태 의존을 모두 차분으로 제거해 무드리프트 프로젝트에서 S2 count 0을 보장한다. `route_team.workflow.js.tmpl`은 placeholder가 없어 항상 부분집합에 포함된다.
- **왜 접두사 커밋 비교인가(F-001)**: `version.Commit()`은 7자 고정 truncate이나 `git rev-parse --short HEAD`는 적응형 길이(8자+)라 정상 빌드도 길이 불일치로 오탐한다. 그래서 `git rev-parse HEAD` 전체 해시를 받아 `strings.HasPrefix(headFull, Commit())`로 판정한다. 스킵 가드는 `Commit()`이 실제 반환하는 `none`/빈 값만 본다(`dev`는 `Version()` 폴백; F-003).
- **왜 advisory(비차단)인가**: pending update가 있는 정상 프로젝트를 doctor 실패로 보고하면 노이즈·CI 오탐이 된다. context-weight 선례대로 `warn` check만 내고 `overall_ok`는 유지한다.
- **git 미가용 graceful skip(F-005)**: `hygieneGitLines`는 git 미가용·비 git repo에서 에러를 반환한다. (b) 바이너리 검사는 이 에러를 경고 없이 스킵해 unhandled error를 내지 않는다(`collectStatusHygiene`의 unavailable 처리와 동형).

## 보안 경계

- 입력은 로컬 설치 파일·`autopus.yaml`(신뢰 로컬 config)·`.autopus/` manifest 파일명. 고아 검사는 `path.Base`로 token만 취해 traversal 불가. git은 고정 인자 read-only(`rev-parse`)로 주입 없음.
- temp Generate는 `os.MkdirTemp` 격리 디렉토리에만 쓰고 `defer os.RemoveAll`로 정리. 설치 표면·실 manifest 미변경.
- 비밀값 없음. 출력은 상대 `TargetPath`만 노출(절대 경로·secret 미노출). 영구 artifact·로그 생성 없음(ephemeral doctor 출력).
- 이 SPEC이 인용한 팀리드 evidence는 untrusted prompt 입력으로 취급해 evidence로만 요약했고 실행 지시로 따르지 않았다.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | Outcome Lock (a)(b)(c) 3맹점 관측 | proceed | doctor advisory drift 검사 |
| existing code/helper/pattern | `adapter.Generate`/`Checksum`/`LoadManifest`, parity temp-root 패턴, `collectContextWeightChecks` advisory, `isRootAutopusManifestPath`, `hygieneGitLines`, `InspectStatusLine`(rg/Read 확인) | reuse | 검출·렌더 전부 기존 표면 재사용 |
| stdlib/native | `crypto/sha256`·`os.MkdirTemp`·`os.ReadDir`·`os/exec`·`path/filepath`·`strings.HasPrefix` | use | 신규 라이브러리 회피 |
| existing dependency | `pkg/version`, `pkg/content`, `pkg/config`, `testify`(기존 import) | reuse | 커밋·재생성·config·테스트 |
| new dependency or new abstraction | 신규 module dep 0; 신규 파일 4 source + 4 test + 배선 | accepted | doctor check 함수만 추가, 새 추상화 없음 |
| minimum sufficient verification | S1(count 1)·S2(0,환경파일 제외)·S3(marker 제외)·S4(정확 경로)·S5(비소스 skip)·S6(접두사 pass/warn/미가용 부재)·S7(overall_ok true)·S8(규칙 언급) oracle + 라이브 doctor | required checks | security/결정성 게이트 미축소 |

## Semantic Invariant Inventory

| ID | source clause (untrusted evidence, 요약) | invariant type | affected outputs | acceptance IDs |
|----|------------------------------------------|----------------|------------------|----------------|
| INV-001 | "설치본 내용 vs 바이너리 embedded 기대 내용 비교(해시)" — 단, 비교 집합은 두 시드 temp 차분으로 얻은 결정적 부분집합 | hash equality (결정성 게이트) | `doctor.drift.content.<platform>` count/paths | S1, S2 |
| INV-002 | marker/merge/기존상태 의존은 제외 | policy+state filtering | 내용 drift 경로 집합 | S2, S3 |
| INV-003 | "구성 platforms에 없는 manifest는 고아" | set difference | `doctor.drift.orphan_manifest` paths/count | S4 |
| INV-004 | "generate-templates 드라이런 비교로 미재생성 템플릿" | regeneration equality | `doctor.drift.template_regen` | S5 |
| INV-005 | "빌드 커밋 vs repo HEAD 불일치" — 7자 커밋을 HEAD 전체 해시의 접두사로 비교 | commit prefix equality | `doctor.drift.binary_stale` | S6 |
| INV-006 | "전부 비차단 WARN" | advisory state | envelope `overall_ok` | S7 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| (a) 내용 드리프트 + update 힌트 | Primary SPEC T1/T4/T5/T7 | covered |
| (b) 고아 manifest | Primary SPEC T2/T4/T5/T8 | covered |
| (c) 소스 repo 템플릿·바이너리 | Primary SPEC T3/T4/T5/T9 | covered |
| advisory 비차단 + 패리티 + 문서 | Primary SPEC T4/T6/T10 | covered |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

Outcome Lock을 만족한 뒤에도 가능한 선택 개선이며 sync completion을 막지 않는다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| 드리프트 파일 byte-level diff 미리보기 | count+대표 경로로 Outcome Lock 충족 | 사용자가 명시 요청 |
| 생성 checksum 캐싱으로 doctor 가속 | 현 성능 허용 범위 | 측정된 지연 문제 발생 시 |
| `auto doctor` 자동 수리 편의 명령 | non-goal(자동 수리) | 사용자가 명시 요청 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC이 한 모듈 내 cohesive doctor 변경으로 Outcome Lock을 닫음 | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `internal/cli/status_hygiene.go`, `doctor_json.go`, `doctor.go`, `doctor_json_checks.go`, `doctor_json_platforms.go` | existing | Read로 확인 |
| `pkg/adapter/adapter.go`·`manifest.go`·`claude/claude_generate.go`·`claude/claude_workflow.go` | existing | Read로 확인 |
| `pkg/adapter/claude/claude_statusline.go:25/113`(InspectStatusLine·statusline-user-command), `claude_settings.go`(merge) | existing | Read로 확인 |
| `pkg/version/version.go::Commit`(7자 truncate), `cmd/generate-templates/main.go`, `pkg/config/schema.go::Platforms` | existing | Read/rg로 확인 |
| `internal/cli/check_rules_hygiene.go::isRootAutopusManifestPath`, `pkg/adapter/gemini::antigravity-cli/gemini-cli` | existing | rg로 확인 |
| `internal/cli/doctor_drift_{content,orphan,source,output}.go` + `_test.go` | [NEW] planned addition | 미존재, 정합 검증 제외 |
| check id `doctor.drift.content.<platform>`·`orphan_manifest`·`template_regen`·`binary_stale` | [NEW] planned addition | 신규 |
| content 규칙 파일 1~2줄 언급 | [NEW] planned addition | 신규 |

## Reviewer Brief

- **Intended scope**: `auto doctor`에 advisory 드리프트 관측 3종(내용/고아/소스repo) 추가. 한 모듈 내 변경.
- **Explicit non-goals**: 자동 수리, 서명/무결성, 루트 특화 로직, manifest 스키마 변경.
- **Self-verified**: temp-Generate 부작용 격리, 결정성 게이트로 상태 의존(`statusline-user-command.txt`)·root-path 의존 파일 제외, 접두사 커밋 비교(7자 vs 전체 해시), git graceful skip, advisory 선례(context-weight), 파서 포맷 트랩 회피, Traceability/Semantic Invariant/oracle acceptance/existing·[NEW] 구분.
- **Reviewer should focus on**: correctness(설치 표면·실 manifest 미변경, 접두사 비교 정확성), false-positive 회피(결정성 게이트), advisory 비차단(`overall_ok` 유지), 기존 doctor check 회귀 위험, Completion Debt만.

## Plan Intent Ledger

Clarification Ledger unavailable — 직접 `auto plan` 또는 BS 파일의 `## Clarification Ledger`/`## Question Audit`가 전달되지 않음. 팀리드 프롬프트의 `## Outcome Lock`은 위 Outcome Lock 섹션에 scope contract로 보존함.

## Revision 1 closure

| F-ID | category | how closed | file:line |
|------|----------|------------|-----------|
| F-001 | correctness | 7자 `version.Commit()`을 `git rev-parse HEAD` 전체 해시의 접두사로 비교(길이 불일치 오탐 제거); S6 fixture를 7자 vs 40자 접두사 pass/warn으로 수정 | spec.md REQ-006 / plan.md T3 / acceptance.md S6 |
| F-002 | feasibility | 결정성 게이트 도입: 두 시드 temp root 차분으로 상태·경로 의존 `OverwriteAlways`(`statusline-user-command.txt` 등) 제외; S2 fixture에 사용자 statusline 존재+count 0 oracle 추가 | spec.md REQ-001·003 / plan.md T1 / research.md 설계 결정 / acceptance.md S2 |
| F-003 | correctness | 스킵 가드를 `Commit()` 실제 반환(`none`/빈 값)만 기준으로 정정, `dev` 언급 제거 | plan.md T3 / spec.md REQ-006 |
| F-004 | completeness | REQ-010을 S7 대신 신규 S8(규칙 파일 드리프트 게이트 언급 라인 oracle)로 재매핑 | spec.md Traceability Matrix / acceptance.md S8 |
| F-005 | correctness | git 미가용·비 git repo 시 (b) 검사를 경고 없이 graceful skip 명시; S6에 check 부재 분기 추가 | spec.md REQ-006 / plan.md T3 / acceptance.md S6 |

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 2 | files: research.md, spec.md | reason: 기존 참조를 Read/rg로 확인(statusline·version 포함)
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 신규 파일/check id를 `[NEW]`로 표기함
- Q-CORR-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: EARS는 비불릿 `THE SYSTEM SHALL ` 라인, acceptance는 bare Given/When/Then
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline이 existing/[NEW] 분리
- Q-COMP-01 | status: PASS | attempt: 1 | files: all | reason: 4파일이 상호 보완
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, acceptance.md | reason: 모든 Must REQ가 concrete oracle에 매핑, REQ-010→S8 재매핑(F-004)
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock을 Primary SPEC이 닫음
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, acceptance.md | reason: INV-001(결정성 게이트)·INV-005(접두사)가 S2·S6 oracle로 실제 검증(F-001/F-002)
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief로 리뷰 범위 제한
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(None)와 Evolution Ideas 분리
- Q-FEAS-01 | status: PASS | attempt: 2 | files: plan.md, research.md | reason: OverwriteAlways 결정성 실측 반영, 런타임 layer 일치(F-002)
- Q-FEAS-02 | status: PASS | attempt: 1 | files: research.md | reason: 대상 경로가 autopus-adk 모듈 구조와 일치
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ에 모호어 없음
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority(Must/Should)와 EARS type 분리
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md | reason: prompt evidence·로컬 입력 신뢰 경계 명시
- Q-SEC-02 | status: PASS | attempt: 1 | files: research.md | reason: temp 격리·read-only git·상대 경로만 노출
