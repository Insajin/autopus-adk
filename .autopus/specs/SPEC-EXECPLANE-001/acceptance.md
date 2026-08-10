# SPEC-EXECPLANE-001 Acceptance: 실행 평면 분리와 티어·계정·카탈로그 정합

이 문서는 설계 고정 SPEC의 수용 오라클이다. 시나리오는 구현 코드를 요구하지 않고, 각 요구가 무엇을 관측했을 때 충족으로 판정되는지를 확정한다.

## Test Scenarios

spec.md `## Traceability Matrix`에서 파생한 커버리지(권위 있는 매핑은 spec.md 쪽이다): S1=REQ-001, S2=REQ-002, S3=REQ-003·REQ-004, S4=REQ-005, S5=REQ-006, S6=REQ-007, S7=REQ-008, S8=REQ-009, S9=REQ-003·REQ-009. 9개 요구가 9개 시나리오로 빠짐없이 덮인다. REQ-003은 S3(실행 계정과 프로브 계정이 갈라진 경우)과 S9(실행 계정을 특정할 수 없는 경우) 둘로, REQ-009는 S8(검증 수단 부재)과 S9 둘로 나뉜다.

### S1: 세 평면의 소유 표면이 배타적으로 열거된다

Priority: Must
Given research.md `## Feature Coverage Map`이 7개 실행 표면을 열거하고 각 행에 소유자를 지정한다
When 각 표면의 소유자를 정책(autopus-adk)·모델(omp)·프로세스(orca) 세 평면 중 하나로 분류한다
Then 소유 평면이 0개이거나 2개 이상인 표면 행이 0건이다
And 신설 표면 `티어와 실행 계정의 정합 검증`의 소유자는 정책 평면 하나이며, omp `modelRoles`와 orca `account list` 표면 목록에 같은 이름이 나타나지 않는다
And 이 배타 분류가 INV-001의 관측 근거로 기록된다

### S2: 프로세스 평면 경계를 넘는 값에 티어 어휘가 없다

Priority: Must
Given 정책 평면이 프로세스 평면에 실행을 위임하는 유일한 경계는 모델·effort를 받는 `orchestration worker-start`이고, 이 명령은 `--model`에 opaque provider model id를, `--effort`에 effort 레벨을 받는다
When 정책 평면이 결정한 quality tier를 그 경계 인자로 직렬화한다
Then `--model` 값은 provider 고유 model id 문자열이고 `--effort` 값은 effort 레벨 문자열이다
And 경계를 넘는 인자 전체에서 `quality`, `tier`, `balanced`, `ultra`, `opus`, `sonnet`, `haiku` 토큰의 출현 횟수가 각각 0이며, 이는 research.md F4가 `orca agent-context --json` 223개 명령 전수 검색에서 얻은 0건과 일치한다
And 티어 사다리의 단일 출처는 정책 평면에 남는다

### S3: 실행 계정과 로컬 로그인 계정이 다를 때 실행 계정 기준으로 검증한다

Priority: Must
Given 프로세스 평면이 보고하는 활성 Codex 실행 계정은 `orca account list --json`의 `result.codex.activeAccountId` 기준 `bitgapnam@gmail.com`이고, 같은 응답의 `result.codex.systemDefault`가 가리키는 로컬 `codex` CLI 계정은 `gnkong@alipeople.kr`이다(research.md F1)
And 두 값은 한 번의 `orca account list --json` 호출로 함께 얻는다. 실행 계정은 `activeAccountId`, 비교 대상인 프로브 계정은 `systemDefault`(codex) 또는 provider CLI 신원이므로, 비교에 추가 조회가 필요하지 않다
And 일반화하면 실행 계정 A와 로컬 로그인 계정 B가 존재하고 A != B이며, 이 시나리오의 판정은 두 계정의 구체 문자열이 아니라 A != B라는 조건과 검증에 사용된 계정이 A인지 여부에만 의존한다
And 카탈로그 프로브는 PATH 바이너리를 실행하므로 기본값은 B의 카탈로그다(`pkg/codexruntime/probe.go:39`)
When 정합 점검이 요청 티어를 provider 모델로 해석한다
Then 검증에 사용된 카탈로그의 출처 계정은 A이고, B의 `codex debug models` 결과는 A의 티어 근거로 채택되지 않는다
And 영수증의 실행 계정 식별자와 카탈로그 출처 계정이 같은 값 A다
And 두 값이 다르면 검증은 성립하지 않은 것으로 처리되어 통과 판정을 만들지 않는다
And A 기준 카탈로그를 조회할 수단이 없으면 통과가 아니라 S8의 unverified 경로로 분기한다

### S4: 정합 영수증이 6개 필드를 모두 담는다

Priority: Must
Given 정합 점검을 마친 워크로드 하나
When 정합 영수증을 방출한다
Then 영수증은 요청 티어, 해석된 provider 모델, 실행 계정 식별자, 카탈로그 출처, resolution reason, verification status 6개 필드를 모두 담는다
And 영수증은 `pipeline_execution_owner_receipt.v1`과 같은 형태의 스키마 버전 태그를 갖는다
And 6개 필드 중 하나라도 부재하거나 빈 문자열이면 영수증은 불완전으로 판정되고 점검은 실패한다

### S5: 티어 불일치는 요청값과 실제값을 모두 남기고 fail-loud 한다

Priority: Must
Given 실행 계정 A의 카탈로그에 요청 티어가 해석한 모델이 존재하지 않는다
When 정합 점검이 그 티어를 해석한다
Then 점검은 fail closed 하거나 명시적 강등을 기록하며, 기록 없이 더 낮은 모델로 대체하지 않는다
And 영수증의 `resolution.reason`이 빈 문자열이 아니다
And 영수증에 요청 티어 값과 실제 제공 모델 값이 둘 다 존재한다. research.md F3의 실측 강등 쌍 `gpt-5.6-terra/medium` -> `gpt-5.5/xhigh`처럼 두 값이 지목 가능해야 한다
And 두 값 중 하나라도 없거나 `reason`이 비어 있으면 INV-004 위반으로 판정한다

### S6: 점검 실패는 생성된 리소스를 남기지 않는다

Priority: Must
Given 실행 계정 카탈로그가 요청 티어를 제공하지 않아 반드시 실패하도록 구성된 정합 점검
When 워크로드 준비 단계를 실행한다
Then 점검은 워크트리·Run·워커·provider 세션 중 어느 것도 생성하기 전에 종료한다
And 실행 후 새로 생성된 워크트리 0건, Run 0건, 워커 0건, provider 세션 0건이다
And 호출자에게 정합 실패가 반환된다

### S7: handoff 결과가 영수증을 참조하고 실패 시 handoff를 대체한다

Priority: Must
Given `--execution-owner orca` 경로가 `status: handoff_required`와 `required_action`을 담은 handoff 결과를 반환하는 스텁이다(`internal/cli/pipeline_run_owner.go:160-164`)
When 정합 점검이 handoff 결과 방출 직전에 실행된다
Then 점검이 통과하면 handoff 결과에 S4 영수증에 대한 참조가 포함되어, 받는 쪽이 이미 검증된 티어 계약에서 출발한다
And 점검이 실패하면 `handoff_required` 결과 대신 정합 실패가 반환되고 handoff 결과는 방출되지 않는다
And 이 경로는 OMP task DAG를 만들지 않으므로 DAG 소유자는 하나로 유지된다(`pkg/adapter/omp/omp_workflow_render.go:99`)

### S8: 검증 수단이 없는 항목은 unverified로 표기된다

Priority: Must
Given REQ-009가 흡수하는 인스턴스는 셋이다. (1) Claude 모델 가용성 — `claude` CLI 명령 집합(`agents`/`auth`/`auto-mode`/`doctor`/`gateway`/`import`/`install`/`mcp`/`plugin`/`project`/`setup-token`/`ultrareview`)에 모델 열거 명령이 없고 `omp models --json`은 정적 레지스트리라 계정 스코프가 아니므로, 계정별 카탈로그 프로브가 존재하지 않는다(research.md F8)
And (2) 원격 orca 환경 — `orca account list`가 `--environment`를 "rejected rather than ignored"로 거부하므로 원격 호스트의 계정을 조회할 수단이 없다. `orca environment list --json`이 `{"environments": []}`라 현재 영향 범위는 0건이지만 규칙은 같다(research.md F8)
And (3) 실행 계정 미특정 — 활성 계정이 없고 등록 계정이 1개가 아닌 경우다. 이 인스턴스의 판정은 S9가 담당하고 S8은 같은 `unverified` 규칙을 공유한다는 사실만 참조한다
And Codex와 달리 Claude는 신원 대조까지는 가능하다. `claude auth status --json`의 `orgId`가 orca `claude.accounts[].organizationUuid`와 같은 값 `0e61c3e2-35c8-4bf8-91bc-40848f4dc147`이다(research.md F1)
When 그 항목의 요청 티어에 대해 정합 점검을 수행한다
Then 영수증의 verification status 값이 `unverified`다
And 같은 항목이 `verified`로 표기되지 않는다
And Claude에서 신원 대조가 성공해도 모델 가용성 항목은 여전히 `unverified`다. 신원 검증 성공과 카탈로그 미검증은 동시에 성립하는 정상 상태이며, 신원이 맞았다는 이유로 티어를 `verified`로 승격하면 실패다
And `subscriptionType`(`max`/`pro`/`free`)에서 플랜 -> 모델 표를 유도해 `verified`를 만들어내지 않는다. 그것은 PR #154가 제거한 하드코딩 티어 테이블의 재도입이다
And 사유 필드에 검증 수단 부재가 기록되어 미검증과 검증 실패가 구분되고, 세 인스턴스 중 어느 것인지 복원 가능하다

### S9: 실행 계정을 특정할 수 없으면 추측하지 않고 unverified로 분기한다

Priority: Must
Given 프로세스 평면이 보고하는 어떤 provider의 `activeAccountId`가 `null`이다
And 이 워크스테이션의 Claude가 정확히 그 상태다. `result.claude.activeAccountId`는 `null`이고 `result.claude.accounts[]`에는 `jroad1049@gmail.com`(organizationUuid `0e61c3e2-35c8-4bf8-91bc-40848f4dc147`) 하나만 등록되어 있으며, Claude 블록에는 codex와 달리 `systemDefault` 필드가 없다(research.md F1)
When 정합 점검이 그 provider의 실행 계정을 확정한다
Then 등록 계정이 정확히 1개면 그 계정을 실행 계정으로 채택하고, 영수증의 실행 계정 식별자가 그 값으로 채워진다. 현재 머신의 Claude가 이 분기이며 `jroad1049@gmail.com`이 채택된다
And 등록 계정이 0개면 실행 계정을 특정하지 않고 REQ-009의 `unverified` 경로로 분기하며, 사유 필드에 계정 부재가 기록된다
And 등록 계정이 2개 이상이면 어느 것도 고르지 않고 REQ-009의 `unverified` 경로로 분기한다. 활성 계정이 없다는 이유로 첫 번째 계정이나 최근 사용 계정을 고르면 실패다
And 세 분기 어디에서도 로컬 CLI 로그인 계정을 실행 계정의 대체값으로 쓰지 않는다. 그 값은 S3의 프로브 계정 B이지 실행 계정 A가 아니다
And 1개 채택 분기로 실행 계정이 특정되더라도 그 계정의 카탈로그 프로브가 없으면 모델 가용성은 S8의 `unverified`로 남는다

## Oracle Acceptance Notes

이 SPEC은 설계 고정 문서이므로 위 시나리오는 지금 자동화된 테스트로 실행되지 않는다. S1·S2는 문서 검토로 즉시 판정 가능하고, S3~S9는 정합 게이트를 구현하는 후속 SPEC의 수용 오라클로 그대로 이월된다. 이월 시 아래 예상 값을 완화하는 것은 범위 축소로 취급한다.

각 시나리오의 판정 기준은 다음 구체 값으로 고정한다. `정상 동작한다` 같은 서술은 통과 근거가 아니다.

- **S1** — 예상 값: Feature Coverage Map 7개 행 각각의 소유 평면 수 = 1. 소유자 미지정 행 0건, 복수 소유 행 0건. 신설 표면 `티어와 실행 계정의 정합 검증`의 소유자 문자열은 정책 평면 하나여야 하며, 같은 표면 이름이 모델·프로세스 평면 목록에 재등장하면 실패.
- **S2** — 예상 값: 경계 인자 집합에서 `quality`=0, `tier`=0, `balanced`=0, `ultra`=0, `opus`=0, `sonnet`=0, `haiku`=0. 하나라도 1 이상이면 실패. `--model`, `--effort` 두 인자 외의 티어 파생 인자가 추가되어도 실패.
- **S3** — 예상 값: `receipt.execution_account == receipt.catalog_source_account == A`. 검증에 B의 카탈로그가 쓰였으면 실패. 판정은 A != B 조건에만 의존해야 하며, `bitgapnam@gmail.com`·`gnkong@alipeople.kr` 문자열을 하드코딩해 비교하는 오라클은 F1 시점의 계정 구성이 바뀌면 무의미해지므로 부적합으로 본다. 실측 계정 쌍은 A != B가 가설이 아니라 현재 상태임을 보이는 근거로만 쓴다. 두 값은 `orca account list --json` 한 응답의 `result.codex.activeAccountId`와 `result.codex.systemDefault`에서 나오므로, 프로브 계정을 별도 명령이나 로컬 파일에서 다시 읽는 구현은 같은 응답으로 대체 가능한 중복 조회로 본다.
- **S4** — 예상 값: 영수증 필드 6개가 모두 존재. 요청 티어 / 해석된 provider 모델 / 실행 계정 식별자 / 카탈로그 출처 / resolution reason / verification status. 이 중 하나라도 누락되거나 빈 문자열이면 실패로 판정한다. 스키마 버전 태그 부재도 실패.
- **S5** — 예상 값: `resolution.reason != ""`. 빈 문자열이면 위반. 요청 티어 값과 실제 제공 모델 값이 둘 다 non-empty여야 하고, 둘 중 하나만 있는 영수증은 실패. 영수증 없이 낮은 모델로 실행된 경우는 조용한 강등이므로 즉시 실패.
- **S6** — 예상 값: 점검 실패 후 신규 워크트리 수 = 0, Run 수 = 0, 워커 수 = 0, provider 세션 수 = 0. 어느 하나라도 1 이상이면 INV-005 위반. 실패 후 정리(cleanup)로 0에 도달한 경우도 생성이 발생했으므로 실패로 본다.
- **S7** — 예상 값: 통과 경로의 handoff 결과에 영수증 참조 필드가 1개 이상 존재. 실패 경로에서는 `status: handoff_required`가 반환되지 않고 정합 실패가 반환되어야 하며, 두 결과가 동시에 나오면 실패. 이 경로에서 생성된 OMP task DAG 수 = 0.
- **S8** — 예상 값: `verification_status == "unverified"`이고 사유 필드 non-empty. 같은 항목에 `verified`가 오면 실패. 프로브가 없다는 이유로 티어를 검증됨으로 낙관 처리하거나, 반대로 검증 실패(카탈로그에 모델 없음)와 같은 상태값으로 뭉뚱그리면 실패. Claude는 신원 대조(`orgId` == `organizationUuid`, 실측 `0e61c3e2-35c8-4bf8-91bc-40848f4dc147`)가 성공해도 모델 가용성 항목이 `unverified`여야 하고, `subscriptionType`을 근거로 `verified`를 만들면 실패. 원격 환경 대상은 `--environment` 거부로 계정 조회 자체가 불가능하므로 같은 `unverified`이며, 사유 문자열이 세 인스턴스(Claude 카탈로그 부재 / 원격 환경 / 계정 미특정) 중 어느 것인지 지목해야 한다.
- **S9** — 예상 값: `activeAccountId == null`일 때 등록 계정 수 n으로 분기가 갈린다. n == 1이면 `receipt.execution_account`가 그 계정으로 non-empty, n == 0 또는 n >= 2이면 `receipt.execution_account`가 특정되지 않고 `verification_status == "unverified"` + 사유 non-empty. n >= 2에서 임의의 한 계정을 골라 실행 계정 필드를 채운 영수증은 값이 있어도 실패로 본다. 현재 머신의 Claude는 n == 1(`jroad1049@gmail.com`)이므로 채택 분기가 도달 가능함을 보이는 근거로만 쓰고, 계정 문자열을 하드코딩해 비교하지 않는다.

판정 시 주의할 점 세 가지.

- S3와 S8은 경계가 인접하다. 실행 계정 A를 특정했으나 A의 카탈로그를 조회할 수단이 없는 경우는 S3의 통과가 아니라 S8의 `unverified`다. 이 분기를 통과로 세면 INV-003이 무력화된다.
- S5·S8·S9는 상태값이 서로 달라야 한다. 카탈로그를 조회했고 모델이 없어 강등한 경우(S5)는 사유가 붙은 명시적 강등, 계정은 특정했으나 조회 수단 자체가 없는 경우(S8)는 실행 계정 필드가 채워진 `unverified`, 실행 계정조차 특정하지 못한 경우(S9)는 실행 계정 필드가 비어 있는 `unverified`다. 셋을 한 값으로 합치거나 뒤의 둘을 사유 없이 같은 항목으로 묶으면 영수증 독자가 근거의 강도를 복원할 수 없다.
- 신원 검증 성공과 모델 가용성 `unverified`가 한 영수증에 함께 있는 것은 모순이 아니라 정상 상태다. Claude에서 `orgId`와 `organizationUuid`가 일치하면 "실행 계정이 검증 대상 계정과 같다"까지는 확정되지만, 그 계정이 어떤 모델을 쓸 수 있는지는 조회할 수단이 없다. 실행 계정 식별자는 확정값이고 verification status는 `unverified`인 조합을 읽고 결함으로 판정하면 안 된다. 두 축은 별개이며, 신원이 맞았다는 이유로 티어를 `verified`로 올리는 쪽이 실제 결함이다.

## Definition of Done

- [ ] S1~S9가 REQ-001~REQ-009를 빠짐없이 덮는다(REQ-003은 S3·S9, REQ-004는 S3, REQ-009는 S8·S9)
- [ ] 각 시나리오의 Then이 관측 가능한 값을 지명한다
- [ ] S3~S9의 예상 값이 후속 구현 SPEC의 수용 오라클로 이관된다
- [ ] 세 평면 경계에 대한 리뷰 합의가 기록된다
