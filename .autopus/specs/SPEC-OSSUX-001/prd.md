# SPEC-OSSUX-001: Worker ADK 비사용 유저를 위한 오픈소스 UX 설계

- **SPEC-ID**: SPEC-OSSUX-001
- **Mode**: Standard
- **Date**: 2026-04-02
- **Status**: Draft

## 1. Problem & Context

ADK는 오픈소스 CLI 도구(`auto`)로, SPEC-Driven Development(plan/go/sync) 워크플로우를 제공한다. ADK Worker(SPEC-ADKW-001)는 Autopus 플랫폼과 연동하여 AI 에이전트 팀이 로컬에서 태스크를 실행하는 기능이다.

현재 문제:
- Worker 기능이 Core 개발 도구(plan/go/sync)와 구분 없이 노출되어 오픈소스 유저에게 인지 부하 증가
- 오픈소스 유저가 플랫폼을 발견할 자연스러운 동선 부재
- ADK는 오픈소스 → 플랫폼 전환 퍼널 역할을 하지만 전환 메커니즘이 없음
- `auto init`에서 Worker 관련 질문이 Developer-only 유저에게도 노출

비즈니스 임팩트: ADK 사용자 중 플랫폼으로 전환하는 비율이 측정 불가할 정도로 낮음. 자연스러운 발견 동선이 전환율을 개선할 수 있음.

## 2. Goals & Success Metrics

| Goal | Success Metric | Target |
|------|---------------|--------|
| 초기 설정 경험 최적화 | Init Wizard 완료율 (profile 선택 포함) | 95%+ |
| 플랫폼 인지도 향상 | 힌트 표시 후 opt-out하지 않는 비율 | 70%+ |
| 불필요한 인지 부하 감소 | Developer 프로파일 유저의 Worker 관련 질문 노출 횟수 | 0회 |

## 3. Target Users

| User Group | Role | Frequency | Key Expectation |
|-----------|------|-----------|-----------------|
| OSS Developer | ADK를 개발 도구로만 사용 | 매일 | 깔끔한 CLI, 불필요한 기능 미노출 |
| Potential Upgrader | ADK 사용 중 플랫폼에 관심 | 주 1-2회 | 자연스러운 발견, 강제 없음 |
| Fullstack User | ADK + Worker + 플랫폼 사용 | 매일 | 전체 기능 접근 |

## 4. User Stories (Job Stories)

**JS-01**: WHEN 나는 처음 auto init을 실행할 때 / I want to 내 사용 목적에 맞는 설정만 받고 싶다 / so I can 빠르게 개발을 시작할 수 있다

**JS-02**: WHEN 나는 auto go로 성공적으로 기능을 구현했을 때 / I want to AI 에이전트 팀이 이것을 자동화할 수 있다는 것을 자연스럽게 알고 싶다 / so I can 관심이 생기면 플랫폼을 시도해볼 수 있다

**JS-03**: WHEN 나는 플랫폼에 관심이 없을 때 / I want to 힌트를 영구적으로 끌 수 있다 / so I can 방해 없이 ADK만 사용할 수 있다

**JS-04**: WHEN 나는 auto help를 실행할 때 / I want to 내 프로파일에 맞는 명령어만 보고 싶다 / so I can 불필요한 정보에 혼란스럽지 않다

**JS-05**: WHEN 나는 자율화 수준을 설정하고 싶을 때 / I want to 명확한 레벨 옵션이 있으면 좋겠다 / so I can Worker 없이도 CI/자동화 수준을 제어할 수 있다

## 5. Functional Requirements (MoSCoW + EARS)

### P0 — Must Have

| ID | Requirement |
|----|-------------|
| FR-01 | WHEN `auto init` is executed, THE SYSTEM SHALL present a "usage profile" selection step with options: "Developer" (plan/go/sync only) and "Fullstack" (+ Worker/Platform) |
| FR-02 | WHEN the user selects "Developer" profile, THE SYSTEM SHALL skip Worker-related configuration steps |
| FR-03 | WHEN the user selects a profile, THE SYSTEM SHALL write `profile: developer` or `profile: fullstack` to autopus.yaml |
| FR-04 | WHEN `auto go` pipeline completes successfully AND `profile: developer` AND `hints.platform != false` AND first successful completion, THE SYSTEM SHALL display a one-line platform hint |
| FR-05 | WHEN `auto go` completes successfully 3+ times AND profile is developer AND hints not disabled, THE SYSTEM SHALL display a second (final) platform hint |
| FR-06 | WHEN `auto config set hints.platform false` is executed, THE SYSTEM SHALL permanently disable platform hints |
| FR-07 | WHEN autopus.yaml has no `profile` field, THE SYSTEM SHALL default to `developer` behavior |

### P1 — Should Have

| ID | Requirement |
|----|-------------|
| FR-08 | WHILE profile is "developer", THE SYSTEM SHALL dim or label Worker-related help entries as [optional] |
| FR-09 | WHEN displaying the platform hint, THE SYSTEM SHALL use a single non-intrusive line format |
| FR-10 | WHEN `auto init` runs with existing profile, THE SYSTEM SHALL pre-select the existing choice |

### P2 — Could Have

| ID | Requirement |
|----|-------------|
| FR-11 | WHEN profile is developer AND user runs a Worker subcommand, THE SYSTEM SHALL offer to switch to fullstack profile |
| FR-12 | WHEN profile changes to fullstack, THE SYSTEM SHALL trigger Worker setup wizard |
| FR-13 | WHILE profile is developer, THE SYSTEM SHALL prioritize community contribution CTA over platform CTA at milestones |
| FR-14 | THE SYSTEM SHALL support `autonomy.level: manual|assisted|autonomous` in autopus.yaml for CI/automation control |

## 6. Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR-01 | 힌트 상태(실행 카운터, 표시 여부)는 `~/.autopus/state.json` 로컬 상태 파일에 저장 |
| NFR-02 | 힌트 시스템의 CLI 응답 시간 영향 < 5ms |
| NFR-03 | 모든 신규 소스 파일은 300줄 이하 |
| NFR-04 | autopus.yaml 스키마 변경은 하위 호환성 유지 (새 필드 없으면 기본값 적용) |

## 7. Technical Constraints

- Go 1.26 바이너리 단일 배포 (빌드 태그 분리 제외)
- charmbracelet/huh 기반 Init Wizard 확장
- autopus.yaml 스키마: pkg/config/schema.go의 HarnessConfig 구조체 확장
- 힌트 카운터 저장: `~/.autopus/state.json` (프로젝트별 키)
- 기존 Init Wizard 코드(internal/cli/tui/wizard_steps.go) 통합

## 8. Out of Scope

- 텔레메트리/분석 시스템 구축
- Worker 기능 자체 변경
- Autopus 플랫폼 백엔드 변경
- Homebrew Core/Full 분리 배포
- 빌드 태그 기반 바이너리 분리
- A/B 테스트 프레임워크

## 9. Risks & Open Questions

| Risk | Severity | Mitigation |
|------|----------|------------|
| 오픈소스 커뮤니티 "상업 미끼" 반감 | High | 최대 2회 힌트, 영구 opt-out, 커뮤니티 CTA 우선 |
| autopus.yaml 하위 호환성 파괴 | Medium | 모든 새 필드에 기본값, 마이그레이션 불필요 |
| ~/.autopus/state.json 동시 접근 충돌 | Low | 프로젝트별 키로 격리, 파일 락 미적용 (최악: 힌트 1회 추가 표시) |

**Open Questions:**
- Q1: 힌트 메시지의 정확한 문구와 URL은? → TBD (구현 시 결정)
- Q2: Developer/Fullstack 외 추가 프로파일이 필요한가? → Won't Have (현재 릴리즈)

## 10. Pre-mortem

"이 기능이 6개월 후 실패한다면 이유는?"

| Scenario | Probability | Prevention |
|----------|------------|------------|
| 힌트가 귀찮다는 커뮤니티 피드백 폭주 | Medium | 최대 2회 제한 + 즉시 opt-out + 첫 릴리즈 전 피드백 수집 |
| Developer 프로파일이 기본이라 Fullstack 기능을 못 찾는 유저 | Medium | Help에서 [optional] 라벨로 존재 표시 + auto worker 실행 시 전환 제안 |
| profile 필드가 다른 설정과 충돌 | Low | 스키마 검증 + 기본값 처리 철저 |
| state.json이 다양한 OS에서 경로 문제 | Low | os.UserHomeDir() + 생성 실패 시 graceful degradation |
| autonomy.level 개념이 직관적이지 않아 미사용 | Medium | Phase 3로 분리하여 독립 검증 가능 |

## 11. Practitioner Q&A

| Question | Answer |
|----------|--------|
| Init Wizard에 새 스텝을 어떻게 추가하나? | wizard_steps.go의 huh.NewForm()에 Group 추가. SPEC-TUI-001 패턴 따름 |
| autopus.yaml에 새 필드 추가 절차는? | schema.go의 HarnessConfig에 필드 추가, YAML 태그 지정, 기본값 처리 |
| 힌트 표시 로직은 어디에 위치하나? | 새 패키지 internal/cli/hint/ 또는 pkg/hint/ 생성 |
| 프로파일별 Help 필터링은 어떻게 구현하나? | Cobra 커맨드의 Annotations 또는 별도 메타데이터로 profile 태깅 |

## Implementation Phases

이 PRD는 3 Phase로 나뉘어 구현됩니다:

| Phase | Ideas | SPEC |
|-------|-------|------|
| Phase 1 | #1 비침습 업셀 + #2 Init Wizard 분기 | SPEC-OSSUX-001 |
| Phase 2 | #3 브랜딩 최소화 + #4 Help 계층화 | SPEC-OSSUX-002 (별도) |
| Phase 3 | #5 autonomy.level | SPEC-OSSUX-003 (별도) |

Ref: BS-020
