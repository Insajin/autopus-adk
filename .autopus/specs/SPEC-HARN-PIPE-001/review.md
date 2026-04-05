# Review: SPEC-HARN-PIPE-001

**Verdict**: REJECT
**Revision**: 0
**Date**: 2026-04-05 09:13:34

## Findings

| Provider | Severity | Description |
|----------|----------|-------------|
| claude | critical | ** SPEC 문서 본문이 제공되지 않았습니다. 제목("Go 바이너리 기반 플랫폼-무관 파이프라인 오케스트레이터")과 기존 코드 컨텍스트만 있으며, 실제 SPEC의 요구사항(REQ-*), 수용 기준(AC-*), 마일스톤, 아키텍처 설계, 제약조건 등 핵심 내용이 전혀 없습니다. |
| claude | critical | ** 리포지토리 내에 `SPEC-HARN-PIPE-001` 파일이 존재하지 않습니다. SPEC이 아직 작성되지 않았거나 다른 위치에 있을 수 있습니다. |
| codex | major | 스펙 본문이 현재 요청과 저장소 어디에도 없어 correctness/completeness를 검증할 수 없습니다. 지금 제공된 것은 제목과 구현 컨텍스트뿐이라 요구사항, 비기능 요구, 실패 처리, 수용 기준을 대조할 대상이 없습니다. `.autopus/specs`에도 `SPEC-HARN-PIPE-001` 문서는 없습니다. |
| codex | major | 현재 구현은 “하나의 공통 파이프라인 계약”이 아니라 provider별로 서로 다른 훅 프로토콜입니다. Codex/Claude는 `last_assistant_message`, Gemini는 `prompt_response`, Opencode는 `text`를 읽고, round IPC도 Opencode에는 없습니다. “플랫폼-무관 오케스트레이터” 스펙이라면 이 차이를 흡수하는 정규화 계약을 명시해야 합니다. [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L33) [hook-claude-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-claude-stop.sh#L33) [hook-gemini-afteragent.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-gemini-afteragent.sh#L33) [hook-opencode-complete.ts](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-opencode-complete.ts#L17) |
| codex | major | 현재 round 기반 양방향 IPC는 shell 훅 쪽 계약에 박혀 있습니다. `*-done`, `*-ready`, `*-input.json`, `*-abort`, 그리고 `cmux wait-for`까지 포함되므로, 새 Go 바이너리가 이것을 완전히 대체할지 아니면 호환 shim을 둘지 스펙에 반드시 있어야 합니다. 이 경계가 없으면 기존 orchestration과 바로 충돌합니다. [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L52) [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L61) [hook-claude-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-claude-stop.sh#L51) [hook-gemini-afteragent.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-gemini-afteragent.sh#L51) |
| codex | major | “플랫폼-무관” 주장과 현재의 파일시스템 계약이 맞지 않습니다. 구현은 `/tmp/autopus/<session>` 경로, POSIX 파일 모드 `0600`, 빈 파일 done signal에 의존합니다. 스펙이 OS-중립 세션 스토어와 권한 모델을 정의하지 않으면 Windows/비-POSIX 환경에서 feasibility가 없습니다. [hook-opencode-complete.ts](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-opencode-complete.ts#L12) [hook-opencode-complete.ts](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-opencode-complete.ts#L40) [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L18) [hook-claude-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-claude-stop.sh#L18) [hook-gemini-afteragent.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-gemini-afteragent.sh#L18) |
| codex | major | 통합 경계가 불명확합니다. 서버에는 이미 `DevelopmentHarnessService`가 `CodingRelayService` 위에서 2-phase orchestration을 수행하고, Codex는 별도의 JSON-RPC app-server provider를 가집니다. 새 Go 바이너리가 “hook 대체”인지, “bridge 대체”인지, “server-side orchestrator 대체”인지 명확하지 않으면 중복 오케스트레이션이 됩니다. [development_harness_service.go](/Users/bitgapnam/Documents/github/autopus-co/Autopus/backend/internal/services/development_harness_service.go#L191) [coding_relay_service.go](/Users/bitgapnam/Documents/github/autopus-co/Autopus/backend/internal/services/coding_relay_service.go#L3) [module.go](/Users/bitgapnam/Documents/github/autopus-co/Autopus/backend/cmd/server/modules/agent/module.go#L845) [codex_app_server.go](/Users/bitgapnam/Documents/github/autopus-co/Autopus/backend/internal/ai/codex_app_server.go#L23) |
| codex | minor | 비호환 방지 요구사항이 스펙에 명시돼야 합니다. 현재 훅은 session ID 검증, 세션 디렉터리 미존재 시 no-op, 결과 파일 권한 제한, abort/timeout 정리 같은 안전 동작을 수행합니다. Go 전환 시 이 동작이 빠지면 보안/운영 회귀가 생깁니다. [hook-opencode-complete.ts](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-opencode-complete.ts#L9) [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L13) [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L77) |
| gemini | critical | Missing SPEC Content - The provided document only contains the title "SPEC-HARN-PIPE-001 — Go 바이너리 기반 플랫폼-무관 파이프라인 오케스트레이터" and the "Existing Code Context". It completely lacks the actual specification details, such as objectives, architecture, requirements, implementation plan, and acceptance criteria. |

## Provider Responses

### Response 1

SPEC-HARN-PIPE-001 문서가 리포지토리에 존재하지 않습니다.

---

## 리뷰 결과

**VERDICT: REJECT**

### Findings

**FINDING: [critical]** SPEC 문서 본문이 제공되지 않았습니다. 제목("Go 바이너리 기반 플랫폼-무관 파이프라인 오케스트레이터")과 기존 코드 컨텍스트만 있으며, 실제 SPEC의 요구사항(REQ-*), 수용 기준(AC-*), 마일스톤, 아키텍처 설계, 제약조건 등 핵심 내용이 전혀 없습니다.

**FINDING: [critical]** 리포지토리 내에 `SPEC-HARN-PIPE-001` 파일이 존재하지 않습니다. SPEC이 아직 작성되지 않았거나 다른 위치에 있을 수 있습니다.

### 근거

리뷰 대상이 되는 SPEC 문서의 본문 — 목적, 배경, 요구사항, 수용 기준, 구현 계획, 의존성, 리스크 분석 — 이 없으면 정확성, 완전성, 실현 가능성을 평가할 수 없습니다. 제공된 코드 컨텍스트(hook 스크립트, auxiliary bootstrap, main.go)는 기존 시스템의 참고 자료일 뿐 SPEC 자체가 아닙니다.

### 다음 단계

1. SPEC 문서 본문을 제공해 주시거나
2. `/auto plan` 등으로 SPEC을 먼저 작성한 후 리뷰를 요청해 주세요


### Response 2

FINDING: [major] 스펙 본문이 현재 요청과 저장소 어디에도 없어 correctness/completeness를 검증할 수 없습니다. 지금 제공된 것은 제목과 구현 컨텍스트뿐이라 요구사항, 비기능 요구, 실패 처리, 수용 기준을 대조할 대상이 없습니다. `.autopus/specs`에도 `SPEC-HARN-PIPE-001` 문서는 없습니다.

FINDING: [major] 현재 구현은 “하나의 공통 파이프라인 계약”이 아니라 provider별로 서로 다른 훅 프로토콜입니다. Codex/Claude는 `last_assistant_message`, Gemini는 `prompt_response`, Opencode는 `text`를 읽고, round IPC도 Opencode에는 없습니다. “플랫폼-무관 오케스트레이터” 스펙이라면 이 차이를 흡수하는 정규화 계약을 명시해야 합니다. [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L33) [hook-claude-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-claude-stop.sh#L33) [hook-gemini-afteragent.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-gemini-afteragent.sh#L33) [hook-opencode-complete.ts](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-opencode-complete.ts#L17)

FINDING: [major] 현재 round 기반 양방향 IPC는 shell 훅 쪽 계약에 박혀 있습니다. `*-done`, `*-ready`, `*-input.json`, `*-abort`, 그리고 `cmux wait-for`까지 포함되므로, 새 Go 바이너리가 이것을 완전히 대체할지 아니면 호환 shim을 둘지 스펙에 반드시 있어야 합니다. 이 경계가 없으면 기존 orchestration과 바로 충돌합니다. [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L52) [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L61) [hook-claude-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-claude-stop.sh#L51) [hook-gemini-afteragent.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-gemini-afteragent.sh#L51)

FINDING: [major] “플랫폼-무관” 주장과 현재의 파일시스템 계약이 맞지 않습니다. 구현은 `/tmp/autopus/<session>` 경로, POSIX 파일 모드 `0600`, 빈 파일 done signal에 의존합니다. 스펙이 OS-중립 세션 스토어와 권한 모델을 정의하지 않으면 Windows/비-POSIX 환경에서 feasibility가 없습니다. [hook-opencode-complete.ts](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-opencode-complete.ts#L12) [hook-opencode-complete.ts](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-opencode-complete.ts#L40) [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L18) [hook-claude-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-claude-stop.sh#L18) [hook-gemini-afteragent.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-gemini-afteragent.sh#L18)

FINDING: [major] 통합 경계가 불명확합니다. 서버에는 이미 `DevelopmentHarnessService`가 `CodingRelayService` 위에서 2-phase orchestration을 수행하고, Codex는 별도의 JSON-RPC app-server provider를 가집니다. 새 Go 바이너리가 “hook 대체”인지, “bridge 대체”인지, “server-side orchestrator 대체”인지 명확하지 않으면 중복 오케스트레이션이 됩니다. [development_harness_service.go](/Users/bitgapnam/Documents/github/autopus-co/Autopus/backend/internal/services/development_harness_service.go#L191) [coding_relay_service.go](/Users/bitgapnam/Documents/github/autopus-co/Autopus/backend/internal/services/coding_relay_service.go#L3) [module.go](/Users/bitgapnam/Documents/github/autopus-co/Autopus/backend/cmd/server/modules/agent/module.go#L845) [codex_app_server.go](/Users/bitgapnam/Documents/github/autopus-co/Autopus/backend/internal/ai/codex_app_server.go#L23)

FINDING: [minor] 비호환 방지 요구사항이 스펙에 명시돼야 합니다. 현재 훅은 session ID 검증, 세션 디렉터리 미존재 시 no-op, 결과 파일 권한 제한, abort/timeout 정리 같은 안전 동작을 수행합니다. Go 전환 시 이 동작이 빠지면 보안/운영 회귀가 생깁니다. [hook-opencode-complete.ts](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-opencode-complete.ts#L9) [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L13) [hook-codex-stop.sh](/Users/bitgapnam/Documents/github/autopus-co/.claude/hooks/autopus/hook-codex-stop.sh#L77)

VERDICT: REVISE

이유는 간단합니다. 개념 자체는 feasible합니다. shell/python 기반 훅과 파일 IPC를 Go 바이너리로 수렴하는 방향은 충분히 구현 가능합니다. 하지만 현재 제공된 스펙 정보만으로는 대체 대상, 호환 대상, provider 정규화 규약, OS abstraction, cutover 전략이 빠져 있어 correctness와 completeness를 만족했다고 볼 수 없습니다. 본문을 보강해 위 호환성·경계·비기능 요구를 명시하면 재검토 가능한 수준입니다.


### Response 3

MCP issues detected. Run /mcp list for status.1. VERDICT: REJECT

2. FINDING: [critical] Missing SPEC Content - The provided document only contains the title "SPEC-HARN-PIPE-001 — Go 바이너리 기반 플랫폼-무관 파이프라인 오케스트레이터" and the "Existing Code Context". It completely lacks the actual specification details, such as objectives, architecture, requirements, implementation plan, and acceptance criteria.

3. REASONING: It is impossible to review the specification for correctness, completeness, and feasibility because the specification content itself is absent from the prompt. A proper evaluation requires the actual design and requirements for the "Go binary-based platform-agnostic pipeline orchestrator." Please provide the full SPEC document text for review.


### Response 4



