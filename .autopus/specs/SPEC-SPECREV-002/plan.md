# SPEC-SPECREV-002 구현 계획

## Implementation Strategy

전략은 "최소 표면, 최대 관측성"이다. 각 결함은 단일 함수 또는 정규식 한 줄 수준의 좁은
변경으로 닫고, 변경마다 순수 함수(`loopAwareMaxRevisions`, `resolveSpecReviewMaxRevisions`,
`ParseEARSWithWarnings`, `wrapSpecLoadError`)를 도입해 oracle 테스트가 결정적으로 단언할 수
있게 한다. 리프 헬퍼뿐 아니라 실제 소비 배선(`runSpecReviewWithOptions`의 한도 반영,
`ValidateSpec`의 경고 표면화, `RenderChecklistSection`의 N/A 마커)을 함께 테스트해 새 함수가
호출자 없이 남는 inert seam을 만들지 않는다.

후방호환 원칙:
- `reChecklist`는 status alternation에 `N/A`만 추가한다. PASS/FAIL 매칭과 reason 캡처
  그룹은 그대로 유지되어 기존 테스트(`TestParseVerdict_ParsesChecklistOutcomes`)가 보존된다.
- `ParseEARS(text) ([]Requirement, error)` 시그니처는 변경하지 않는다. 경고는
  `ParseEARSWithWarnings`라는 신규 함수가 반환하고, `ParseEARS`는 그 wrapper로 경고를
  버린다. 기존 호출자(`template.go:69`, `parser_test.go`)는 무영향.
- `AppendSelfVerifyEntry`의 N/A 빈 reason 거부는 새 검증이다. 기존 N/A 테스트
  (`spec_self_verify_test.go`의 `TestSpecSelfVerifyCmd_AcceptsNAStatus`)는 이미 reason을
  전달하므로 회귀하지 않는다.
- `RenderChecklistSection`의 N/A 마커는 빈 reason N/A에만 적용된다. PASS/FAIL의 빈 reason은
  기존 `emptyNotePlaceholder`(`-`)를 유지한다.
- `--loop`는 floor 의미론을 쓴다. `loopModeMinRevisions`(5)는 기본 `defaultMaxRevisions`(3)
  보다 크므로 기본 설정에서도 효과가 관측된다. 설정값이 floor 이상이면 변경 없음.

소비처 배선(inert seam 방지):
- `--loop`: `runSpecReviewWithOptions`가 `flags.LoopMode`를 읽어 `resolveSpecReviewMaxRevisions`를
  호출하고, 결과를 `specReviewLoopParams.maxRevisions`로 전달한다. `resolveSpecReviewMaxRevisions`는
  `gate.MaxRevisions`(없으면 `defaultMaxRevisions`)와 LoopMode를 함께 소비한다 → S11이 검증.
- EARS 경고: `ValidateSpec`가 `doc.RawContent`에 대해 경고를 산출하여 warning 레벨
  `ValidationError`로 추가하고, 기존 `auto spec validate`(spec.go) stderr 출력 경로로 흐른다 → S12가 검증.
- N/A 빈 reason 관측: `RenderChecklistSection`이 review.md `## Checklist Summary` 표에 전용 마커를
  렌더한다 → S13이 검증.

## Visual Planning Brief

리뷰 파이프라인의 N/A 데이터 흐름과 본 SPEC이 닫는 누락 지점(굵게):

```
provider stdout
   │  "CHECKLIST: Q-SEC-01 | N/A | doc-only"  또는  "CHECKLIST: Q-SEC-01 | N/A" (빈 reason)
   ▼
spec.ParseVerdict ─► parseChecklistOutcomes
   │                      │
   │                reChecklist  ◄── [REQ-001] (PASS|FAIL) → (PASS|FAIL|N/A)
   ▼                      ▼
ChecklistOutcome{Status: N/A, Reason: ""}
   │                      │
   ▼                      ▼
RenderChecklistSection ◄── [REQ-007] 빈 reason N/A → naMissingReasonNote ("reason missing")
   │                          (PASS/FAIL 빈칸은 "-" 유지)
   ▼
review.md "## Checklist Summary" (N/A 카운트 + 누락 마커로 구분)

auto spec self-verify --status N/A (reason 누락)
   ▼
AppendSelfVerifyEntry ◄── [REQ-002] N/A + empty reason → reject (no .self-verify.log write)

auto spec review --loop
   ▼
runSpecReviewWithOptions ─► resolveSpecReviewMaxRevisions(gate, LoopMode) ◄── [REQ-003]
   │                              └► loopAwareMaxRevisions(cfg, true) = max(cfg, loopModeMinRevisions=5)
   ▼
specReviewLoopParams.maxRevisions ─► runSpecReviewLoop

spec.md 라인 "The button SHALL respond"
   ▼
ParseEARSWithWarnings ◄── [REQ-004] detectEARSType=="" + uppercase SHALL → warning
   ▼
ValidateSpec ──► ValidationError{Level: warning} ──► auto spec validate (stderr)

auto spec review (spec.Load 실패: 예) SPEC ID 헤더 부재)
   ▼
wrapSpecLoadError(specID, err) ◄── [REQ-005] 정확한 prefix + %w (≠ "본문이 비어있습니다")
   (참고: 성공 Load의 RawContent=="" 가드는 도달 불가 방어코드, 유지하되 acceptance 전제 안 함)
```

서킷 브레이커 첫 반복 가드(`revision > 0`)는 의도된 설계이므로 흐름을 변경하지 않는다.

## Feature Completion Scope

Primary SPEC 단독으로 Outcome Lock을 닫는다. 7개 요구사항은 모두 단일 모듈(`autopus-adk`) 내
`pkg/spec`와 `internal/cli`에 국한되며, 서로 독립적으로 구현·검증 가능하다. REQ-007은 Outcome
Lock의 "provider N/A가 손실 없이 파싱·검증된다"의 검증(관측) 측면을 provider 파싱 경로까지
확장해 닫는다. 승인된 sibling 의존성 없음. 남은 Completion Debt 없음
(research.md `## Completion Debt` = None). 서킷 브레이커 finding은 by-design으로 기각되어
범위에서 제외되었고, 이는 Outcome Lock을 막지 않는다.

## Tasks

- [ ] T1: `pkg/spec/reviewer.go`의 `reChecklist` status 그룹을 `(PASS|FAIL)`에서 `(PASS|FAIL|N/A)`로 확장한다. `parseChecklistOutcomes`의 `strings.ToUpper` 매핑이 `ChecklistStatusNA`를 산출하는지 확인한다 (이미 동작, 회귀 테스트로 잠금). (REQ-001)
- [ ] T2: `pkg/spec/selfverify.go`의 `AppendSelfVerifyEntry`에 `Status == ChecklistStatusNA && strings.TrimSpace(Reason) == ""`이면 descriptive 오류를 반환하는 가드를 추가한다. 로그 기록 전에 거부한다. (REQ-002)
- [ ] T3: `internal/cli/spec_review.go`에 `loopModeMinRevisions` 상수(5), 순수 함수 `loopAwareMaxRevisions(configured int, loopMode bool) int`, `resolveSpecReviewMaxRevisions(gate config.ReviewGateConf, loopMode bool) int`(gate.MaxRevisions 폴백 + loopAwareMaxRevisions)를 추가하고, `runSpecReviewWithOptions`의 maxRevisions 산출부를 `resolveSpecReviewMaxRevisions(gate, flags.LoopMode)`로 교체하여 `loopParams.maxRevisions`에 전달한다. (REQ-003)
- [ ] T4: `pkg/spec/parser.go`에 `ParseEARSWithWarnings(text string) ([]Requirement, []string, error)`를 추가하고 `ParseEARS`를 그 wrapper로 만든다. 경고 기준은 detectEARSType=="" 이면서 대문자 `SHALL` 토큰을 포함하는 비-불릿·비-주석 라인. `pkg/spec/validator.go`의 `ValidateSpec`가 `doc.RawContent`에 대해 경고를 산출하여 warning 레벨 `ValidationError`로 추가한다. (REQ-004)
- [ ] T5: `internal/cli/spec_review.go`에 `wrapSpecLoadError(specID string, err error) error`를 추가하고 line 71의 `SPEC 본문이 비어있습니다: %s (%w)` 메시지를 `SPEC 로드 실패` 계열 정확한 prefix + `%w`로 교체한다. line 75-77의 진짜 빈 본문 가드는 방어코드로 유지한다. (REQ-005)
- [ ] T6: oracle 테스트를 추가/확장한다 — `reviewer_checklist_test.go`(S1·S2 N/A 파싱), `[NEW] selfverify_na_test.go`(S3·S4 빈 reason 거부), `[NEW] spec_review_revisions_test.go`(S5·S6 loopAwareMaxRevisions, S11 resolveSpecReviewMaxRevisions, S9·S10 wrapSpecLoadError), `parser_test.go`(S7·S8 ParseEARSWithWarnings), `validator_test.go`(S12 ValidateSpec 경고). 각 변경 파일이 300줄 한도 내인지 확인한다. (REQ-006)
- [ ] T7: `pkg/spec/checklist_render.go`에 `naMissingReasonNote` 상수("reason missing")를 추가하고, `RenderChecklistSection`이 빈 reason이면서 `Status == ChecklistStatusNA`인 행만 이 마커로 렌더하도록 분기한다(그 외 빈칸은 `emptyNotePlaceholder` 유지). `[NEW] checklist_render_test.go` 또는 인접 테스트에 S13 oracle을 추가한다. (REQ-007)

## 구현 순서·의존성

T1~T5·T7은 서로 독립적이라 병렬 가능. T6은 T1~T5·T7 완료에 의존. 파일 크기 여유: 현재
`spec_review.go` 240줄(T3+T5 헬퍼 약 18줄 추가 후에도 한도 내), `parser.go` 69줄, `validator.go`
66줄, `selfverify.go` 84줄, `reviewer.go` 199줄, `checklist_render.go` 52줄. 모두 300줄 한도 내.
