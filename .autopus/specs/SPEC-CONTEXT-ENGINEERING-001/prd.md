# PRD: 네 플랫폼 generated context contract 정렬

## 1. Problem

Autopus-ADK에는 command profile, body-free hash receipt, verified full-document delivery, native skill progressive disclosure가 이미 있다. 그러나 generated Claude, Codex, OpenCode, Gemini/Antigravity surface에서 다음 계약이 한 번에 검증되지 않는다.

- `go`의 required, delegated-worker optional recall, default excluded 문서 집합
- project-relative safe reference를 사용하는 JIT guidance
- 기존 five-field worker receipt에 추가되는 bounded condensed return 의미
- doctor context-weight 용어와 공개 심볼/JSON check ID의 호환성
- scenario rotation 시 executable wire 보존

이는 새 runtime selector가 필요한 문제가 아니라 기존 생성 source와 deterministic oracle의 drift 문제다.

## 2. Goal

네 adapter의 scratch generation 결과가 동일한 generated-surface 계약을 제공하게 한다.

- Claude thin router는 `auto-router.md.tmpl`에서 생성되어 정확히 하나의 detail을 선택하고, `claude_workflow_skills.go`가 route detail을 만든다.
- `go` supervisor delivery는 core, resolved SPEC/plan/acceptance, 현재 존재하는 architecture 문서를 전문으로 검증해 receipt 밖에 전달한다.
- delegated worker optional recall은 signature, learning, task-declared extra refs만 포함하고 required body를 중복하지 않는다.
- JIT guidance는 clean project-relative regular-file refs만 허용하고 sanitization/redaction과 injection evidence 보존을 요구한다.
- worker return은 기존 정확한 five-field receipt를 유지하면서 2,000 estimated tokens 이하의 condensed evidence를 추가한다.

## 3. Non-Goals

- runtime `ContextPlan` 또는 JIT selector의 신설·변경
- provider-native conversation history, compaction, tool catalog 제어
- `autopus.context_delivery.v1`의 complete required-document integrity 축소
- default `skills.shared_surface=full` 변경
- `templates/claude/commands/auto-workflows.md.tmpl` 상단 generation-only preamble을 effective installed selector로 취급하거나 설치
- root generated `.claude/`, `.codex/`, `.gemini/`, `.opencode/` 직접 수정
- scenario parser/runner empty-field 호환성 변경

## 4. Success Metrics

| Metric | Target |
|---|---|
| four-adapter scratch generated command/document matrix | 4/4 exact match |
| Claude router `go` detail reference count | exactly 1 |
| installed `auto-workflows.md` artifact | 0 |
| five-field receipt owner parity | exact set, 0 drift |
| unsafe JIT reference token coverage | absolute, traversal, symlink, non-regular, sanitization/redaction, injection evidence |
| `ContextLoadSet` and doctor check IDs | unchanged |
| authoritative Must acceptance | S1-S8 and Edge Case 1-3 PASS |
| focused adapter/template/doctor tests | PASS |

## 5. Constraints

- source of truth는 nested `autopus-adk/`이다.
- 기존 dirty WIP와 root generated/runtime surface를 보존한다.
- 새 dependency, serialized schema, runtime abstraction을 추가하지 않는다.
- generation-only contract는 runtime enforcement로 과장하지 않는다.
