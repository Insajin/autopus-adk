# SPEC-ORCH-023 구현 계획

## Tasks

- [ ] T1 (REQ-001): `pkg/orchestra/cc21_monitor.go`에 공유 에러 처리 헬퍼 추가. detector `WaitForCompletion` 호출 3지점(라인 89/96/103)을 헬퍼로 경유시켜 (a) non-nil 에러를 provider 이름과 함께 로깅, (b) `ctx.Err() != nil`(취소)을 I/O 실패와 구분 로깅, (c) 에러 시 completed=false 강제. `[NEW] cc21_monitor_error_test.go`로 stub detector 주입 oracle(S1). 파일 105줄 → 헬퍼 추가 후 300 미만 유지.
- [ ] T2 (REQ-002): `pkg/learn/store.go`에서 공개 `Append`를 `s.mu` 잠금 wrapper로, 본문은 `[NEW] appendUnlocked`로 분리. `AppendAtomic`는 `appendUnlocked` 호출(이미 lock 보유). `UpdateReuseCount`에 `s.mu` 잠금 추가(내부 `Read`는 unlocked). `pkg/learn/prune.go::Prune`에 `store.mu` 잠금 추가(`Read`+`rewriteStore` unlocked 호출). 동시성 oracle 테스트(S2)를 `-race`로. store.go 163줄·prune.go 30줄 → 한도 여유.
- [ ] T3 (REQ-003): `[NEW] pkg/orchestra/provider_patterns.go`에 `FastFailRule{Substring,Reason}` 타입·`DefaultFastFailRules()`(현재 4-rule)·`DefaultHookProviders()`(claude/gemini/codex)·`DefaultPromptPatterns()` accessor. `types.go` `ProviderConfig`에 `[NEW] FastFailPatterns []FastFailRule`·`[NEW] HasHook *bool` 추가. `provider_runner.go` `fastFailBuffer`에 rules 필드 주입(미설정 시 `DefaultFastFailRules()`), `detectProviderFastFail`을 rules 기반으로 재작성. `hook_signal.go`는 config의 `HasHook`로 hook map 파생(미설정 시 `defaultHookProviders`). `interactive_detect.go`의 `defaultPromptPatterns`는 accessor로 노출(글로벌 기본값 단일 출처). default-equivalence oracle(S3/S3b). provider_runner.go 294줄 보호를 위해 타입·기본값은 신규 파일로.
- [ ] T4 (REQ-004): `prompt_data.go`에 `[NEW] PromptData.Sentinel string`. `[NEW]` sentinel generator(`crosspolinate.go` 또는 `[NEW] debate_sentinel.go`)가 `uniqueHeredocDelimiter` 사상으로 `AUTOPUS_PART_<randomHex>` 생성 후 모든 참가자 출력에 부재 보장(충돌 시 suffix 연장). `BuildDebaterR2`(pipeline.go:75-76)·`BuildJudge`(judge_builder.go:21) 경로가 sentinel 설정. `orchestra-debater-r2.md.tmpl`·`orchestra-judge.md.tmpl`에 BEGIN/END 펜스+untrusted 안내 추가(`{{$.Sentinel}}-BEGIN`/`-END`). fencing+충돌회피 oracle(S4/S4b).
- [ ] T5 (REQ-005): `reliability_bundle.go` `reliabilityStore`에 `[NEW] degraded bool` 추가. `writeJSON`이 최종 실패로 ""를 반환하기 직전, `degraded`가 false→true 전이 시에만 store당 1회 경고 emit(뮤텍스 보유 중). `interactive_debate_round.go` 라인 84 `SendRoundEnvToPane` 에러를 경고 로깅. 반환 계약("") 불변. oracle(S5). reliability_bundle.go 287줄 → 한도 근접: 초과 시 redact/preview 헬퍼를 `[NEW]` 파일로 추출.
- [ ] T6 (REQ-006): `controlplane.go`에 `[NEW] unsignedWarnOnce sync.Once`+warn 헬퍼. 3개 검증 진입점이 `secret == ""`로 `nil` 반환하기 직전 헬퍼 호출(프로세스당 1회 경고). `SignedControlPlaneEnforced()`(순수 질의)는 경고하지 않음. 테스트 격리용 once-reset 노출. 반환값·정책 불변. oracle(S6).
- [ ] T7 (REQ-007): `surface_tracker.go`에 `[NEW] surfaceTrackerRoot()`(`reliabilityRuntimeRoot()` 패턴, 홈 우선·TempDir fallback). `trackSurface`가 기록 전 대상 dir의 `syscall.Stat_t` uid==`os.Getuid()` 및 mode==0700(group/other 비트 0) 검증, 불일치 시 skip. `ReapOrphanSurfaces`가 ref 형식 정규식(예: `^([A-Za-z]+:[0-9]+|%[0-9]+)$`, 선행 대시·공백·메타문자 배제) 통과분만 `Close`에 전달+불일치 로깅, 레거시 `/tmp/autopus/surfaces`는 생성 없이 읽기 전용 reap. 보안 oracle(S7/S7b).

## Implementation Strategy

- Brownfield 후방호환: 모든 변경은 추가(오버라이드 필드·경고 로그·잠금·펜스)이며, 오버라이드 미설정/실패 미발생 시 관측 가능한 동작은 불변. 각 태스크는 기존 단위 테스트 green을 선결 조건으로 한다.
- 비재진입 잠금(T2)은 "공개 잠금 연산 → unlocked primitive만 호출" 규칙으로 데드락을 구조적으로 배제한다. 잠긴 메서드가 다른 잠긴 메서드를 호출하지 않음을 코드 리뷰 체크.
- 패턴 선언화(T3)는 "기본값 = 현재 하드코딩 값" 불변식(INV-003)을 oracle로 고정해 회귀를 차단한다. fast-fail이 유일한 진짜 gap이고 hook/prompt는 기존 seam 형식화임을 research.md가 명시.
- sentinel(T4)은 promptlayer manifest에서 참가자 출력을 ephemeral task 레이어(GroupUserRequest)로 유지하여 stable/snapshot 캐시를 오염시키지 않는다.
- 각 태스크는 독립 실행 가능(파일·관심사 분리). T1·T6·T7은 단일 파일+테스트로 병렬화 가능, T3·T4는 다중 파일이므로 단독 진행 권장.
- 검증: 태스크별 `go test ./pkg/learn/... -race`(T2), `go test ./pkg/orchestra/... -run <oracle> -race`(T1/T3/T4/T5), `go test ./pkg/worker/controlplane/...`(T6), surface 보안 테스트(T7). 전체 `go build ./...`+`go vet ./...`+영향 패키지 `-race`. 신규/수정 소스 ≤300줄 확인.

## Visual Planning Brief

silent 실패 → 관측 가능 변환의 데이터/제어 플로우:

```mermaid
flowchart TD
    subgraph F1[cc21 완료 감지]
      A1[detector.WaitForCompletion err] -->|현재 _ 버림| A2[미완료와 구분 불가]
      A1 -->|수정 공유 헬퍼| A3[로그 provider+err / ctx취소 구분 / completed=false]
    end
    subgraph F2[learn 스토어]
      B1[UpdateReuseCount/Prune read→truncate→rewrite] -->|현재 무잠금| B2[동시 Append 항목 유실]
      B1 -->|수정 s.mu 직렬화| B3[Read 항목수·ReuseCount 보존]
    end
    subgraph F3[프로바이더 패턴]
      C1[fast-fail/hook/prompt 하드코딩] -->|수정 config 오버라이드| C2[미설정=현재값 동일 / 설정=오버라이드]
    end
    subgraph F4[디베이트/judge 프롬프트]
      D1["{{.Output}} 무펜스 삽입"] -->|현재| D2[위조 ## 헤더 주입 가능]
      D1 -->|수정 랜덤 sentinel 펜스| D3[BEGIN/END 안쪽 = untrusted 데이터]
    end
    subgraph F5[reliability 영수증]
      E1[writeJSON 실패 → 빈 경로] -->|현재 무관측| E2[영속화 실패 invisible]
      E1 -->|수정 degraded 1회 경고| E3[store당 경고 1회 / 반환 불변]
    end
    subgraph F6[control plane]
      G1[secret 미설정 → nil skip] -->|수정 Once 경고| G2[프로세스당 경고 1회 / 정책 불변]
    end
    subgraph F7[surface tracker]
      H1[/tmp 공유경로 + 무검증 Close] -->|수정| H2[홈 0700 소유검증 / ref 형식검증 / legacy read-only reap]
    end
```

## Feature Completion Scope

- Primary SPEC가 Outcome Lock 전체(F1-F7)를 단일 cohesive 변경으로 닫는다. mandatory(REQ-001~005)는 Must oracle로, secondary(REQ-006~007)는 Should oracle로 검증된다.
- 승인된 sibling 의존성: 없음(research.md `## Sibling SPEC Decision`).
- 남은 Completion Debt: 없음. ws_client `StateRecoverer` 배선·서명 필수화·prompt 패턴 글로벌 스레딩은 Outcome Lock 밖 Evolution Ideas로만 잔존하며 sync completion을 막지 않는다.
