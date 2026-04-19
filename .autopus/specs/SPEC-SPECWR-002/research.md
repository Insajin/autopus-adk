# SPEC-SPECWR-002 리서치

## 기존 코드 분석

### pkg/spec/prompt.go — 현재 프롬프트 조립 흐름

`BuildReviewPrompt(doc, codeContext, opts, specDir...)` (라인 17)이 현재 다음 순서로 `strings.Builder`에 문자열을 쌓는다.

1. 서두: `"You are reviewing a SPEC document..."` (라인 20)
2. `## SPEC: {ID} — {Title}` 헤더 (라인 21)
3. `### Full SPEC Document` 블록 — `doc.RawContent` 원문 주입 (라인 26-29), 없으면 `### Requirements` fallback(라인 32-38)
4. `### Acceptance Criteria` 블록 — 파싱된 criteria 열거 (라인 41-47)
5. `injectAuxDocs`로 `plan.md` / `research.md` / `acceptance.md` 순서대로 주입 (라인 54-60)
6. `### Existing Code Context` — 코드 ``` 펜스 블록 (라인 62-67)
7. 모드 분기 — verify 또는 discover 지시문 (라인 69-73)

**체크리스트 주입 지점 후보**: 6번(코드 컨텍스트) 이후, 7번(Instructions) 이전. 정확히는 라인 68과 69 사이에 `injectChecklist(&sb, opts)` 한 줄을 추가하면 의미론적으로 자연스럽다. 이유는 (a) 리뷰어가 SPEC/AUX/CODE를 모두 본 상태에서 체크리스트 기준을 받아야 하고, (b) 체크리스트가 Instructions의 직전 컨텍스트로 작용해야 응답 포맷 지시(R05)와 자연스럽게 이어진다.

현재 `trimToLines`(라인 111-119)는 content를 개행 분할하여 maxLines 초과 시 `... (trimmed N more lines)` 공지 추가. 체크리스트 주입 시 그대로 재사용 가능.

### pkg/spec/types.go — 타입 구조

핵심 관찰:
- 라인 35: `Criterion.Priority string` 문자열 필드, enum 타입 아님. 값으로 `Must / Should / Nice` 문자열 허용(gherkin_parser.go 라인 18의 정규식이 유일한 제약).
- 라인 84-90: `FindingCategory` 5개(correctness/completeness/feasibility/style/security).
- 라인 125-131: `ReviewResult{SpecID, Verdict, Findings, Responses, Revision}`. `ChecklistOutcomes` 필드가 현재 없음.

### pkg/spec/gherkin_parser.go — Priority 어휘

라인 17-18:
```
rePriority = regexp.MustCompile(`(?i)^\s*Priority:\s*(Must|Should|Nice)`)
```
정규식이 `Must|Should|Nice` 캡처 그룹만 허용. 다른 어휘(`High/Med/Low`, `P0/P1/P2`)는 파서에서 버려진다.

### pkg/spec/reviewer.go — 응답 파싱

라인 10-14의 정규식 4개:
- `verdictRe` — `VERDICT: PASS|REVISE|REJECT`
- `findingRe` — legacy `FINDING: [sev] desc`
- `structFindingRe` — `FINDING: [sev] [cat] scope desc`
- `findingStatusRe` — `FINDING_STATUS: F-ID | status | reason`

체크리스트 파싱용 `reChecklist` 추가는 기존 선언 블록에 한 줄 추가만으로 충분. `ParseVerdict`(라인 18-53) 안에서 `structFindingRe` 매칭 직전/직후에 매칭 루프 추가.

### internal/cli/spec_review.go — CLI 엔트리

`runSpecReview`(라인 44-142)가 최종 결과를 출력하는 블록은 라인 130-139:
```go
fmt.Printf("SPEC 리뷰 완료: %s\n", specID)
fmt.Printf("판정: %s\n", finalResult.Verdict)
if len(finalResult.Findings) > 0 {
    fmt.Printf("발견 사항: %d건\n", len(finalResult.Findings))
}
```
체크리스트 결과 렌더링은 이 직후에 삽입.

### internal/cli/spec_review_loop.go — 루프 구조

`runSpecReviewLoop`에서 `ReviewResult`를 `PersistReview`로 기록(라인 99). 체크리스트 결과를 review.md에 포함하려면 `persist_review`(`pkg/spec/review_persist.go` 별도 파일)를 손봐야 하나, 이는 R13(P2)으로 선택적.

### content/embed.go — 임베드 패턴

라인 9:
```go
//go:embed skills/*.md agents/*.md hooks/*.sh hooks/*.md methodology/*.yaml rules/*.md statusline.sh profiles/executor/*.md
```

현재 패턴에 `rules/*.md`(평탄)가 이미 포함되어 있다. SPEC-001의 결정에 따라 체크리스트는 `content/rules/spec-quality.md`(평탄)로 배치되므로 **기존 embed 패턴이 자동으로 커버한다**. 별도 디렉티브 변경이 불필요하며, 본 SPEC은 회귀 테스트로 이 자동 포함 사실을 명시적으로 잠그기만 한다(R02, S9).

초기 작성 시 `content/rules/autopus/spec-quality.md` 경로를 가정하고 `rules/autopus/*.md` 패턴 추가를 필수 변경점으로 기술했으나, 리뷰 1회차 피드백과 실제 리포 구조 확인(`ls autopus-adk/content/rules/`: 11개 평탄 md) 결과 SPEC-001이 평탄 배치로 결정하여 불필요한 embed 변경을 제거했다. 이는 변경 표면을 축소하고 회귀 리스크를 낮추는 순방향 수정이다.

### content/rules/ 현재 상태

`ls autopus-adk/content/rules/` 결과: 11개 md 파일이 평탄하게 배치(`branding.md`, `context7-docs.md`, `deferred-tools.md`, `doc-storage.md`, `file-size-limit.md`, `language-policy.md`, `lore-commit.md`, `objective-reasoning.md`, `project-identity.md`, `subagent-delegation.md`, `worktree-safety.md`). SPEC-001이 `spec-quality.md`를 이 평탄 레이어에 12번째 파일로 추가한다. 본 SPEC은 파일이 생성되었을 때 동작하는 로더와 그렇지 않을 때 soft-fallback을 함께 구현한다.

## 설계 결정

### 결정 1: soft-fallback vs fail-fast (R04)

**선택**: soft-fallback.

**근거**:
- SPEC-001과 SPEC-002가 병렬 개발될 수 있고, SPEC-001 머지 전에 SPEC-002 코드가 프로덕션에 올라가도 런타임 크래시가 나면 안 된다.
- 체크리스트는 "보강" 자산이다. 기존 5 FindingCategory 정적 검증 + provider 자유 리뷰가 이미 판정 충분성을 제공한다.
- fail-fast를 선택하면 `content/rules/spec-quality.md` 누락 시 모든 `auto spec review` 호출이 실패하여 장애 영향이 크다.
- 대신 stderr 경고 로그를 명시적으로 남겨 silent degradation을 방지한다.

**트레이드오프**: 체크리스트 없이 리뷰가 진행되는 상태를 사용자가 놓칠 수 있다. 완화책으로 CLI 출력에 "체크리스트 결과: 0건" 같은 명시 라인을 넣는 방안을 P2(R14 인근)로 고려.

### 결정 2: Priority enum 유지 vs 확장

**선택**: 유지.

**근거**:
- 현재 `Criterion.Priority` 타입이 `string`이고 정규식 제약으로 `Must / Should / Nice`만 파서를 통과한다. SPEC-001의 체크리스트 우선순위 어휘도 동일 3단계를 사용한다.
- 확장을 선택하면 (a) 정규식 변경, (b) `string` → 신규 enum 타입 전환, (c) 기존 19개 `SPEC-ORCH-*` 등 acceptance.md 파일 전면 재검증이 필요하다. 변경 범위가 본 SPEC Out of Scope인 기존 정적 검증 재작성을 강제한다.
- P0 어휘 일관성은 SPEC-001에서 문서 레벨로 잡고, Go 타입은 그대로 둔다. 추후 enum 타입화 제안이 오면 `SPEC-SPECWR-003`으로 분리.

**근거 증거**: `pkg/spec/gherkin_parser.go` 라인 17-18, `pkg/spec/types.go` 라인 35.

### 결정 3: embed vs 디스크 읽기

**선택**: embed 우선, 디스크 fallback.

**근거**:
- 바이너리 배포 환경(`auto` CLI 단일 바이너리)에서는 embed가 유일한 신뢰 경로이다.
- 개발 환경에서 `content/rules/spec-quality.md`를 편집 후 재빌드 없이 테스트하려면 디스크 읽기 경로가 편리하다.
- 디스크 경로가 존재하고 읽기 성공하면 디스크 값을 우선시하여 hot-reload 유사 DX를 제공한다.

### 결정 4: self-verification 로그를 Go 헬퍼로 고정

**선택**: LLM이 로그 파일을 직접 편집하지 않고 `auto spec self-verify --record` 서브커맨드로만 기록.

**근거**:
- LLM의 직접 파일 편집은 포맷 불일치, race condition, retention 정책 미준수 위험이 있다.
- Go 함수에 retention(100 라인 캡)을 내장하면 로그 폭주가 자동 방지된다.
- 동일 규칙을 CI 워크플로우 등 비-LLM 컨텍스트에서도 재사용 가능.

### 결정 5: ReviewResult 필드 추가 vs 신규 타입

**선택**: `ReviewResult` 구조체에 `ChecklistOutcomes` 필드 추가.

**근거**:
- 체크리스트 결과는 논리적으로 리뷰 결과의 일부이다. 별도 타입을 두면 CLI, persist, merge 모든 경로에서 이중 전달이 필요하다.
- 기존 `Findings`와 성격이 다르므로 `ReviewFinding` 구조체는 손대지 않는다.
- `json.Marshal` 호환성 유지(기존 필드 순서 보존, 신규 필드는 뒤에 추가).

## SPEC-001과의 의존 관계 (텍스트 다이어그램)

```
    SPEC-SPECWR-001                         SPEC-SPECWR-002
    ───────────────                         ───────────────
         │                                       │
         │ produces                              │ consumes
         ▼                                       ▼
    ┌──────────────────────────────────┐   ┌────────────────────────────┐
    │ content/rules/spec-quality.md    │   │ pkg/spec/checklist.go      │
    │ (체크리스트 정의, 5개 차원       │───▶  LoadChecklist(content.FS) │
    │  + cohesion 부록)                │   │                            │
    └──────────────────────────────────┘   │ pkg/spec/prompt.go         │
                                           │   BuildReviewPrompt(...)   │
    ┌──────────────────────────────────┐   │                            │
    │ content/agents/spec-writer.md    │   │ pkg/spec/reviewer.go       │
    │ content/skills/spec-review.md    │   │   ParseVerdict(...)        │
    │ (체크리스트 참조 절차 문서화)    │   │                            │
    └──────────────────────────────────┘   │ internal/cli/              │
                │                          │   spec_review.go           │
                │ 자체 검증 호출           │   spec_self_verify.go      │
                ▼                          └────────────────────────────┘
    ┌──────────────────────────────────┐            ▲
    │ spec-writer 에이전트이 Bash 도구로│           │
    │ auto spec self-verify --record   │───────────┘
    │ 호출 (R11)                       │
    └──────────────────────────────────┘
```

**의존 방향**: 002 → 001 (단방향). 001의 산출물은 md 파일과 에이전트 문서이며, 이를 002의 Go 코드가 소비한다. 001은 002 없이도 독립적으로 가치(문서화)를 제공하지만, 실제 런타임 효과는 002 구현 후에 나타난다.

## 체크리스트 주입 지점 후보 (요약)

| 후보 | 위치 | 장점 | 단점 |
|------|------|------|------|
| A (채택) | `prompt.go` 라인 68 직후, 69 직전 | Instructions 직전 배치로 응답 포맷 지시와 자연 연결 | 코드 컨텍스트가 길 때 체크리스트가 뒤로 밀림 |
| B | `prompt.go` 라인 47 직후 (AC 이후) | SPEC 구조 설명과 바로 이어짐 | 코드 컨텍스트 보기 전 체크리스트를 먼저 보게 되어 feasibility 판단이 약화 |
| C | `injectAuxDocs` 내부의 네 번째 항목 | 기존 헬퍼 재사용 | aux docs는 SPEC 디렉토리 파일, 체크리스트는 content embed 자산 — 의미론 혼합 |

A 채택. 코드상 라인 번호는 R03의 요구사항에 명시.

## 변경량(LOC) 재검토

plan.md의 추정표 기준: production ~256 LOC, tests ~160 LOC, 총 ~416 LOC. 300 LOC/파일 제한은 신규 파일 각각 100 LOC 이하이고, 기존 파일 중 가장 증가폭이 큰 `prompt.go`(현재 200) + 약 40 = 240으로 제한 내이다.

feasibility 판단: 단일 개발자 2~3일 작업량. 의존성은 SPEC-001(문서 작업)뿐이며 외부 시스템 연동 없음. 정적 컴파일 Go 프로젝트이므로 회귀 리스크는 낮다.

## 참조 파일 존재 확인

- [x] `autopus-adk/pkg/spec/prompt.go` — 확인 (200 라인)
- [x] `autopus-adk/pkg/spec/types.go` — 확인 (131 라인)
- [x] `autopus-adk/pkg/spec/reviewer.go` — 확인 (179 라인)
- [x] `autopus-adk/pkg/spec/gherkin_parser.go` — 확인
- [x] `autopus-adk/pkg/spec/validator.go` — 확인
- [x] `autopus-adk/pkg/spec/findings.go` — 확인 (188 라인)
- [x] `autopus-adk/internal/cli/spec_review.go` — 확인
- [x] `autopus-adk/internal/cli/spec_review_runtime.go` — 확인
- [x] `autopus-adk/internal/cli/spec_review_loop.go` — 확인
- [x] `autopus-adk/content/embed.go` — 확인
- [x] `autopus-adk/content/rules/spec-quality.md` — 확인. 기존 `rules/*.md` 패턴으로 embed FS에 포함되며 `content.FS` 회귀 테스트로 고정했다.
- [x] `autopus-adk/pkg/spec/checklist.go` — 구현 완료
- [x] `autopus-adk/pkg/spec/selfverify.go` — 구현 완료
- [x] `autopus-adk/internal/cli/spec_self_verify.go` — 구현 완료

## 구현 결과 요약

- `pkg/spec/checklist.go`: embed 우선 + 디스크 fallback 로더, `## Quality Checklist` 주입, 체크리스트 예시 포맷 생성.
- `pkg/spec/prompt.go`: 코드 컨텍스트 직후 체크리스트 섹션 삽입, 체크리스트가 있을 때만 `CHECKLIST:` 응답 형식 요구.
- `pkg/spec/reviewer.go` / `pkg/spec/types.go`: `ChecklistOutcome`, `ChecklistStatus`, `ReviewResult.ChecklistOutcomes` 추가 및 `CHECKLIST:` 라인 구조화 파싱.
- `internal/cli/spec_review_loop.go` / `internal/cli/spec_review.go`: provider checklist outcomes 집계와 최종 CLI 요약 출력 연결.
- `pkg/spec/selfverify.go` / `internal/cli/spec_self_verify.go`: JSON Lines self-verify append + 100라인 retention + `auto spec self-verify` 명령 추가.
- `.gitignore`: `.self-verify.log` 제외 규칙 추가.

## 구현 검증

- `go test ./pkg/spec`
- `go test ./internal/cli`
- `go test ./content/...`
- `git check-ignore -v .autopus/specs/SPEC-TMP-001/.self-verify.log`
