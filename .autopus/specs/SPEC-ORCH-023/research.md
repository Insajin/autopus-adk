# SPEC-ORCH-023 리서치: 오케스트라·learn·worker 런타임 견고성 하드닝

## 기존 코드 분석 (코드 실측 완료)

모든 참조는 작성 시점에 Read/rg로 직접 확인했다. 라인 번호는 변동 가능하므로 심볼명을 1차 기준으로 삼는다.

### F1 — cc21 완료 감지 에러 무시 (`pkg/orchestra/cc21_monitor.go`)
- `waitForCompletion`이 `resolved.detector.WaitForCompletion(...)`을 3곳에서 호출하며 모두 에러를 `_`로 버린다: non-event-driven 경로(현재 라인 89), event-driven monitor 경로(현재 라인 96), polling fallback(현재 라인 103).
- event-driven 경로는 `if completed || ctx.Err() != nil { return completed }`(라인 97)로 ctx 취소만 부분 처리하고, I/O 실패와 미완료를 구분하지 못한다. non-event-driven 경로(라인 88-90)는 ctx 에러조차 보지 않는다.
- 결과: detector의 ctx 취소/I/O 실패가 "미완료"와 동일하게 조용히 흡수되어 관측 불가.

### F2 — learn 스토어 데이터 경합 (`pkg/learn/store.go`, `pkg/learn/prune.go`)
- `Store`는 `mu sync.Mutex`를 갖지만(store.go:18) **공개 `Append`(라인 33-49)는 잠그지 않는다**. `os.OpenFile(O_APPEND)`로 추가한다.
- `AppendAtomic`(라인 103-125)만 `s.mu`를 잡고 `NextID`+`Append`를 호출한다.
- `UpdateReuseCount`(라인 128-163)는 **뮤텍스 없이** `Read()` 스냅샷 → `os.Create`(truncate) → 전체 재작성한다.
- `Prune`(prune.go:7-30)도 free 함수로 **뮤텍스 없이** `store.Read()` → `rewriteStore`(rewrite.go:10) 한다.
- 경합: `UpdateReuseCount`/`Prune`의 read→truncate→rewrite 사이에 `AppendAtomic`/`Append`가 항목을 추가하면, truncate+rewrite가 그 항목을 덮어써 **유실**된다. mutex가 양쪽을 상호배제하지 않기 때문.
- 호출자(비-test): `AppendAtomic`←`record.go:14`, `Prune`←`internal/cli/learn_prune.go:31`. 공개 `Append`의 외부 직접 호출자는 현재 `AppendAtomic` 내부뿐(store.go:124).

### F3 — 프로바이더 패턴 하드코딩 (부분 reframe, 아래 "Finding 검증·Reframe" 참조)
- fast-fail 문구: `pkg/orchestra/provider_runner.go`의 `detectProviderFastFail`(라인 245-258, 프롬프트가 가리킨 222-235는 `fastFailBuffer` struct/`Write`였음). 4개 substring switch가 하드코딩. **오버라이드 경로 없음(진짜 gap).** 호출처는 `fastFailBuffer.Write`(라인 233)이고 버퍼는 provider context를 아는 `runProviderWithProgress`에서 생성됨.
- hook 가용 provider: `pkg/orchestra/hook_signal.go`의 `defaultHookProviders`(라인 27-31, claude/gemini/codex). **이미 `SetHookProviders`(라인 175) 오버라이드 메서드 존재** + `@AX:NOTE`(라인 26)가 하드코딩임을 명시.
- prompt 마커: `pkg/orchestra/interactive_detect.go`의 `defaultPromptPatterns`(라인 31-39). **per-provider 오버라이드는 이미 `DefaultCompletionPatterns()`(types.go:152)·`ProviderConfig.WorkingPatterns`(types.go:42)로 존재**하며 `isPromptVisible`(라인 119-143)가 per-provider 패턴 우선 후 `defaultPromptPatterns` fallback. 단 no-provider 경로(`isPromptLine`→`filterPromptLines`→`cleanScreenOutput`)는 글로벌 기본값만 사용.
- 설정 패턴 선례: `ProviderConfig`(types.go:32-48)는 이미 `WorkingPatterns []string`, `ResultReadyPatterns []string` 등 per-provider 패턴 필드를 보유 → 동일 패턴으로 확장 가능.

### F4 — 디베이트/judge 프롬프트 델리미터 미중화 (`templates/shared/orchestra-debater-r2.md.tmpl`, `orchestra-judge.md.tmpl`, `pkg/orchestra/crosspolinate.go`)
- debater-r2 템플릿: `{{range .PreviousResults}}### {{.Alias}}:` 다음 줄에 `{{.Output}}`를 **그대로** 삽입. 펜스 없음.
- judge 템플릿: `{{range .AllResults}}### {{.Alias}}` 아래 `{{.Round1}}`/`{{.Round2}}`를 **그대로** 삽입.
- `crosspolinate.go`의 `Anonymize`(라인 35-54)·`AnonymizeForJudge`(라인 58-77)는 ICE 점수 제거+토큰 cap만 하고 경계 sentinel을 두지 않는다 → 참가자가 `### Debater Z:` 위조 헤더나 `## Judging Instructions` 위장 블록을 출력하면 다음 라운드/judge가 구조·지시로 오인 가능(Medium prompt injection).
- 재사용 패턴: `pkg/orchestra/pane_shell.go:51` `uniqueHeredocDelimiter(base, content, randomSuffix)`는 content에 base가 있으면 random suffix를 붙여 충돌 회피. `pkg/orchestra/pane_runner.go:275` `randomHex()`(8 hex)와 `const sentinel = "__AUTOPUS_DONE__"`(라인 16). PromptData(`prompt_data.go`)·`prompt_builder.go`(BuildDebaterR2/BuildJudge)·`judge_builder.go:21`(AllResults)·`pipeline.go:75-76`(PreviousResults)가 데이터 주입 지점.

### F5 — ReliabilityStore 기록 실패 (reframe: error swallow 아님, `pkg/orchestra/reliability_bundle.go`)
- `recordPrompt`(라인 141-146)은 **error가 아니라 string(영수증 경로)** 을 반환한다. 실패 시 빈 문자열("")을 돌려준다.
- 체인: `record*` → `writeJSON`(라인 207-226). `writeJSON`은 marshal 실패/디렉토리 비가용/WriteFile 실패(1회 재시도 포함) 시 **조용히 ""** 반환. 즉 영속화 실패의 유일 신호가 빈 경로이고, 호출자는 `interactive_debate_round.go`에서 `_ =`로 버린다(라인 91/101/111/117/172/182/203).
- 프롬프트가 가리킨 라인 84는 `_ = SendRoundEnvToPane(...)`(round_signal.go:46, error 반환)로 **ReliabilityStore가 아님**. 별개의 silent 실패.
- 따라서 "error 무시"가 아니라 "writeJSON이 영속화 실패를 ""로 평탄화 → 관측 불가"가 정확한 결함. 올바른 수정 위치는 chokepoint `writeJSON`.

### F6 — unsigned 제어평면 무경고 통과 (`pkg/worker/controlplane/controlplane.go`)
- `signingSecret()`(라인 129-131)이 빈 문자열이면 `ValidateSecurityPolicySignature`(라인 24-26), `VerifyCachedPolicyFile`(라인 56-58), `ValidateControlPlaneSignature`(라인 72-74)가 모두 **조용히 `nil`(skip)** 반환.
- `@AX:ANCHOR`(라인 16)가 env-driven on/off가 의도된 fail-open임을 명시 → 정책 자체는 by-design. 그러나 서명 검증 비활성으로 진입하는 경고가 **0** → 운영자가 인지 불가.

### F7 — surface tracker 공유 /tmp 경로 (`pkg/orchestra/surface_tracker.go`)
- `surfaceTrackerBase = filepath.Join(os.TempDir(), "autopus", "surfaces")`(라인 28). PID 기반 예측 가능 파일명(`{pid}.surfaces`, 라인 34-36).
- `trackSurface`(라인 52-65)의 `os.MkdirAll(base, 0o700)`은 디렉토리가 이미 있으면 소유권/권한을 **검증·교정하지 않는다**(MkdirAll no-op). 공유 TempDir에 선점된 world-writable 디렉토리를 그대로 사용 가능.
- `ReapOrphanSurfaces`(라인 94-117)는 tracking 파일의 ref를 **형식 검증 없이** `term.Close(ctx, ref)`(라인 113)로 전달. cmux `Close`(cmux.go:160)는 `execCommand("cmux", "close-surface", "--surface", name)`로 shell 없이 argv 전달하나, ref가 `-`로 시작하면 **argument injection** 여지(`isCmuxRef` gate가 일부 완화).
- 안전 경로 선례: `reliability_bundle.go`의 `reliabilityRuntimeRoot()`(라인 65-78)는 `os.UserHomeDir()`→`~/.autopus/runtime/orchestra`(0700), 미가용 시 `os.TempDir()/autopus-runtime/orchestra` fallback.

## Outcome Lock

- User-visible outcome: 완료 감지·learn 스토어·프롬프트 경계의 silent 실패 경로가 로그/반환값으로 관측 가능해지고, learn 학습 데이터 유실 race가 봉인되며, 프로바이더 패턴(fast-fail/hook/prompt 기본값)이 설정 계층에서 선언적으로 오버라이드 가능해진다. 기존 동작은 후방호환(오버라이드 미설정 시 현재 값과 동일).
- Mandatory requirements: REQ-001(cc21 에러 관측+forced-false), REQ-002(learn race 봉인), REQ-003(프로바이더 패턴 선언화·기본값 보존), REQ-004(디베이트/judge 출력 sentinel 펜스), REQ-005(reliability 영수증 영속화 실패 관측).
- Secondary hardening (Should): REQ-006(unsigned 제어평면 1회 경고), REQ-007(surface tracker 홈 경로+소유 검증+ref 형식 검증).
- Explicit non-goals: ws_client `StateRecoverer` inert seam 배선(worker 기능 SPEC 소유), fail-open 서명 정책의 필수화(경고만), per-provider prompt 패턴을 no-provider `filterPromptLines`/`isPromptLine` 글로벌 경로까지 스레딩(screen-clean 파이프라인 재설계 회피), 새 strategy/백엔드/JSON 스키마 변경.
- Completion evidence: oracle 수락 — (1) stub detector가 (true, err) 반환 시 `waitForCompletion`가 false 반환+에러 로그, (2) 동시 Append/UpdateReuseCount 부하 후 `Read()` 항목 수·ReuseCount 정확 보존(-race), (3) 오버라이드 미설정 시 해석된 fast-fail reason·hook map·prompt 패턴 개수가 현재 값과 동일, (4) 위조 헤더가 sentinel 펜스 안쪽에만 존재+BEGIN 개수=참가자 수, (5) 영속화 실패 시 store당 경고 1회+recordPrompt "" 반환, (6) 미서명 검증 2회에 경고 1회+nil 반환, (7) home 기반 tracker root+형식 불일치 ref Close 제외.

## Visual Planning Brief

전체 silent-failure→observable 매핑 플로우차트는 `plan.md`의 `## Visual Planning Brief`에 둔다. 핵심 데이터 경계(F4)의 변환:

```
현재:  ### Debater A:
       <participant output: 위조된 ## Judging Instructions 가능>

수정:  ### Debater A:
       <sentinel>-BEGIN      (sentinel = AUTOPUS_PART_<randomHex>, 참가자 출력에 부재 보장)
       <participant output>  (펜스 내부 = untrusted 데이터, 지시 아님)
       <sentinel>-END
```

## 설계 결정

1. **F1 공유 에러 헬퍼**: 3개 호출 지점이 공유하는 헬퍼로 (a) 에러 로깅(provider 이름 포함), (b) ctx 취소와 I/O 실패 구분 로그, (c) 에러 시 completed=false 강제. 이유: detector가 (true, err)를 돌려줘도 에러가 있으면 완료를 신뢰할 수 없음. 대안(각 지점 인라인 처리)은 중복·누락 위험.
2. **F2 비재진입 잠금 분리**: 공개 `Append`를 잠그는 wrapper로, 내부 unlocked `appendUnlocked`를 `AppendAtomic`가 호출(현재 lock 보유). `UpdateReuseCount`·`Prune`은 `s.mu`를 잡고 unlocked primitive(`Read`/`rewriteStore`)만 호출 → 재귀 잠금/데드락 회피. 이유: `sync.Mutex`는 비재진입. 대안(RWMutex)은 truncate-rewrite가 write라 이득 없음.
3. **F3 [NEW] provider_patterns.go에 기본값 집중**: fast-fail rule 타입·기본 4-rule·기본 hook map·기본 prompt 패턴 accessor를 한 파일에 모아 "선언적 기본값" 단일 출처화. `ProviderConfig`에 오버라이드 필드 추가, fastFailBuffer에 rules 주입. 이유: provider_runner.go(294줄)·interactive_detect.go(277줄) 300줄 한도 보호 + 기본값/오버라이드 분리. 동작 불변 불변식(오버라이드 미설정=현재 값)으로 회귀 방지.
4. **F4 라운드별 랜덤 sentinel 펜스**: `uniqueHeredocDelimiter` 사상을 차용해 모든 참가자 출력에 부재가 보장되는 sentinel을 생성, `PromptData.Sentinel`로 주입하고 템플릿이 BEGIN/END 펜스+untrusted 안내를 렌더. 이유: 라운드 간 prompt injection 차단을 "비활성 데이터 경계"로 고정. promptlayer manifest(`buildPromptLayers`)에서 참가자 출력은 ephemeral/GroupUserRequest task 레이어로 유지(stable/snapshot 아님) → 캐시 오염 없음.
5. **F5 chokepoint 경고**: `reliabilityStore`에 `degraded bool` 추가, `writeJSON`이 최종 실패로 ""를 반환하기 직전 store당 1회 경고. 이유: 7개 호출처를 모두 고치는 대신 단일 실패 지점에서 관측화. 반환 계약("") 불변 → 후방호환. SendRoundEnvToPane(F5 부수)는 그 자리에 경고 추가.
6. **F6 sync.Once 경고**: 패키지 레벨 `sync.Once`로 미서명 검증 진입 시 프로세스당 1회 경고. 반환값(nil)·정책 불변. 테스트 격리를 위해 once-guard를 test-reset 가능하게 노출.
7. **F7 홈 경로+검증**: `surfaceTrackerRoot()`를 `reliabilityRuntimeRoot()` 패턴으로 신설(홈 우선, TempDir fallback). `trackSurface`는 대상 dir이 현재 uid 소유·0700(group/other 비트 0)인지 stat 검증 후에만 기록(불일치 시 best-effort skip). `ReapOrphanSurfaces`는 ref 형식 정규식 통과분만 `Close`에 전달, 레거시 `/tmp/autopus/surfaces`는 생성 없이 읽기 전용 reap(업그레이드 경계 orphan 보존).

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go stdlib only (`sync`, `log`, `crypto/rand`, `os`, `syscall`, `text/template`) | 기존 `go.mod` 유지 (신규 의존성 0) | `autopus-adk/go.mod` (기존 매니페스트) | 2026-06-12 | 외부 race/lock 라이브러리 — stdlib `sync.Mutex`로 충분, 의존성 추가 불필요 |

신규 프레임워크/런타임/의존성 도입 없음. brownfield 후방호환 제약: 기존 major 버전·테스트 green 유지.

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "cc21 완료 감지 에러 3중 무시 → 에러 시 completed=false 강제" | error-to-state mapping (control-flow) | `waitForCompletion` 반환 bool + 에러/취소 로그 | S1 |
| INV-002 | "UpdateReuseCount/Prune의 read→truncate→rewrite와 동시 Append 충돌 시 항목 유실 → 항목 수 보존" | concurrency / data-integrity (count preservation) | `Store.Read()` 항목 수 + 갱신 ReuseCount | S2 |
| INV-003 | "현재 하드코딩 값을 기본값으로 보존(동작 불변)" | behavior-preservation / default-equivalence (comparison) | resolved fast-fail reason·hook map·prompt 패턴 집합 | S3, S4 |
| INV-004 | "프로바이더 출력 원문이 {{.Output}}로 그대로 삽입 → 랜덤 sentinel로 감싸 비활성 데이터 경계 고정" | boundary-fencing / injection-containment (parser/structure) | 렌더된 Round2/judge 프롬프트 텍스트 | S5, S6 |
| INV-005 | "ReliabilityStore 기록 실패(빈 경로) 무관측 → 최소 경고 로그" | observability / failure-signal (count) | 경고 로그 횟수 + recordPrompt 반환값 | S7 |
| INV-006 | "unsigned 모드 진입 시 1회 경고 로그(정책 불변)" | observability / once-warning (count) | 경고 로그 횟수 + 검증 반환값(nil) | S8 |
| INV-007 | "홈 경로 우선 + 디렉토리 소유/0700 검증 + Close 전 ref 형식 검증" | path-safety + input-validation (filter/parser) | tracker 기록 위치 + Close에 전달된 ref 집합 | S9, S10 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| cc21 완료 감지 에러 관측+forced-false (F1) | Primary SPEC REQ-001/T1/S1 | covered |
| learn 스토어 유실 race 봉인 (F2) | Primary SPEC REQ-002/T2/S2 | covered |
| 프로바이더 패턴 선언화·기본값 보존 (F3) | Primary SPEC REQ-003/T3/S3·S4 | covered |
| 디베이트/judge 출력 sentinel 펜스 (F4) | Primary SPEC REQ-004/T4/S5·S6 | covered |
| reliability 영수증 영속화 실패 관측 (F5) | Primary SPEC REQ-005/T5/S7 | covered |
| unsigned 제어평면 1회 경고 (F6) | Primary SPEC REQ-006/T6/S8 | covered |
| surface tracker 경로·소유·ref 하드닝 (F7) | Primary SPEC REQ-007/T7/S9·S10 | covered |
| ws_client StateRecoverer 배선 | (없음) | non-goal (Evolution) |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | Primary SPEC가 Outcome Lock을 단일 cohesive 변경으로 닫음 |

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| `pkg/worker/a2a/ws_client.go:19` `StateRecoverer` inert seam 배선 | Outcome Lock 밖, worker 기능 SPEC 소유 | 사용자가 명시적으로 worker 복구 기능을 요청 |
| 서명 검증 필수화(strict mode, fail-closed 옵션) | 현재 fail-open은 의도된 by-design 정책 | 사용자가 서명 강제를 명시적으로 요청 |
| per-provider prompt 패턴을 `filterPromptLines`/`isPromptLine` 글로벌 경로까지 스레딩 | screen-clean 파이프라인 재설계 필요, 현재 per-provider 오버라이드는 CompletionPattern으로 이미 존재 | no-provider 경로의 오탐이 실제 문제로 보고됨 |
| 감사가 표시한 나머지 `_ =` ignored-error 지점(본 SPEC 범위 밖) 일괄 관측화 | Outcome Lock 미포함, 개별 위험 낮음 | 특정 지점이 실제 장애로 보고됨 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | 7개 결함이 모두 "런타임 silent 실패 관측화 + race 봉인 + 패턴 선언화"라는 단일 cohesive 하드닝 스토리에 수렴. 태스크 7개·소스 파일 ~17개로 sibling 임계(25 태스크 AND 40 파일) 미달. 보안 항목(F6/F7)도 경고/경로 하드닝 수준이라 별도 컴플라이언스 경계 불요 | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/orchestra/cc21_monitor.go::waitForCompletion` (라인 86-105) | existing | Read 확인, 에러 discard 3지점(89/96/103) |
| `pkg/learn/store.go::{Append,AppendAtomic,UpdateReuseCount}` + `mu` | existing | Read 확인, Append/UpdateReuseCount unlocked |
| `pkg/learn/prune.go::Prune`, `pkg/learn/rewrite.go::rewriteStore` | existing | Read/rg 확인, unlocked rewrite |
| `pkg/orchestra/provider_runner.go::detectProviderFastFail` (라인 245-258) | existing | Read 확인 (프롬프트 222-235는 fastFailBuffer였음) |
| `pkg/orchestra/hook_signal.go::{defaultHookProviders,SetHookProviders,HasHook}` | existing | Read 확인, SetHookProviders 오버라이드 존재 |
| `pkg/orchestra/interactive_detect.go::{defaultPromptPatterns,isPromptVisible,isPromptLine,filterPromptLines}` | existing | Read 확인 |
| `pkg/orchestra/types.go::{ProviderConfig,OrchestraConfig,DefaultCompletionPatterns}` | existing | Read 확인, WorkingPatterns/ResultReadyPatterns 선례 |
| `templates/shared/orchestra-debater-r2.md.tmpl`, `orchestra-judge.md.tmpl` | existing | cat 확인, `{{.Output}}`/`{{.Round1}}`/`{{.Round2}}` 무펜스 삽입 |
| `pkg/orchestra/crosspolinate.go::{Anonymize,AnonymizeForJudge}` | existing | Read 확인 |
| `pkg/orchestra/prompt_data.go::{PromptData,PreviousResult,JudgeResult}` | existing | Read 확인 |
| `pkg/orchestra/prompt_builder.go::{BuildDebaterR2,BuildJudge}`, `judge_builder.go:21`, `pipeline.go:75-76` | existing | rg 확인 (주입 지점) |
| `pkg/orchestra/pane_shell.go:51::uniqueHeredocDelimiter`, `pane_runner.go:275::randomHex` | existing | rg/Read 확인 (재사용 패턴) |
| `pkg/orchestra/reliability_bundle.go::{writeJSON,recordPrompt,reliabilityRuntimeRoot}` | existing | Read 확인, recordPrompt는 string 반환·writeJSON ""평탄화 |
| `pkg/orchestra/interactive_debate_round.go` (라인 84 SendRoundEnvToPane / 91·101·… recordPrompt) | existing | Read 확인 (라인 84는 ReliabilityStore 아님) |
| `pkg/orchestra/round_signal.go:46::SendRoundEnvToPane` | existing | rg 확인 (error 반환) |
| `pkg/worker/controlplane/controlplane.go::{signingSecret,ValidateSecurityPolicySignature,VerifyCachedPolicyFile,ValidateControlPlaneSignature}` | existing | Read 확인, fail-open nil skip |
| `pkg/orchestra/surface_tracker.go::{surfaceTrackerBase,trackSurface,ReapOrphanSurfaces}` | existing | Read 확인 |
| `pkg/terminal/cmux.go:160::Close`, `pkg/terminal/terminal.go:17::PaneID` | existing | Read/rg 확인 (argv 전달, surface:/pane:/workspace: ref) |
| `[NEW] pkg/orchestra/provider_patterns.go` (FastFailRule, DefaultFastFailRules, DefaultHookProviders, DefaultPromptPatterns accessor) | [NEW] planned addition | 미존재, 300줄 한도 분산용 신규 파일 |
| `[NEW] ProviderConfig.FastFailPatterns`, `[NEW] ProviderConfig.HasHook` (types.go) | [NEW] planned addition | 미존재, WorkingPatterns 선례 따름 |
| `[NEW] PromptData.Sentinel` (prompt_data.go), `[NEW] debate sentinel generator` | [NEW] planned addition | 미존재 |
| `[NEW] reliabilityStore.degraded` (reliability_bundle.go) | [NEW] planned addition | 미존재 |
| `[NEW] surfaceTrackerRoot()` + 소유/ref 검증 헬퍼 (surface_tracker.go) | [NEW] planned addition | 미존재, reliabilityRuntimeRoot 패턴 |
| `[NEW] controlplane unsignedWarnOnce` + warn 헬퍼 (controlplane.go) | [NEW] planned addition | 미존재 |
| `[NEW] learn appendUnlocked` (store.go) | [NEW] planned addition | 미존재, 비재진입 분리용 |
| `[NEW]` 테스트: `cc21_monitor_error_test.go`, learn race test, `provider_patterns_test.go`, `debate_sentinel_test.go`, reliability observability test, controlplane unsigned test, surface tracker security test | [NEW] planned addition | 미존재 |

## Reviewer Brief

- Intended scope: 7개 검증된 런타임 결함(F1-F7)을 silent 실패 관측화·race 봉인·패턴 선언화로 닫되 기존 동작 후방호환. 단일 cohesive 하드닝.
- Explicit non-goals: ws_client StateRecoverer 배선, 서명 정책 필수화, prompt 패턴 글로벌 경로 스레딩, 새 strategy/백엔드/스키마. 리뷰어는 이 항목들로 scope를 확장하지 말 것.
- Self-verified: Traceability Matrix, Semantic Invariant Inventory(7행 모두 REQ/Task/oracle 추적), oracle acceptance(structural-only 아님), existing/[NEW] reference discipline, Finding 검증·reframe(F3 부분 reframe·F5 메커니즘 reframe·라인번호 교정).
- Reviewer should focus on: correctness(특히 INV-001 forced-false·INV-002 비재진입 잠금·INV-004 sentinel 충돌 회피), convergence safety, regression risk(오버라이드 미설정 동작 불변·반환 계약 불변·기존 테스트 green), Completion Debt 유무. 새 제품 scope 제안은 범위 밖.

## Finding 검증·Reframe (objective reasoning)

전수 코드 실측 결과 full false-positive는 없었으나 2건 reframe + 라인번호 교정이 있었다.

- **F3 부분 reframe**: 전제 "모든 패턴이 하드코딩이고 오버라이드 불가"는 부분적으로 부정확. `defaultHookProviders`는 이미 `SetHookProviders`로, prompt 패턴은 이미 `CompletionPattern`/`WorkingPatterns`로 per-provider 오버라이드 가능. **진짜 gap은 fast-fail 패턴(오버라이드 경로 0)**. REQ-003은 (a) fast-fail 선언화(신규), (b) hook map을 config에서 파생(기존 seam 형식화), (c) prompt 글로벌 기본값을 단일 accessor로 노출(no-provider 글로벌 경로 스레딩은 non-goal)로 정직하게 재범위화.
- **F5 메커니즘 reframe**: 전제 "error를 `_ =`로 무시"는 부정확. `recordPrompt`는 error가 아니라 string(경로)을 반환하고 실패 시 ""를 돌려준다(writeJSON 내부 1회 재시도 존재). 진짜 결함은 "영속화 실패가 빈 경로로 평탄화되어 무관측". 수정을 호출처 7곳이 아닌 chokepoint `writeJSON`으로 이동. 프롬프트가 가리킨 라인 84는 ReliabilityStore가 아니라 `SendRoundEnvToPane`(별개 silent 실패)로 교정, 같은 REQ-005에서 경고 추가.
- **F1/F3/F5 라인번호 교정**: fast-fail은 222-235가 아니라 `detectProviderFastFail`(245-258); F5 라인 84는 SendRoundEnvToPane. 심볼명 기준으로 모두 재확인.
- **F6 by-design 보존**: fail-open은 `@AX:ANCHOR`로 명시된 의도. 정책 불변, 경고만 추가(요구사항도 경고만 요청) → 요구사항화 적정.
- 전수 결론: F1·F2·F4·F7 그대로 채택, F3·F5 reframe 채택, F6 경고-only 채택. 기각된 finding 없음.

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: 모든 비-[NEW] 경로/심볼을 Read/rg로 실측(cc21_monitor·learn store·templates·controlplane·surface_tracker 등)
- Q-CORR-02 | status: PASS | attempt: 1 | files: research.md, plan.md | reason: 신규 파일/타입/필드/테스트를 [NEW]로 표기, 정합성 PASS 근거에서 제외
- Q-CORR-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: EARS 헤더(Ubiquitous/Event-Driven/Unwanted/Where + Priority)·bare Given/When/Then을 SPEC-ORCH-022 검증 통과 포맷과 일치
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline이 existing(rg/Read 검증)과 [NEW] planned addition 분리, generated 표면 없음
- Q-COMP-01 | status: PASS | attempt: 1 | files: 4파일 | reason: spec/plan/acceptance/research가 각 역할 분담, 상호 보완
- Q-COMP-02 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: REQ-001~007이 Traceability Matrix로 Task·Scenario·INV에 1:1 추적
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md | reason: 각 REQ에 EARS type·조건·기대결과·관측 지점 명시
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock의 mandatory가 Primary SPEC requirements/plan/Must oracle로 닫힘, Completion Debt none
- Q-COMP-05 | status: PASS | attempt: 1 | files: research.md, spec.md, acceptance.md | reason: INV-001~007 각각 REQ+Task+Must/Should oracle(concrete expected value)로 추적, structural-only 없음
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief로 review scope·non-goal·focus 한정
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(none)와 Evolution Ideas(4건, SPEC/task ID 미부여) 분리
- Q-FEAS-01 | status: PASS | attempt: 1 | files: plan.md | reason: 전부 런타임 Go 코드 변경, 문서-only 약속 없음
- Q-FEAS-02 | status: PASS | attempt: 1 | files: research.md | reason: 모든 경로가 autopus-adk 모듈 내 실재 디렉토리, templates/는 source of truth(embed)
- Q-FEAS-03 | status: PASS | attempt: 1 | files: plan.md, acceptance.md | reason: `go test ./pkg/learn/... -race` 등 실행 가능 검증, 비례적
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ 본문에 should/might/could 등 모호어 없음, SHALL 단정
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority(Must/Should)와 EARS type 분리 축, 별칭 없음
- Q-STYLE-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: bare Given/When/Then/And, 완결 문장
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: 참가자 출력(provider output)을 untrusted prompt 입력으로 취급, sentinel 펜스로 trust 경계 고정(F4), control-plane fail-open 경계(F6) 명시
- Q-SEC-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: surface tracker 경로(소유/0700 검증·ref 형식 검증·legacy read-only)·서명 secret 비노출 다룸
- Q-SEC-03 | status: PASS | attempt: 1 | files: spec.md | reason: 경고 로그는 1회/store·1회/process로 noise 제한, secret 미기록, retained artifact 추가 없음
- Q-COH-01 | status: PASS | attempt: 1 | files: 4파일 | reason: 단일 "런타임 silent-failure 하드닝" 스토리로 수렴
- Q-COH-02 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock 후속 작업이 Primary에 포함되거나 Evolution(비차단)으로만 잔존
- Q-COH-03 | status: PASS | attempt: 1 | files: research.md | reason: sibling 없음(단일 cohesive), 재귀 sibling 미생성
