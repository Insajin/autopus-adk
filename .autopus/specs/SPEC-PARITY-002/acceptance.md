# SPEC-PARITY-002 수락 기준

## Test Scenarios

모든 Must 시나리오는 oracle-first다. 각 시나리오는 concrete expected output(정확한 항목 수·정확한
문자열·정확한 집합)을 Then에 명시하며, 파일 존재·heading·exit code·non-empty output 같은 structural
신호만으로 닫지 않는다. 검증 헬퍼는 임시 디렉터리에 어댑터 `Generate`를 실행하여 생성물을 직접
읽는다(설치된 표면 비의존).

### S1: Gemini가 누락 규칙 3종을 실제 내용과 함께 생성 (Must, REQ-001 / INV-001, INV-002)
Given 전체 구성 `config.DefaultFullConfig("parity-test")`로 Gemini 어댑터가 준비되었다.
When 임시 디렉터리에 `gemini.Generate`를 실행한다.
Then `.gemini/rules/autopus/deferred-tools.md`, `.gemini/rules/autopus/project-identity.md`, `.gemini/rules/autopus/spec-quality.md`가 모두 존재한다.
And `deferred-tools.md` 본문은 예상 문자열 `# Deferred Tools Loading`와 `Antigravity CLI`를 모두 포함한다.
And `project-identity.md` 본문은 예상 문자열 `# Project Identity`를 포함한다.
And `spec-quality.md` 본문은 예상 문자열 `SPEC Quality Checklist`를 포함한다.

### S2: Gemini 규칙 집합이 content 소스 집합과 정확히 일치 (Must, REQ-001, REQ-002 / INV-001)
Given `content/rules/`에 14개의 `.md` 파일이 있다.
When Gemini 어댑터가 규칙을 생성한다.
Then 생성된 `.gemini/rules/autopus/*.md` basename 집합은 예상 집합과 정확히 같다: `branding.md`, `context7-docs.md`, `deferred-tools.md`, `doc-storage.md`, `file-size-limit.md`, `language-policy.md`, `lore-commit.md`, `objective-reasoning.md`, `project-identity.md`, `shell-portability.md`, `spec-quality.md`, `subagent-delegation.md`, `techstack-freshness.md`, `worktree-safety.md` (정확히 14종).
And Gemini의 규칙 exclusion 집합은 공집합이다.

### S3: 패리티 게이트가 누락을 양방향으로 감지 (Must, REQ-002 / INV-003)
Given 소스 집합에는 존재하지만 플랫폼 P의 생성 집합과 P의 exclusion 집합 어디에도 없는 합성 항목 이름 `__parity_probe__`가 주어졌다.
When 패리티 커버리지 게이트가 P의 커버리지를 평가한다.
Then 게이트는 예상 출력으로 `__parity_probe__`와 플랫폼 `P`를 함께 담은 FAIL finding을 정확히 1건 반환한다.
And `__parity_probe__`를 P의 exclusion 집합에 추가한 뒤 다시 평가하면 게이트는 그 항목에 대해 finding을 0건 반환한다.

### S4: Gemini extended skill에 Claude 전용 경로 참조가 남지 않음 (Must, REQ-004 / INV-004)
Given `content/skills/agent-pipeline.md`는 `.claude/skills/autopus/worktree-isolation.md` 정규 참조를 포함한다.
When Gemini 어댑터가 extended skill을 생성한다.
Then 생성된 `agent-pipeline` 스킬 본문은 예상대로 부분 문자열 `.claude/skills/autopus/`를 포함하지 않는다.
And 해당 참조는 Gemini 네이티브 경로 `.gemini/skills/autopus/worktree-isolation/SKILL.md`로 해소된다.

### S5: platform frontmatter 값이 어댑터 식별자와 일치 (Should, REQ-003 / INV-005)
Given Gemini와 Codex가 규칙을 생성했다.
When 각 생성 규칙의 frontmatter `platform:` 필드를 검사한다.
Then `platform:` 필드를 가진 모든 Gemini 규칙의 값은 예상값 `antigravity-cli`이다.
And `platform:` 필드를 가진 모든 Codex 규칙의 값은 예상값 `codex`이다.
And 패리티 게이트는 platform-value 불일치 finding을 정확히 0건 보고한다.

### S6: 기존 플랫폼 출력 후방호환 유지 (Must, REQ-005 / INV-006)
Given 변경 후의 Codex·Claude·OpenCode 어댑터가 주어졌다.
When 각 어댑터가 규칙을 생성한다.
Then Codex는 정확히 14종 규칙을 생성하고, frontmatter를 가진 8종 규칙에 `platform: codex`가 유지된다.
And Claude와 OpenCode는 각각 정확히 14종 규칙을 생성한다.
And 어떤 기존 플랫폼도 규칙을 잃거나 추가로 얻지 않는다.

## Oracle Acceptance Notes

- Must 시나리오(S1·S2·S3·S4·S6)는 모두 concrete expected output을 Then에 고정한다: S1은 3개 파일
  존재 + 예상 본문 문자열(`# Deferred Tools Loading`·`Antigravity CLI`·`# Project Identity`·
  `SPEC Quality Checklist`), S2는 정확한 14종 basename 집합, S3은 `{__parity_probe__, P}` finding
  정확히 1건 및 exclusion 추가 후 0건, S4는 `.claude/skills/autopus/` 부분 문자열 부재 + 해소 경로,
  S6은 정확한 규칙 카운트 14 및 `platform: codex` 유지.
- Should 시나리오 S5의 예상 출력: Gemini `platform:` 값 = `antigravity-cli`, Codex `platform:` 값 =
  `codex`, 불일치 finding 0건.
- structural 신호(file exists, heading, exit code, non-empty output)는 보조일 뿐 단독 통과 기준이
  아니며, 각 시나리오는 위의 explicit 예상 값으로 판정한다.
- INV-001~INV-006과 S1~S6의 매핑은 spec.md `## Traceability Matrix` 및 research.md
  `## Semantic Invariant Inventory`와 양방향 일치한다.
