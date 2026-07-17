# SPEC-ADK-DRIFT-GATE-001 수락 기준

각 Must 시나리오는 oracle-first다. 구조 신호(제목/경로 존재)만으로 닫지 않고 concrete expected output(예상 값)과 정확 일치를 요구한다. 아래 시나리오는 expected json 값과 예상 출력을 명시한다.

## Test Scenarios

### S1: 내용 드리프트 감지 (Must, oracle)
Given claude-code 설치 표면의 결정적 파일 `.claude/workflows/route_team.workflow.js` 바이트가 현재 바이너리 생성 바이트와 다르고 설치본에는 모델 id `claude-sonnet-4-6`, 생성본에는 `claude-sonnet-5`가 들어 있다.
When `auto doctor --json`을 실행한다.
Then JSON checks 배열에 `doctor.drift.content.claude-code` check가 있고 status의 예상 값은 `warn`, drift count의 예상 값은 `1`, detail은 `route_team.workflow.js`를 포함하며 remediation 문자열은 `auto update`를 포함한다.
And 이 check의 severity 예상 값은 `warning`이다.

### S2: 무드리프트 + 환경 의존 파일 제외 (Must, oracle)
Given claude-code 설치 표면이 현재 바이너리 생성 내용과 바이트 동일하게 방금 생성됐고, 사용자가 별도 statusline 명령을 설정해 `.claude/statusline-user-command.txt`가 사용자 값을 담고 있다.
When `auto doctor --json`을 실행한다.
Then `doctor.drift.content.claude-code` check의 status 예상 값은 `pass`이고 drift count 예상 값은 `0`이며, `statusline-user-command.txt`는 `InspectStatusLine` 기반 환경 의존 파일이라 결정성 게이트에서 제외되어 count에 기여하지 않는다.

### S3: marker·merge 정책 제외 (Must, oracle)
Given `CLAUDE.md`(marker 정책)에 AUTOPUS 마커 밖 사용자 작성 내용이 있고 `.mcp.json`·`settings.json`(merge 정책)에 사용자 항목이 있다.
When `auto doctor`가 내용 드리프트를 비교한다.
Then 드리프트 경로 목록에 `CLAUDE.md`·`.mcp.json`·`settings.json`은 나타나지 않고 marker·merge 파일에서 유래한 drift count의 예상 값은 `0`이다.

### S4: 고아 manifest (Must, oracle)
Given `autopus.yaml`의 platforms가 `[claude-code, codex, antigravity-cli, opencode]`이고 `.autopus/`에 `antigravity-cli-manifest.json`과 `gemini-cli-manifest.json`이 함께 놓여 있다.
When `auto doctor --json`을 실행한다.
Then `doctor.drift.orphan_manifest` check의 status 예상 값은 `warn`, count 예상 값은 `1`, paths 예상 값은 정확히 `.autopus/gemini-cli-manifest.json` 하나이며 구성된 `antigravity-cli-manifest.json`은 목록에 없다.
And detail은 제거 힌트 문자열 `rm`을 포함한다.

### S5: 소스 repo 템플릿 재생성 드리프트 (Should, oracle)
Given 디렉토리가 ADK 소스 repo이고 `content/skills/agent-pipeline.md`가 현재 `templates/codex/skills/agent-pipeline.md.tmpl`이 담은 내용과 다르게(미재생성) 있다.
When `auto doctor --json`을 실행한다.
Then `doctor.drift.template_regen` check의 status 예상 값은 `warn`이고 detail은 `agent-pipeline.md.tmpl`과 `generate-templates` 힌트를 포함한다.
And 같은 실행을 `content/`·`templates/`가 없는 최종 사용자 설치 디렉토리에서 하면 `doctor.drift.template_regen` check는 결과에서 빠진다(예상 값: check 부재).

### S6: 바이너리 스테일함 접두사 비교 (Should, oracle)
Given 실행 바이너리 빌드 커밋이 7자 `a1b2c3d`이고 소스 repo HEAD 전체 해시가 `a1b2c3d9e8f70011223344556677889900aabbcc`다.
When `auto doctor --json`을 소스 repo에서 실행한다.
Then `a1b2c3d`가 HEAD 전체 해시의 접두사이므로 `doctor.drift.binary_stale` check의 status 예상 값은 `pass`다(길이 차이만으로 오탐하지 않는다).
And Given 빌드 커밋 `a1b2c3d`이고 HEAD 전체 해시가 `f0e1d2c3b4a5000000000000000000000000abcd`면 접두사 불일치라 status 예상 값은 `warn`이고 detail은 `a1b2c3d`와 HEAD 접두사 `f0e1d2c`, rebuild 힌트 `go build`를 포함한다.
And Given git 미가용이거나 비 git repo면 `doctor.drift.binary_stale` check는 결과에서 빠진다(graceful skip, 예상 값: check 부재).

### S7: advisory 비차단 (Must, oracle)
Given 내용 드리프트와 고아 manifest가 동시에 존재한다.
When `auto doctor --json`을 실행한다.
Then 모든 드리프트 check의 status는 `warn`이지만 envelope의 `data.overall_ok` 예상 값은 `true`로 유지된다(드리프트는 git hygiene과 달리 advisory).
And `doctor.drift.content.<platform>`와 `doctor.drift.orphan_manifest` 두 check가 JSON 출력에 모두 존재한다.

### S8: 규칙 문서 드리프트 게이트 언급 (Should, oracle)
Given `[NEW]` content 규칙 파일이 드리프트 게이트를 문서화한다.
When 해당 규칙 파일 본문을 검사한다.
Then 드리프트 게이트를 가리키는 1~2줄 언급이 담겨 있고 expected 부분 문자열 `drift`(또는 `드리프트`)와 `auto doctor`가 그 언급 라인에 함께 포함된다.

## Oracle Acceptance Notes

이 SPEC의 oracle acceptance는 concrete expected output(예상 값)으로 닫는다. 구조 신호(섹션 제목/경로 존재)만으로는 Must를 충족하지 않는다.

- S1 oracle: 입력 `route_team.workflow.js`에 `claude-sonnet-4-6`(설치본) vs `claude-sonnet-5`(생성본). expected json = `doctor.drift.content.claude-code` status `warn`, count `1`, detail이 `route_team.workflow.js` 포함. 정확 일치.
- S2 oracle(INV-001 결정성 경계): 무드리프트 + `statusline-user-command.txt` 사용자 값 존재 → 같은 check status `pass`, count `0`. 환경 의존 파일이 있어도 count가 0인 것이 결정성 게이트의 concrete 증거다.
- S3 oracle: marker(`CLAUDE.md`)·merge(`.mcp.json`/`settings.json`)에서 유래한 drift count `0`(제외 불변식).
- S4 oracle: 입력 platforms `[claude-code, codex, antigravity-cli, opencode]` + `.autopus/{antigravity-cli,gemini-cli}-manifest.json`. expected json = `doctor.drift.orphan_manifest` count `1`, paths 정확히 `.autopus/gemini-cli-manifest.json`.
- S5 oracle: 소스 repo에서 `agent-pipeline.md.tmpl` 미재생성 → `doctor.drift.template_regen` `warn`; 비소스 repo → check 부재.
- S6 oracle(INV-005 접두사 비교): 빌드 `a1b2c3d`(7자) vs HEAD 전체 `a1b2c3d9e8f7...` → 접두사 일치 → `pass`; HEAD 전체 `f0e1d2c3...` → 불일치 → `warn`(두 값 포함); git 미가용 → check 부재. 길이 차이(7 vs 40) 자체로는 오탐하지 않는다.
- S7 oracle: 드리프트 존재해도 `data.overall_ok` 예상 값 `true`(advisory 불변식).
- S8 oracle: `[NEW]` 규칙 파일 언급 라인에 expected 부분 문자열 `drift`/`드리프트` + `auto doctor` 포함.
- REQ-009 패리티 oracle: S1의 `auto update`와 S4의 `rm` 힌트는 플랫폼 무관 동일 문자열이라 4플랫폼 사용자 모두에게 유효하다.

expected json 값과 예상 출력의 정확 일치를 요구하므로 numeric/paired/parser 계열 invariant(INV-001·003·005)는 concrete expected value로 검증된다.
