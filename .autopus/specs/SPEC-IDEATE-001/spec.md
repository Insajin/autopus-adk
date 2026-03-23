# SPEC-IDEATE-001: 멀티 프로바이더 브레인스토밍 단계 추가

**Status**: completed
**Created**: 2026-03-23
**Domain**: IDEATE

## 목적

현재 `/auto plan --multi` 파이프라인은 SPEC 리뷰 게이트(Step 4)에서만 멀티 프로바이더를 활용한다. 아이디어 기획 단계에서는 단일 프로바이더만 사용하므로 claude, codex, gemini 등 여러 AI의 다양한 관점을 활용한 발산적 사고가 불가능하다. 이 기능은 PRD 생성 전에 멀티 프로바이더 브레인스토밍 단계(Step 1.3)를 삽입하여, 발산된 아이디어가 PRD의 입력으로 활용되도록 한다.

## 요구사항

### P0 — 핵심 요구사항

- R1: WHEN `--multi` 플래그가 활성화된 상태에서 plan 파이프라인이 실행되면, THE SYSTEM SHALL Step 1.5.1 (SPEC-ID 생성) 직후, Step 1.5.2 (PRD 템플릿 선택) 이전에 Step 1.5.1.5 (Multi-Provider Brainstorm)을 실행한다. 이는 SPEC-ID가 이미 생성된 시점이므로 brainstorm.md 저장이 가능하다.
- R2: WHEN Step 1.5.1.5가 실행되면, THE SYSTEM SHALL 모든 설정된 프로바이더(claude, codex, gemini)에게 독립적으로(Phase 1: 병렬 발산) SCAMPER/HMW 기반 브레인스토밍 프롬프트를 전송하고 각 프로바이더의 아이디어를 수집한다.
- R3: WHEN 각 프로바이더의 아이디어가 수집되면(Phase 2: 교차 검증), THE SYSTEM SHALL debate 전략의 rebuttal 단계를 통해 아이디어를 교차 보강한다. 이때 judge 프롬프트는 "아이디어 필터링이 아닌 보강 및 통합"을 목적으로 커스터마이즈한다 (divergence-preserving judge prompt).
- R4: WHEN 브레인스토밍 결과가 생성되면, THE SYSTEM SHALL 결과를 `.autopus/specs/SPEC-{ID}/brainstorm.md`에 저장한다.
- R5: WHEN Step 1.5.2 (PRD Generation)이 실행되면, THE SYSTEM SHALL brainstorm.md 파일을 읽어 PRD 생성의 입력 컨텍스트로 활용한다. 구체적으로, spec-writer 에이전트 프롬프트에 `Brainstorm file: .autopus/specs/SPEC-{ID}/brainstorm.md` 경로를 포함한다.

### P1 — 확장 요구사항

- R6: WHEN autopus.yaml에 `brainstorm` 커맨드 엔트리가 존재하면, THE SYSTEM SHALL 해당 설정(strategy, providers)을 사용하여 브레인스토밍을 실행한다.
- R7: WHEN `--multi` 플래그가 비활성화된 상태이면, THE SYSTEM SHALL Step 1.5.1.5를 건너뛰고 기존 파이프라인을 유지한다.
- R8: WHEN 브레인스토밍 결과가 병합되면, THE SYSTEM SHALL debate 전략의 judge가 ICE(Impact, Confidence, Ease) 스코어링으로 상위 아이디어를 선별하여 출력한다. judge가 최종 통합 및 스코어링의 주체이다.

### P2 — 비기능 요구사항

- R9: WHILE 브레인스토밍이 실행되는 동안, THE SYSTEM SHALL 각 프로바이더의 타임아웃을 orchestra.timeout_seconds 설정에 따라 per-provider per-round 기준으로 제한한다. 즉, 각 프로바이더는 라운드당 timeout_seconds 이내에 응답해야 한다.
- R10: WHILE 프로바이더가 실패하면, THE SYSTEM SHALL graceful degradation으로 나머지 프로바이더의 결과만으로 진행한다.

## 생성 파일 상세

| 파일 | 역할 |
|------|------|
| `autopus.yaml` (수정) | `orchestra.commands.brainstorm` 엔트리 추가 |
| `pkg/config/defaults.go` (수정) | DefaultFullConfig에 brainstorm CommandEntry 추가 |
| `internal/cli/orchestra_brainstorm.go` (신규) | `newOrchestraBrainstormCmd()` 서브커맨드 + `buildBrainstormPrompt()` (orchestra.go 274줄이므로 분리 필수) |
| `.claude/commands/auto.md` (수정) | plan 파이프라인에 Step 1.5.1.5 삽입 |
| `.claude/skills/autopus/brainstorming.md` (수정) | 멀티 프로바이더 브레인스토밍 워크플로우 섹션 추가 |
| `.autopus/specs/SPEC-{ID}/brainstorm.md` (신규, 런타임) | 브레인스토밍 결과 저장 파일 |
| `internal/cli/orchestra_brainstorm_test.go` (신규) | 브레인스토밍 서브커맨드 및 프롬프트 빌더 테스트 |
| `pkg/config/defaults_test.go` (수정) | brainstorm 커맨드 엔트리 존재 확인 테스트 |

## Out of Scope

- 브레인스토밍 전용 새로운 오케스트레이션 전략 구현 (기존 debate 전략을 divergence-preserving judge prompt로 커스터마이즈)
- 브레인스토밍 결과의 UI/시각화
- `--multi` 없이 단독 브레인스토밍 실행 (`/auto brainstorm` 독립 커맨드)
- 프로바이더별 커스텀 프롬프트 템플릿
- ICE 스코어링 기준치의 autopus.yaml 커스텀 설정
