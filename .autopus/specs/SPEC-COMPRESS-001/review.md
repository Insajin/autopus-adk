# Review: SPEC-COMPRESS-001

**Verdict**: REJECT
**Revision**: 0
**Date**: 2026-04-03 09:52:37

## Findings

| Provider | Severity | Description |
|----------|----------|-------------|
| claude | critical | REQ-001과 REQ-005는 논리적으로 모순됨 |
| claude | critical | 컨텍스트 윈도우 크기를 알 수 있는 메커니즘이 없음 |
| claude | major | 토큰 추정 전략이 정의되지 않음 |
| claude | major | "구조화된 요약"의 포맷 정의가 없음 |
| claude | major | "오래된 tool 결과" 정의가 모호함 |
| claude | major | REQ-006의 "5% 제한"에 대한 근거와 예외 처리 없음 |
| claude | minor | 압축 실패 시의 폴백 전략이 없음 |
| claude | minor | 기존 `budget` 패키지와의 관계 미정의 |
| claude | suggestion | phase 간 전이 시점을 명시적 트리거로 정의 |
| gemini | critical | REQ-001 and REQ-005 are logically contradictory. REQ-001 mandates replacing the previous phase output with a structured summary, whereas REQ-005 mandates passing the full phase output to the next phase without modification. The system cannot both summarize the output and pass it unmodified simultaneously. |
| gemini | major | REQ-002 and REQ-006 are architecturally infeasible given the current system design. The system wraps external CLI tools (e.g., `claude-code`, `opencode`) as subprocesses and communicates via IPC hooks (as seen in `hook-opencode-complete.ts`). `autopus-adk` does not have direct API access to the underlying model's context window size or token usage metrics, as these are completely encapsulated by the external CLI tools. |
| gemini | minor | The SPEC introduces the concept of a "token budget" (REQ-002), which conflicts with the established `budget` package in the codebase (`pkg/worker/budget/budget.go`). The existing system exclusively uses `IterationBudget` to track the number of tool calls, not tokens. Reusing the term "budget" for tokens will cause architectural confusion. |

## Provider Responses

### Response 1

이제 코드베이스를 충분히 파악했습니다. 리뷰 결과를 정리합니다.

---

## SPEC-COMPRESS-001 Review

### VERDICT: **REVISE**

---

### FINDING: [critical] REQ-001과 REQ-005는 논리적으로 모순됨

REQ-001은 "이전 phase 출력을 구조화된 요약으로 **교체**해야 한다"고 명시하고, REQ-005는 "전체 phase 출력을 다음 phase에 **수정 없이** 전달해야 한다"고 명시합니다. 두 요구사항은 동시에 충족할 수 없습니다. 현재 파이프라인 코드(`pipeline.go:110`)는 `prevOutput = pr.Output`으로 이전 phase 출력 전체를 다음 phase에 그대로 전달하고 있어 REQ-005가 현행 동작이지만, REQ-001의 압축은 이와 정면으로 충돌합니다.

**가능한 해석**: REQ-005는 "압축이 불필요할 때(컨텍스트 윈도우에 여유가 있을 때)" 적용되는 조건부 규칙일 수 있지만, 두 요구사항 모두 "Ubiquitous"로 표시되어 조건 분기가 없습니다. 압축 트리거 조건(토큰 사용량 임계값 등)을 명시하고, REQ-001과 REQ-005의 적용 조건을 구분해야 합니다.

---

### FINDING: [critical] 컨텍스트 윈도우 크기를 알 수 있는 메커니즘이 없음

REQ-002는 "프로바이더의 모델 컨텍스트 윈도우 크기에 기반하여 토큰 예산을 추정해야 한다"고 요구하지만, 현재 `ProviderAdapter` 인터페이스(`adapter/interface.go:10-18`)에는 컨텍스트 윈도우 크기를 반환하는 메서드가 없습니다. Claude, Gemini, Codex 어댑터 어디에도 모델 컨텍스트 윈도우 정보가 없습니다.

이 요구사항을 구현하려면 `ProviderAdapter` 인터페이스에 `ContextWindowSize() int` 같은 메서드를 추가하거나, 설정 파일에서 윈도우 크기를 읽어야 합니다. 인터페이스 변경은 기존 3개 어댑터(Claude, Gemini, Codex) 모두에 영향을 주므로 파급 범위를 SPEC에 명시해야 합니다.

---

### FINDING: [major] 토큰 추정 전략이 정의되지 않음

REQ-002가 "토큰 예산 추정"을 요구하지만, 현재 시스템은 `Output`을 plain string(`adapter/interface.go:43`)으로 다루며 토큰 카운팅 메커니즘이 전혀 없습니다. SPEC은 다음을 명시해야 합니다:
- 토큰 카운팅 방법 (문자 수 기반 근사치? tiktoken 라이브러리? 프로바이더 API?)
- 프로바이더별 토크나이저 차이 처리 방안
- 성능 오버헤드 허용 범위

---

### FINDING: [major] "구조화된 요약"의 포맷 정의가 없음

REQ-001과 REQ-004가 "structured summary"와 "progressive summarization"을 언급하지만, 요약의 구체적 스키마/포맷이 정의되지 않았습니다. 현재 `PhaseResult` 구조체(`pipeline.go:26-33`)에는 `Output string`만 있으며 파일 변경 목록, 결정 사항 등의 구조화된 필드가 없습니다. 요약에 포함해야 할 필드들(변경된 파일, 주요 결정, 에러 등)을 명시해야 합니다.

---

### FINDING: [major] "오래된 tool 결과" 정의가 모호함

REQ-003은 "오래된 tool 결과를 placeholder 요약으로 교체"해야 한다고 하는데, 파이프라인에서 tool 결과는 서브프로세스 내부에서 처리됩니다(`loop_exec.go:128-202`). 시스템이 서브프로세스에 전달하기 전의 **프롬프트 내** tool 결과를 의미하는지, 서브프로세스가 반환한 **출력 내** tool 결과를 의미하는지 불명확합니다. 현재 아키텍처에서 각 phase는 독립 서브프로세스(`pipeline.go:120-163`)이므로 이전 phase의 tool 결과에 직접 접근할 수 없고, `prevOutput` 문자열만 받습니다.

---

### FINDING: [major] REQ-006의 "5% 제한"에 대한 근거와 예외 처리 없음

최소 출력이 필요한 상황에서 5%가 너무 작을 수 있습니다. 예: Claude의 200K 윈도우 기준 5% = 10K 토큰은 합리적이지만, 작은 윈도우(8K)에서 5% = 400 토큰은 의미 있는 요약을 담기 부족합니다. 최소 토큰 하한선(floor)이 필요합니다.

---

### FINDING: [minor] 압축 실패 시의 폴백 전략이 없음

요약 생성이 실패하거나 원본보다 큰 요약이 생성된 경우의 에러 처리가 정의되지 않았습니다. 파이프라인이 중단되어야 하는지, 원본을 그대로 전달해야 하는지 명시가 필요합니다.

---

### FINDING: [minor] 기존 `budget` 패키지와의 관계 미정의

`pkg/worker/budget/` 패키지가 이미 존재하며 tool call 카운팅과 예산 관리를 담당합니다. 토큰 기반 예산 추정(REQ-002)이 기존 budget 패키지와 어떻게 통합되는지(확장? 별도 모듈?) 명시되어야 합니다.

---

### FINDING: [suggestion] phase 간 전이 시점을 명시적 트리거로 정의

현재 파이프라인은 순차 실행(`pipeline.go:80-114`)이므로 압축이 `for` 루프의 각 이터레이션 사이에서 발생하는 것이 자연스럽습니다. 하지만 SPEC에는 "phase transition" 시점이 구체적으로 어디인지(phase 완료 직후? 다음 phase 프롬프트 생성 시?), 그리고 `executorPrompt`, `testerPrompt` 등 프롬프트 래퍼 함수(`pipeline.go:220-233`)와의 통합 지점이 명시되어야 합니다.

---

### 종합 판단

SPEC의 핵심 목표(phase 전이 시 컨텍스트 압축)는 파이프라인 확장성에 필요한 기능이며 방향성은 타당합니다. 그러나 **REQ-001/REQ-005의 논리적 모순**이 해소되지 않으면 구현 방향을 결정할 수 없고, **컨텍스트 윈도우 크기 접근 메커니즘**이 없어 REQ-002/REQ-006은 현 아키텍처에서 구현 불가합니다. 요약 포맷 정의와 토큰 카운팅 전략도 보충이 필요합니다.

**REQ-005를 제거하거나 조건부로 변경하고, ProviderAdapter 인터페이스 확장 범위를 포함하여 재작성**을 권장합니다.

─── 🐙 applied: objective-reasoning · language-policy


### Response 2

MCP issues detected. Run /mcp list for status.1. VERDICT: REJECT

2. FINDING: [critical] REQ-001 and REQ-005 are logically contradictory. REQ-001 mandates replacing the previous phase output with a structured summary, whereas REQ-005 mandates passing the full phase output to the next phase without modification. The system cannot both summarize the output and pass it unmodified simultaneously.

3. FINDING: [major] REQ-002 and REQ-006 are architecturally infeasible given the current system design. The system wraps external CLI tools (e.g., `claude-code`, `opencode`) as subprocesses and communicates via IPC hooks (as seen in `hook-opencode-complete.ts`). `autopus-adk` does not have direct API access to the underlying model's context window size or token usage metrics, as these are completely encapsulated by the external CLI tools. 

4. FINDING: [minor] The SPEC introduces the concept of a "token budget" (REQ-002), which conflicts with the established `budget` package in the codebase (`pkg/worker/budget/budget.go`). The existing system exclusively uses `IterationBudget` to track the number of tool calls, not tokens. Reusing the term "budget" for tokens will cause architectural confusion.

**Reasoning:** 
The SPEC must be rejected due to a fundamental logical contradiction regarding how phase outputs are passed to subsequent phases. Additionally, the core premise of context compression based on model context windows is not viable in the current architecture, because the system orchestrates opaque third-party CLI agents and lacks the requisite introspection capabilities to monitor token counts or read model context limits.


### Response 3



