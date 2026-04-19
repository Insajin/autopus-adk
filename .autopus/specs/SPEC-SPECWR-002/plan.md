# SPEC-SPECWR-002 구현 계획

## 태스크 목록

- [x] T1: `content/embed.go`는 이미 `rules/*.md` 평탄 패턴을 포함하므로 embed 디렉티브는 수정하지 않는다. 대신 `pkg/spec/checklist_test.go`에 `content.FS`에서 `rules/spec-quality.md`를 읽을 수 있음을 검증하는 회귀 테스트를 추가한다. (R02)
- [x] T2: `pkg/spec/checklist.go` [NEW] 작성. 함수 시그니처: `LoadChecklist(fsys fs.FS) (string, error)`, `InjectChecklistSection(sb *strings.Builder, body string, maxLines int)`. `content.FS`와 디스크 fallback 모두 지원. (R01, R03)
- [x] T3: `pkg/spec/prompt.go`의 `BuildReviewPrompt`에서 T2의 헬퍼 호출. 삽입 지점은 코드 컨텍스트 블록 직후, verify/discover 지시문 직전. 로드 실패 시 `fmt.Fprintf(os.Stderr, ...)` 후 스킵(R04).
- [x] T4: `pkg/spec/types.go`에 `ChecklistStatus`(enum: `"PASS"`, `"FAIL"`)와 `ChecklistOutcome` 구조체 추가. `ReviewResult`에 `ChecklistOutcomes []ChecklistOutcome` 필드 추가. (R07)
- [x] T5: `pkg/spec/reviewer.go`의 `ParseVerdict`에 `reChecklist` 정규식과 매칭 루프 추가. 결과를 `result.ChecklistOutcomes`에 append. (R06)
- [x] T6: `pkg/spec/checklist.go`에 `writeChecklistExamples` 함수 추가하고, 체크리스트 섹션이 주입된 경우에만 verify/discover 지시문에서 호출하도록 연결한다. (R05)
- [x] T7: `internal/cli/spec_review.go`의 `runSpecReview` 말미 출력 블록 확장. `finalResult.ChecklistOutcomes` 비어있지 않으면 요약 + FAIL 목록 출력. (R08)
- [x] T8: `pkg/spec/selfverify.go` [NEW] 작성. `SelfVerifyEntry` 구조체, `AppendSelfVerifyEntry` 함수, retention 로직(100 라인 초과 시 앞부분 제거). JSON Lines append-only. (R10)
- [x] T9: `internal/cli/spec_self_verify.go` [NEW] 작성. `newSpecSelfVerifyCmd()` cobra 커맨드, `--record`/`--dimension`/`--status`/`--reason` 플래그. `spec.go`에 등록. (R11)
- [x] T10: 저장소 루트 `.gitignore`에 `**/.autopus/specs/**/.self-verify.log` 추가. (R12)
- [x] T11: 테스트 - `pkg/spec/checklist_test.go` [NEW]. embed 로드 성공/실패, 200 라인 trim 동작, 삽입 헤더 포함 확인.
- [x] T12: 테스트 - `pkg/spec/selfverify_test.go` [NEW]. append 라인 증가, 100 라인 초과 시 앞부분 제거, JSON 파싱 가능성.
- [x] T13: 테스트 - `pkg/spec/checklist_test.go` [NEW]에 체크리스트 섹션 주입과 soft-fallback 경로를 검증하고, prompt regression을 런타임 헬퍼 기준으로 고정한다.
- [x] T14: 테스트 - `pkg/spec/reviewer_checklist_test.go` [NEW]에 `CHECKLIST:` 라인 파싱 결과 검증을 추가한다.
- [x] T15: 통합 확인 - `go test ./pkg/spec ./internal/cli ./content/...`로 체크리스트 주입, CLI 출력, ignore 규칙, self-verify 경로를 패키지 단위로 검증한다.

## 구현 전략

### 접근 방법

1. **점진적 머지**: 타입 확장(T4) → 로더(T2) → 프롬프트 주입(T3,T6) → 파싱(T5) → CLI 출력(T7) → 로깅(T8-T9) 순서. 각 단계가 독립 PR 단위로 분리 가능.
2. **기존 구조 재사용**: `trimToLines`, `injectAuxDocs`와 유사한 패턴으로 체크리스트 주입. 새로운 trim/loader 로직 중복 작성 금지.
3. **embed 우선, 디스크 fallback**: 배포 환경에서는 embed FS가 source of truth. 개발 중 `content/rules/spec-quality.md`를 수정할 때는 재빌드 없이 디스크에서 읽는 fallback이 DX를 개선.
4. **soft-fallback 채택**: 체크리스트는 "보강" 자산이지 판정 필수 입력이 아니다. SPEC-001 머지 전에도 SPEC-002 코드가 안전하게 merge되도록 soft-fallback 선택.

### 기존 코드 활용

- `pkg/spec/prompt.go:trimToLines` → 체크리스트 trim에 재사용.
- `pkg/spec/prompt.go:injectAuxDocs` → 헤더 삽입 패턴 모방.
- `pkg/spec/reviewer.go:structFindingRe` → 정규식 선언/사용 스타일 일관성.
- `internal/cli/spec.go` → 서브커맨드 등록 관례.

### 변경 범위 및 LOC 추정

| 영역 | 기존 파일 수정 | 신규 파일 | LOC 추정 |
|------|----------------|-----------|----------|
| embed 회귀 테스트 | 0 | 0 | 0 (회귀 테스트는 T11 포함) |
| 체크리스트 로더/주입 | 1 (prompt.go) | 1 (checklist.go) | ~80 |
| 타입 확장 | 1 (types.go) | 0 | ~20 |
| 응답 파싱 | 1 (reviewer.go) | 0 | ~25 |
| CLI 출력 | 1 (spec_review.go) | 0 | ~20 |
| self-verify 로깅 | 0 | 1 (selfverify.go) | ~60 |
| CLI 서브커맨드 | 1 (spec.go 등록) | 1 (spec_self_verify.go) | ~50 |
| 테스트 | 2 (prompt/reviewer_test 확장) | 2 (checklist/selfverify_test) | ~160 |
| gitignore | 1 | 0 | ~1 |
| **소계 (production)** | | | **~256** |
| **소계 (tests)** | | | **~160** |
| **총계** | | | **~416** |

300 LOC/파일 제한 준수 확인: 신규 파일은 각각 100 LOC 이하. 기존 파일 중 위험 구간은 `prompt.go`(현재 200) + 약 40 = 240. 경계 내 유지.

### 리스크 및 완화

- **embed 자동 포함 보장**: 기존 `rules/*.md` 평탄 패턴이 SPEC-001의 `content/rules/spec-quality.md`를 자동 포함한다. 디렉티브 변경이 필요 없으므로 embed 관련 회귀 위험 자체가 제거된다. 회귀 테스트(T11)로 자동 포함 사실을 명시적으로 잠근다.
- **Priority enum 결정**: research.md의 분석에 따라 현재 enum 유지. 확장 제안이 오면 별도 SPEC으로 분리.
- **프롬프트 길이 폭증**: 체크리스트 원문이 500+ 라인이면 trim으로도 과다. R14 P2 안전장치가 있으나 P0 범위에서는 기본 trim(200)에 의존. SPEC-001이 체크리스트 본문을 200 라인 이하로 설계할 것으로 가정.
- **soft-fallback의 은폐 위험**: 체크리스트 없이 리뷰가 통과하는 상황이 조용히 발생. 완화: `runSpecReview` 출력에 "체크리스트 결과: 0건 (file not loaded)" 같은 명시적 상태 라인 추가 고려(P2).

### 선행 조건

- SPEC-SPECWR-001 구현 완료(`content/rules/spec-quality.md` 존재). 단, R04 soft-fallback으로 인해 SPEC-002 머지 자체는 SPEC-001 완료를 강제하지 않는다.

### 검증 지점

- T3 완료 후 `go test ./pkg/spec/...` 전체 통과.
- T15에서 실제 `auto spec review SPEC-SPECWR-001` 실행 시 provider 응답에 `CHECKLIST:` 라인 최소 1건 포함.
