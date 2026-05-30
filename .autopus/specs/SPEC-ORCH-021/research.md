# SPEC-ORCH-021 리서치

## 기존 코드 분석

두 개의 공존하는 오케스트레이션 아키텍처가 이 SPEC의 핵심이다.

### Architecture 1 — legacy `RunOrchestra` 경로 (brainstorm/plan/review/secure)

- `internal/cli/orchestra.go:86 runOrchestraCommand` (fan_in=4)가 `orchestra.go:162-164`에서 `term := terminal.DetectTerminal()` + `interactive := term != nil && term.Name() != "plain"`로 판정 후 `OrchestraConfig{Interactive, SubprocessMode, Terminal, ...}`를 만든다.
- `pkg/orchestra/runner.go:25`: `if !cfg.SubprocessMode && cfg.Terminal != nil && cfg.Terminal.Name() != "plain" { return RunPaneOrchestra(ctx, cfg) }` — **진짜 인터랙티브 pane 경로**. `SubprocessMode==true`면 cmux에서도 pane을 건너뛴다.
- `RunPaneOrchestra` (`pane_runner.go:28`) → `Interactive`면 `RunInteractivePaneOrchestra` (`interactive.go:19`): split panes → launch sessions → send prompts → `waitForCompletion` → `ReadScreen(Scrollback)` → `cleanScreenOutput` (`interactive_collect.go:43-64`).
- `internal/cli/orchestra_brainstorm.go:60`: `cmd.Flags().BoolVar(&subprocess, "subprocess", true, ...)` — brainstorm만 기본값 `true`라 pane 경로를 우회한다. plan(`orchestra.go:67`)·review/secure(`orchestra_file_cmds.go:37,79`)는 `SubprocessMode`를 넘기지 않아 기본 `false` → **이미 pane을 쓴다(변경 불필요, 비목표)**.
- History: commit `ca7f026 "feat(orchestra): brainstorm 기본 백엔드를 subprocess로 변경"` (Ref SPEC-ORCH-019)가 "cmux pane 모드에서 SignalDetector 타임아웃 → poll 폴백으로 불필요한 지연"을 이유로 brainstorm 기본값을 subprocess로 설정했다. 이 SPEC은 그 기본값을 역전한다(아래 "역전 trade-off" 참조).

### Architecture 2 — `ExecutionBackend` 인터페이스 (structured: spec review, orchestra run)

- `pkg/orchestra/backend.go:10` 인터페이스 `ExecutionBackend{ Execute(ctx, req) (*ProviderResponse, error); Name() string }`.
- `backend.go:49-54 SelectBackend(cfg)`: 현재 조건은 `if cfg.SubprocessMode || cfg.Terminal == nil { NewSubprocessBackendImpl() } else { NewPaneBackend() }` (라이브 재확인). **F-001 결함**: legacy `runner.go:25` 가드(`!cfg.SubprocessMode && cfg.Terminal != nil && cfg.Terminal.Name() != "plain"`)와 달리 `SelectBackend`는 `Name() == "plain"`을 검사하지 않아, non-nil `"plain"` 터미널에서 pane 분기를 타고 가짜 pane(`NewPaneBackend`→child-process)을 반환한다. 이는 REQ-002/REQ-003의 "plain→subprocess"와 acceptance R3/S3에 반하므로 이 SPEC scope에서 수정한다(T2).
- **핵심 결함**: `backend.go:37 PaneBackend.Execute`는 `runProvider(ctx, req.Config, req.Prompt)`를 호출한다. `pkg/orchestra/provider_runner.go:23 runProvider`는 child process를 띄우고 `provider_runner.go:27-29`에서 `if provider.PromptViaArgs { args = append(args, "-p", prompt) }` else stdin. 즉 `PaneBackend`와 `SubprocessBackend` 둘 다 `-p`/stdin child process를 실행하며, `SubprocessBackend`(`subprocess_runner.go:105-107`)만 schema flag(`SchemaFlag`+`SchemaPath`)를 추가한다. **structured 경로에는 진짜 인터랙티브 pane 실행 backend가 현재 없다.**
- spec review hardcode: `internal/cli/spec_review_runtime.go:14 specReviewBackendFactory = orchestra.NewSubprocessBackendImpl`. `spec_review_structured.go:19 runStructuredSpecReviewOrchestra`가 `:36`에서 `specReviewBackendFactory()`를 직접 호출하고 per-provider로 `:57 backend.Execute(ctx, req)`를 돌린다. `SelectBackend`를 전혀 거치지 않는다.
- **terminal nil 확인**: `spec_review.go`는 `DetectTerminal()`을 호출하지 않으며, `spec_review_loop.go:53`의 `OrchestraConfig`에 `Terminal` 필드를 채우지 않는다. 따라서 설령 `SelectBackend`를 쓰더라도 `Terminal==nil`이라 항상 subprocess를 고른다 → terminal 감지 주입이 필요하다(REQ-006).
- orchestra run hardcode: `orchestra_run_runtime.go:8-9 orchestraRunBackendFactory = NewSubprocessBackendImpl`, `orchestraRunExecutePipeline = RunSubprocessPipeline`. `orchestra_run.go:130-135`가 `SubprocessMode: forceSubprocess`로 cfg를 만들고 `:135 _ = orchestra.SelectBackend(cfg)`로 **결과를 버린다**(주석 "validate selection"). 실제 backend는 `:138 orchestraRunBackendFactory()`(hardcode subprocess). cfg에 `Terminal`도 없다.
- **두 메커니즘 공존(F-002/F-006)**: brainstorm은 `runOrchestraCommand→RunOrchestra→runner.go:25` 인라인 가드로 pane 여부를 판정하며 `ExecutionBackend`/`SelectBackend`를 전혀 호출하지 않는다. structured 두 경로는 `ExecutionBackend`/`SelectBackend`를 쓴다. 즉 pane-vs-subprocess 선택 로직이 **두 군데에 독립 복제**되어 있어, 한쪽만 고치면 분기 규칙이 어긋날 수 있다. 이 SPEC은 선택 로직을 단일 술어 `paneCapable`로 추출하여 양쪽이 소비하게 한다(설계 결정 참조).

### Parser / sanitizer 재사용 가능성

- `pkg/orchestra/output_parser.go:96-100 unmarshal`이 `extractJSON`으로 주변 prose를 허용 → sanitize된 screen scrollback에 JSON이 있으면 `ParseReviewer`(`:46`)/`ParseDebaterR1`(`:13`) 파싱 가능. 새 pane backend가 structured 출력을 낼 수 있는 근거.
- `pkg/orchestra/screen_sanitizer.go:42 SanitizeScreenOutput`(ANSI/status bar/CLI banner 제거) + `interactive_detect.go:233 CleanScreenForCrossPollination`이 이미 존재. pane backend 출력 정제에 재사용.
- `pkg/orchestra/cc21_monitor.go:70 waitForCompletion`: `cfg.MonitorEnabled`면 event-driven detector를 쓰고 `cfg.MonitorTimeout` 초과 시 `ScreenPollDetector`로 fallback(`cc21_monitor.go:85-88`). 이것이 `ca7f026`이 우려한 "SignalDetector 타임아웃→poll 지연"을 완화한다(아래).

## Provider 호출 정합성 진단 (Revision 3, 라이브 재현)

사용자 명시: "gemini는 왜 실패, codex는 왜 무응답"을 함께 고친다. 두 근본 원인 모두 라이브 재현·검증했다(codex 0.135.0, agy 1.0.0).

### gemini 실패 — `agy --print` 무값 argv

- config: `autopus.yaml` gemini 블록은 `binary: agy` + `subprocess.output_format: text`만 두고 `args` override가 없다. 따라서 argv는 **fallback registry** `orchestra_helpers.go:147` `{Binary:"agy", Args:["--print"], PaneArgs:["--print"], PromptViaArgs:false, OutputFormat:"text"}`가 지배한다.
- `provider_runner.go:26-29`: `PromptViaArgs:false`면 프롬프트를 stdin으로 보내고 argv는 `agy --print`만. 그러나 `agy`의 `--print`/`-p`/`--prompt`는 프롬프트를 **값으로 받는 string 플래그**다(`agy --help` 확인: `--print Run a single prompt non-interactively`, `-p` = `--print` 단축, `--prompt` = alias).
- 라이브 재현: `printf 'say hello' | agy --print` → `flag needs an argument: -print`(exit 2). 즉 stdin 읽기 전에 flag 파싱에서 즉사. `agy --print "<prompt>"`(값 인자)는 flag 에러 없이 모델 대기로 진입(확인). agy는 대화형 `-i`/`--prompt-interactive`도 보유.
- 정정(REQ-014/REQ-015): subprocess = 프롬프트를 `--print` 값으로(placeholder `Args:["--print",""]`+`PromptViaArgs:true`가 `buildSubprocessArgs`/`injectPromptArg`에서 동작), pane = `--print` 제거 후 대화형 세션.

### codex 무응답 — review provider 집합 누락(호출 자체 정상)

- 근본 원인: `autopus.yaml:34 review_gate.providers: [claude, gemini]` — codex가 구조화 리뷰 provider 목록에 없어 **아예 호출되지 않음**. orchestra 커맨드(`brainstorm/plan/review/secure`)는 `[claude, codex, gemini]`로 codex 포함. `runProviderTransportSmoke`(`doctor_provider_smoke.go:91`)도 `resolveSpecReviewProviderNames(cfg,false)`로 review_gate를 읽어 codex가 빠진다.
- codex 호출 자체는 정상: `codex exec [PROMPT]`는 PROMPT를 인자 또는 stdin(`-`/미제공)으로 받음. 정정(REQ-016): review_gate에 codex 추가 + codex `SchemaFlag:"--output-schema"`로 structured 응답.

### codex `--full-auto` 라이브 검증 결과 (사용자 요청)

- `codex exec --help`(0.135.0) 유효 플래그: `--output-schema <FILE>`, `--json`, `-o/--output-last-message <FILE>`, `-m/--model`, `-s/--sandbox <read-only|workspace-write|danger-full-access>`, `-c <key=value>`, `--skip-git-repo-check`, `--cd` 등. `-p`는 `--profile`(프롬프트 아님)임에 주의.
- `--full-auto`는 help에 없으나 **deprecated alias로 수용**된다: `codex exec --full-auto ...` 실행 시 stderr에 `warning: --full-auto is deprecated; use --sandbox workspace-write instead.`를 내고 정상 동작(approval: never로 실행됨). 즉 하드 실패는 아니나 경고를 남기므로 SPEC은 `--sandbox workspace-write`로 표준화(fallback registry `orchestra_helpers.go:146`은 이미 올바름).
- 결론: codex 무응답의 원인은 `--full-auto`가 아니라 review_gate 누락이다. `--full-auto`는 정합성 정리(경고 제거) 차원에서 표준화한다.

### per-provider 미묘함 (generic 처리 + nuance 명시)

- "`-p`=API" 전제는 provider마다 다를 수 있다. 예컨대 `agy --print`는 gemini 로그인(구독) 세션을 사용할 가능성이 있어 항상 별도 API 키를 요구하지 않을 수 있다(코드만으로 단정 불가 — confidence: low). SPEC은 호출 형태(argv 정합)를 generic하게 처리하고, "구독 세션은 인터랙티브가 기본"이라는 원칙은 유지하되 provider별 `-p` 가용성 차이를 강제 가정하지 않는다(REQ-005 best-effort + REQ-013 actionable 에러가 이 불확실성을 흡수).
- source of truth: provider 설정의 SoT는 `autopus-adk/autopus.yaml`의 `orchestra.providers`이며, `orchestra_helpers.go:143-163 buildProviderConfigs`는 **config 부재 시 fallback** registry다. gemini 버그는 config에 `args`가 없어 fallback이 적용된 경우이므로, 정정은 **양쪽(config + fallback)** 모두에 해야 한다.

## Subscription vs API 근거 (우선순위 재구성)

- `-p`(headless print)는 provider의 **API 경로**다(`provider_runner.go:27-29`가 `-p`를 argv에 붙여 child process 실행). API 경로는 별도 API 키/과금을 요구한다.
- 그러나 대부분의 유저는 구독제(Claude Pro/Max, ChatGPT Plus, Gemini 구독)로 AI를 쓰며, 구독 세션은 **로그인된 인터랙티브 CLI 세션**으로만 접근 가능하다. 즉 구독-only 유저에게 `-p`는 인증되지 않아 동작하지 않을 수 있고, **인터랙티브 pane이 멀티프로바이더를 돌릴 수 있는 유일한 실행 경로**다.
- 함의: (1) pane은 "fallback 회피용 default"가 아니라 다수 유저의 primary path다. (2) `-p` fallback은 best-effort이며 구독-only 유저에겐 미가용일 수 있으므로, pane+`-p` 동시 실패 시 raw API 에러가 아니라 actionable 에러가 필요하다(REQ-005/REQ-013). (3) 진짜 목표는 default flip이 아니라 **pane의 안정성**이다 — `ca7f026`이 드러낸 약점을 정면으로 닫는다(REQ-009~REQ-012).
- confidence: high(코드상 `-p`=API 경로 확인). 구독 세션이 인터랙티브 CLI로만 동작한다는 점은 provider별 정책에 의존하므로 메커니즘 설계는 그 가정을 강제하지 않고, `-p` 미가용을 일반적 실패 케이스로 다룬다(actionable 에러).

## Pane 신뢰성 메커니즘 인벤토리 (재사용 — 재발명 금지)

라이브 확인된 기존 메커니즘. 새 pane `ExecutionBackend.Execute`는 이들을 단일 provider 단위로 묶어 재사용한다.

- **세션 ready 감지**: `interactive_session_ready.go` — `SessionReadyPatterns()`(`:21`, claude `❯`/codex `codex>`/gemini `> Type your`/opencode `Ask anything`), `isSessionReady(screen, patterns)`(`:33`, shell `$`/`#` 제외로 false positive 방지), `startupTimeoutFor(provider)`(`:49`, claude 15s/gemini 10s/default 30s). 오케스트레이션 레벨에서 `interactive.go:66-69`은 launch→`waitForSessionReady`(`:147`)→`sendPrompts`(`:216`) 순서로 **ready 후에만 전송**한다. `pollUntilSessionReady`(`:191`)가 per-pane 폴링. → REQ-009.
- **완료 감지 계층**: `cc21_monitor.go:70 waitForCompletion` — `resolveCompletionDetector`(`:16`)가 `cfg.MonitorEnabled` 시 event-driven detector, 아니면 `ScreenPollDetector`. event-driven이 `MonitorTimeout` 내 미완료면 `:85-88`에서 `ScreenPollDetector`로 fallback. → REQ-010/REQ-011.
- **hook IPC (가장 안정적, 설치 의존)**: `hook_signal.go:36 NewHookSession`, `completion_file_ipc.go:23 FileIPCDetector.WaitForCompletion`(SECONDARY, 200ms 파일 폴링). `auto init` hook 설치에 의존(SPEC-ORCH-007 R5/R6). `resolveCompletionDetector`는 `cfg.HookMode`가 꺼져 있으면 `FileIPCDetector`를 만들지 않고 monitor/poll로 **degrade**. → REQ-012.
- **결정적 실패**: 위 계층이 모두 bounded(`MonitorTimeout`, per-provider `req.Timeout`)이므로, 미완료 시 `Execute`는 `ProviderResponse{TimedOut:true}`를 결정적으로 반환할 수 있다(무한 hang 금지). 기존 sentinel 경로의 `collectPaneResults`(`pane_runner.go:139`)도 `TimedOut`을 채운다. → REQ-011.

## Outcome Lock

- **User-visible outcome**: 구독 세션만 보유한 유저가 cmux/tmux 가능한 터미널에서 `/auto idea`, `/auto spec review`, `auto orchestra run`을 실행하면 provider가 `-p`(API) 없이 **로그인된 인터랙티브 pane 세션으로 안정적으로 멀티프로바이더 실행**된다(세션 ready 후 전송, bounded 완료 감지, hang 없음, hook 미설치여도 degrade하여 동작). plain/CI면 `-p` subprocess로 시도. pane 실패 시 `-p`를 best-effort로 시도하되, `-p`도 미가용이면 raw API 에러가 아니라 **actionable 에러**로 끝내고 실행 경로(또는 둘 다 실패)를 기록한다.
- **Mandatory requirements**: brainstorm pane 기본값(REQ-001); spec review structured pane-first(REQ-002); orchestra run structured pane-first(REQ-003); structured용 진짜 pane `ExecutionBackend` 존재(REQ-004); pane 실패 시 best-effort subprocess fallback + 실패 시 actionable 에러 + 경로 기록(REQ-005); spec review에 terminal 감지 주입(REQ-006); 단일 공유 pane-capability 술어를 양쪽 선택 사이트가 소비(REQ-007); pane screen 출력 정제(REQ-008); **pane 신뢰성 그룹** — 세션 ready 전 미전송(REQ-009), 완료 감지 monitor→poll bounded(REQ-010), timeout 시 결정적 실패 no-hang(REQ-011), hook 미설치 degrade(REQ-012), pane+`-p` 동시 실패 시 actionable 에러(REQ-013); **provider 호출 정합성 그룹** — subprocess argv 프롬프트 형태 정합(REQ-014, gemini `--print` 값/codex `exec`+`--output-schema`), pane argv 대화형 형태(REQ-015), 구조화 리뷰 provider 참여 정합 + codex `--output-schema`(REQ-016).
- **Accepted assumptions**: (a) sanitize된 pane scrollback에 provider가 JSON을 출력하면 `extractJSON`이 추출 가능하다(파서가 prose-tolerant임을 코드로 확인). (b) CC21 Monitor 경로가 활성일 때 pane 완료 지연이 `ca7f026` 당시보다 완화됐다(`waitForCompletion` 구조로 확인). (c) 기존 interactive 헬퍼(launch/세션ready/완료감지/수집)를 단일 provider 단위로 묶어 재사용할 수 있다. (d) 구독-only 유저에게 `-p`는 미가용일 수 있으므로 fallback은 best-effort이고, 미가용은 정상적 실패 케이스로 actionable 에러를 낸다. (e) hook IPC가 가장 안정적이나 설치 의존이므로 hook 미설치 시 monitor→poll degrade가 허용 가능한 신뢰성을 준다(`resolveCompletionDetector` 구조로 확인). If wrong: hook 없이 poll만으로 지연이 과도하면 후속 튜닝이 필요(Evolution). (f) `-p`가 API를 요구하는 정도는 provider별로 다를 수 있다(예: `agy --print`는 gemini 로그인 세션 사용 가능성 — confidence: low, 코드만으로 단정 불가). SPEC은 argv 정합을 generic하게 다루고 provider별 `-p` 가용성을 강제 가정하지 않으며, 불확실성은 best-effort fallback(REQ-005)+actionable 에러(REQ-013)로 흡수한다. (g) provider 설정 SoT는 `autopus-adk/autopus.yaml`이고 `orchestra_helpers.go buildProviderConfigs`는 config 부재 시 fallback이므로 정정은 양쪽에 적용한다(라이브 확인: gemini는 config에 `args` 없어 fallback이 버그 형태를 지배).
- **Deferred decisions**: 실행 경로 식별자를 `ProviderResponse` 신규 필드로 노출할지 기존 `FailedProvider.CollectionMode`/Receipt를 확장할지는 구현 시 결정(둘 다 oracle 단언을 만족 가능, 최소 변경 우선).
- **Explicit non-goals**: plan/review/secure 변경 / provider-API 백엔드 신설(Evolution) / JSON·structured 계약 자체 변경 / 새 strategy / brainstorm 프롬프트 변경 / 새 완료감지·세션ready·hook IPC 메커니즘 재발명(재사용만) / `auto init` hook 설치 수행 / provider CLI 자체 수정. (provider 호출 정합성 = 기존 설정·argv 정정이므로 scope IN.)
- **Completion evidence**: truth table oracle S1~S9 + pane 신뢰성 S10~S14 + provider 호출 정합성 argv oracle S15~S20 통과(S15 gemini 무값 `--print` 금지·값 형태, S16 codex `exec`+`--output-schema`, S17 gemini pane `--print` 미포함, S18/S20 codex review 참여, S19 codex pane 비-exec), `go build ./...` + `go test ./pkg/orchestra/... ./internal/cli/...` 통과, plan/review/secure 회귀 없음. 운영 검증: `auto doctor` provider-smoke(`runProviderTransportSmoke`)로 gemini/codex 실응답 보강(환경 의존).

## Visual Planning Brief

`plan.md`의 `## Visual Planning Brief` Mermaid flowchart 참조. 흐름: 구독 세션→pane(primary) / API→`-p`(best-effort fallback) / 둘 다 실패→actionable error. pane 경로 내부는 세션 ready 게이트(REQ-009)→bounded 완료 감지(monitor→poll, REQ-010)→timeout 시 결정적 실패(REQ-011)→실패 시 best-effort `-p`(REQ-005)→`-p`도 미가용 시 actionable 에러(REQ-013)로 흐른다. `SelectBackend`는 상위 분기, `Execute` 내부 신뢰성 스테이지는 하위 분기이며 모든 종착에서 실행 경로를 기록한다.

## 설계 결정

### REQ-004: 새 pane ExecutionBackend vs RunPaneOrchestra 라우팅 — **새 backend 채택**

- structured 경로는 per-provider `ExecutionBackend.Execute(ctx, req)` 계약 위에서 자체 병렬 수집(`spec_review_structured.go:40-82`)과 스키마/판정을 수행한다. `RunPaneOrchestra`는 merged `*OrchestraResult`를 반환하고 strategy별 merge(`pane_runner.go:70 mergeByStrategy`)를 내장하므로 per-provider `Execute` 계약과 임피던스 불일치가 크다.
- 새 backend는 기존 인터페이스(`backend.go:10`)에 정확히 맞고 `SubprocessPipelineConfig.Backend`(`pipeline.go:18`)에 시그니처 변경 없이 drop-in되며, 두 structured 진입점이 추가 분기 없이 동일하게 혜택을 본다. → 변경 표면이 작고 회귀 위험이 낮다.
- 대안(RunPaneOrchestra+OutputParser 라우팅)은 merged 결과를 다시 per-provider로 쪼개는 어댑터가 필요하고 strategy 로직 중복을 유발 → 거부.

### REQ-007: "단일 SelectBackend 호출"이 아니라 "공유 술어" — **shared predicate 채택** (F-002/F-006)

- 초안의 REQ-007/S1/S9는 세 진입점이 단일 `SelectBackend` 호출을 거친다고 단언했으나, brainstorm은 `RunOrchestra→runner.go:25` 가드를 타고 `SelectBackend`를 호출하지 않는다(라이브 확인). 즉 선택 로직이 두 곳에 복제되어 있다. "동일 backend 이름 반환 사후 비교"(초안 T6)는 동치를 *관찰*할 뿐 *구조적으로 보장*하지 못한다 — 한쪽 가드만 바뀌면 조용히 어긋난다.
- 결정: pane-vs-subprocess 선택 입력을 `[NEW] paneCapable(term terminal.Terminal, subprocessMode bool) bool`(반환식 `!subprocessMode && term != nil && term.Name() != "plain"`) 단일 술어로 추출한다. `runner.go:25` 가드와 `SelectBackend`(`backend.go:50`)가 **둘 다 이 술어를 소비**한다. F-001의 plain 처리도 이 술어에 포함되어 두 경로가 동일하게 plain→subprocess로 수렴한다.
- 두 경로는 실행 *모델*이 다르다: brainstorm은 술어 true 시 `RunPaneOrchestra`(멀티라운드 pane), structured는 술어 true 시 per-provider pane `ExecutionBackend`. 술어가 단일이므로 *모드 선택*은 분기 불가능하지만, 실행 객체는 경로별로 의도적으로 다르다. 이 사실을 REQ-007/INV-001/S9가 명시적으로 모델링한다(S9는 두 소비자가 동일 술어 결과에 합의함을 단언).
- 대안(brainstorm을 ExecutionBackend 경로로 재라우팅하여 물리적으로 단일 `SelectBackend`로 만들기)은 검증된 `RunInteractivePaneOrchestra` 멀티라운드 debate 경로를 해체해야 하므로 회귀 위험이 크고 SPEC scope("재설계 금지")를 벗어난다 → 거부. 술어 추출이 최소 변경으로 단일성을 확보한다.

### 역전 trade-off (ca7f026 대비)

- `ca7f026`은 brainstorm pane에서 SignalDetector 타임아웃→poll fallback 지연을 이유로 subprocess 기본값을 택했다. 그러나 (1) provider CLI들의 `-p` 제한/API 과금 이전이라는 새 제약이 비용/가용성 측면에서 더 크고, (2) `cc21_monitor.go`의 Monitor-우선/poll-fallback(`waitForCompletion`)과 `monitor-patterns.md`가 그 지연을 완화한다. trade-off: pane 기본값은 평균 지연이 다소 늘 수 있으나, fallback 경로가 보존되어 최악의 경우에도 완료성은 유지된다. confidence: medium(지연 개선은 코드 구조로 확인했으나 실측 벤치마크는 이 SPEC 범위 밖).

### Pane 신뢰성 설계 — 기존 메커니즘 재사용, 재발명 금지 (REQ-009~REQ-013)

- 진짜 목표는 default flip이 아니라 pane 안정성이므로, 새 `Execute`는 오케스트레이션 레벨(`RunInteractivePaneOrchestra`)이 이미 검증한 신뢰성 순서를 **단일 provider 단위로 압축 재사용**한다: split → launch → `pollUntilSessionReady`(ready 전 미전송, REQ-009) → send → `waitForCompletion`(monitor→poll, bounded, REQ-010) → 미완료 시 결정적 `TimedOut`(REQ-011) → `ReadScreen`+sanitize.
- hook 의존성: `FileIPCDetector`(가장 안정적)는 `HookSession`+`auto init` hook에 의존(SPEC-ORCH-007 R5/R6). `resolveCompletionDetector`(`cc21_monitor.go:16`)가 `HookMode` off일 때 hook 경로를 만들지 않고 monitor/`ScreenPollDetector`로 내려가므로 hook 미설치 환경에서도 동작(REQ-012). 이 SPEC은 hook을 설치하지 않으며, 설치는 운영자/`auto init` 책임으로 남긴다(non-goal).
- fallback 의미(REQ-005/REQ-013): 구독-only 유저에게 `-p`는 인증 실패할 수 있다. 따라서 pane 실패 시 `-p`를 **best-effort**로 시도하되(성공 시 `executed=subprocess` 기록), pane AND `-p` 둘 다 실패면 양 원인 + 복구 지시("ensure a logged-in cmux/tmux CLI session")를 담은 actionable 에러를 낸다. raw provider/API 에러로 끝내지 않음으로써 "구독으로는 `-p`가 안 된다"는 혼란을 숨기지 않는다.
- ca7f026 정면 대응: `ca7f026`의 "pane SignalDetector 타임아웃→poll 지연"은 (a) monitor 우선 + bounded(REQ-010), (b) timeout 시 결정적 실패로 무한 대기 제거(REQ-011)로 닫는다. 지연이 0이 되는 것은 아니나, 완료성과 결정성이 보장되어 pane을 신뢰 가능한 기본값으로 쓸 수 있다. confidence: medium(구조적 보장은 코드로 확인, 실측 지연은 범위 밖 — Evolution).
- 새 메커니즘 작성 금지: 위 모든 것은 `interactive_session_ready.go`/`cc21_monitor.go`/`completion_file_ipc.go`/`screen_sanitizer.go`의 기존 심볼 재사용이다. 새 backend 파일은 "단일 provider 래퍼 + best-effort fallback + 에러 구성"만 추가한다.

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "interactive pane mode ... is the DEFAULT, and `-p` headless subprocess mode is the FALLBACK" | backend-selection keyed by the shared predicate `paneCapable(term, subprocessMode)` (incl. non-nil `"plain"` → subprocess, F-001); consumed by both `runner.go` guard and `SelectBackend` | pane-vs-subprocess mode decision (predicate result) consumed by both selection sites | S1, S2, S3, S4, S5, S7, S9 |
| INV-002 | "spec review structured path selects pane-first when terminal capable, subprocess fallback otherwise; ... same [for] orchestra run" | conditional selection (capable→pane, plain/nil→subprocess) | SelectBackend result, SubprocessPipelineConfig.Backend.Name() | S2, S3, S4, S5 |
| INV-003 | "a real interactive-pane execution path that yields parseable structured output exists for the structured commands" | existence + output-parseability for oracle-asserted roles (`reviewer`, `debater_r1`, `judge` — each has a validating parser entry point); `debater_r2` shares the same sanitize+extractJSON path with no role-specific validation, so it is covered transitively, not separately oracle-asserted (F-002 부속) | pane backend ProviderResponse.Output parseable by `ParseReviewer`/`ParseDebaterR1`/`ParseJudge` | S8a, S8b, S8c |
| INV-004 | "pane 실패→`-p` best-effort 시도→그것도 실패하면 actionable 에러+실행 경로 기록" | best-effort fallback transition + path recording (구독-only 시 `-p` 미가용 허용) | executed-backend identifier on fallback; both-failed marker | S6, S14 |
| INV-005 | "record which path ran" | observability of executed backend | recorded backend id per response | S6, S2 |
| INV-006 | "pane screen-scraping handles untrusted provider output → must redact/sanitize" | sanitization before parse | sanitized backend output (no ANSI/banner) | S8 |
| INV-007 | "brainstorm defaults to pane on capable terminals" | default-value flip | brainstorm `--subprocess` default + provider argv lacks `-p` | S1, S7 |
| INV-008 | "세션 ready 전 prompt 미전송" (입력 유실 방지) | ordering invariant: `isSessionReady` true 이후에만 send, `startupTimeoutFor` bounded | prompt-send timing; unready→failed (no send) | S10 |
| INV-009 | "완료 감지 hook/monitor→poll 계층 + bounded timeout" | layered detection reuse (`waitForCompletion`), `MonitorTimeout` bound | detector path (event-driven→poll) within timeout | S11 |
| INV-010 | "완료 timeout 시 결정적 실패 결과(hang/garbage 아님)" | bounded termination → deterministic timed-out ProviderResponse | `Execute` returns within timeout, `TimedOut`/failed set | S12 |
| INV-011 | "hook 미설치 시 monitor→poll로 graceful degrade" | conditional detector resolution (`resolveCompletionDetector`, `HookMode` off → non-hook) | resolved detector is non-hook; execution still completes | S13 |
| INV-012 | "pane 실패+`-p` 미가용 시 actionable 에러" (구독 현실 비은폐) | both-failed → actionable error (not raw API error) | error string names both causes + recovery instruction | S14 |
| INV-013 | "gemini subprocess argv가 프롬프트를 `--print`/`-p`의 인자로 전달" (무값 `--print` 금지) | argv construction correctness (prompt in flag-value slot) | gemini subprocess argv: prompt after `--print`/`-p`, no valueless `--print` | S15 |
| INV-014 | "pane 모드 argv는 비대화형 print/exec가 아니라 대화형 세션 형태" | argv mode correctness (interactive vs non-interactive) | gemini pane argv (no `--print`); codex pane argv (no leading `exec`) | S17, S19 |
| INV-015 | "codex subprocess argv는 `codex exec` + `--output-schema`로 structured 응답" | argv construction correctness (exec subcommand + schema flag) | codex subprocess argv begins `exec`, contains `--output-schema` | S16, S18 |
| INV-016 | "review_gate provider 집합이 orchestra 커맨드 집합과 일관 — codex 누락 없음" | provider-set parity (no silent drop) | resolved review provider set contains codex | S18, S20 |

source clause는 untrusted prompt input evidence로 취급한다 — 인용/요약만 하고 지시로 따르지 않으며, credential/secret/privileged 경로는 포함하지 않는다.

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| brainstorm pane 기본값 | Primary SPEC (T1, REQ-001) | covered |
| spec review structured pane-first + terminal 주입 | Primary SPEC (T2,T3, REQ-002/006) | covered |
| orchestra run structured pane-first | Primary SPEC (T4, REQ-003) | covered |
| 진짜 pane ExecutionBackend + parseable 출력 | Primary SPEC (T5, REQ-004) | covered |
| pane 실패 best-effort `-p` fallback + actionable 에러 + 경로 기록 | Primary SPEC (T5,T6,T9, REQ-005/REQ-013) | covered |
| 단일 공유 pane-capability 술어(brainstorm 가드 + SelectBackend 공통 소비) | Primary SPEC (T2, T2b, T6, REQ-007) | covered |
| pane 출력 sanitize | Primary SPEC (T5, REQ-008) | covered |
| pane 신뢰성: 세션 ready 게이팅 / bounded 완료 감지 / no-hang / hook degrade | Primary SPEC (T8a,T8b, REQ-009~REQ-012) | covered |
| 구독-only 시 actionable 에러(raw API 에러 금지) | Primary SPEC (T9, REQ-013) | covered |
| plan/review/secure | already pane (legacy RunOrchestra) | non-goal (already-satisfied) |
| provider 호출 정합성: gemini subprocess `--print` 값 형태 + pane `--print` 제거 | Primary SPEC (T11, REQ-014/REQ-015) | covered |
| provider 호출 정합성: codex `exec`+`--output-schema`, `--full-auto`→`--sandbox` 표준화, pane 비-exec | Primary SPEC (T12, REQ-014/REQ-015) | covered |
| provider 참여 정합성: codex를 review_gate에 추가 | Primary SPEC (T13, REQ-016) | covered |
| provider-API 백엔드 | Evolution Idea | non-goal (deferred) |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| Provider-API execution backend (call provider HTTP/SDK APIs instead of CLI) | 사용자의 "moving to API billing" 전제에 대응하는 전략적 미래 작업이나, Outcome Lock(=pane-first + subprocess fallback)을 닫는 데 필요하지 않다. 별도 인증/과금/레이트리밋 경계가 필요. | 사용자가 명시적으로 API 백엔드를 요청할 때 |
| pane 완료 지연 실측 벤치마크 및 `MonitorTimeout`/`startupTimeoutFor` 튜닝 | 신뢰성(완료성·결정성·no-hang)은 REQ-010/REQ-011로 보장됨; 지연 수치 최적화는 정확성 밖. | 사용자가 지연 수치 목표를 제시할 때 |
| skill prose의 전체 backend 설명 재작성 | 바이너리 기본값 역전으로 동작은 정합화됨; prose 전면 개정은 선택. | 문서 일관성 작업이 별도로 요청될 때 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC가 단일 cohesive change(세 structured 진입점의 backend 기본값 역전 + 진짜 pane backend + pane 신뢰성 경화 + best-effort fallback)로 Outcome Lock을 닫는다. 신뢰성 그룹(REQ-009~013)은 동일 pane backend의 내부 동작이라 분리하면 스캐폴드만 남으므로 같은 SPEC에 둔다. 독립 배포 경계·migration sequencing·보안 경계 분리 사유 없음. 태스크 ~13개/신규 소스 파일 ~4개 + 수정 6개로 25태스크·40파일 임계 미만이다. | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `internal/cli/orchestra.go:86` runOrchestraCommand, `:162-164` DetectTerminal/interactive | existing | Read 확인 |
| `pkg/orchestra/runner.go:25` pane 분기 가드 (`!cfg.SubprocessMode && cfg.Terminal != nil && cfg.Terminal.Name() != "plain"`) | existing | Read 확인 — T2b에서 `paneCapable(...)` 호출로 치환 |
| `pkg/orchestra/backend.go:10` ExecutionBackend, `:37` PaneBackend.Execute→runProvider, `:49-54` SelectBackend (`SubprocessMode || Terminal == nil`만 검사; non-nil `"plain"` 미검사 = F-001) | existing | Read 확인 |
| `pkg/orchestra/provider_runner.go:23` runProvider, `:27-29` `-p` 주입 | existing | Read 확인 |
| `pkg/orchestra/subprocess_runner.go:105-107` schema flag | existing | Read 확인 |
| `internal/cli/orchestra_brainstorm.go:60` `--subprocess` 기본 true | existing | Read 확인 |
| `internal/cli/orchestra_file_cmds.go:37,79` review/secure (SubprocessMode 미전달) | existing | Read 확인 |
| `internal/cli/spec_review_runtime.go:14` specReviewBackendFactory hardcode | existing | Read 확인 |
| `internal/cli/spec_review_structured.go:19,36,57` runStructuredSpecReviewOrchestra / backend.Execute | existing | Read 확인 |
| `internal/cli/spec_review.go` / `spec_review_loop.go:53` OrchestraConfig (Terminal 미설정) | existing | Read 확인 — DetectTerminal 호출 없음 |
| `internal/cli/orchestra_run.go:130-135,138` SubprocessMode/SelectBackend(버려짐)/backend factory | existing | Read 확인 |
| `internal/cli/orchestra_run_runtime.go:8-9` hardcode factory/pipeline | existing | Read 확인 |
| `pkg/orchestra/pipeline.go:17-18` SubprocessPipelineConfig.Backend (ExecutionBackend) | existing | Read 확인 |
| `pkg/orchestra/interactive.go:19` RunInteractivePaneOrchestra, launch/sessionReady/sendPrompts | existing | Read 확인 |
| `pkg/orchestra/interactive_collect.go:17,43-64` waitAndCollectResults/ReadScreen/cleanScreenOutput | existing | Read 확인 |
| `pkg/orchestra/cc21_monitor.go:70-88` waitForCompletion monitor→poll fallback | existing | Read 확인 |
| `pkg/orchestra/cc21_monitor.go:16-48` resolveCompletionDetector/monitorCompletionDetector (HookMode off → non-hook degrade) | existing | Read 확인 |
| `pkg/orchestra/interactive_session_ready.go:21` SessionReadyPatterns, `:33` isSessionReady, `:49` startupTimeoutFor (claude 15s/gemini 10s/default 30s) | existing | Read 확인 |
| `pkg/orchestra/interactive.go:66-69` launch→waitForSessionReady→sendPrompts 순서, `:147` waitForSessionReady, `:191` pollUntilSessionReady, `:216` sendPrompts | existing | Read 확인 |
| `pkg/orchestra/completion_file_ipc.go:23` FileIPCDetector.WaitForCompletion (SECONDARY, hook IPC); `pkg/orchestra/hook_signal.go:36` NewHookSession (auto init 의존, SPEC-ORCH-007 R5/R6) | existing | Read 확인 |
| `pkg/orchestra/pane_runner.go:139` collectPaneResults (TimedOut 채움 — 결정적 실패 선례) | existing | Read 확인 |
| `pkg/orchestra/screen_sanitizer.go:42` SanitizeScreenOutput; `interactive_detect.go:233` CleanScreenForCrossPollination | existing | Read 확인 |
| `pkg/orchestra/output_parser.go:46,96-100` ParseReviewer/unmarshal(extractJSON, prose-tolerant) | existing | Read 확인 |
| `content/skills/idea.md:188` `auto orchestra brainstorm ... --no-detach` (`--subprocess=false` 미전달) | existing | rg 확인 — source of truth; `.claude/.codex/.gemini/.opencode`는 generated |
| commit `ca7f026` brainstorm 기본 subprocess 전환 (Ref SPEC-ORCH-019) | existing | git log 컨텍스트 |
| `internal/cli/orchestra_helpers.go:145-147` provider fallback registry (claude/codex/gemini Binary·Args·PaneArgs·PromptViaArgs·SchemaFlag) | existing | Read 확인 — gemini `Binary:"agy",Args:["--print"],PromptViaArgs:false`(버그 형태) |
| `autopus.yaml:31-34` review_gate.providers `[claude, gemini]`; `:71-85` orchestra.providers(codex/gemini 블록); `:89-98` 커맨드별 `[claude, codex, gemini]` | existing | Read 확인 — review_gate에 codex 누락(F=무응답) |
| `agy --print`/`-p`/`--prompt` = 프롬프트를 값으로 받는 플래그, `-i`/`--prompt-interactive` = 대화형 | existing(외부 CLI) | `agy --help` + 라이브 재현 확인 |
| `codex exec [PROMPT]` stdin/인자, `--output-schema`/`-o`/`-m`/`-s` 유효, `--full-auto`=deprecated alias | existing(외부 CLI) | `codex exec --help` + 라이브 확인 |
| `internal/cli/doctor_provider_smoke.go:83` runProviderTransportSmoke, `:112` providerSmokePrompt, `:116` classifyProviderSmokeResult | existing | Read 확인 — 운영 통합 스모크 검증 경로 |
| `pkg/orchestra/subprocess_codex.go:10` attachCodexLastMessageCapture, `codex_last_message.go:9` applyCodexLastMessageOutput | existing | Read 확인 — codex last-message 캡처(미변경) |
| pane `ExecutionBackend` 구현 | `[NEW] pkg/orchestra/pane_backend.go` | planned addition |
| 공유 pane-capability 술어 (runner.go 가드 + SelectBackend 공통 소비) | `[NEW] paneCapable` in `pkg/orchestra` (e.g. `pane_capable.go` 또는 `backend.go`) | planned addition |
| pane backend 수집 분할(300줄 초과 시) | `[NEW] pkg/orchestra/pane_backend_collect.go` | planned addition |
| structured 경로 terminal 주입 + 선택 배선 헬퍼 | `[NEW] internal/cli/orchestra_terminal.go` | planned addition |
| truth table oracle 테스트 | `[NEW] pkg/orchestra/pane_backend_test.go`, `[NEW] internal/cli/orchestra_terminal_test.go` | planned addition |
| best-effort `-p` fallback + actionable 에러 구성 (REQ-005/REQ-013) | `[NEW] pkg/orchestra/pane_fallback.go` (또는 backend 파일 내 함수) | planned addition |
| pane 신뢰성 oracle/behavioral 테스트 (S10~S14) | `[NEW] pkg/orchestra/pane_backend_reliability_test.go` | planned addition |

## Reviewer Brief

- **Intended scope**: 세 structured 진입점의 실행 backend 기본값을 pane-first로 역전 + 진짜 pane `ExecutionBackend` 추가 + **pane 신뢰성 경화(세션 ready 게이팅·bounded 완료 감지·no-hang·hook degrade, REQ-009~012)** + best-effort `-p` fallback + pane+`-p` 동시 실패 시 actionable 에러(REQ-005/REQ-013) + 실행 경로 기록 + spec review에 terminal 감지 주입. 신뢰성은 기존 메커니즘 재사용(재발명 없음).
- **Explicit non-goals (리뷰어가 새 scope로 확장하지 말 것)**: plan/review/secure 경로(이미 pane), provider-API 백엔드(Evolution Idea), JSON/structured 계약 변경, 새 strategy, brainstorm 프롬프트 변경, pane 지연 실측 벤치마크, **새 완료감지·세션ready·hook IPC 메커니즘 작성(기존 재사용만)**, `auto init` hook 설치.
- **Self-verified**: Traceability Matrix(REQ-001~013↔task↔AC↔INV), Semantic Invariant Inventory(INV-001~012), oracle/behavioral acceptance S1~S14(S6 best-effort fallback 단언, S8a/b/c parse 단언, S9 술어 합의, S10 ready-게이팅, S11 monitor→poll, S12 no-hang 결정적 timeout, S13 hook degrade, S14 both-failed actionable 에러), 신뢰성 메커니즘이 기존 심볼 재사용임을 Reference Discipline에서 확인, existing/[NEW] 경계.
- **Reviewer should focus on**: correctness(공유 술어 `paneCapable`의 6행 정확성 + F-001 plain + F-002 양쪽 소비), pane 신뢰성(세션 ready 전 미전송, `waitForCompletion` 재사용으로 monitor→poll bounded, timeout 시 결정적 no-hang, hook 미설치 degrade가 기존 `resolveCompletionDetector` 동작과 일치하는가), 재발명 금지(새 backend가 새 감지기를 만들지 않고 기존 심볼만 재사용하는가), fallback 의미(best-effort `-p` + both-failed actionable 에러가 raw API 에러를 숨기지 않는가), regression risk(legacy `RunInteractivePaneOrchestra` 멀티라운드 경로·plan/review/secure 미변경, 새 backend/fallback 파일 300줄 한계), Completion Debt(없음 — 반증 시에만).

## Revision 1 closure

- F-001 | correctness | `SelectBackend`가 non-nil `"plain"`을 검사하지 않던 결함을 SPEC scope로 편입 — `SelectBackend`가 공유 술어를 소비하고 plain→subprocess가 되도록 plan task로 명시, 잘못된 "기존대로" 서술 정정 | spec.md REQ-002/REQ-003·생성파일(backend.go 항목), plan.md T2, research.md Architecture 2 `backend.go:49-54`·INV-001·Reference Discipline
- F-002 | correctness | "단일 SelectBackend 호출" 단언을 "공유 pane-capability 술어 `paneCapable`를 `runner.go` 가드와 `SelectBackend`가 둘 다 소비"로 재구성, 두 실행 모델(brainstorm=RunOrchestra, structured=ExecutionBackend) 차이를 명시 | spec.md REQ-007, plan.md T2b·Visual Planning Brief flowchart, research.md 설계결정(REQ-007 subsection)·Architecture 2 두-메커니즘 노트
- F-006 | completeness | REQ-007/S9를 사후 backend-name 동치가 아니라 공유 술어 합의 검증으로 다시 씀(S9가 legacy 가드와 SelectBackend가 동일 `paneCapable` 결과에 합의함을 단언), 사후 동치의 한계를 research에 기록 | spec.md REQ-007·Traceability Matrix, acceptance.md S9, plan.md T6, research.md 설계결정
- F-002(Q-COMP-05 부속) | completeness | INV-003/REQ-004의 parseable invariant를 oracle이 실제 단언 가능한 role(reviewer/debater_r1/judge)로 좁히고 각각 S8a/S8b/S8c oracle 추가; `debater_r2`는 role 고유 검증이 없어 transitive 커버로 명시(임의 확장 금지 준수) | spec.md REQ-004·Traceability Matrix, acceptance.md S8a/S8b/S8c, research.md INV-003·설계결정 feasibility

## Revision 2 closure

사용자 우선순위 재구성(subscription-first, pane 안정성 = 진짜 목표) 반영. 기존 8 REQ 유지, 신뢰성 그룹 추가.

- 우선순위 재구성 | motivation | `-p`=API 경로이고 다수 유저는 구독-only(인터랙티브 CLI만 가능)라 pane이 primary path임을 명시; Outcome Lock user-visible outcome을 "구독 세션으로 -p 없이 안정적 멀티프로바이더 실행"으로 강화 | spec.md 제목·목적·Outcome Boundary, research.md `## Subscription vs API 근거`·Outcome Lock
- REQ-009 (신규) | completeness | 세션 ready 전 prompt 미전송(입력 유실 방지), `isSessionReady`/`SessionReadyPatterns`/`startupTimeoutFor` 재사용 | spec.md REQ-009·Matrix, plan.md T8a, acceptance.md S10, research.md INV-008·신뢰성 인벤토리/설계
- REQ-010 (신규) | completeness | 완료 감지 hook/monitor→poll 계층 + bounded, `waitForCompletion` 재사용(재발명 금지) | spec.md REQ-010·Matrix, plan.md T8b, acceptance.md S11, research.md INV-009
- REQ-011 (신규) | correctness | 완료 timeout 시 결정적 timed-out 결과, 무한 hang/garbage 금지 | spec.md REQ-011·Matrix, plan.md T8b, acceptance.md S12, research.md INV-010
- REQ-012 (신규) | completeness | hook 미설치 시 `resolveCompletionDetector` 비-hook 경로로 graceful degrade(SPEC-ORCH-007 R5/R6 의존 명시) | spec.md REQ-012·Matrix·Related SPECs, plan.md T8b, acceptance.md S13, research.md INV-011
- REQ-005 정정 + REQ-013 (신규) | correctness | fallback을 "무조건 -p"에서 "best-effort -p → 실패 시 actionable 에러(구독 현실 비은폐)"로 정정; pane+`-p` 동시 실패 시 raw API 에러 금지 | spec.md REQ-005 재작성·REQ-013·Matrix, plan.md T9·flowchart, acceptance.md S6 정정·S14, research.md INV-004 정정·INV-012·설계
- Visual Planning Brief | completeness | flowchart에 subscription→pane(primary)/API→-p(best-effort)/both-fail→actionable error + pane 내부 신뢰성 스테이지(ready 게이트·bounded 완료·timeout·fallback) 반영 | plan.md `## Visual Planning Brief`
- 재발명 금지 가드 | feasibility | 모든 신뢰성 REQ가 기존 심볼 재사용임을 Reference Discipline·설계·non-goals에 명시(새 감지기/세션ready/hook IPC 작성 금지) | research.md Reference Discipline·non-goals, spec.md non-goals
- Completion Debt | completeness | 여전히 None(provider-API 백엔드와 지연 실측 튜닝만 Evolution). 신뢰성은 Primary SPEC가 닫음 | research.md Completion Debt·Evolution Ideas

## Revision 3 closure

사용자 명시("gemini는 왜 실패, codex는 왜 무응답 — 같이 고치자") 반영. 기존 13 REQ 유지, provider 호출 정합성 그룹(REQ-014~016) 추가. 두 근본 원인 모두 라이브 재현(codex 0.135.0, agy 1.0.0).

- gemini 실패 진단 | correctness | `printf 'x' \| agy --print` → `flag needs an argument: -print`(exit 2) 재현 — agy의 `--print`/`-p`/`--prompt`는 프롬프트를 값으로 받는 string 플래그인데 `orchestra_helpers.go:147` `Args:["--print"],PromptViaArgs:false`가 stdin으로 보내 무값 argv를 만듦 | research.md `## Provider 호출 정합성 진단`, spec.md REQ-014
- REQ-014 (신규) | correctness | subprocess argv가 프롬프트를 올바른 형태로 전달: gemini=`--print`/`-p` 값, codex=`exec`+`--output-schema` | spec.md REQ-014·Matrix, plan.md T11/T12, acceptance.md S15/S16/S18, research.md INV-013/INV-015
- REQ-015 (신규) | correctness | pane 모드 argv는 대화형 형태: gemini는 `--print` 제거(대화형 또는 `-i`/`--prompt-interactive`), codex는 `exec` 없이 TUI | spec.md REQ-015·Matrix, plan.md T11/T12, acceptance.md S17/S19, research.md INV-014
- REQ-016 (신규) | completeness | `review_gate.providers:[claude,gemini]`에 codex 누락이 무응답 근본 원인 — orchestra 집합과 일관되게 codex 추가, codex `--output-schema`로 structured 응답 | spec.md REQ-016·Matrix, plan.md T13, acceptance.md S18/S20, research.md INV-016
- codex `--full-auto` 검증 | feasibility | 라이브 확인: `--full-auto`는 deprecated alias(경고 후 동작) — 무응답 원인 아님. plan T12에서 `--sandbox workspace-write`로 표준화(경고 제거) | research.md 진단 섹션, plan.md T12
- source of truth | feasibility | provider 설정 SoT=`autopus.yaml orchestra.providers`, `orchestra_helpers.go buildProviderConfigs`는 config 부재 시 fallback. gemini는 config에 `args` 없어 fallback이 버그를 지배 → 정정은 양쪽(config+fallback) | research.md 진단 섹션·Reference Discipline

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 4 | files: research.md, spec.md, plan.md | reason: Rev 2 신뢰성 심볼(`interactive_session_ready.go:21/33/49`, `interactive.go:66-69/147/191/216`, `cc21_monitor.go:16-48/70-88`, `completion_file_ipc.go:23`, `hook_signal.go:36`, `pane_runner.go:139`)에 더해 Rev 3 provider 정합성 참조를 라이브 재현·검증: `orchestra_helpers.go:145-147` registry, `autopus.yaml:31-34/71-98`, `agy --print` flag 에러 재현, `codex exec --help` 플래그, `doctor_provider_smoke.go:83/112/116`. 모든 신규 REQ가 실제 심볼에 매핑됨(재발명 없음).
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, plan.md, research.md | reason: 신규 파일/함수는 `[NEW]`로 표기하고 정합성 PASS 근거에서 제외함.
- Q-CORR-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: acceptance 예시가 bare Given/When/Then이며 parser가 받는 형식. Priority/EARS type은 메타 라인 분리.
- Q-CORR-04 | status: PASS | attempt: 2 | files: research.md, spec.md, plan.md | reason: Rev 1에서 `backend.go:49-54`의 실제 조건(plain 미검사=F-001)과 `runner.go:25`(predicate 치환 대상)을 정확히 기술하고, 신규 `paneCapable` 술어를 `[NEW]`로 분리함. existing/[NEW] 경계 유지.
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 네 파일이 각자 역할(요구/계획/검증/근거)을 갖고 상호 보완함.
- Q-COMP-02 | status: PASS | attempt: 4 | files: spec.md, acceptance.md, plan.md | reason: Rev 2의 REQ-009~013↔T8a/T8b/T9↔S10~S14↔INV-008~012에 더해 Rev 3 REQ-014~016↔T11/T12/T13↔S15~S20↔INV-013~016을 Traceability Matrix(spec.md:158-165)·INV Inventory(research.md:130-133)에 추가. 16 REQ 전부 task/AC/INV로 추적(누락 없음).
- Q-COMP-03 | status: PASS | attempt: 2 | files: spec.md | reason: Rev 2 신규 REQ-009~013도 Event-driven EARS + WHEN/WHERE 조건 + Observability(no-prompt-before-ready, detector path, returns-within-timeout, non-hook detector, error string 내용)를 명시함.
- Q-COMP-04 | status: PASS | attempt: 2 | files: research.md, spec.md | reason: Rev 2에서 Outcome Lock의 user-visible outcome을 '구독 세션으로 -p 없이 안정적 멀티프로바이더 실행 + 둘 다 실패 시 actionable 에러'로 강화하고 REQ-009~013 + S10~S14로 닫음. Completion Debt 여전히 None.
- Q-COMP-05 | status: PASS | attempt: 4 | files: research.md, spec.md, plan.md, acceptance.md | reason: Rev 2 신규 INV-008~012를 각각 단언 가능한 behavioral/oracle(S10 ready-게이팅, S11 monitor→poll, S12 bounded no-hang, S13 non-hook detector, S14 error string 내용)로 매핑. 임의 확장 없이 실제 관측 가능한 것만 단언(예: 지연 수치는 Evolution으로 분리).
- Q-COMP-06 | status: PASS | attempt: 3 | files: spec.md, research.md | reason: Rev 2에서 Reviewer Brief의 intended scope/non-goals/self-verified/focus를 신뢰성·재발명금지·fallback 의미로 갱신하고 Traceability Matrix를 13 REQ로 동기화.
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(None)와 Evolution Ideas(provider-API 등)를 분리; optional idea에 SPEC/task/AC ID를 붙이지 않음.
- Q-FEAS-01 | status: PASS | attempt: 2 | files: plan.md, research.md | reason: Rev 2 신뢰성도 런타임 변경이며 기존 심볼 재사용으로 명시. hook 설치는 SPEC-ORCH-007/`auto init` 책임으로 분리(non-goal)하여 구현 레이어 혼동 없음.
- Q-FEAS-02 | status: PASS | attempt: 2 | files: plan.md, spec.md | reason: Rev 2 신뢰성 헬퍼(`waitForCompletion`, `pollUntilSessionReady`, `resolveCompletionDetector`)가 모두 `pkg/orchestra`에 존재하고 단일 provider 단위 재사용이 가능함을 확인. 새 파일(pane_fallback.go/reliability_test.go)은 module 경계 내.
- Q-FEAS-03 | status: PASS | attempt: 2 | files: plan.md | reason: 신뢰성 behavioral 테스트는 기존 mock terminal(ReadScreen 제어, `pane_mock_test.go`/`interactive_*_test.go` 패턴) 재사용으로 timeout/ready/degrade를 결정적으로 검증 가능; 비례적.
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ 문구에 should/might/could 등 모호어 없음.
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority(Must)와 EARS type을 별도 축으로 표기.
- Q-STYLE-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: 문장 완결, acceptance는 bare Given/When/Then/And.
- Q-SEC-01 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: pane screen-scraping이 untrusted provider output을 다루는 trust boundary임을 명시하고 REQ-008+INV-006으로 sanitize 완화를 요구함.
- Q-SEC-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: secret/credential 예시 노출 없음; pane/log에 비밀 미노출 요구. provider name 경로 traversal은 기존 sanitizeProviderName(pane_runner.go:106)로 처리됨을 참조.
- Q-SEC-03 | status: PASS | attempt: 2 | files: research.md, spec.md | reason: actionable 에러 메시지는 복구 지시만 담고 secret/token/raw API 응답 본문을 노출하지 않도록 설계(REQ-013 observability는 원인+복구 문구만). 실행 경로 기록은 기존 Receipt/CollectionMode 재사용.
- Q-COH-01 | status: PASS | attempt: 2 | files: spec.md, research.md | reason: 신뢰성 그룹은 '동일 pane backend가 안정적으로 동작'이라는 단일 문제의 일부이며 별도 SPEC로 쪼개면 스캐폴드만 남으므로 같은 SPEC 유지(Sibling Decision=none). cohesive.
- Q-COH-02 | status: PASS | attempt: 1 | files: plan.md, research.md | reason: 후속 런타임 작업을 Primary SPEC에 포함; optional은 Evolution Ideas로만, 자동 follow-up SPEC 예약 없음.
- Q-COH-03 | status: N/A | attempt: 1 | files: research.md | reason: sibling SPEC 없음(Sibling SPEC Decision=none). 재귀 분할 미적용.
