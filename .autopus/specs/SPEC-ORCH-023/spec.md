# SPEC-ORCH-023: 오케스트라·learn·worker 런타임 견고성 하드닝

**Status**: completed
**Created**: 2026-06-12
**Domain**: ORCH

## 목적

런타임 견고성 감사에서 오케스트라·learn·worker 경로의 silent 실패와 데이터 경합 7건을 코드로 재검증했다(상세·reframe는 research.md `## Finding 검증·Reframe`). 공통 문제는 (1) 완료 감지·영수증 영속화·서명 검증 실패가 로그/반환값 없이 흡수되어 관측 불가, (2) learn 스토어의 read-modify-rewrite가 동시 append와 경합해 학습 데이터가 유실, (3) 프로바이더 패턴이 일부 오버라이드 경로 없이 하드코딩, (4) 디베이트/judge 프롬프트가 참가자 출력을 경계 없이 삽입해 라운드 간 prompt injection 여지가 있다는 것이다.

이 SPEC은 새 strategy·백엔드·JSON 스키마 변경 없이, 기존 동작을 후방호환으로 보존하면서(오버라이드/실패 신호는 추가만) 이 silent 실패 경로를 관측 가능하게 만들고, learn 유실 race를 봉인하며, 프로바이더 패턴을 설정 계층에서 선언적으로 오버라이드 가능하게 한다.

## Outcome Boundary

- Outcome Lock: 완료 감지(F1)·learn 스토어(F2)·프롬프트 경계(F4)·reliability 영수증(F5)의 silent 실패 경로가 로그/반환값으로 관측 가능해지고, learn 학습 데이터 유실 race(F2)가 봉인되며, 프로바이더 패턴(fast-fail/hook/prompt)이 설정 계층에서 선언적으로 오버라이드 가능(F3)해진다. 오버라이드 미설정 시 해석값은 현재 하드코딩 값과 정확히 동일하다. 부가 보안 하드닝으로 unsigned 제어평면 진입(F6)과 surface tracker 경로/ref(F7)를 관측·검증한다.
- Mandatory requirements: REQ-001(cc21 에러 관측+forced-false), REQ-002(learn race 봉인), REQ-003(프로바이더 패턴 선언화·기본값 보존), REQ-004(디베이트/judge 출력 sentinel 펜스), REQ-005(reliability 영수증 영속화 실패 관측). Secondary(Should): REQ-006(unsigned 제어평면 1회 경고), REQ-007(surface tracker 홈 경로+소유/ref 검증).
- Explicit non-goals: `pkg/worker/a2a/ws_client.go:19` `StateRecoverer` inert seam 배선(worker 기능 SPEC 소유), fail-open 서명 정책의 필수화(경고만), per-provider prompt 패턴을 no-provider `filterPromptLines`/`isPromptLine` 글로벌 경로까지 스레딩, 새 strategy/백엔드/JSON 스키마 변경.
- Completion evidence: REQ-001~007이 acceptance.md S1~S7b oracle 시나리오로 검증되고(concrete expected value 포함), 오버라이드 미설정 시 동작 불변·반환 계약 불변·기존 테스트 green이 유지된다.

## Requirements

### Event-Driven / Priority: Must (REQ-001 — cc21 완료 감지 에러 관측 및 forced-false)
WHEN `waitForCompletion`(`pkg/orchestra/cc21_monitor.go`)이 완료 detector의 `WaitForCompletion` 호출에서 non-nil 에러를 받으면 THEN THE SYSTEM SHALL 그 에러를 provider 이름과 함께 로깅하고, 에러가 발생한 호출의 completed 값을 false로 강제하며, context 취소(`ctx.Err() != nil`)를 I/O 실패와 구분하여 로깅한다. 관측 지점은 detector 호출 3지점(현재 라인 89/96/103)이 공유하는 에러 처리 헬퍼의 반환 bool과 emit된 로그 텍스트다.

### Ubiquitous / Priority: Must (REQ-002 — learn 스토어 유실 race 봉인)
THE SYSTEM SHALL `pkg/learn` 스토어의 append 및 read-modify-rewrite 연산(`Append`, `AppendAtomic`, `UpdateReuseCount`, `Prune`)을 단일 store 뮤텍스로 직렬화하여 동시 실행에서도 append된 항목이 truncate-rewrite에 의해 유실되지 않게 한다. 뮤텍스는 비재진입이므로 잠긴 공개 연산은 잠그지 않는 내부 primitive(`Read`, `NextID`, unlocked append, `rewriteStore`)만 호출한다. 관측 지점은 동시 부하 후 `Store.Read()`가 반환하는 항목 수와 갱신된 ReuseCount다.

### Ubiquitous / Priority: Must (REQ-003 — 프로바이더 패턴 선언화·기본값 보존)
THE SYSTEM SHALL fast-fail 패턴(`detectProviderFastFail`), hook 가용 provider 집합(`defaultHookProviders`), prompt 패턴 기본값(`defaultPromptPatterns`)을 `ProviderConfig`/설정 계층에서 선언적으로 오버라이드 가능한 형태로 노출하되, 오버라이드가 설정되지 않으면 해석된 패턴 집합이 현재 하드코딩 값과 정확히 동일하도록 기본값을 보존한다. 관측 지점은 오버라이드 미설정 시 해석된 fast-fail reason 문자열·hook-provider map·prompt-pattern 개수가 현재 값과 일치하는지다.

### Event-Driven / Priority: Must (REQ-004 — 디베이트/judge 출력 sentinel 펜스)
WHEN Round 2 cross-pollination 또는 judge 프롬프트가 참가자 출력을 삽입하면 THEN THE SYSTEM SHALL 각 참가자 출력을 라운드별 랜덤 sentinel(모든 참가자 출력에 부재가 보장됨)로 BEGIN/END 펜스로 감싸고, 펜스 내부가 지시가 아닌 untrusted 데이터임을 템플릿이 명시하게 한다. 관측 지점은 렌더된 프롬프트에서 위조된 `###`/`##` 헤더가 펜스 경계 안쪽에만 존재하는지와 `sentinel-BEGIN` 출현 횟수가 참가자 수와 일치하는지다.

### Unwanted / Priority: Must (REQ-005 — reliability 영수증 영속화 실패 관측)
IF reliability 영수증 쓰기가 재시도 후에도 영속화에 실패하여 `reliabilityStore.writeJSON`이 빈 경로를 반환하면 THEN THE SYSTEM SHALL store 인스턴스당 정확히 한 번 경고를 emit하여 영수증이 영속화되지 않음을 알리고, 호출자에 대한 빈 문자열 반환 계약은 변경하지 않는다. WHEN `interactive_debate_round.go`의 `SendRoundEnvToPane` 호출이 에러를 반환하면 THE SYSTEM SHALL 그 에러를 경고로 로깅한다. 관측 지점은 store 저하 전이 시 emit된 경고 횟수와 recordPrompt 반환값(빈 문자열)이다.

### Where / Priority: Should (REQ-006 — unsigned 제어평면 1회 경고)
WHERE `AUTOPUS_A2A_POLICY_SIGNING_SECRET`가 미설정이어서 control-plane/policy 서명 검증이 fail-open으로 skip되면 THE SYSTEM SHALL 검증 진입점(`ValidateSecurityPolicySignature`/`VerifyCachedPolicyFile`/`ValidateControlPlaneSignature`)이 처음 unsigned 경로를 택할 때 프로세스당 정확히 한 번 경고를 emit하여 서명 검증이 비활성임을 알리되, 검증 반환값(미서명 시 nil)과 fail-open 정책 자체는 변경하지 않는다. 관측 지점은 미서명 검증 2회 호출 시 경고 횟수(정확히 1)와 반환값(nil)이다.

### Where / Priority: Should (REQ-007 — surface tracker 홈 경로·소유/ref 검증)
WHERE surface tracking 파일을 기록하면 THE SYSTEM SHALL `reliabilityRuntimeRoot()` 패턴을 따라 per-user 홈 기반 디렉토리(미가용 시 TempDir fallback)를 사용하고, 기록 전 대상 디렉토리가 현재 uid 소유이며 모드가 0700(group/other 권한 비트 없음)인지 검증하여 불일치 시 추적을 건너뛴다. WHEN `ReapOrphanSurfaces`가 tracking 파일의 ref로 surface를 닫으면 THE SYSTEM SHALL surface-ref 형식(선행 대시·공백·shell 메타문자 없는 정해진 형태)에 맞는 ref만 `Close`에 전달하고 불일치 ref는 건너뛰며 로깅하고, 레거시 `/tmp/autopus/surfaces` 경로는 생성하지 않고 읽기 전용으로만 reap한다. 관측 지점은 resolved tracker root 경로·소유/모드 검증 분기·Close에 전달된 ref 집합이다.

## Acceptance Criteria

- [ ] REQ-001 ~ REQ-007이 acceptance.md S1 ~ S10 시나리오로 oracle 검증된다(structural-only 금지, concrete expected value 포함).

## 생성 파일 상세

`[NEW] pkg/orchestra/provider_patterns.go`는 fast-fail rule 타입(`FastFailRule`)·기본 4-rule·기본 hook provider map·기본 prompt 패턴 accessor를 모아 "선언적 기본값" 단일 출처를 제공한다. `provider_runner.go`(294줄)·`interactive_detect.go`(277줄) 300줄 한도 보호 목적.

수정 대상(existing): `pkg/orchestra/cc21_monitor.go`(공유 에러 헬퍼), `pkg/learn/store.go`+`pkg/learn/prune.go`(비재진입 잠금 분리), `pkg/orchestra/provider_runner.go`+`hook_signal.go`+`interactive_detect.go`+`types.go`(패턴 오버라이드 배선), `templates/shared/orchestra-debater-r2.md.tmpl`+`orchestra-judge.md.tmpl`+`pkg/orchestra/prompt_data.go`+`crosspolinate.go`(sentinel 펜스), `pkg/orchestra/reliability_bundle.go`+`interactive_debate_round.go`(영속화/송신 실패 경고), `pkg/worker/controlplane/controlplane.go`(unsigned 경고), `pkg/orchestra/surface_tracker.go`(홈 경로·소유/ref 검증).

`[NEW]` 신규 필드/헬퍼: `ProviderConfig.FastFailPatterns`/`HasHook`, `PromptData.Sentinel`, sentinel generator, `reliabilityStore.degraded`, `surfaceTrackerRoot()`, controlplane `unsignedWarnOnce`, learn `appendUnlocked`.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1 | S1 | INV-001 |
| REQ-002 | T2 | S2 | INV-002 |
| REQ-003 | T3 | S3, S4 | INV-003 |
| REQ-004 | T4 | S5, S6 | INV-004 |
| REQ-005 | T5 | S7 | INV-005 |
| REQ-006 | T6 | S8 | INV-006 |
| REQ-007 | T7 | S9, S10 | INV-007 |

## Related SPECs

- SPEC-ORCH-022 (lineage): `surface_tracker.go`는 ORCH-022 follow-up으로 도입됨. 본 SPEC의 REQ-007이 그 경로/ref 안전성을 하드닝한다.
- SPEC-ORCH-007 (context): hook 파일 시그널 프로토콜·`defaultHookProviders` 기원. REQ-003은 이를 재발명하지 않고 config 오버라이드만 추가한다.
- Sibling SPEC 없음 (단일 cohesive 런타임 하드닝, research.md `## Sibling SPEC Decision` 참조).

## Out of Scope

ws_client `StateRecoverer` inert seam 배선, fail-open 서명 정책의 필수화, per-provider prompt 패턴의 글로벌 `filterPromptLines`/`isPromptLine` 스레딩, 새 strategy/백엔드/JSON 스키마 변경은 이 SPEC의 범위 밖이다(research.md `## Evolution Ideas`).

## Completion Verdict

- Outcome Lock: satisfied — cc21 완료 감지·learn 스토어·reliability 영수증·unsigned 제어평면의 silent 실패가 관측 가능해지고, learn 데이터 유실 race가 뮤텍스 직렬화로 봉인되며(-race oracle), 프로바이더 fast-fail/hook/prompt 패턴이 기본값 보존 하에 선언화되고, 디베이트/judge 프롬프트가 라운드별 랜덤 sentinel 펜스로 위조 헤더를 무력화하며, surface tracker가 홈 0700 검증 경로로 이동했다.
- Mandatory requirements: 5/5 Must (REQ-001~005), Should 2/2 (REQ-006/007)
- Must acceptance: S1~S10 전부 oracle 테스트 green (`go test ./pkg/orchestra/... ./pkg/learn/... ./pkg/worker/... -race`)
- Review: multi-provider debate PASS (69/69), Phase 4 reviewer APPROVE + security-auditor PASS (Critical/High/Med 0)
- Completion Debt: none
- Evolution Ideas: StateRecoverer 배선, 서명 필수화 등은 advisory로만 잔존
