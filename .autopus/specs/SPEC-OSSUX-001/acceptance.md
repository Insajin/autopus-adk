# SPEC-OSSUX-001 Acceptance Criteria

**SPEC**: SPEC-OSSUX-001
**Status**: draft
**Created**: 2026-04-02

## AC-01: Init Wizard 프로파일 선택 표시 (R1)

**Given** 사용자가 TTY 터미널에서 `auto init`을 실행할 때
**When** Init Wizard가 시작되면
**Then** 첫 번째 스텝으로 "Usage Profile" 선택이 표시되어야 한다
**And** "Developer — plan/go/sync 개발 도구"와 "Fullstack — + Worker/Platform 연동" 옵션이 제공되어야 한다

## AC-02: Developer 프로파일 시 Worker 스텝 스킵 (R2)

**Given** 사용자가 Init Wizard에서 "Developer" 프로파일을 선택했을 때
**When** 나머지 Wizard 스텝이 진행되면
**Then** Worker 관련 설정 스텝이 표시되지 않아야 한다

## AC-03: 프로파일 값 autopus.yaml 저장 (R3)

**Given** 사용자가 Init Wizard에서 프로파일을 선택하고 완료했을 때
**When** autopus.yaml이 생성/업데이트되면
**Then** `usage_profile: developer` 또는 `usage_profile: fullstack`이 기록되어야 한다

## AC-04: 기본 프로파일 backward compatibility (R4)

**Given** autopus.yaml에 `usage_profile` 필드가 없을 때
**When** ADK가 설정을 로드하면
**Then** `developer` 프로파일 동작이 적용되어야 한다
**And** 에러가 발생하지 않아야 한다

## AC-05: 첫 번째 성공 시 힌트 표시 (R5)

**Given** `usage_profile`이 `developer`이고 `hints.platform`이 비활성화되지 않았을 때
**When** `auto go` 파이프라인이 처음으로 성공적으로 완료되면
**Then** 파이프라인 완료 출력 후 한 줄 플랫폼 힌트가 표시되어야 한다

## AC-06: 첫 번째 힌트 미조건 시 미표시 (R5 negative)

**Given** `usage_profile`이 `fullstack`일 때
**When** `auto go` 파이프라인이 성공적으로 완료되면
**Then** 플랫폼 힌트가 표시되지 않아야 한다

## AC-07: 힌트 비활성화 시 미표시 (R5 negative)

**Given** `hints.platform`이 `false`로 설정되어 있을 때
**When** `auto go` 파이프라인이 성공적으로 완료되면
**Then** 플랫폼 힌트가 표시되지 않아야 한다

## AC-08: 3회 성공 시 두 번째 힌트 표시 (R6)

**Given** `usage_profile`이 `developer`이고 `hints.platform`이 비활성화되지 않았을 때
**When** `auto go` 파이프라인이 3번째로 성공적으로 완료되면
**Then** 두 번째(최종) 플랫폼 힌트가 표시되어야 한다

## AC-09: 두 번째 힌트 후 추가 힌트 없음 (R6 boundary)

**Given** 첫 번째와 두 번째 힌트가 모두 표시된 이후
**When** `auto go` 파이프라인이 추가로 성공적으로 완료되면
**Then** 더 이상 힌트가 표시되지 않아야 한다

## AC-10: config set으로 힌트 비활성화 (R7)

**Given** 사용자가 터미널에서 실행할 때
**When** `auto config set hints.platform false`를 실행하면
**Then** autopus.yaml에 `hints.platform: false`가 기록되어야 한다
**And** 이후 `auto go` 성공 시 힌트가 표시되지 않아야 한다

## AC-11: 힌트 포맷 검증 (R8)

**Given** 힌트 표시 조건이 충족되었을 때
**When** 힌트가 출력되면
**Then** 단일 줄 포맷으로 출력되어야 한다
**And** 줄바꿈 없이 `💡` 이모지로 시작해야 한다

## AC-12: 기존 프로파일 pre-select (R9)

**Given** autopus.yaml에 `usage_profile: fullstack`이 이미 존재할 때
**When** `auto init`이 실행되면
**Then** Usage Profile 스텝에서 "Fullstack"이 미리 선택되어 있어야 한다

## AC-13: --yes 모드 기본 프로파일 (R10)

**Given** 사용자가 `auto init --yes`를 실행할 때
**When** Init Wizard가 non-interactive 모드로 완료되면
**Then** `usage_profile`이 `developer`로 설정되어야 한다

## AC-14: 상태 파일 프로젝트 격리 (NFR-1)

**Given** 두 개의 서로 다른 프로젝트 디렉토리에서 ADK를 사용할 때
**When** 프로젝트 A에서 `auto go`를 3번 성공적으로 완료하면
**Then** 프로젝트 B의 힌트 상태는 영향받지 않아야 한다
**And** `~/.autopus/state.json`에 프로젝트별 키로 저장되어야 한다

## AC-15: 상태 파일 없음 시 graceful degradation (NFR-1)

**Given** `~/.autopus/state.json`이 존재하지 않거나 읽을 수 없을 때
**When** `auto go`가 성공적으로 완료되면
**Then** 에러 없이 정상 동작해야 한다
**And** 힌트 표시를 건너뛰어야 한다

## AC-16: 힌트 성능 (NFR-2)

**Given** `~/.autopus/state.json`이 존재할 때
**When** 힌트 조건 평가가 수행되면
**Then** 5ms 이내에 완료되어야 한다

## AC-17: 스키마 하위 호환성 (NFR-4)

**Given** `usage_profile`과 `hints` 필드가 없는 기존 autopus.yaml이 있을 때
**When** ADK v(new)가 해당 파일을 로드하면
**Then** 정상적으로 로드되어야 한다
**And** `usage_profile`은 `developer`로, `hints.platform`은 `true`(enabled)로 기본 동작해야 한다
