# PRD: SPEC-CODEXQUAL-001 - GPT-5.6 품질별 Codex 실행 프로필

**Status**: completed
**Created**: 2026-07-10
**Updated**: 2026-07-11
**Domain**: CODEXQUAL

## 문제

변경 전 Autopus-ADK의 Codex surface는 supervisor, managed agent, native multi-agent,
orchestra를 모두 `gpt-5.5`에 고정했다. `quality.default`가 `ultra` 또는 `balanced`여도
모델 계층은 달라지지 않았고 일부 reasoning effort만 바뀌었다. Codex 0.144는 역할이 다른
`gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna`와 `max`, `ultra` effort를 제공하므로,
기존 설정만으로는 품질 모드의 의도와 최신 Codex 실행 능력을 반영할 수 없다.

## 사용자 결과

새 프로젝트의 Codex 주 세션은 사용자가 선택한 runtime 기본 모델을 상속한다. 사용자는 하네스의
persistent quality와 명시적 orchestra runtime override에 따라 관리형 agent와 orchestra의 모델과
effort를 일관되게 적용한다. 기존 사용자 소유 설정과 pinned provider는 바뀌지 않으며, 현재 계정이나
CLI가 GPT-5.6을 제공하지 않아도 관측 가능한 fallback을 사용한다.

| 실행 역할 | Balanced | Ultra |
|---|---|---|
| Fresh supervisor (`inherit`) | 사용자 Codex 기본값 | 사용자 Codex 기본값 |
| Quality-managed supervisor | `gpt-5.6-sol` + `xhigh` | `gpt-5.6-sol` + `ultra` |
| `planner`/`architect`/`security-auditor` | `gpt-5.6-sol` + `xhigh` | `gpt-5.6-sol` + `max` |
| Other Opus-tier managed agent | `gpt-5.6-sol` + `xhigh` | `gpt-5.6-sol` + `xhigh` |
| Other Sonnet-tier managed agent | `gpt-5.6-terra` + 역할 effort | `gpt-5.6-sol` + `xhigh` |
| Other Haiku-tier managed agent | `gpt-5.6-luna` + 역할 effort | `gpt-5.6-sol` + `xhigh` |
| Quality-managed Codex orchestra | `gpt-5.6-sol` + `xhigh` | `gpt-5.6-sol` + `ultra` |

`ultra` effort는 최대 추론에 자동 task delegation을 더한다. 따라서 depth 0 supervisor와
독립 orchestra에만 자동 적용한다. supervisor가 이미 명시적으로 배치한 managed worker 중
planner, architect, security-auditor는 `max`를 사용하고, 나머지는 `xhigh`를 사용해 일반 작업의
토큰 상한을 낮춘다. Balanced Opus worker는 전략 작업의 품질을 유지하면서 Ultra와 비용 경계를
분명히 하기 위해 source declared effort와 관계없이 `xhigh`로 고정한다.

## 목표

- 한 중앙 정책에서 quality, 실행 역할, source tier, declared worker effort를 Codex 프로필로 해석한다.
- orchestra quality 우선순위를 `runtime --quality > quality.default > balanced`로 고정한다.
- 명시적 runtime `--effort`가 quality에서 파생된 effort보다 우선하도록 하되,
  `model_policy: quality` orchestra에만 적용한다.
- quality-managed supervisor, managed agent, orchestra의 capability fallback이 같은 resolver 결과를 사용한다.
- fresh supervisor는 project-local model/effort를 생략해 사용자 Codex runtime 기본값을 상속한다.
- 기존 root model/effort 우변 literal과 pinned/custom provider argv slice를 각 소유권 단위로 보존한다.
- GPT-5.6을 사용할 수 없는 계정이나 구형 Codex CLI에 관측 가능한 호환 fallback을 제공한다.

## 비목표

- Claude model/effort 정책 변경
- OpenCode의 `openai/gpt-5.4`, `--model`, `--variant` 정책 변경
- Codex fan-out, `max_threads`, `max_depth` 기본값 변경
- 이미 로드된 custom agent에 per-spawn model/effort override를 추가하는 일
- 기존 사용자 소유 root 설정이나 pinned/custom provider argv의 강제 이행
- 과거 SPEC과 CHANGELOG의 역사적 `gpt-5.5` 기록 재작성
- 계정 entitlement 원격 탐지 또는 telemetry 집계 시스템 추가

## 성공 기준

- REQ-001~REQ-010이 `acceptance.md`의 S1~S22와 semantic invariant로 완전히 추적된다.
- full-support, partial-support, malformed, valid-but-missing catalog fixture가 서로 분리된 oracle로 고정된다.
- fresh root, managed agent, subprocess Args, interactive PaneArgs가 동일 capability 결과를 소비한다.
- persistent quality, runtime quality, runtime effort, pinned ownership의 우선순위 테스트가 통과한다.
- root 설정과 provider argv 보존 단위를 혼동하지 않는 회귀 테스트가 통과한다.
- strict SPEC validation, focused/full Go tests, vet, build, local catalog smoke가 완료된다.
