# SPEC-EXECPLANE-001 Acceptance: 실행 평면 분리와 티어·계정·카탈로그 정합

이 문서는 설계 고정 SPEC의 수용 오라클이다. 시나리오는 구현 코드를 요구하지 않고, 각 요구가 무엇을 관측했을 때 충족으로 판정되는지를 확정한다.

## Test Scenarios

spec.md `## Traceability Matrix`에서 파생한 커버리지(권위 있는 매핑은 spec.md 쪽이다): S1=REQ-001, S2=REQ-002, S3=REQ-003·REQ-004, S4=REQ-005, S5=REQ-006, S6=REQ-007, S7=REQ-008, S8=REQ-009, S9=REQ-003·REQ-009, S10=REQ-004, S11=REQ-005, S12=REQ-004. 9개 요구가 12개 시나리오로 빠짐없이 덮인다. REQ-003은 S3(실행 계정과 프로브 계정의 자격 등급을 비교하는 경우)과 S9(실행 계정을 특정할 수 없는 경우) 둘로, REQ-004는 S3(등급 비교로 판정 경로가 갈리는 경우)·S10(카탈로그가 자격에 종속된다는 전제 자체)·S12(등급 동일 시 재프로브 0회와 등급 상이 시 실행 계정 기준 재프로브) 셋으로, REQ-005는 S4(영수증 7개 필드의 완전성)와 S11(`evidence_kind`가 같은 `verified` 아래에서 근거 강도를 구분하는 것) 둘로, REQ-009는 S8(검증 수단 부재)과 S9 둘로 나뉜다.

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

### S3: 실행 계정과 프로브 계정의 자격 등급을 비교해 기존 판정의 유효성을 가른다

Priority: Must
Given 프로세스 평면이 보고하는 활성 Codex 실행 계정은 `orca account list --json`의 `result.codex.activeAccountId` 기준 `bitgapnam@gmail.com`이고, 같은 응답의 `result.codex.systemDefault`가 가리키는 로컬 `codex` CLI 계정은 `gnkong@alipeople.kr`이다(research.md F1). 두 값은 한 번의 호출로 함께 얻으므로 비교에 추가 조회가 필요하지 않다
And 두 계정은 조직이 다르지만(`org-HPIVIN3REM0LDCuu56Y8bEWe` "Autopus" vs `org-PwLx2gQdz2XBgqkf5EafQJ3s` "Personal") 자격 등급은 둘 다 `chatgpt_plan_type: "pro"`이고, 이 경우 판정에 쓰이는 필드가 완전히 동일했다 — 슬러그 집합이 같고 `supported_reasoning_levels`가 모든 모델에서 같다(research.md F9). 즉 현재 이 워크스테이션은 등급 동일 분기에 해당한다
And Codex의 자격 등급은 `auth.json` id_token의 `https://api.openai.com/auth.chatgpt_plan_type` 클레임에서 네트워크 호출 없이 읽힌다
And 일반화하면 실행 계정의 자격 등급 P_exec와 프로브 계정의 자격 등급 P_probe가 있고, 이 시나리오의 판정은 두 계정의 구체 문자열이나 신원 일치 여부가 아니라 `P_exec == P_probe`인지에만 의존한다. 계정이 서로 달라도 등급이 같으면 기존 판정은 유효하다
And 카탈로그 프로브는 PATH 바이너리를 실행하므로 기본값은 프로브 계정의 카탈로그다(`pkg/codexruntime/probe.go:39`)
When 정합 점검이 요청 티어를 provider 모델로 해석한다
Then `P_exec == P_probe`이면 프로브 계정으로 얻은 기존 카탈로그 판정을 그대로 근거로 채택하고 실행 계정 기준 재프로브를 수행하지 않는다. 이 분기에서 발생한 카탈로그 프로브 호출 수는 0이고 네트워크 호출도 0이다
And `P_exec != P_probe`이면 기존 카탈로그는 근거에서 배제되고, 실행 계정 기준으로 다시 프로브한 카탈로그만 티어 근거가 된다
And 두 등급을 비교할 수 없거나 등급이 달라 재프로브가 필요한데 그 수단이 없으면 통과가 아니라 REQ-009의 unverified 경로(S8)로 분기한다
And 영수증의 카탈로그 출처에 프로브 계정과 그 자격 등급이 함께 남아, 독자가 세 분기 중 어디로 판정됐는지 복원할 수 있다
And 등급이 다른데 재프로브 없이 통과 판정을 만들면 INV-003 위반이다

### S4: 정합 영수증이 7개 필드를 모두 담는다

Priority: Must
Given 정합 점검을 마친 워크로드 하나
When 정합 영수증을 방출한다
Then 영수증은 요청 티어, 해석된 provider 모델, 실행 계정 식별자, 카탈로그 출처, resolution reason, verification status, `evidence_kind` 7개 필드를 모두 담는다
And 카탈로그 출처 필드는 프로브 계정 식별자와 그 계정의 자격 등급(Codex는 `chatgpt_plan_type`, Claude는 `organizationType` 값)을 함께 담는다. 계정만 있고 등급이 없는 영수증은 다른 여섯 필드가 채워져 있어도 불완전으로 판정되고 점검은 실패한다
And `evidence_kind` 필드는 `probed_catalog`·`entitlement_parity`·`none` 셋 중 하나를 담는다. 이 값이 비어 있으면 다른 여섯 필드가 모두 채워져 있어도 `Complete()`가 false이고 점검은 실패한다 — 증거 종류가 없는 영수증은 독자가 판정의 근거 강도를 복원할 수 없기 때문이다
And 영수증은 `pipeline_execution_owner_receipt.v1`과 같은 형태의 스키마 버전 태그를 갖는다
And 7개 필드 중 하나라도 부재하거나 빈 문자열이면 영수증은 불완전으로 판정되고 점검은 실패한다

### S5: 티어 불일치는 요청값과 실제값을 모두 남기고 fail-loud 한다

Priority: Must
Given 실행 계정과 같은 자격으로 얻은 카탈로그에 요청 티어가 해석한 모델이 존재하지 않는다
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

### S7: handoff 결과가 영수증과 판정 상태를 함께 드러낸다

Priority: Must
Given `--execution-owner orca` 경로가 `status: handoff_required`와 `required_action`을 담은 handoff 결과를 반환하는 스텁이다(`internal/cli/pipeline_run_owner.go:160-164`)
And 검증 수단이 없는 provider가 하나라도 있으면 집계 판정은 `unverified`가 되며, 이는 REQ-009가 인정한 정상 상태다
When 정합 점검이 handoff 결과 방출 직전에 실행된다
Then handoff 결과가 정합 영수증 경로와 verification status와 사유를 모두 담아, 받는 쪽이 티어 계약의 근거 강도를 알고 출발한다
And 판정이 `unverified`여도 handoff는 그대로 방출된다. 게이트는 판정을 기록하고 드러낼 뿐 기존 경로를 차단하지 않는다 — 검증 불가를 치명적으로 다루면 근거 없이 회귀를 만든다
And 사유는 비어 있지 않고 어느 provider가 왜 강등됐는지 지목하므로, 판정이 통과하지 못했다는 사실이 조용히 넘어가지 않는다
And fail-closed는 REQ-006의 더 강한 조건(실행 계정이 요청 티어를 제공할 수 없음)에 속하며 이 시나리오의 범위가 아니다
And 이 경로는 OMP task DAG를 만들지 않으므로 DAG 소유자는 하나로 유지된다(`pkg/adapter/omp/omp_workflow_render.go:99`)

### S8: 검증 수단이 없는 항목은 unverified로 표기된다

Priority: Must
Given REQ-009가 흡수하는 인스턴스는 넷이다. (1) 원격 orca 환경 — `orca account list`가 `--environment`를 "rejected rather than ignored"로 거부하므로 원격 호스트의 계정을 조회할 수단이 없다. `orca environment list --json`이 `{"environments": []}`라 현재 영향 범위는 0건이지만 규칙은 같다(research.md F8)
And (2) 실행 계정 미특정 — 활성 계정이 없고 등록 계정이 1개가 아닌 경우다. 이 인스턴스의 판정은 S9가 담당하고 S8은 같은 `unverified` 규칙을 공유한다는 사실만 참조한다
And (3) 등급 비교 불가 — 자격증명 자체가 없거나 등급 클레임(Codex `chatgpt_plan_type`, Claude `organizationType`)을 담지 않아 P_exec와 P_probe의 비교가 성립하지 않는 경우다. 등급을 읽지 못한 것을 등급이 같은 것으로 낙관 처리하면 실패다
And (4) 보유 증거 부재 — `evidence_kind`가 `none`인 provider다. 등급이 **완전히 동일**해도 parity는 증거를 옮길 뿐 만들지 않으므로 `verified`가 될 수 없다(research.md F10). `Evaluate`가 `verified`에 요구하는 세 조건(실행 계정 determined + 등급 동일 + 증거 보유) 중 셋째가 빠진 상태다
And Claude 모델 가용성은 더 이상 이 목록에 속하지 않는다. 관리 계정(`<support>/orca/claude-accounts/<accountId>/auth/oauth-account.json`)과 호스트 CLI(`~/.claude.json`의 `oauthAccount`)가 동일 필드명 `organizationType`으로 등급을 노출하므로, Claude는 `entitlement_parity` 증거로 `verified`가 될 수 있다(research.md F10). 이 워크스테이션 실측은 양쪽 다 `claude_max`이고 판정은 `verified`다
When 그 항목의 요청 티어에 대해 정합 점검을 수행한다
Then 영수증의 verification status 값이 `unverified`다
And 같은 항목이 `verified`로 표기되지 않는다
And 네 번째 인스턴스의 사유는 등급 불일치 사유와 문자열로 구별된다. "등급은 같음을 확인했으나 옮길 증거가 없다"와 "등급이 다르다"는 서로 다른 관측이며, 둘을 한 사유로 뭉뚱그리면 독자가 재프로브가 필요한 상황인지 아닌지를 복원할 수 없어 실패다
And `subscriptionType`(`max`/`pro`/`free`)이나 `organizationType`에서 플랜 -> 모델 표를 유도해 `verified`를 만들어내지 않는다. 그것은 PR #154가 제거한 하드코딩 티어 테이블의 재도입이다. 등급은 동일성 비교에만 쓰인다
And 사유 필드에 검증 수단 부재가 기록되어 미검증과 검증 실패가 구분되고, 네 인스턴스 중 어느 것인지 복원 가능하다

### S9: 실행 계정을 특정할 수 없으면 추측하지 않고 unverified로 분기한다

Priority: Must
Given 프로세스 평면이 보고하는 어떤 provider의 `activeAccountId`가 `null`이다
And 이 워크스테이션의 Claude가 정확히 그 상태다. `result.claude.activeAccountId`는 `null`이고 `result.claude.accounts[]`에는 `jroad1049@gmail.com`(organizationUuid `0e61c3e2-35c8-4bf8-91bc-40848f4dc147`) 하나만 등록되어 있으며, Claude 블록에는 codex와 달리 `systemDefault` 필드가 없다(research.md F1)
When 정합 점검이 그 provider의 실행 계정을 확정한다
Then 등록 계정이 정확히 1개면 그 계정을 실행 계정으로 채택하고, 영수증의 실행 계정 식별자가 그 값으로 채워진다. 현재 머신의 Claude가 이 분기이며 `jroad1049@gmail.com`이 채택된다
And 등록 계정이 0개면 실행 계정을 특정하지 않고 REQ-009의 `unverified` 경로로 분기하며, 사유 필드에 계정 부재가 기록된다
And 등록 계정이 2개 이상이면 어느 것도 고르지 않고 REQ-009의 `unverified` 경로로 분기한다. 활성 계정이 없다는 이유로 첫 번째 계정이나 최근 사용 계정을 고르면 실패다
And 세 분기 어디에서도 로컬 CLI 로그인 계정을 실행 계정의 대체값으로 쓰지 않는다. 그 값은 S3의 프로브 계정이지 실행 계정이 아니며, 두 계정의 자격 등급이 같더라도 영수증의 실행 계정 필드를 그 값으로 채우면 실패다
And 1개 채택 분기로 실행 계정이 특정되면 그다음 판정은 등급 동일성과 증거 종류가 가른다. Claude는 계정 스코프 카탈로그가 없어도 `organizationType` 등급이 같으면 `entitlement_parity` 증거로 `verified`가 되고, 등급을 읽을 수 없거나 옮길 증거가 없으면 S8의 `unverified`로 남는다

### S10: 카탈로그는 자격에 종속되므로 자격과 무관한 재사용은 위반이다

Priority: Must
Given 같은 워크스테이션에서 `codex debug models`를 두 번 프로브했다. 한 번은 인증된 `auth.json`을 둔 임시 `CODEX_HOME`으로, 한 번은 자격이 없는 상태로 실행했다(research.md F9)
And 인증 프로브의 슬러그 집합은 `gpt-5.6-sol` `gpt-5.6-sol-wm` `gpt-5.6-terra` `gpt-5.6-luna` `gpt-5.5` `gpt-5.4` `gpt-5.4-mini` `gpt-5.3-codex-spark` `codex-auto-review`이고, 미인증 프로브에서는 `gpt-5.6-sol-wm`과 `gpt-5.3-codex-spark`가 사라진 대신 `gpt-5.2`가 나타난다
When 두 프로브의 슬러그 집합을 비교한다
Then 두 집합은 같지 않다. 서버가 호출자의 자격을 보고 목록을 바꾼다는 사실이 관측값으로 고정되며, 이것이 REQ-004가 자격 등급 비교를 요구하는 전제다
And 따라서 카탈로그를 자격과 무관한 것으로 취급하는 구현은 REQ-004 위반이다. 자격 등급을 키에 넣지 않은 카탈로그 캐시, 자격이 바뀐 뒤에도 이전 결과를 그대로 재사용하는 경로, 자격이 확인되지 않은 상태에서 얻은 목록을 티어 근거로 채택하는 판정 셋 다 해당한다
And 이 시나리오는 REQ-004의 전제를 고정할 뿐 자격에서 모델을 유추하지 않는다. 자격 등급은 S3에서 동일성 비교에만 쓰이며 등급 -> 모델 매핑 표를 만들지 않는다

### S11: 같은 verified 안에서도 증거 종류가 구분되어 기록된다

Priority: Must
Given 이 워크스테이션에서 게이트를 종단 실행한 결과 집계 판정은 `verification_status: verified`이고 사유는 `claude: execution and probe accounts share entitlement claude_max; codex: execution and probe accounts share entitlement pro`다
And 그 영수증에서 claude 항목의 실행 계정은 `jroad1049@gmail.com`, 카탈로그 출처는 `jroad1049@gmail.com` @ `claude_max`, `evidence_kind`는 `entitlement_parity`이고, codex 항목의 실행 계정은 `bitgapnam@gmail.com`, 카탈로그 출처는 `gnkong@alipeople.kr` @ `pro`, `evidence_kind`는 `probed_catalog`다. 두 provider가 **둘 다** `verified`인 상태에서 관측된 값이다
And 두 판정의 근거 강도는 같지 않다. codex는 provider가 내려준 카탈로그를 보유하므로 등급 동일성이 그 카탈로그를 실행 계정으로 옮기지만, claude는 카탈로그가 없어 등급 동일성이 기준 세션의 작업 가정만 옮긴다(research.md F10)
When 독자가 그 영수증을 읽고 각 provider 판정이 무엇에 기대고 있는지 복원한다
Then 두 항목의 `evidence_kind`가 `probed_catalog`(codex)와 `entitlement_parity`(claude)로 서로 다른 값으로 기록되어, 같은 `verified` 아래에서도 근거 강도가 구분된다
And 두 값을 하나로 뭉뚱그리면 위반이다. 둘 다 `verified`라는 이유로 같은 증거 종류를 부여하거나, `evidence_kind`를 영수증에서 빼면 이 SPEC이 막으려던 증거 강도의 조용한 뭉갬이 그대로 재발한다
And `evidence_kind`는 verification status를 대체하지 않는다. 두 필드는 함께 존재하며 `verified` + `probed_catalog`와 `verified` + `entitlement_parity`는 서로 다른 관측값이다
And 실측 계정 문자열과 등급 값은 두 증거 종류가 한 영수증에 공존함을 보이는 근거로만 쓰고, 오라클이 그 문자열을 하드코딩해 비교하지 않는다

### S12: 등급이 같으면 재프로브하지 않고, 다르면 실행 계정 기준으로 다시 프로브한다

Priority: Must
Given Codex 카탈로그 프로브는 `CODEX_HOME`이 가리키는 자격으로 `codex debug models`를 실행하므로, 어느 계정의 카탈로그를 받을지는 그 환경 변수가 결정한다(`pkg/codexruntime/probe.go:39`, research.md F9)
And 실행 계정의 자격 등급 P_exec와 프로브 계정의 자격 등급 P_probe는 각 `auth.json` id_token의 `https://api.openai.com/auth.chatgpt_plan_type` 클레임에서 네트워크 호출 없이 읽힌다
When 정합 점검이 두 등급을 비교하고 그 결과로 재프로브 여부를 결정한다
Then `P_exec == P_probe`이면 재프로브 횟수가 **0회**이고 그 분기에서 발생한 네트워크 호출도 0회다. 등급이 같을 때 카탈로그를 다시 받지 않는 것이 이 설계의 핵심이므로, 이 분기에서 재프로브가 한 번이라도 일어나면 실패다. 현재 이 워크스테이션의 codex(실행 `bitgapnam@gmail.com`, 프로브 `gnkong@alipeople.kr`, 둘 다 `pro`)가 이 분기이며 재프로브 0회로 `verified`가 됐다
And `P_exec != P_probe`이면 실행 계정의 `CODEX_HOME`을 기준으로 재프로브를 1회 수행한다. 성공하면 영수증의 `catalog_source`가 프로브 계정이 아니라 실행 계정을 가리키고, 그 카탈로그만 티어 근거가 되며 판정은 `verified`가 될 수 있다. `evidence_kind`는 `probed_catalog`로 남는다
And 재프로브가 실패하면 판정은 `unverified`이고, 사유에 재프로브 실패가 등급 불일치와 **함께** 등장한다. 둘 중 하나만 남기면 독자가 "등급이 달랐다"와 "다시 확인하려 했으나 실패했다"를 구분할 수 없다
And 등급이 다른데 재프로브 없이 이전 프로브 계정의 카탈로그가 근거로 남아 통과 판정이 나오면 INV-003 위반이다
And Claude는 계정 스코프 카탈로그가 없으므로 등급이 달라도 재프로브하지 않는다. 옮길 카탈로그가 없어 재시도할 대상 자체가 없으므로 REQ-009의 `unverified` 경로(S8)로 떨어지며, 그 사유는 재프로브 실패가 아니라 증거 부재로 기록된다

## Oracle Acceptance Notes

이 SPEC은 설계 고정 문서이므로 위 시나리오는 지금 자동화된 테스트로 실행되지 않는다. S1·S2는 문서 검토로 즉시 판정 가능하고, S10의 관측값은 research.md F9의 실측 프로브로, S11·S12의 등급 동일 분기 관측값은 이 워크스테이션의 종단 실행으로 이미 한 번 고정되어 있으며, S3~S9와 S11·S12는 정합 게이트를 구현하는 후속 SPEC의 수용 오라클로 그대로 이월된다. 이월 시 아래 예상 값을 완화하는 것은 범위 축소로 취급한다.

각 시나리오의 판정 기준은 다음 구체 값으로 고정한다. `정상 동작한다` 같은 서술은 통과 근거가 아니다.

- **S1** — 예상 값: Feature Coverage Map 7개 행 각각의 소유 평면 수 = 1. 소유자 미지정 행 0건, 복수 소유 행 0건. 신설 표면 `티어와 실행 계정의 정합 검증`의 소유자 문자열은 정책 평면 하나여야 하며, 같은 표면 이름이 모델·프로세스 평면 목록에 재등장하면 실패.
- **S2** — 예상 값: 경계 인자 집합에서 `quality`=0, `tier`=0, `balanced`=0, `ultra`=0, `opus`=0, `sonnet`=0, `haiku`=0. 하나라도 1 이상이면 실패. `--model`, `--effort` 두 인자 외의 티어 파생 인자가 추가되어도 실패.
- **S3** — 예상 값: `receipt.catalog_source`가 프로브 계정과 그 자격 등급을 함께 담고, 그 등급이 실행 계정의 등급과 같다. 등급이 같은 분기에서는 **카탈로그 프로브 호출 수 = 0**이 기대값이다. 재프로브가 한 번이라도 일어나면 설계 단순화의 핵심이 무너진 것이므로 실패로 본다. 등급이 다른 분기에서는 실행 계정 기준 재프로브 결과만 근거여야 하고, 이전 카탈로그가 근거로 남아 있으면 실패. 등급 비교도 재프로브도 불가능하면 통과가 아니라 S8의 `unverified`다. **계정 식별자가 다르다는 것만으로 실패를 판정하는 오라클은 부적합하다** — 조직만 다르고 등급이 같은 실측 쌍(`bitgapnam@gmail.com` vs `gnkong@alipeople.kr`, 둘 다 `chatgpt_plan_type: "pro"`)을 오탐하기 때문이다. 같은 이유로 계정 문자열을 하드코딩해 비교하는 오라클도 F1 시점의 계정 구성이 바뀌면 무의미해지므로 부적합이다. 실측 쌍은 등급 동일 분기가 현재 상태임을 보이는 근거로만 쓴다. 두 계정 값은 `orca account list --json` 한 응답의 `result.codex.activeAccountId`와 `result.codex.systemDefault`에서, 두 등급은 각 계정 `auth.json` id_token의 `chatgpt_plan_type`에서 네트워크 호출 없이 나오므로, 프로브 계정이나 등급을 별도 명령으로 다시 읽는 구현은 중복 조회로 본다.
- **S4** — 예상 값: 영수증 필드 7개가 모두 존재. 요청 티어 / 해석된 provider 모델 / 실행 계정 식별자 / 카탈로그 출처 / resolution reason / verification status / `evidence_kind`. 이 중 하나라도 누락되거나 빈 문자열이면 실패로 판정한다. 특히 `evidence_kind`가 비어 있으면 다른 여섯 필드가 모두 채워져 있어도 `Complete()`는 false여야 하며, true를 반환하면 실패. 카탈로그 출처는 프로브 계정 식별자와 그 자격 등급 두 값을 모두 담아야 하며, 계정만 담고 등급이 없는 영수증은 필드 개수가 7이어도 불완전으로 실패. 스키마 버전 태그 부재도 실패.
- **S5** — 예상 값: `resolution.reason != ""`. 빈 문자열이면 위반. 요청 티어 값과 실제 제공 모델 값이 둘 다 non-empty여야 하고, 둘 중 하나만 있는 영수증은 실패. 영수증 없이 낮은 모델로 실행된 경우는 조용한 강등이므로 즉시 실패.
- **S6** — 예상 값: 점검 실패 후 신규 워크트리 수 = 0, Run 수 = 0, 워커 수 = 0, provider 세션 수 = 0. 어느 하나라도 1 이상이면 INV-005 위반. 실패 후 정리(cleanup)로 0에 도달한 경우도 생성이 발생했으므로 실패로 본다.
- **S7** — 예상 값: 통과 경로의 handoff 결과에 영수증 참조 필드가 1개 이상 존재. 실패 경로에서는 `status: handoff_required`가 반환되지 않고 정합 실패가 반환되어야 하며, 두 결과가 동시에 나오면 실패. 이 경로에서 생성된 OMP task DAG 수 = 0.
- **S8** — 예상 값: `verification_status == "unverified"`이고 사유 필드 non-empty. 같은 항목에 `verified`가 오면 실패. 프로브가 없다는 이유로 티어를 검증됨으로 낙관 처리하거나, 반대로 검증 실패(카탈로그에 모델 없음)와 같은 상태값으로 뭉뚱그리면 실패. 사유 문자열은 네 인스턴스(원격 환경 / 계정 미특정 / 등급 비교 불가 / 증거 부재) 중 어느 것인지 지목해야 한다. 원격 환경 대상은 `--environment` 거부로 계정 조회 자체가 불가능하므로 `unverified`. 증거 부재 인스턴스는 `evidence_kind == "none"`이고 등급이 동일하게 관측되더라도 `verified`가 되면 실패이며, 그 사유가 등급 불일치 사유와 같은 문자열로 축약되면 실패. **Claude 모델 가용성을 이 목록에 남겨 둔 오라클은 부적합하다** — `organizationType` 등급 소스가 있어 Claude는 `entitlement_parity`로 `verified`가 될 수 있고(실측 `claude_max`), 이를 `unverified`로 강제하면 과소 판정이다. 플랜 -> 모델 매핑을 유도해 `verified`를 만드는 반대 방향도 여전히 실패.
- **S9** — 예상 값: `activeAccountId == null`일 때 등록 계정 수 n으로 분기가 갈린다. n == 1이면 `receipt.execution_account`가 그 계정으로 non-empty, n == 0 또는 n >= 2이면 `receipt.execution_account`가 특정되지 않고 `verification_status == "unverified"` + 사유 non-empty. n >= 2에서 임의의 한 계정을 골라 실행 계정 필드를 채운 영수증은 값이 있어도 실패로 본다. 현재 머신의 Claude는 n == 1(`jroad1049@gmail.com`)이므로 채택 분기가 도달 가능함을 보이는 근거로만 쓰고, 계정 문자열을 하드코딩해 비교하지 않는다.
- **S10** — 예상 값: 인증 프로브와 미인증 프로브의 슬러그 집합이 다르고, 차이가 정확히 인증 전용 `gpt-5.6-sol-wm`·`gpt-5.3-codex-spark` 대 미인증 전용 `gpt-5.2`다. 두 집합이 같게 관측되면 REQ-004의 전제가 흔들린 것이므로 재측정 전까지 계약을 그대로 두는 판단은 실패로 본다. 구현 판정으로는 자격 등급이 키에 포함되지 않은 카탈로그 캐시 경로 수 = 0, 자격이 확인되지 않은 상태에서 얻은 목록을 티어 근거로 채택한 사례 수 = 0.
- **S11** — 예상 값: 두 provider 모두 `verification_status == "verified"`인 영수증에서 `evidence_kind`가 codex는 `"probed_catalog"`, claude는 `"entitlement_parity"`로 서로 다른 값이다. 두 항목의 `evidence_kind`가 같은 값이면, 또는 필드가 없으면 실패. 실측 대응은 claude(실행 `jroad1049@gmail.com`, 출처 `jroad1049@gmail.com` @ `claude_max`)와 codex(실행 `bitgapnam@gmail.com`, 출처 `gnkong@alipeople.kr` @ `pro`)이며, 이 값들은 두 증거 종류의 공존을 보이는 근거로만 쓰고 하드코딩 비교 대상으로 삼지 않는다. `evidence_kind`를 verification status로 대체하거나 둘을 한 필드로 합친 영수증은 실패.
- **S12** — 예상 값: 등급 동일 분기의 **재프로브 호출 수 = 0, 네트워크 호출 수 = 0**. 1 이상이면 실패이며, 이것이 S12에서 가장 먼저 확인할 값이다. 등급 상이 분기에서는 실행 계정 `CODEX_HOME` 기준 재프로브 호출 수 = 1이고, 성공 시 `receipt.catalog_source`의 계정이 실행 계정과 같아야 한다. 이전 프로브 계정이 `catalog_source`에 남아 있으면 실패. 재프로브 실패 시 `verification_status == "unverified"`이고 사유 문자열에 등급 불일치와 재프로브 실패가 **둘 다** 등장해야 하며, 하나만 있으면 실패. Claude 경로에서 재프로브 호출이 관측되면 실패다(카탈로그가 없어 재시도 대상이 없다). 다른 자격 등급의 계정 쌍은 이 워크스테이션에 없으므로 등급 상이 분기는 픽스처로 검증하고, 등급 동일 분기만 실측으로 고정한다.

판정 시 주의할 점 세 가지.

- S3와 S8은 경계가 인접하다. 실행 계정을 특정했으나 그 자격 등급을 읽을 수 없거나, 등급이 달라 재프로브가 필요한데 조회 수단이 없는 경우는 S3의 통과가 아니라 S8의 `unverified`다. 이 분기를 통과로 세면 INV-003이 무력화된다. 반대 방향도 오판이다. 등급이 같음을 확인한 경우는 재프로브 없이도 정상 통과이며, 계정이 다르다는 이유로 이를 `unverified`나 실패로 내리면 과잉 판정이다.
- S5·S8·S9는 상태값이 서로 달라야 한다. 카탈로그를 조회했고 모델이 없어 강등한 경우(S5)는 사유가 붙은 명시적 강등, 계정은 특정했으나 조회 수단 자체가 없는 경우(S8)는 실행 계정 필드가 채워진 `unverified`, 실행 계정조차 특정하지 못한 경우(S9)는 실행 계정 필드가 비어 있는 `unverified`다. 셋을 한 값으로 합치거나 뒤의 둘을 사유 없이 같은 항목으로 묶으면 영수증 독자가 근거의 강도를 복원할 수 없다.
- 같은 `verified`라도 `evidence_kind`가 다르면 근거 강도가 다르다. codex의 `verified` + `probed_catalog`는 provider가 실제로 내려준 카탈로그가 등급 동일성을 타고 실행 계정으로 옮겨진 상태이고, claude의 `verified` + `entitlement_parity`는 옮겨진 것이 기준 세션의 작업 가정뿐인 상태다. 두 영수증을 같은 강도로 읽거나, 반대로 `entitlement_parity`라는 이유로 claude 판정을 결함으로 내리는 쪽 모두 오판이다. 실제 결함은 셋 중 하나다. 두 증거 종류를 한 값으로 합치는 것, `evidence_kind`를 영수증에서 빼는 것, 그리고 `evidence_kind`가 `none`인데 등급이 같다는 이유만으로 `verified`를 부여하는 것 — 마지막은 parity가 증거를 옮길 뿐 만들지 않는다는 규칙을 정면으로 어긴다.

## Definition of Done

- [ ] S1~S12가 REQ-001~REQ-009를 빠짐없이 덮는다(REQ-003은 S3·S9, REQ-004는 S3·S10·S12, REQ-005는 S4·S11, REQ-009는 S8·S9)
- [ ] 각 시나리오의 Then이 관측 가능한 값을 지명한다
- [ ] S3~S12의 예상 값이 후속 구현 SPEC의 수용 오라클로 이관된다
- [ ] 세 평면 경계에 대한 리뷰 합의가 기록된다
