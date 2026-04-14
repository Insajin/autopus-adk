# SPEC-OPHARN-001 수락 기준

## 시나리오

### S1: OpenCode router surface 확장

- Given: `auto init --platform opencode` 또는 `auto update --platform opencode`가 실행되는 환경
- When: `.opencode/commands/auto.md`와 `.agents/skills/auto/SKILL.md`가 생성된다
- Then: 기존 7개 workflow 외에 `setup`, `status`, `map`, `why`, `verify`, `secure`, `test`, `dev`, `doctor` helper flow가 명시된다

### S2: Canonical parity contract 사용

- Given: OpenCode router 생성 로직이 수정된 상태
- When: router skill이 렌더링된다
- Then: shared triage, global flags, project-context bootstrap, SPEC path resolution 관련 섹션이 포함된다
- Then: `codex/prompts/auto.md.tmpl` 한 파일만을 유일 소스로 삼지 않는다

### S3: Plugin activation 자동 등록

- Given: fresh OpenCode harness generate 환경
- When: `opencode.json`이 생성된다
- Then: `plugin` 배열에 `.opencode/plugins/autopus-hooks.js`가 포함된다

### S4: Existing plugin merge 보존

- Given: 기존 `opencode.json`에 `plugin: ["existing-plugin"]`가 설정된 환경
- When: OpenCode harness update가 실행된다
- Then: `existing-plugin`은 유지되고 Autopus hook plugin만 추가된다

### S5: Validate/doctor mismatch 탐지

- Given: `.opencode/plugins/autopus-hooks.js` 파일은 존재하지만 `opencode.json`에는 plugin 등록이 없는 환경
- When: `Validate()` 또는 `auto doctor`를 실행한다
- Then: file 존재만으로 PASS 하지 않고 registration mismatch를 actionable finding으로 보고한다

### S6: Claude-only 잔재 제거

- Given: OpenCode commands/skills/agents가 생성된 환경
- When: 생성 파일을 검사한다
- Then: `Agent(`, `spawn_agent`, `TeamCreate`, `settings.json`, Claude permission field 같은 잔재가 포함되지 않는다

### S7: Unsupported feature 분류

- Given: Claude 전용 runtime 기능 또는 source가 없는 전략 스킬이 존재하는 환경
- When: parity report 또는 generated docs를 확인한다
- Then: 해당 항목은 `unsupported-by-platform` 또는 `unsupported-by-source`로 분리 보고된다
- Then: 단순 누락과 혼동되지 않는다

### S8: Strategy skill inventory 정리

- Given: `product-discovery`, `competitive-analysis`, `metrics`에 대한 source 또는 분류가 구현된 상태
- When: OpenCode extended skills를 생성한다
- Then: 3개 스킬이 생성되거나, 생성되지 않는 경우 명시적 분류 정보가 남는다

### S9: Parity gate 회귀 방지

- Given: adapter/parity 관련 테스트가 존재하는 상태
- When: OpenCode P0 helper flow 또는 plugin activation이 빠진 변경이 들어온다
- Then: 테스트가 실패하여 회귀가 merge되지 않는다

### S10: Native surface 보존

- Given: helper flow와 runtime wiring이 모두 복구된 상태
- When: 생성된 OpenCode 산출물을 읽는다
- Then: Claude `settings.json` 구조를 베끼지 않고 OpenCode-native 설명과 대체 경로를 사용한다
