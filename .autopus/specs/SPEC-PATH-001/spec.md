# SPEC-PATH-001: Module-Local SPEC Path Resolution

**Status**: completed (2026-04-21)
**Created**: 2026-03-26
**Domain**: PATH

**Sync Summary**:
- `pkg/spec/resolve.go`의 resolution contract와 관련 테스트가 실제로 추가됐다.
- generated router/skill surface도 `SPEC_PATH`, `TARGET_MODULE`, `WORKING_DIR` 기반으로 이동해 module-local path resolution이 canonical contract가 됐다.

## 목적

autopus-co 모노레포에서 사용자는 항상 최상단 디렉토리에서 Claude Code를 실행한다. 현재 `git rev-parse --show-toplevel` 기반의 SPEC 저장 로직은 항상 autopus-co/ 루트로 해석되어, 서브모듈(autopus-adk, Autopus, autopus-bridge 등)의 `.autopus/specs/`에 SPEC을 저장하거나 탐색할 수 없다.

Module-Local First 전략을 도입하여 SPEC이 대상 서브모듈의 `.autopus/specs/`에 저장되고, 공통 resolution 절차로 모든 서브커맨드에서 일관되게 탐색되도록 한다.

## 요구사항

### R1: SPEC Path Resolution (공통 절차)
WHEN any `/auto` subcommand receives a SPEC-ID, THE SYSTEM SHALL resolve the SPEC path by searching in order:
1. `.autopus/specs/{SPEC-ID}/spec.md` (최상단 — 레거시 및 크로스 모듈)
2. `*/.autopus/specs/{SPEC-ID}/spec.md` (서브모듈 depth 1)

THE SYSTEM SHALL extract from the resolved path:
- `SPEC_PATH`: 전체 상대 경로
- `SPEC_DIR`: SPEC 디렉토리 경로
- `TARGET_MODULE`: 서브모듈 경로 (최상단이면 ".")
- `WORKING_DIR`: TARGET_MODULE (executor가 cd할 경로)

### R2: Resolution 에러 처리
WHEN resolution 결과가 0건이면, THE SYSTEM SHALL "SPEC-{ID} not found" 에러를 출력해야 한다.
WHEN resolution 결과가 2건 이상이면, THE SYSTEM SHALL "Duplicate SPEC-{ID}" 에러를 출력하고 각 경로를 나열해야 한다.

### R3: SPEC 생성 시 대상 모듈 결정
WHEN `/auto plan` 명령으로 새 SPEC을 생성할 때, THE SYSTEM SHALL 다음 우선순위로 대상 모듈을 결정해야 한다:
1. `--target <module>` 플래그가 있으면 명시적 지정
2. 코드베이스 검색으로 가장 관련된 서브모듈 자동 감지
3. 감지 실패 또는 크로스 모듈이면 사용자에게 질문 (`--auto` 시 최상단에 저장)

### R4: auto-router의 go 서브커맨드 WORKING_DIR 주입
WHEN `/auto go {SPEC-ID}`를 실행할 때, THE SYSTEM SHALL resolve된 TARGET_MODULE을 executor에 WORKING_DIR로 전달하여 빌드/테스트가 해당 서브모듈 내에서 실행되도록 해야 한다.

### R5: auto-router의 sync 서브커맨드 모듈 인식
WHEN `/auto sync {SPEC-ID}`를 실행할 때, THE SYSTEM SHALL resolve된 TARGET_MODULE에서 git 작업을 수행해야 한다.

### R6: auto-router의 status 서브커맨드 전체 글로빙
WHEN `/auto status`를 실행할 때, THE SYSTEM SHALL 최상단 및 모든 서브모듈(depth 1)의 `.autopus/specs/`를 글로빙하여 모듈별 그룹화 대시보드를 표시해야 한다.

### R7: spec-writer의 저장 경로 변경
WHEN spec-writer 에이전트가 SPEC을 생성할 때, THE SYSTEM SHALL `git rev-parse --show-toplevel` 대신 명시적 target module 경로를 기준으로 `{target-module}/.autopus/specs/SPEC-{DOMAIN}-{NUMBER}/`에 저장해야 한다.

### R8: idea 스킬의 BS 파일 모듈 인식
WHEN idea 스킬이 BS 파일을 저장할 때, THE SYSTEM SHALL plan의 동일한 모듈 인식 로직을 적용하여 대상 서브모듈의 `.autopus/brainstorms/`에 저장해야 한다.

### R9: 레거시 호환성
WHILE 최상단 `.autopus/specs/`에 기존 SPEC이 존재하는 동안, THE SYSTEM SHALL resolution 순서 1번에서 이를 정상적으로 발견하고 처리해야 한다.

### R10: prd 스킬의 경로 참조 변경
WHEN prd 스킬이 기존 SPEC을 탐색하거나 새 PRD를 저장할 때, THE SYSTEM SHALL resolution 기반 경로를 사용해야 한다.

## 생성 파일 상세

### 수정 대상 파일

| 파일 | 역할 | 변경 내용 |
|------|------|-----------|
| `templates/claude/commands/auto-router.md.tmpl` | /auto 커맨드 라우터 | SPEC Path Resolution 공통 섹션 추가, go/plan/sync/status 경로 교체 |
| `content/agents/spec-writer.md` | SPEC 생성 에이전트 | git root 감지 로직 → target module 기반 저장 경로 |
| `content/agents/planner.md` | 기획 에이전트 | SPEC 경로 참조를 resolution 기반으로 변경 |
| `content/skills/prd.md` | PRD 작성 스킬 | 기존 SPEC 탐색/저장 경로를 resolution 기반으로 변경 |
| `content/skills/idea.md` | 아이디어 스킬 | BS 파일 저장을 동일 모듈 인식 로직으로 변경 |

### 신규 추가 없음

모든 변경은 기존 파일의 수정으로, 신규 파일 생성은 없다.
