# SPEC-CODEXQUAL-001 구현 계획

## Implementation Strategy

정책 계산을 template 문자열에 분산하지 않고 `pkg/config`의 순수 profile/capability resolver에
모은다. adapter, content transformer, orchestra CLI는 resolver 결과만 소비한다. capability
탐지는 bounded runner와 structured JSON parser로 분리해 테스트할 수 있게 만들고, 탐지 실패는
관측 가능한 fallback 결과로 처리한다.

사용자 소유권 경계는 유지한다. fresh root template은 새 정책을 사용하고, 기존 root의 사용자
소유 `model`/`model_reasoning_effort` 우변 literal은 키 단위로 보존한다. 정책이 없는 레거시
설정은 보존 우선으로 처리하고, 명시적 supervisor 정책에서만 exact generated tuple을 회수한다.
orchestra에는 managed policy 표시를 추가하고, 명시적 pinned/custom provider의
Binary·Args·PaneArgs slice는 runtime quality/effort overlay에서도 그대로 둔다.

## Visual Planning Brief

```text
runtime --effort ───────────────────────────────────────────────┐
runtime --quality ─┐                                            │
quality.default ───┼─> effective quality ─> desired profile ───┼─┐
balanced fallback ─┘                                            │ │
                                                               ▼ ▼
                                                   capability resolver
                                ┌──────────────────────┼──────────────────────┐
                                ▼                      ▼                      ▼
                        requested model        legacy gpt-5.5        runtime default
                         lower effort           xhigh ceiling          omit fields
                                │                      │                      │
                                └──────────────────────┼──────────────────────┘
                                                       ▼
                                  root / agent / Args / PaneArgs consumers
```

`runtime --effort`는 quality-managed orchestra의 quality-derived effort만 덮어쓴다. persistent
managed agent와 pinned provider에는 전달하지 않는다.

## Tasks

- [x] T1: `pkg/config`에 Sol/Terra/Luna/legacy 모델 상수, quality·role·tier profile resolver,
  quality precedence, structured catalog parser, fallback reason을 추가한다. full-support와 partial
  catalog를 사용하는 table-driven RED/GREEN 테스트로 `supported`, `effort_unavailable`,
  `model_unavailable`, `runtime_default`, `catalog_unknown`을 고정한다.
- [x] T2: `templates/codex/config.toml.tmpl`과 Codex adapter merge를 갱신한다. fresh root는
  `inherit`로 project-local model/effort를 생략하고, 명시적 quality-managed root만 full-support
  catalog에서 Balanced=`Sol+xhigh`, Ultra=`Sol+ultra`를 생성한다. 사용자 root model/effort는
  키 목록 마커와 파싱된 우변 literal로 보존하고, 레거시 markerless tuple·비모델 checksum
  drift·quoted key·multiline TOML fixture를 분리 검증한다.
- [x] T3: `pkg/content` transformer가 agent name, source fallback tier, preset role tier, source declared
  effort를 resolver에 전달하도록 갱신한다. model과 effort를 서로 다른 declared-effort 입력으로
  해석하지 않도록 한 tuple에서 계산한다. blank/unknown declared effort=`medium`, worker
  declared `ultra`=`max`, Balanced Opus=`xhigh`, Ultra worker=`Sol+max` RED 테스트를 먼저 추가한다.
  이후 16개 Codex agent template을 source와 동기화하고 native subagent/team surface가 같은 generated
  agent 파일을 참조하는지 검증한다.
- [x] T4: canonical Codex orchestra provider에 `model_policy: quality`를 표시하고 config migration 및
  runtime overlay를 일반 orchestra, `orchestra run`, structured SPEC review에 배선한다. effective
  quality는 `runtime --quality > quality.default > balanced`, effective effort는
  `runtime --effort > quality-derived effort` 순서로 계산한다. exact historical canonical tuple만
  quality로 이행한다. custom/pinned Binary·Args·PaneArgs와 `--` suffix를 slice equality로 보존하는
  테스트를 먼저 작성한다.
- [x] T5: init/update 및 orchestra 실행 지점에서 bounded capability probe를 사용하고 모든 consumer가
  `ResolveCodexProfile` 결과를 소비하도록 배선한다. 같은 모델 effort downgrade, 같은 모델에 낮은
  effort가 없을 때 model 유지+effort 생략, GPT-5.5 `xhigh` ceiling, malformed/oversized
  `catalog_unknown`, valid-but-missing `runtime_default`를 서로 다른 fixture로 검증한다. 하나의
  capability matrix를 root model/effort, agent model/effort, subprocess Args, interactive PaneArgs에
  투영하는 통합 oracle을 추가하고 fallback receipt의 `requested`, `selected`, `reason`을 검증한다.
- [x] T6: adaptive-quality/agent-pipeline의 source와 Codex generated guidance를 새 행렬과 runtime
  경계로 동기화한다. persistent worker 변경에는 `auto quality <mode> --apply`와 새 세션이 필요하고
  per-run `--quality`/`--effort`는 quality-managed orchestra에만 즉시 적용된다고 명시한다.
  planner/validator agent body, `ARCHITECTURE.md`, `.autopus/project/product.md`,
  `.autopus/project/tech.md`, active `autopus.yaml` 예시를 감사하고 OpenCode/Claude golden 회귀를 실행한다.
- [x] T7: `acceptance.md`의 S1~S20 evidence map에 따라 focused tests, template parity,
  `go test ./... -count=1`, race tests, `go vet ./...`, `go build ./...`, 로컬
  `codex debug models`, strict SPEC validation, dirty-worktree 및 tracked-but-ignored 점검을 수행한다.

## Sequencing

T1의 RED/GREEN resolver가 T2~T5의 선행 조건이다. T2와 T4는 파일 소유권이 분리되므로 병렬로
구현할 수 있다. T3는 사용자가 수정 중인 agent source/template과 겹칠 수 있으므로 전체 generator를
실행하지 않고 transformer 결과와 대상 diff를 대조해 안전하게 반영한다. T5는 consumer별 배선이
끝난 뒤 통합 capability oracle로 수렴시킨다. T6은 행렬과 runtime 경계가 고정된 뒤 수행하고 T7로
종료한다.

## Feature Completion Scope

Primary SPEC 하나가 정책 SoT, fresh supervisor 상속 정책, opt-in supervisor 및 managed agent 생성,
canonical orchestra runtime overlay, capability fallback, 사용자 설정 보존, 문서 parity를 함께 닫는다. 별도 sibling SPEC에
의존하지 않는다. Per-run quality가 이미 로드된 custom agent를 바꾸지 못하는 제약은 REQ-009의
명시적 사용자 계약으로 닫는다. mode-qualified agent 파일을 두 벌 생성하는 확장은 Outcome Lock에
포함하지 않는다.

## Completion Debt

없음. REQ-001~REQ-010과 S1~S20을 구현 및 hermetic test evidence로 모두 닫는다.

## Verification Limitation

현재 개발 계정은 Sol/Terra/Luna와 전체 effort를 제공하므로 GPT-5.6 entitlement가 없는 실제 계정에서
live smoke를 실행할 수 없다. 이 환경 제약은 unsupported, malformed, valid-but-missing catalog를
주입한 hermetic fixture와 runtime receipt oracle로 대체한다. 현재 계정에서는
`codex-cli 0.144.0`의 live catalog smoke를 별도로 실행한다. 이 제한은 미구현 기능이나 후속 완료
작업을 뜻하지 않는다.
