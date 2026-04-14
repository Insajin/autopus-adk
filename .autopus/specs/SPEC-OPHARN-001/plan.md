# SPEC-OPHARN-001 구현 계획

## 목표

OpenCode 하네스의 체감 결손을 파일 수가 아니라 실제 UX 표면 기준으로 복구한다. 가장 먼저 훅 활성화 bug를 막고, 그 다음 router/helper flow 표면과 parity gate를 보강한다.

## 에이전트 배정 테이블

| Task | Agent | Parallel Group | Mode | File Ownership | Est. Lines |
|------|-------|----------------|------|----------------|------------|
| T1 | planner | G1 | sequential | parity contract, helper flow inventory | ~80 |
| T2 | executor | G2 | parallel | `pkg/adapter/opencode/opencode_specs.go`, `opencode_skills.go`, shared router asset | ~180 |
| T3 | executor | G2 | parallel | `pkg/adapter/opencode/opencode_commands.go` | ~80 |
| T4 | executor | G3 | parallel | `pkg/adapter/opencode/opencode_config.go`, `opencode_lifecycle.go` | ~120 |
| T5 | executor | G3 | parallel | `pkg/adapter/opencode/opencode_test.go` | ~140 |
| T6 | executor | G4 | parallel | `content/skills/*.md`, `pkg/content/skill_transformer*.go` | ~160 |
| T7 | reviewer | G5 | sequential | parity classification and regression review | ~0 |

## 태스크 목록

### Group G1: Canonical Surface 정의

- [x] T1: OpenCode parity contract 정리
  - Claude monolithic router에서 OpenCode에 필요한 P0 helper flow를 추출한다.
  - `unsupported-by-platform` vs `unsupported-by-source` 분류 기준을 정의한다.
  - 결과물을 `workflowSpecs` 또는 별도 shared contract file이 소비할 수 있는 구조로 정리한다.

### Group G2: Router / Command Surface 확장

- [x] T2: OpenCode router skill generation 리팩터링
  - `renderRouterSkill()`이 `codex/prompts/auto.md.tmpl`에만 의존하지 않도록 변경한다.
  - shared triage, global flags, helper flow 목록, degradation notes를 포함하는 canonical source를 도입한다.
  - `workflowSpecs`를 확장하여 helper flow inventory를 모델링한다.

- [x] T3: OpenCode command entrypoint 확장
  - `.opencode/commands/`에 helper flow 진입점 생성 로직을 추가한다.
  - helper flow가 thin entrypoint일지라도 실제 skill/CLI/대체 경로를 가리키도록 만든다.

### Group G3: Runtime Wiring / Diagnostics 복구

- [x] T4: plugin registration bug fix + diagnostics 보강
  - `prepareConfigMapping()`이 standard generate/update 경로에서 Autopus plugin path를 자동 등록하도록 수정한다.
  - `Validate()`가 plugin file existence와 plugin registration mismatch를 구분해 보고하도록 확장한다.
  - 필요하면 `doctor` 경로에서 재사용 가능한 validation helper를 추가한다.

- [x] T5: OpenCode adapter 회귀 테스트 추가
  - fresh generate 후 `opencode.json.plugin` 배열에 Autopus hook plugin이 존재하는지 검증한다.
  - router/command surface에 helper flow가 포함되는지 검증한다.
  - generated OpenCode files에 금지 토큰(`Agent(`, `TeamCreate`, `settings.json`)이 남지 않는지 검증한다.

### Group G4: Skill Inventory / Classification 정리

- [x] T6: missing strategic skills와 compatibility classification 정리
  - `content/skills/`에 없는 `product-discovery`, `competitive-analysis`, `metrics`를 canonical source로 추가할지 결정한다.
  - source 추가가 범위 과대이면 OpenCode parity report/manifest에서 `unsupported-by-source`로 분류한다.
  - `ReplacePlatformReferences()`와 skill transform report를 활용해 unsupported classification을 표준화한다.

### Group G5: Review Gate

- [x] T7: parity review
  - P0 helper flow, plugin activation, unsupported classification이 모두 구현되었는지 검토한다.
  - Claude semantics가 OpenCode output에 새어 들어가지 않았는지 확인한다.

## 구현 전략

1. **P0 bug first**: plugin file 생성만 하고 등록하지 않는 문제를 먼저 고친다. 이 이슈는 즉시 동작 결손으로 이어진다.
2. **Canonical source next**: router 확장은 문자열 치환 추가가 아니라 shared contract를 도입하는 방식으로 진행한다.
3. **Classification over imitation**: OpenCode가 지원하지 않는 기능은 억지 구현 대신 명시적 분류와 대체 경로로 처리한다.
4. **Behavioral parity tests**: 파일 존재 여부가 아니라 활성화/표면/잔재 토큰 부재를 테스트 기준으로 삼는다.

## 변경 범위

- **수정 파일**: `pkg/adapter/opencode/*` 중심
- **후보 신규 파일**: shared router contract asset, compatibility manifest helper, missing canonical skills
- **영향 범위**: OpenCode adapter + shared content layer. Claude/Codex/Gemini adapter 회귀가 없어야 한다.

## 검증 계획

- `go test ./pkg/adapter/opencode ./pkg/content`
- 필요 시 parity 관련 adapter tests 추가 실행
- fresh generate fixture에서 `opencode.json` plugin 등록 확인
- generated command/skill snapshot 또는 string assertion으로 helper flow 존재 확인

## 위험 및 완화

- helper flow를 monolithic router에 계속 하드코딩하면 유지보수 비용이 커진다
  - 완화: shared contract 추출
- plugin merge 로직이 기존 user plugin을 덮어쓸 수 있다
  - 완화: existing plugin 배열 보존 테스트
- missing skill 3종을 성급히 추가하면 source canon이 더 분산될 수 있다
  - 완화: source 추가 vs unsupported-by-source 분류를 먼저 결정
