# SPEC Quality Checklist

Use this checklist after `spec.md`, `plan.md`, `acceptance.md`, and `research.md` are drafted, and before the SPEC enters `content/skills/spec-review.md`.

## How To Apply

1. Read all four SPEC files as one document set.
2. For every checklist item, record `PASS`, `FAIL`, or `N/A` with a short reason in natural language.
3. If an item fails, fix every affected file required by that dimension. Do not limit edits to the file where the symptom first appeared.
4. Retry the full checklist up to 2 times.
5. If failures remain after the second retry, append `## Open Issues` to the end of `spec.md` and list each remaining `Q-*` ID with the reason.

The five primary dimensions map 1:1 to `FindingCategory` values in `pkg/spec/types.go`:
- `correctness`
- `completeness`
- `feasibility`
- `style`
- `security`

For document-only SPEC work, security items may be marked `N/A` only when the note explains why no real trust boundary, secret, or privileged path is involved.

## correctness

### Q-CORR-01 — Existing references are real

- PASS 기준: `[NEW]` 마커가 없는 경로, 타입, 함수, 규칙 이름이 현재 코드베이스에서 실제로 확인된다.
- FAIL 기준: 기존 참조가 존재하지 않거나, 다른 이름/다른 위치를 실제 경로처럼 단정한다.
- Example: `pkg/spec/types.go`의 상수 이름을 인용했다면 실제 선언을 직접 확인한다.

### Q-CORR-02 — Planned additions are marked as planned additions

- PASS 기준: 아직 존재하지 않는 새 파일, 타입, 함수, 섹션은 `[NEW]`로 표기되고 기존 참조 검증 대상에서 제외된다.
- FAIL 기준: 미래 변경을 기존 참조처럼 서술하거나, `[NEW]` 없는 신규 항목을 정합성 PASS 근거로 사용한다.
- Example: `[NEW] content/rules/spec-quality.md`는 허용되지만, 같은 경로를 기존 파일처럼 단정하면 FAIL이다.

### Q-CORR-03 — Claimed syntax matches real parser and validator behavior

- PASS 기준: EARS, Gherkin, Priority, validator 제약에 대한 설명과 예시가 실제 파서/validator가 받아들이는 형식과 맞는다.
- FAIL 기준: 문서가 지원하지 않는 구문을 공식 형식처럼 제시하거나, 예시가 실제 parser contract와 어긋난다.
- Example: acceptance 예시는 parser가 인식할 수 있는 `Given/When/Then` 형태를 사용한다.

## completeness

### Q-COMP-01 — The four SPEC files form one complete package

- PASS 기준: `spec.md`, `plan.md`, `acceptance.md`, `research.md`가 각각 목적과 역할을 갖고 서로 보완된다.
- FAIL 기준: 문서 중 하나가 비어 있거나, 핵심 결정·검증 근거가 빠져 다른 문서가 역할을 대신 떠안는다.

### Q-COMP-02 — Requirements, plan, and acceptance stay traceable

- PASS 기준: 요구사항이 구현 태스크와 수락 기준에서 빠짐없이 다뤄지며, 빠진 항목이 있으면 `Open Issues` 또는 out-of-scope로 명시된다.
- FAIL 기준: REQ가 구현 태스크나 acceptance에서 근거 없이 사라지거나, 어떤 문서가 어떤 요구를 검증하는지 추적할 수 없다.
- Example: 추적성 FAIL은 `spec.md`와 `acceptance.md`를 함께 수정해야 할 수 있다.

### Q-COMP-03 — EARS type, trigger, and observability are explicit

- PASS 기준: 각 요구사항의 EARS type, 조건, 기대 결과, 관측 지점이 문서상에서 분명하다.
- FAIL 기준: type만 적고 조건이나 기대 결과가 없거나, 검증 가능한 관측 지점 없이 행동만 선언한다.
- Example: self-verify loop를 요구한다면 어느 문서와 어떤 흔적으로 확인하는지 함께 적는다.

## feasibility

### Q-FEAS-01 — Scope matches the real implementation layer

- PASS 기준: 문서/프롬프트 변경인지, 런타임 코드 변경인지, 후속 SPEC 분리 대상인지가 실제 구현 경로와 맞는다.
- FAIL 기준: 문서만 바꿔서는 보장할 수 없는 동작을 즉시 구현된 것처럼 약속하거나 source of truth를 혼동한다.

### Q-FEAS-02 — Proposed changes fit the owning repo and module boundaries

- PASS 기준: 변경 대상 경로가 실제 repo 구조와 module ownership에 맞고, generated surface와 source of truth를 구분한다.
- FAIL 기준: 존재하지 않는 디렉토리를 편집 대상으로 잡거나, generated copy를 source of truth처럼 다룬다.
- Example: `content/`를 source of truth로 바꾸는지, 설치된 `.codex/` 복사본을 바꾸는지 혼동하면 FAIL이다.

### Q-FEAS-03 — Verification steps are runnable and proportionate

- PASS 기준: 제안한 검증 명령, 수동 점검, 또는 문서 검토 절차가 현재 저장소에서 실제로 수행 가능하다.
- FAIL 기준: 존재하지 않는 명령을 제시하거나, 문서-only 변경에 불필요한 대규모 검증을 강제한다.
- Example: markdown-only 변경이면 포맷, 경로, 라인 수, 교차 참조 점검으로 충분할 수 있다.

## style

### Q-STYLE-01 — Requirement text avoids ambiguous wording

- PASS 기준: REQ description은 `should`, `might`, `could`, `possibly`, `maybe`, `perhaps` 같은 모호어 없이 단정적으로 작성된다.
- FAIL 기준: 모호어가 REQ description에 들어가 validator 경고를 유발하거나, 의미가 흐려진다.
- Example: Priority 값은 description이 아니라 별도 메타 라인에 둔다.

### Q-STYLE-02 — Priority and EARS type remain separate axes

- PASS 기준: Priority는 `Must`, `Should`, `Nice`만 사용하고, EARS type과 같은 규칙으로 섞지 않는다.
- FAIL 기준: Priority와 EARS type을 하나의 FAIL 규칙에 묶거나, `P0`, `P1`, `P2`, `Could` 같은 별칭을 허용한다.
- Example: `Ubiquitous / Priority: Must`는 가능하지만, 둘을 같은 검사 축으로 취급하면 FAIL이다.

### Q-STYLE-03 — Sentences and Gherkin steps are readable and parseable

- PASS 기준: REQ/AC 문장은 완결된 문장으로 끝나며, acceptance 예시는 bare `Given`, `When`, `Then`, `And`, `But` 형식을 따른다.
- FAIL 기준: 문장이 미완성으로 끝나거나, bullet/강조 마크업이 step keyword를 가려 parser와 사람 모두가 읽기 어렵다.
- Example: `Given 사용자가 로그인했다.`는 PASS이고 `- **Given**: ...`는 권장 형식이 아니다.

## security

### Q-SEC-01 — Trust boundaries and prompt-input risk are addressed

- PASS 기준: 외부 입력, prompt injection, provider output, 혹은 untrusted 문서가 품질 루프에 들어오면 그 경계와 완화 방안을 적는다.
- FAIL 기준: 외부 입력이 있는데도 trusted local document처럼 취급하거나, 공격 표면 언급 없이 흐린다.
- N/A 기준: 로컬 저장소의 human-maintained markdown만 다루고 외부 입력 경로가 없다.

### Q-SEC-02 — Sensitive paths and secrets stay protected

- PASS 기준: 비밀값, 토큰, credential, 절대 경로, privileged 파일 접근이 필요하면 노출/오용 방지를 명시한다.
- FAIL 기준: 민감값을 예시로 노출하거나, path traversal 성격의 경로를 검증 없이 허용한다.
- N/A 기준: 비밀값과 privileged path를 전혀 다루지 않는 문서-only SPEC이다.

### Q-SEC-03 — Logging and retained artifacts do not create avoidable risk

- PASS 기준: 로그, audit note, retained artifact를 요구한다면 포맷 안정성, retention, diff noise, secret leakage를 함께 고려한다.
- FAIL 기준: 구조화 기록을 요구하면서 포맷 보장 수단이 없거나, 불필요한 영구 artifact를 남겨 후속 작업을 오염시킨다.
- N/A 기준: 별도 로그/아티팩트를 만들지 않고 결과를 문서 내부에서만 정리한다.

## Appendix — cohesion

This appendix is a secondary axis and remains separate from the five primary dimensions above.

### Q-COH-01 — One SPEC, one cohesive change story

- PASS 기준: SPEC이 하나의 명확한 문제와 소수의 밀접한 변경 대상으로 수렴한다.
- FAIL 기준: unrelated concerns를 한 SPEC에 묶어 reviewer나 executor가 경계를 파악하기 어렵다.

### Q-COH-02 — Follow-on work is split instead of hand-waved

- PASS 기준: 현재 iteration에서 다룰 수 없는 후속 런타임/플랫폼 작업은 별도 SPEC이나 out-of-scope 항목으로 분리된다.
- FAIL 기준: 현재 문서가 해결하지 못하는 문제를 같은 iteration 안에서 암묵적으로 해결된 것처럼 남긴다.
