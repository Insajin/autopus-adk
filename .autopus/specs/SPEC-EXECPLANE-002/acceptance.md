# SPEC-EXECPLANE-002 Acceptance: orca 감독 실행을 PhaseBackend로 구현

이 문서는 설계 고정 문서의 오라클이다. 시나리오는 구현 작업(T101~T104)이 통과해야 하는 판정 기준이고, 이 SPEC 자체의 완료 증거는 4개 문서의 `auto spec validate` 통과다.

## Test Scenarios

### S101: orca 경로가 omp 경로와 같은 엔진·같은 EngineConfig로 구동된다
Priority: Must
Given `auto pipeline run --platform omp --execution-owner orca`가 티어 정합 게이트를 통과하고 체크포인트 로드까지 끝낸 상태
When 엔진이 5-phase 실행을 시작한다
Then 실행 주체는 omp 경로와 같은 `pipeline.SubprocessEngine`이고, `EngineConfig`의 phase 목록·게이트 종류·재시도 상한·체크포인트 디렉터리 값이 omp 경로와 같다
And 두 경로의 유일한 차이는 주입된 `Backend` 구현 하나이며, orca 전용 phase 러너·orca 전용 순서 테이블·orca 전용 게이트 판정 코드가 각각 0건이다
And orca 백엔드가 엔진에 노출하는 메서드는 `pkg/pipeline/engine.go:29-31`의 `Execute(ctx, PhaseRequest) (*PhaseResponse, error)` 하나이고, run-scoped 리소스 해제는 `PhaseBackendCloser`의 `Close()`로만 한다
And 별도 러너가 같은 실행 결과를 내더라도 이 시나리오는 실패다 — 관측 대상은 결과 일치가 아니라 순서·게이트·재시도의 단일 구현 소유다

### S102: orca task가 의존성 간선 없이 생성되고 순서는 엔진만 소유한다
Priority: Must
Given 백엔드 생성 시 Run이 한 번 만들어져 있고, 백엔드가 phase 실행을 위해 orca task를 만드는 상태
When 5-phase가 전부 실행된다
Then 이 Run에 속한 모든 task의 deps가 빈 배열이고, `task-create` 호출 argv에서 `--deps`의 출현 횟수가 0이다
And 관측된 실행 순서 plan → test_scaffold → implement → validate → review가 `pkg/pipeline/phase.go:34-42`의 `DefaultPhases()` 정의와 일치하고, 프로세스 평면에 같은 순서를 표현한 두 번째 자료구조가 0건이다
And 어느 한 phase에서라도 `--deps`가 채워지면 INV-101 위반이며, `pkg/adapter/omp/omp_workflow_render.go:77-78`의 단일 DAG 소유자 불변식도 같이 깨진다
And `--deps`를 쓰지 않는 것은 기능 낭비가 아니라 평면 분리의 관측 지점이다. 프로세스 평면이 DAG를 가지면 게이트·재시도·체크포인트가 그쪽에도 복제되어야 한다

### S103: phase attempt 하나가 Dispatch 하나에 1:1로 대응한다
Priority: Must
Given 백엔드가 한 phase의 attempt 1에 대해 `Execute`를 정확히 한 번 호출받은 상태
When 그 호출이 반환된다
Then 그 attempt 동안 시작된 Dispatch가 정확히 1건이고, 그 Dispatch가 release·stop·abandon 중 정확히 하나로 정확히 1회 종결된다
And 재시도는 같은 Dispatch의 재사용이 아니라 새 attempt이므로 새 Dispatch를 시작한다. 엔진의 attempt 루프 상한이 `MaxRetries + 1`이므로 validate(`MaxRetries: 3`)의 Dispatch 수는 최대 4건, review(`MaxRetries: 2`)는 최대 3건이다
And 게이트가 첫 판정에 통과한 클린 런의 총 Dispatch 수는 phase 수와 같은 5건이고, 실행 영수증의 `dispatch_count`가 시작된 워커 수와 같다. 두 값이 어긋나면 1:1이 깨진 것이다
And 한 attempt에서 워커를 2개 이상 띄우는 fan-out은 이 SPEC의 비목표이므로, 한 attempt에 2건 이상이 관측되면 실패다

### S104: 워커 대기와 출력이 둘 다 바운드된다
Priority: Must
Given 종결 신호를 내지 않는 워커 하나와 출력을 계속 쏟는 워커 하나가 있는 상태
When 백엔드가 각 워커의 종결을 기다리고 그 출력을 읽는다
Then 응답 없는 워커의 대기는 백엔드 deadline에서 끊기고 `PhaseResponse.TimedOut`이 true로 반환되어, 파이프라인이 그 phase에서 무한 대기하지 않는다
And 출력 읽기는 `worker-read --cursor --limit`의 상한 안에서만 진행되어 백엔드가 보유하는 출력 바이트 수가 상한 이하로 유지되고, 상한 때문에 잘렸다는 사실이 `PhaseResponse.FailureClass`에 남는다
And 두 한계 중 어느 쪽으로 끊겼는지 `TimedOut`과 `FailureClass` 두 필드만 보고 구분할 수 있다
And 두 필드가 모두 비어 있는 채로 phase가 실패하면 사유가 관측 불가이므로 실패다. 대기와 읽기 중 한쪽만 바운드된 구현도 실패다

### S105: 실패·취소·백엔드 종료 세 경로 모두 살아있는 Dispatch를 남기지 않는다
Priority: Must
Given phase attempt가 (a) 실패로 끝나는 경로, (b) `ctx` 취소로 끊기는 경로, (c) 백엔드 `Close()`로 정리되는 경로 세 가지
When 각 경로가 끝난다
Then 세 경로 각각에서 이 Run에 속한 `worker-list --terminal-state active` 결과가 0건이다
And 시작이 실패해 응답이 잔여 리소스를 보고한 경로에서는 그 리소스가 각각 명시적으로 정리되어 남지 않는다. `active` 집계 0건만으로는 이 조건을 대신하지 못한다
And 백엔드가 시작한 Dispatch 전부가 release·stop·abandon 중 하나로 종결 상태를 갖고, 종결되지 않은 Dispatch가 0건이다
And 워커 프로세스가 실제로 멈췄다고 주장할 수 없는 경로에서는 release 대신 `worker-abandon`으로 펜싱하고, abandon을 택한 사유가 실행 기록에 남는다
And 취소 경로에서 `PhaseResponse`가 없고 오류만 있어도 정리 책임은 백엔드에 있다. 엔진이 실패를 반환했다는 사실이 정리 누락의 사유가 되지 않는다

### S106: 티어 정합 게이트가 Run 생성보다 먼저 끝난다
Priority: Must
Given `--execution-owner orca` 결정이 내려졌고, `internal/cli/pipeline_run.go:147-163`에서 정합 게이트가 백엔드 생성보다 앞에 있는 상태
When 준비 단계가 orca Run을 만든다
Then `<SPEC-ID>.tier-integrity.json`의 `checked_at`이 `run-create`가 반환한 Run 생성 시각보다 앞선다
And 게이트를 실행하지 않고 `run-create`에 도달하는 코드 경로가 0건이고, 게이트가 통과하지 못하면 그 실행의 `run-create` 호출 수가 0이며 `worker-start` 호출 수도 0이다
And 정합 영수증이 없는 실행, 또는 `checked_at`이 Run 생성 시각 이후인 실행은 INV-105 위반으로 판정한다
And 게이트가 unverified로 떨어진 구성은 워커를 띄우는 대신 그 사실을 결과로 보고한다

### S107: 프로세스 평면 경계 인자에 티어 어휘가 없다
Priority: Must
Given 정책 평면이 phase별 quality tier를 결정하고 그 결정을 `worker-start` 인자로 직렬화하는 상태
When 백엔드가 `worker-start --task --agent --worktree --model --effort --timeout-ms`를 호출한다
Then `--model` 값은 provider가 정의한 opaque model id 문자열이고 `--effort` 값은 effort 레벨 문자열이며, 티어 이름을 값으로 갖는 인자가 0건이다
And 경계 argv에서 `balanced`, `ultra`, `opus`, `sonnet`, `haiku` 각 토큰의 출현 횟수가 각각 0이다. 판정은 인자 값 전체와의 완전 일치이므로 `claude-opus-4-1`처럼 provider가 정의한 model id 안에 부분 문자열로 포함된 경우는 위반이 아니고, `--model opus`처럼 티어·패밀리 이름을 그대로 넘기는 것이 위반이다
And 티어 사다리의 단일 출처는 정책 평면에 남는다. `--model`과 `--effort` 외에 티어에서 파생된 인자가 경계에 추가되면 실패다
And SPEC-EXECPLANE-001 S2가 orca 명령 표면 전수 검색에서 얻은 0건과 이 시나리오가 실제 argv에서 얻는 0건은 같은 어휘 무증식 원칙의 두 관측 지점이다

### S108: handoff 스텁이 실행으로 대체되고 결과가 omp 경로와 같은 형태로 남는다
Priority: Must
Given 지원되는 구성, 즉 로컬 worktree·검증된 티어 정합·`--platform omp --execution-owner orca`로 파이프라인을 실행한 상태
When 실행이 완주하거나 실패로 끝난다
Then 최종 결과의 `status`가 `handoff_required`가 아니고, 이 구성에서 `internal/cli/pipeline_run.go:162`의 handoff 반환이 호출된 횟수가 0이다
And 5개 phase 결과와 체크포인트가 omp 경로와 같은 스키마로 같은 `pipelineStateDir`에 남아, 결과 파일만으로는 실행 소유자를 구분할 수 없다
And 실패로 끝난 경우에도 terminal 상태와 실패 phase가 omp 경로와 같은 필드로 관측되고, 사람이 손으로 이어받으라는 안내가 최종 결과에 없다
And 비지원 구성(원격 환경 등)은 이 SPEC의 비목표이므로 그 경로가 `handoff_required`를 반환하는 것은 위반이 아니다. 판정 범위는 지원 구성으로 한정한다

## Oracle Acceptance Notes

- **S101** — 예상 값: orca 전용 러너·순서 테이블·게이트 판정 구현 수 = 0, 백엔드가 엔진에 노출하는 메서드 수 = 1(`Execute`). 두 경로의 `EngineConfig` 비교에서 다른 필드 = `Backend` 1개. "같은 결과가 나오면 통과"는 오라클이 아니다.
- **S102** — 예상 값: task deps 길이 = 0, `--deps` 출현 횟수 = 0, 프로세스 평면의 phase 순서 자료구조 수 = 0, 관측된 phase 순서 = plan, test_scaffold, implement, validate, review.
- **S103** — 예상 값: attempt 1회당 시작 Dispatch = 1, 종결 = 1. 클린 런 총 Dispatch = 5. validate 최대 4, review 최대 3(각각 `MaxRetries + 1`). 영수증 `dispatch_count`와 워커 시작 수의 차이 = 0.
- **S104** — 예상 값: 무응답 워커에서 `TimedOut` = true, 백엔드 보유 출력 바이트 = 설정 상한 이하, 잘림이 발생한 실행의 `FailureClass` = 비어 있지 않음. deadline 값과 읽기 상한 값 자체는 구현 시 확정하되, 둘 다 유한해야 한다는 점이 판정 조건이다.
- **S105** — 예상 값: 세 경로 각각에서 `worker-list --terminal-state active` 행 수 = 0, 미종결 Dispatch = 0, 보고된 잔여 리소스 중 정리되지 않은 것 = 0. abandon으로 펜싱한 건에는 사유가 1건씩 남는다. 세 번째 값이 따로 필요한 이유는 실측에서 확인됐다 — readiness가 실패하면 터미널이 만들어져도 dispatch가 소유하지 않아 `active`에 잡히지 않고, `worker-release`는 `no_owned_resource`로 지나가며, 터미널은 살아남는다.
- **S106** — 예상 값: `checked_at` < Run 생성 시각(부호는 엄격 부등호). 게이트 미통과 실행의 `run-create` 호출 = 0, `worker-start` 호출 = 0.
- **S107** — 예상 값: 경계 argv에서 `balanced` = 0, `ultra` = 0, `opus` = 0, `sonnet` = 0, `haiku` = 0. 판정 규칙은 인자 값 전체의 완전 일치이며, 이 규칙을 부분 문자열 검색으로 바꾸면 정상적인 provider model id가 오탐된다.
- **S108** — 예상 값: 지원 구성에서 `status == "handoff_required"`인 최종 결과 수 = 0, 체크포인트에 기록된 완료 phase 수 = 5, omp 경로와 다른 결과 필드 수 = 0.
- **자동화 이월**: 이 SPEC은 설계 고정 문서이고 구현을 포함하지 않는다. 따라서 S101~S108은 지금 자동 테스트로 닫히지 않고, T101~T104 구현 작업의 오라클로 이월된다. 이 SPEC의 통과 조건은 4개 문서의 `auto spec validate` 통과와 DAG 소유권 선택에 대한 리뷰 합의다.
- **오류 경로 위치**: 별도 Edge Cases 절을 두지 않았다. 실패·취소·종료 경로는 S105가, 무응답·과다 출력 경로는 S104가, 비지원 구성 경로는 S108이 각각 담당한다.
- **Completion Debt와 자동화 가능 범위**: `--agent` 값 집합이 미정이므로 S107의 argv 전수 검사는 `--agent` 값을 판정 대상에서 제외한 상태로만 고정된다. 워커 settled 판정 방법이 미정이므로 S103과 S104는 stub 백엔드 수준에서만 고정 가능하고, 실제 orca 런타임 기준 자동화는 상태 필드 이름과 종료 상태 집합이 확정된 뒤에 가능하다. 워커 출력에서 phase 결과를 뽑는 규약이 미정이므로 S108의 결과 형태 일치 판정은 스키마 비교까지만 가능하고 내용 추출 경로는 규약 확정 후에 붙는다. 재개 시 Run 재바인딩 정책이 미정이므로 S106의 시각 비교는 신규 실행 기준으로만 고정되며, `--continue` 경로의 기준 Run 시각은 정책 확정 후 S106에 추가된다.
- **주의: S101과 S102는 같은 불변식의 두 각도다**. S102는 프로세스 평면에 DAG가 생기지 않았음을, S101은 정책 평면에 순서·게이트·재시도 구현이 하나뿐임을 잰다. 한쪽만 통과해도 평면 분리는 깨질 수 있다. `--deps` 없이 orca task를 만들면서 autopus 쪽에 orca 전용 러너를 따로 두면 S102는 통과하고 S101이 실패한다. 반대로 엔진을 그대로 쓰면서 task에 deps를 채우면 S101은 통과하고 S102가 실패한다. 두 시나리오는 함께 통과해야 의미가 있고, 하나라도 실패하면 REQ-101·REQ-102를 함께 미달로 판정한다.

## Definition of Done

- [ ] S101~S108이 REQ-101~REQ-108과 1:1로 대응하고 각 Then이 관측 가능한 값을 지명한다
- [ ] 5개 불변식(INV-101~INV-105)이 각각 최소 한 시나리오의 판정에 연결된다
- [ ] Completion Debt 4건이 어느 시나리오의 자동화를 막는지 명시된다
- [ ] `auto spec validate .autopus/specs/SPEC-EXECPLANE-002`가 통과한다
