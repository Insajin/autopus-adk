# SPEC-REVFIX-001 리서치

## 근본 원인 분석 — 코드 증거 기반

메인 세션이 `pkg/spec/` 및 `internal/cli/spec_review.go`를 grep·직독하여 확인한 7개 결함. 각 항목에 정확한 파일·줄 번호 근거를 붙인다.

### 결함 #1: MergeVerdicts의 unanimous PASS 강제

**파일**: `pkg/spec/reviewer.go:254-267`

```go
func MergeVerdicts(results []ReviewResult) ReviewVerdict {
    verdict := VerdictPass
    for _, r := range results {
        switch r.Verdict {
        case VerdictReject:
            return VerdictReject
        case VerdictRevise:
            verdict = VerdictRevise  // ← 1명이라도 REVISE면 최종 REVISE
        }
    }
    return verdict
}
```

3 provider 환경에서 PASS를 받으려면 세 명 전원의 동의가 필요하다. 독립적으로 판단하는 LLM 3개가 **모두 일치할 확률**은 현실적으로 낮다. 이것이 실 운영에서 거의 모든 SPEC이 REVISE로 종료되는 1차 원인이다.

### 결함 #2: revision 루프가 spec.md를 재로드하지 않음

**파일**: `internal/cli/spec_review.go:51, 101-177`

```go
doc, err := spec.Load(specDir)  // line 51 — 루프 시작 전 1회
...
for revision := 0; revision <= maxRevisions; revision++ {  // line 101
    opts := buildPromptOpts(priorFindings, revision)
    prompt := spec.BuildReviewPrompt(doc, codeContext, opts)  // 같은 doc 재사용
    ...
}
```

사용자가 findings를 보고 spec.md를 수정해도 revision 1, 2, 3에서 동일한 낡은 문서가 사용된다. provider 입장에서는 "왜 내가 지적한 문제가 여전히 있지?"가 반복되어 `currCount >= prevCount` (line 223) 조건이 성립 → **circuit breaker 즉시 발동** → revision 1에서 루프 조기 종료.

### 결함 #3: plan/research/acceptance.md 본문 미주입

**파일**: `pkg/spec/prompt.go:21-34`

```go
if doc.RawContent != "" {
    sb.WriteString("### Full SPEC Document\n\n")
    sb.WriteString(doc.RawContent)    // spec.md만 포함
    ...
}
// plan.md, research.md, acceptance.md는 참조되지 않음
```

`BuildReviewPrompt` 파라미터에 `specDir`조차 없어 다른 파일을 읽을 경로 정보도 전달되지 않는다. **직접 증거**: SPEC-REVCONV-001의 review.md에 "acceptance.md의 Finding ID 형식이 spec.md와 모순" 같은 지적이 포함되어 있는데, 이 지적이 가능하려면 acceptance.md가 프롬프트에 들어가야 한다. 현재 코드로는 불가능 → provider가 **허구의 근거로 REVISE**를 내고 있음.

### 결함 #4: PASS 판정 기준 미명문화

**파일**: `pkg/spec/prompt.go:73-74, 92-93`

```go
sb.WriteString("Respond with:\n")
sb.WriteString("1. VERDICT: PASS, REVISE, or REJECT\n")
```

라벨만 있고 판정 기준이 없다. provider는 주관적으로 판단하므로 같은 SPEC을 두고 3명의 verdict가 쉽게 엇갈린다. 결함 #1과 결합되어 PASS 도달 확률을 더 낮춘다.

### 결함 #5: Few-shot 예시 부재 + empty RawContent guard 부재

**파일**: `pkg/spec/prompt.go` + `internal/cli/spec_review.go:51`

- structured FINDING 포맷(`FINDING: [severity] [category] [scope_ref] description`)을 요구하지만 **예시가 없다**. provider들은 legacy 포맷(`FINDING: [severity] description`)으로 응답하여 `parseDiscoverFindings`가 category/scopeRef 없는 finding을 생성 → dedup 품질 저하.
- `doc.RawContent == ""`여도 루프가 진행된다. **증거**: SPEC-HARN-PIPE-001의 review.md에서 3 provider 모두 "SPEC 본문이 없다"를 critical로 지적. spec.md 파싱이 실패했거나 빈 내용인데 guard 없이 루프가 돌아간 결과.

### 결함 #6: REVCONV-001 함수가 dead code

**파일**: `internal/cli/spec_review.go:130-138`

```go
for _, r := range reviews {
    merged.Findings = append(merged.Findings, r.Findings...)  // 단순 누적
    merged.Responses = append(merged.Responses, r.Responses...)
}
```

`DeduplicateFindings`, `MergeSupermajority`, `MergeFindingStatuses`는 구현되고 단위 테스트까지 있지만 **프로덕션 경로에서 호출되지 않는다**. 3 provider가 같은 버그를 지적하면 3배로 누적되어 findings 리스트가 팽창하고 circuit breaker `currCount >= prevCount` 비교가 왜곡된다. CHANGELOG.md:350-352는 "구현 완료"로 기록했지만 실제로는 **미통합** 상태다.

### 결함 #7: Provider 간 Finding ID 충돌

**파일**: `pkg/spec/reviewer.go:54-71`

```go
func parseDiscoverFindings(output, provider string, revision int) []ReviewFinding {
    var findings []ReviewFinding
    seq := 1  // ← provider별로 매번 1부터 시작
    for _, m := range structFindingRe.FindAllStringSubmatch(output, -1) {
        ...
        ID: fmt.Sprintf("F-%03d", seq),
        ...
    }
}
```

3 provider 모두 `F-001, F-002, F-003`을 발급 → merge 시 ID 충돌. `ApplyScopeLock`이 `knownIDs[f.ID]`(findings.go:90-93)로 "이미 알려진 finding"을 판단할 때, provider A의 `F-001`과 provider B의 `F-001`이 완전히 다른 이슈인데 동일시되어 **잘못된 scope lock**이 적용된다.

## SPEC-REVCONV-001과의 관계

REVCONV-001은 수렴 로직의 **코어 함수**를 설계·구현했다:

| 함수 | 파일 | 현재 상태 |
|------|------|----------|
| `DeduplicateFindings` | findings.go:42-77 | 구현됨, 테스트 존재, **프로덕션 미사용** |
| `MergeSupermajority` | findings.go:114-161 | 구현됨, 테스트 존재, **프로덕션 미사용** |
| `MergeFindingStatuses` | reviewer.go:159-208 | 구현됨, 테스트 존재, **프로덕션 미사용** |
| `ApplyScopeLock` | findings.go:79-112 | 구현됨, 테스트 존재, **프로덕션 사용 중** (spec_review.go:142) |
| `ShouldTripCircuitBreaker` | reviewer.go:214-224 | 구현됨, 프로덕션 사용 중 (spec_review.go:165) |

**유지**: 모든 공개 API 시그니처. 단위 테스트 100% regression free.
**통합**: `DeduplicateFindings` + `MergeSupermajority` + `MergeFindingStatuses`를 `runSpecReview` 경로에 배선.
**보강**: `MergeVerdicts`에 threshold 인자 추가(시그니처 변경), `BuildReviewPrompt`에 specDir 인자 추가(시그니처 변경).

시그니처 변경 2곳은 내부 호출자(spec_review.go, 테스트)만 수정하면 되며 외부 API 영향 없음.

## 설계 결정 트레이드오프

### D1: `MergeSupermajority` → `DeduplicateFindings` 순서

**옵션 A**: dedup 먼저 → supermajority
- 장점: 중복 제거 후 깔끔한 데이터에서 threshold 계산
- 단점: dedup이 (ScopeRef+Category+Description) 기준이므로 3 provider가 같은 이슈를 제기해도 1개로 줄어든다. 그러면 `len(group)/totalProviders` 계산의 분자가 항상 1이 되어 threshold가 무의미해짐. **기각**.

**옵션 B (채택)**: supermajority 먼저 → dedup
- `MergeSupermajority`는 내부에서 `(category, scopeRef)` 기준으로 그룹핑하여 group size를 세고 threshold 비교 후 대표 1개만 남긴다(findings.go:132-158).
- 이후 `DeduplicateFindings`가 global sequential ID를 재발급한다(findings.go:72-74).
- 장점: REVCONV-001 구현과 정확히 일치. 시그니처 변경 불필요.
- 단점: `MergeSupermajority`가 이미 일종의 dedup을 겸하므로 `DeduplicateFindings`는 사실상 ID 재발급 기능만 수행. 두 함수의 책임이 일부 겹침.
- **판정**: 채택. 책임 중복은 의미상 허용 범위. 재작성 리스크가 더 큼.

### D2: REQ-07 옵션 A vs B (Finding ID 발급 지점)

**옵션 A**: provider prefix ID 사용 (`F-claude-001`, `F-codex-001`)
- 장점: parse 시점에 충돌 없음, 디버깅에 provider 식별 가능
- 단점: merge 후 global 재발급이 필요하여 두 번 rename. `ApplyScopeLock`의 `knownIDs` 맵이 prefix ID와 global ID를 모두 고려해야 함. **기각**.

**옵션 B (채택)**: parseDiscoverFindings는 ID 미발급 (빈 문자열), `DeduplicateFindings`에서 global 재발급
- 장점: 기존 `DeduplicateFindings`의 line 72-74 로직을 그대로 활용. 변경 표면 최소.
- 단점: `parseVerifyFindings`의 FINDING_STATUS 참조(`id := fmt.Sprintf("F-%s", m[1])`, reviewer.go:114)는 prior findings의 ID를 기대하므로, 첫 discover 단계에서 빈 ID로 발급 후 dedup에서 배정된 global ID가 **persist되어 다음 verify 루프에서 안정적으로 참조**되어야 한다. 현재 persist 경로(spec_review.go:150)가 이를 보장함.
- **판정**: 채택. 사용자 프롬프트에서도 권장된 옵션.

### D3: verdict_threshold 기본값 0.67 vs 0.5

| 값 | 3-provider 시 | 장점 | 단점 |
|----|-------------|------|------|
| 0.5 | 2/3 동의면 PASS (과반) | PASS 도달성 최대 | 1 provider의 PASS로도 REJECT가 막아주지만, REVISE 방지력 약함 — false positive 증가 |
| **0.67 (채택)** | 2/3 동의 required (≈0.6667) | supermajority 표준. 5-provider에서는 3/5 ≈ 0.6 으로 자연스러운 스케일 | 3/3 중 2명만 PASS여도 충분 — 설계 의도 부합 |
| 0.75 | 3/4 이상 필요 | 품질 보수적 | 3-provider에서는 결국 unanimous와 동일 — 결함 #1 재현 |
| 1.0 | unanimous | 가장 엄격 | 현재 문제의 원인 |

판정: **0.67**. 3-provider supermajority의 공통 관례이며 5-provider로 확장 시에도 비례적으로 유효.

### D4: pass_criteria 기본 규칙 선택

기본 규칙은 `critical == 0 AND security == 0 AND major ≤ 2 → PASS`로 설정한다.
- `major ≤ 2` 허용은 완벽주의 방지. 작은 개선 사항만 남은 SPEC도 PASS 가능.
- `critical > 0 OR security > 0 → REJECT`는 security 게이트 유지.
- override config key(`pass_criteria`)로 조직별 튜닝 허용.

대안: `critical == 0 AND major == 0 → PASS` (엄격). 기각 — 실무 상 major 2개 이하는 후속 작업으로 처리 가능.

## 위험 요소

### R1: `MergeSupermajority` tolerance 상수

findings.go:155 `float64(count)/float64(totalProviders)+0.005 >= threshold`에 floating point tolerance 0.005가 하드코딩되어 있다. 2/3=0.6666... vs threshold=0.67 매칭을 위한 보정이지만, 0.75를 threshold로 쓰면 3/4=0.75가 `0.755 >= 0.75`로 성립하여 의도대로 동작한다. 0.67 기본값 환경에서는 문제 없음. **보류 (REVCONV-001 범위)**.

### R2: plan.md/research.md가 없는 신규 SPEC

신규 SPEC 생성 직후에는 spec.md만 존재하고 plan/research/acceptance.md가 없을 수 있다. REQ-03의 "파일 부재 시 섹션 생략" 조항이 이 경우를 처리한다. 추가로 판정 기준(REQ-04)에서 "문서 부재는 minor 이하"로 명시하여 provider가 부재 자체를 critical로 잡지 않도록 유도한다.

### R3: spec_review.go 300줄 경계

현재 282줄. T5의 변경으로 30-50줄 추가가 예상된다. 300줄 하드 제한을 넘길 가능성이 크므로 T5 착수 시점에 `runSpecReviewLoop` 함수 분리를 **선제적**으로 수행한다. 분리 대상: `for revision := 0; revision <= maxRevisions; revision++` 블록 전체(line 101-177).

### R4: 기존 `auto spec review` 사용자 호환

- `verdict_threshold` 없이 설정된 autopus.yaml은 기본값 0.67로 자동 상향. 기존 사용자에게는 **PASS 도달이 쉬워진 것으로 체감** — 긍정적 방향.
- `--multi` 없이 단일 provider 실행 시 totalProviders=1이므로 threshold 무관하게 1/1이 무조건 성립 — 동작 불변.
- 불이익 받는 케이스 없음. **Low risk**.

## 기존 코드 참조 인덱스 (빠른 내비게이션)

- `pkg/spec/reviewer.go`
  - Line 10-14: 정규식 정의
  - Line 16-47: `ParseVerdict`
  - Line 49-92: `parseDiscoverFindings` (REQ-07 변경 대상)
  - Line 94-157: `parseVerifyFindings`
  - Line 159-208: `MergeFindingStatuses`
  - Line 210-224: `ShouldTripCircuitBreaker`
  - Line 254-267: `MergeVerdicts` (REQ-01 변경 대상)
- `pkg/spec/findings.go`
  - Line 12-40: persist/load
  - Line 42-77: `DeduplicateFindings`
  - Line 79-112: `ApplyScopeLock`
  - Line 114-161: `MergeSupermajority`
  - Line 163-188: `NormalizeScopeRef`
- `pkg/spec/prompt.go`
  - Line 12-58: `BuildReviewPrompt` (REQ-03/04/05 변경 대상)
  - Line 60-78: `buildVerifyInstructions`
  - Line 80-98: `buildDiscoverInstructions`
- `internal/cli/spec_review.go`
  - Line 44-192: `runSpecReview` (REQ-02/05/06 변경 대상)
  - Line 101-177: revision 루프 본체 (분리 대상)
  - Line 194-221: helper 함수

## Confidence

- 결함 #1, #2, #6, #7: **High** — 코드 직접 증거
- 결함 #3, #4: **High** — 코드 + review.md 샘플 증거
- 결함 #5: **Medium-High** — 결함 자체는 코드 명백, SPEC-HARN-PIPE-001의 "SPEC 본문 없음" critical 지적이 인과 직접 증거
- D2 옵션 B 선택: **High** — 기존 `DeduplicateFindings` 동작과 정확히 정합
- D3 threshold 0.67: **High** — 업계 supermajority 관례

