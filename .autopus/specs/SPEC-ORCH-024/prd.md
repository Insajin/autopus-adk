# PRD: 멀티프로바이더 오케스트레이션 실행 계약 수렴

**SPEC**: SPEC-ORCH-024
**Status**: completed
**Created**: 2026-07-20
**Mode**: standard

## 1. Problem

Autopus-ADK에는 prompt 기반 agent pipeline, Go subprocess pipeline, worker pipeline,
orchestra debate/consensus, SPEC review, 플랫폼별 generated workflow가 함께 존재한다.
현재 일부 공개 명령은 실제 provider dispatch 없이 완료를 선언하고, 일부 strategy와
fallback 옵션은 실행 정책에 연결되지 않으며, provider quorum과 degraded 상태의 의미가
entrypoint마다 다르다. Codex와 Claude 지침도 같은 workflow를 서로 다른 의미로 설명한다.

## 2. Outcome Lock

하나의 versioned orchestration contract가 pipeline, orchestra, review, Codex 및 Claude
surface의 완료·실패·degraded·quorum·strategy 의미를 규정하고, 공개 entrypoint가 실제로
관측한 dispatch와 typed receipt 없이는 성공을 선언하지 못한다.

## 3. Users and Jobs

- Harness operator: 명령의 strategy, provider 수, gate 결과와 실제 실행이 일치함을 신뢰한다.
- Workflow author: 한 canonical semantic contract를 수정해 Codex와 Claude에 같은 의미를 배포한다.
- CI/release gate: 자유 형식 요약이 아니라 machine-readable receipt로 승격 가능 여부를 판정한다.
- Reviewer/security auditor: 소수의 Critical finding이 다수결로 사라지지 않음을 보장받는다.

## 4. Scope

- Pipeline backend authenticity, SPEC resolution, exact gate parsing, canonical checkpoint/dashboard.
- Verified frozen phase prompts, sanitized prior-output handoff, and strict resume identity/dependency closure.
- Orchestra requested/effective strategy, fallback transition, judge outcome, provider set integrity.
- Attempt-complete provider receipts, partial-result preservation, recovery-aware quorum, and judge model-family separation.
- Structured finding consensus, dissent preservation, Critical veto.
- Common run/worker/provider receipt and explicit terminal state.
- Canonical review/idea/team semantic contracts with Codex and Claude capability bindings.
- Generated-surface semantic parity and foreign-primitive prevention tests.
- Typed CLI receipt consumption, authoritative promotion receipts, and concrete generated argv/worker-prompt oracles.

## 5. Non-goals

- 새로운 AI provider 또는 terminal backend 도입.
- 모델 응답의 임베딩/LLM 기반 semantic clustering.
- 기존 deterministic build/test/security gate를 provider vote로 대체.
- root generated surface를 source of truth로 전환.
- 새 외부 Go dependency 추가.

## 6. Success Metrics

- 존재하지 않는 SPEC과 nil backend 실행은 provider 호출 0회 상태에서 명시적으로 실패한다.
- 성공 receipt의 dispatch count와 fake backend 관측 호출 수가 정확히 일치한다.
- 모든 실행은 정확히 하나의 terminal state를 가진다.
- 모든 관측 dispatch는 실제 role/attempt/backend/outcome을 가진 provider receipt 하나와 대응한다.
- requested/configured/resolved/attempted/usable/failed provider 집합이 receipt에 보존된다.
- Codex와 Claude semantic contract fingerprint가 일치하고 foreign primitive 발견 수가 0이다.
- Codex와 Claude가 현재 typed receipt에서 gate/promotion을 판정하며 prompt가 SPEC status를 직접 수정하지 않는다.
- 기존 focused package test, build, vet, strict SPEC validation이 통과한다.

## 7. Constraints

- 기존 helper/type 확장을 우선하고 public API는 additive compatibility를 유지한다.
- source code 파일은 300줄 미만을 유지한다.
- provider fan-out은 advisory이며 deterministic evidence가 authoritative하다.
- generated platform surface는 직접 수정하지 않고 autopus-adk source/template/adapter에서 생성한다.
- high/critical full review와 필수 문서 전문 전달 계약을 약화하지 않는다.

## 8. Risks

- Legacy caller가 fail-open 동작에 의존할 수 있다: compatibility projection과 명시적 dry-run으로 완화한다.
- Pane/subprocess 의미가 달라질 수 있다: 공통 finalizer와 backend transition receipt로 수렴한다.
- template 중복이 재발할 수 있다: semantic manifest/fingerprint gate를 required test로 둔다.
- consensus schema 미지원 응답: legacy text는 dissent-preserving literal fallback으로 처리한다.

## 9. Rollout

1. RED contract tests를 추가한다.
2. pipeline fail-closed와 receipt/state를 배선한다.
3. orchestra strategy/fallback/provider integrity를 배선한다.
4. Codex/Claude contract surface를 생성한다.
5. focused test, full build/vet, strict SPEC, generated drift를 검증한다.

## 10. Decision Receipt

- User authorization: 2026-07-20, 이전 감사 findings 전체 구현 및 Codex·Claude 적용 명시.
- Review evidence: 3개 독립 감사 lane의 runtime/guidance/supervisor findings.
- Approval basis: 사용자의 명시적 전체 진행 승인과 P0 재현 증거. 외부 provider vote로 대체하지 않음.
- Minimality: stdlib 및 기존 package helper만 사용하고 새 dependency를 추가하지 않는다.
