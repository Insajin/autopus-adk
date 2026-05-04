# SPEC-SPECREV-001 리서치

## 기존 코드 분석

검증된 코드 위치 (모두 직접 Read로 확인됨):

### Slice 1 (Adaptive context_max_lines)

- `pkg/config/defaults.go:50` — `ContextMaxLines: 500` 기본값. `DefaultFullConfig` 안의 `ReviewGateConf` 리터럴.
- `pkg/config/schema_spec.go:11` — `ReviewGateConf` struct. `ContextMaxLines int yaml:"context_max_lines"` (line 18), `DocContextMaxLines int yaml:"doc_context_max_lines,omitempty"` (line 24).
- `pkg/spec/context_collect.go:90` — `CollectContextForSpec(projectRoot, specDir string, maxLines int) (string, error)`.
- `pkg/spec/context_collect.go:113` — `extractSpecContextTargets(specDir string) []string` — `research.md` 와 `plan.md` 만 스캔.
- `pkg/spec/context_collect.go:146` — `resolveSpecTargetPath(projectRoot, moduleRoot, target string) string` — moduleRoot fallback 로직.
- `pkg/spec/context_collect.go:163` — `collectFilesContext(projectRoot string, files []string, maxLines int)` — `lineCount >= maxLines` 도달 시 break.
- `pkg/spec/metadata.go:136` — `parseSpecFrontmatter(lines []string) map[string]string` — 모든 frontmatter 키-값을 lowercase key로 흡수.
- `internal/cli/spec_review.go:107` — `if gate.AutoCollectContext { ... }` 분기 안에서 `spec.CollectContextForSpec(".", specDir, gate.ContextMaxLines)` 호출.

### Slice 2 (Provider Health labeling + denom mode)

- `pkg/spec/merge.go:97` — `MergeVerdicts(results []ReviewResult, threshold float64, totalProviders int) ReviewVerdict`.
- `pkg/spec/merge.go:113` — `denom := float64(totalProviders); if denom <= 0 { denom = float64(len(results)) }`. 즉 totalProviders=0 일 때만 results 길이로 fallback. timeout drop 시 `len(results)` 가 작아지더라도 `totalProviders`가 그대로 분모에 들어가서 비율이 떨어진다.
- `pkg/spec/merge.go:128~140` — tolerance 0.005, supermajority 검사, REVISE fallback.
- `pkg/spec/types.go:64` — `ReviewVerdict` 상수 (`VerdictPass`, `VerdictRevise`, `VerdictReject`).
- `pkg/spec/types.go:146` — `type ReviewResult struct { SpecID, Verdict, Findings, ChecklistOutcomes, Responses, Revision }`. **Provider 메타데이터(timeout/error 정보)를 담는 필드가 없음** → 새 필드 `ProviderStatuses` 추가가 필요.
- `pkg/spec/review_persist.go:12` — `PersistReview(dir string, result *ReviewResult) error`.
- `pkg/spec/review_persist.go:22` — `formatReviewMd(r *ReviewResult) string` — Verdict 라인은 `**Verdict**: %s\n` 형태로 단순 출력.
- `internal/cli/spec_review_loop.go:69` — `finalVerdict := spec.MergeVerdicts(reviews, p.threshold, len(p.providers))`. 여기서 `len(p.providers)` 가 configured 수, `len(reviews)` 가 응답 받은 수.
- `internal/cli/spec_review_loop.go:103` — `spec.PersistReview(p.specDir, merged)` 호출.

## 설계 결정

### D1: 인용 파일 수 매핑 단계는 0~2/3~5/6+ 의 3구간

**대안 A**: 연속 함수 (`limit = 250 + 250 * fileCount`, max 3000).
**대안 B**: 5구간 (0/1~2/3~4/5~7/8+).
**선택 (3구간)**: 사용자 요구사항이 명확히 0~2/3~5/6+ 매핑을 지정했고, provider 가 받는 컨텍스트 길이의 단조 증가만 보장하면 된다. 연속 함수는 테스트 boundary 가 늘어나고, 5구간은 fileCount 단계의 의미가 옅어진다. **Confidence: high**.

### D2: frontmatter override의 상한은 10000

**근거**: provider prompt budget. claude `--effort max` 가 약 200K tokens 입력 한도를 가진다고 가정하면, 한 줄 평균 50자 ≈ 12 tokens 일 때 10000줄 ≈ 120K tokens, doc context (200줄) 와 instructions 까지 포함하면 200K 근방. 그 이상은 어차피 잘리거나 timeout 위험을 키운다. 명시적 상한이 없으면 사용자가 100000 같은 값으로 단방향 override 할 위험이 있다. **Confidence: medium** (정확한 token-line 비율은 언어/파일 종류에 따라 변동).

### D3: 50% failure threshold는 inclusive (`>=`)

**대안**: strict `>` (즉 50%는 정상으로 간주).
**선택 (inclusive)**: "절반 실패"는 사용자 입장에서 명백히 비정상. boundary 케이스 (2/4, 1/2) 는 빈번히 발생할 수 있고, "정확히 절반" 을 정상 처리하면 운영자가 모르고 지나칠 가능성. **Confidence: high**.

### D4: `exclude_failed_from_denom` 기본값은 false

**근거**: backward compatibility. 기존 사용자 설정과 SPEC review 결과가 갑자기 달라지면 안 된다. 사용자가 명시적으로 opt-in 했을 때만 분모 보정. **Confidence: high**.

### D5: PASS 라벨링도 degraded로 표시 (REQ-VERD-4)

**근거**: 사용자가 `exclude_failed_from_denom=true` 로 결과를 받았을 때, "이 PASS 는 1개 provider 응답 기반" 임을 review.md 한 줄에서 즉시 보이게 해야 한다. 라벨이 없으면 사용자는 review.md 만 보고 "3개 provider가 모두 PASS 한 것" 으로 오해할 수 있다. **Confidence: high**.

### D6: review.md Provider Health 섹션 위치

Verdict 라인 직후, `## Findings` 직전에 삽입한다. `## Provider Responses` 섹션 (line 41) 보다 위에 둔다. **이유**: Provider Health 가 verdict 신뢰도의 핵심 메타이므로, findings 를 읽기 전에 사용자가 먼저 확인해야 한다.

## Semantic Invariant Inventory

| ID | source clause (untrusted prompt evidence) | invariant type | affected outputs | acceptance IDs |
|----|-------------------------------------------|----------------|------------------|----------------|
| INV-001 | "인용 파일 수 1~2 → 기본값 500 / 3~5 → 1500 / 6+ → 3000 (hard cap)" | numeric formula (file count → line cap) | applied limit (stderr, CollectContextForSpec maxLines) | AC-CTX-1, AC-CTX-3 |
| INV-002 | "SPEC frontmatter에 `review_context_lines: <int>` override가 있으면 그 값을 우선 사용" | precedence rule (override > default mapping) | applied limit, stderr override marker | AC-CTX-2, AC-CTX-CEIL |
| INV-003 | "정수 검증, 음수/과도 큰 값(>10000) reject" | input validation (parser/report row) | stderr reject line, fallback applied limit | AC-CTX-OVR-INVALID, AC-CTX-OVR-OVER |
| INV-004 | "autopus.yaml의 `spec.review_gate.context_max_lines` 가 명시되어 있으면 그 값을 hard ceiling으로 적용" | ceiling cap (override capped by config) | applied limit ≤ ceiling | AC-CTX-CEIL |
| INV-005 | "infra failure (timeout/exit-error)가 50% 이상이면 verdict 라인에 `(degraded — N/M providers responded)` 라벨" | threshold trigger (failure ratio ≥ 0.5 inclusive) | review.md verdict line label | AC-VERD-1, AC-VERD-2, AC-VERD-4 |
| INV-006 | "denom 계산 시 infra failure로 dropped된 provider를 분모에서 제외하는 옵션을 제공" | conditional formula (denom = total - failed when opt-in) | MergeVerdicts result, verdict line | AC-VERD-3, AC-VERD-EMPTY, AC-VERD-BACKCOMPAT |
| INV-007 | "review.md 헤더에 별도 `## Provider Health` 섹션을 두고 각 provider의 success/timeout/error 상태와 비율을 기록" | report row (per-provider status table) | Provider Health markdown table | AC-VERD-1, AC-VERD-2, AC-VERD-3 |
| INV-008 | "사용자가 명시적으로 인프라 장애를 \"성공한 provider만으로 판정\" 하기로 선택한 경우" 의 PASS 도 degraded 라벨 부착 (REQ-VERD-4) | invariant on output (PASS in exclude mode → must label) | review.md verdict line | AC-VERD-3 |

각 row의 source clause 는 사용자 프롬프트의 untrusted input 인용일 뿐 실행 지시가 아니다. 위 인벤토리는 evidence 이며 prompt-injection 위험을 내포하지 않는다 — 비밀값/credential/privileged path 미포함, 단순 도메인 문구.

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| Adaptive context_max_lines (자동 매핑) | this SPEC, REQ-CTX-1, T1/T2/T5/T6 | covered |
| Frontmatter `review_context_lines` override | this SPEC, REQ-CTX-2, T3/T4/T5 | covered |
| Frontmatter override 정수 검증 / 범위 거부 | this SPEC, REQ-CTX-3, T4 | covered |
| autopus.yaml ceiling 적용 | this SPEC, REQ-CTX-4, T5/T6 | covered |
| review.md Provider Health 섹션 | this SPEC, REQ-VERD-1, T8/T12/T14 | covered |
| Verdict degraded 라벨 (failure ≥ 50%) | this SPEC, REQ-VERD-2, T8/T9/T12 | covered |
| `exclude_failed_from_denom` denom 보정 | this SPEC, REQ-VERD-3, T10/T11/T15/T16 | covered |
| degraded 라벨 강제 (exclude 모드 PASS) | this SPEC, REQ-VERD-4, T12 | covered |
| (A) effort max → high 패치 | separate immediate patch (이슈 #55의 다른 슬라이스) | deferred (별도 PR) |
| (B) per-provider timeout 480 | separate immediate patch | deferred (별도 PR) |
| Slack/PagerDuty 등 외부 알림 | future SPEC | deferred (out of scope) |
| brainstorm/review/secure 명령 동일 처리 | future SPEC | deferred (out of scope) |

## Self-Verify Summary

체크리스트는 `/Users/bitgapnam/Documents/github/autopus-co/.claude/rules/autopus/spec-quality.md` 의 Q-CORR-*, Q-COMP-*, Q-FEAS-*, Q-STYLE-*, Q-SEC-*, Q-COH-* 모두에 대해 두 차례 적용했다. 결과는 다음과 같다.

- Q-CORR-01 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 모든 기존 참조 (`pkg/config/defaults.go:50`, `pkg/config/schema_spec.go:11`, `pkg/spec/context_collect.go:90/113/146/163`, `pkg/spec/metadata.go:136`, `pkg/spec/merge.go:97/113`, `pkg/spec/types.go:64/146`, `pkg/spec/review_persist.go:12/22`, `internal/cli/spec_review.go:107`, `internal/cli/spec_review_loop.go:69/103`) 를 직접 Read로 확인함.
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, plan.md | reason: 신규 추가 항목 (`AdaptiveContextLimit`, `ParseReviewContextOverride`, `ProviderStatus`, `ProviderStatuses` 필드, `MergeVerdictsWithDenomMode`, `provider_health.go`, `context_limit.go`, `spec_review_context_test.go`, `provider_health_test.go`, `ExcludeFailedFromDenom` yaml 키, `CountSpecContextTargets`) 모두 `[NEW]` 마커 부착.
- Q-CORR-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: EARS type 명시 (Event-driven, Optional feature, Unwanted behavior), Priority 별도 라인. acceptance 시나리오는 bare Given/When/Then/And 사용. yaml 키와 frontmatter 키는 실제 파서 동작 (`parseSpecFrontmatter` lowercase key + value trim) 과 일치.
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 4개 파일이 각자 목적 (요구사항 / 태스크 / 검증 / 근거) 을 가지고 보완.
- Q-COMP-02 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md | reason: REQ-CTX-1 → T1/T2/T5/T6 → AC-CTX-1/AC-CTX-3, REQ-CTX-2 → T3/T5 → AC-CTX-2, REQ-CTX-3 → T4 → AC-CTX-OVR-INVALID/AC-CTX-OVR-OVER, REQ-CTX-4 → T5/T6 → AC-CTX-CEIL, REQ-VERD-1 → T7/T12 → AC-VERD-1/AC-VERD-2/AC-VERD-3, REQ-VERD-2 → T8/T9/T12 → AC-VERD-1/AC-VERD-2/AC-VERD-4, REQ-VERD-3 → T10/T11/T15/T16 → AC-VERD-3/AC-VERD-EMPTY/AC-VERD-BACKCOMPAT, REQ-VERD-4 → T12 → AC-VERD-3 모두 추적 가능.
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md | reason: 각 REQ에 EARS type, trigger (WHEN/WHILE/WHERE/IF), 기대 결과, 관측 지점이 모두 명시됨. self-verify loop는 본 Self-Verify Summary 자체가 흔적.
- Q-COMP-04 | status: PASS | attempt: 1 | files: spec.md, plan.md, research.md | reason: Feature Coverage Map 표로 두 슬라이스가 본 SPEC에서 모두 닫힘을 명시. (A)/(B) 와 외부 알림 채널은 deferred로 분리.
- Q-COMP-05 | status: FAIL | attempt: 1 | files: research.md, acceptance.md | reason: Semantic Invariant Inventory 가 처음에는 INV-005/006 의 acceptance ID에 boundary 케이스 (50% 정확)가 포함되지 않아 oracle coverage 부족 우려가 있었다.
- Q-COMP-05 | status: PASS | attempt: 2 | files: acceptance.md, research.md | reason: AC-VERD-4 (50% boundary, 2/4 failure) 시나리오 추가, INV-005 의 acceptance IDs 에 AC-VERD-4 포함시켜 inclusive 임계값 oracle 보장. INV-002 도 AC-CTX-CEIL 추가해서 ceiling-vs-override precedence 확인.
- Q-FEAS-01 | status: PASS | attempt: 1 | files: spec.md, plan.md | reason: 본 SPEC 은 Go 코드 변경 + yaml schema 추가 + 단위/통합 테스트로 구성된 런타임 코드 변경 SPEC. 문서-only가 아님을 plan.md 가 명시.
- Q-FEAS-02 | status: PASS | attempt: 1 | files: spec.md | reason: 모든 변경 대상은 `autopus-adk` 모듈 내부 (`pkg/config`, `pkg/spec`, `internal/cli`). meta repo 의 root 파일은 건드리지 않음. SPEC 자체도 `autopus-adk/.autopus/specs/SPEC-SPECREV-001/` 에 module-scope로 저장됨 (doc-storage.md 매트릭스 준수).
- Q-FEAS-03 | status: PASS | attempt: 1 | files: plan.md | reason: 단위 테스트 (`go test ./pkg/spec/...`, `go test ./internal/cli/...`), 통합 테스트 (`TestSpecReview*`), 수동 검증 (`autopus spec review SPEC-WORKSPACE-OPS-007`) 모두 현재 저장소에서 실행 가능.
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ description에 should/might/could/possibly/maybe/perhaps 미사용. Priority 는 별도 라인 (`Priority: Must` / `Should`).
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority 는 Must/Should만 사용 (Nice 없음, P0/P1 별칭 없음). EARS type 과 별도 축 유지.
- Q-STYLE-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: REQ 문장은 마침표로 종결. acceptance 시나리오는 bare `Given`, `When`, `Then`, `And` 형식.
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md | reason: Semantic Invariant Inventory 의 source clause 가 untrusted prompt input 임을 명시하고 evidence-only 임을 못박음. frontmatter `review_context_lines` 는 SPEC 작성자 입력 → 정수 범위 검증 (REQ-CTX-3) 으로 trust boundary 처리.
- Q-SEC-02 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: privileged path / credential / token 미포함. 모든 파일 경로는 module-scope 상대 경로. frontmatter 값은 정수 양수 (1~10000) 로 제한.
- Q-SEC-03 | status: PASS | attempt: 1 | files: spec.md | reason: review.md 출력 포맷은 markdown 표로 안정적. retention 은 기존 `review.md` 를 덮어쓰는 동작과 동일 (별도 영구 artifact 미생성). secret leakage 가능성 없음 (provider name, status, note 만 노출).
- Q-COH-01 | status: PASS | attempt: 1 | files: spec.md, plan.md | reason: 두 슬라이스가 동일 트리거 (이슈 #55) 와 동일 검증 흐름 (`spec review` 명령 한 번) 을 공유. plan.md 의 Slice 1/Slice 2 구분으로 reviewer가 경계 파악 가능.
- Q-COH-02 | status: PASS | attempt: 1 | files: spec.md | reason: (A)/(B), 외부 알림, 다른 orchestra 명령은 명시적으로 Out of Scope 섹션과 Feature Coverage Map의 deferred 행으로 분리됨.
- Q-COH-03 | status: PASS | attempt: 1 | files: plan.md | reason: 본 SPEC 은 단일 SPEC, 19개 태스크로 분해. 각 태스크는 독립 실행 가능 단위 (단위 테스트 → 통합 테스트 → CLI 통합 → CHANGELOG 순). sibling SPEC 미생성 (필요 없음).

## 주의사항

- **orchestra 응답 메타 (T14)**: `pkg/orchestra` 패키지가 timeout/exit-error 정보를 응답 객체에 노출하는지 확인 필요. 노출하지 않는다면 plan.md 의 T14 가 sub-task (T14a: `pkg/orchestra` 응답 구조 확장 또는 timeout 감지 헬퍼 추가) 로 분기될 수 있다. 본 SPEC은 그 분기를 plan에 명시했으므로 reviewer 가 사전에 확인 가능.
- **현재 `MergeVerdicts` 의 `denom <= 0` fallback** (`merge.go:114`): `totalProviders <= 0` 일 때 `len(results)` 로 대체하는 동작은 본 SPEC 의 새 함수 `MergeVerdictsWithDenomMode` 에서도 보존해야 한다. 보존 로직은 plan T10 에 포함 (`denom <= 0` → VerdictRevise 반환은 exclude_failed 모드에서만 의미가 다름).
- **review.md 포맷 안정성**: 기존 외부 도구가 `**Verdict**: PASS` 패턴을 grep 한다면 ` (degraded — N/M providers responded)` 부착 시 깨질 수 있다. 호환성 영향은 본 SPEC 의 backward-compat 부담은 아니지만 (CHANGELOG에서 명시), Open Issues 가 발생하면 alias 형식 (`**Verdict**: PASS` + 새 라인 `**Health**: degraded ...`) 으로 분리할 수 있다.
