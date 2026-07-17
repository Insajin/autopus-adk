# SPEC-ADK-EVIDENCE-LOOPS-001 수락 기준

각 Must 시나리오는 oracle-first다. 파일 존재·heading·exit code·비어있지 않은 출력만으로 닫지 않고 concrete expected output(반환 행 수, skip count, 정규식, JSON status/값)과 정확 일치를 요구한다.

## Test Scenarios

### S1: Fallback 수용 — `+0900`은 skip이 아니다 (Must, oracle)
Given `pipeline.jsonl`에 정상 엔트리 2개(timestamp offset `+09:00`와 `+00:00`)와 offset `+0900`(콜론 없음) 1개가 있고 셋 다 packages `["pkg/learn"]`을 담는다.
When `auto learn query --packages pkg/learn`을 실행한다.
Then exit code 예상 값은 `0`이고 반환 행 수 예상 값은 `3`이며 `+0900` 엔트리(예: 그 ID)가 결과에 포함되고 skip 경고는 나타나지 않는다(skip count 예상 값 `0`).

### S2: Tolerant skip — 손상 줄은 건너뛰고 카운트한다 (Must, oracle)
Given `pipeline.jsonl`에 정상 엔트리 2개와, 3번째 줄에 파싱 불가한 손상 줄(예: `{"id":"L-BAD","timestamp":`처럼 잘린 JSON) 1개가 있다.
When `auto learn query --packages pkg/learn`을 실행한다.
Then exit code 예상 값은 `0`이고 반환 행 수 예상 값은 `2`이며 skip count 예상 값은 `1`이고 경고 문자열이 손상 줄의 줄번호 `3`을 포함한다.

### S3: Canonical 직렬화 + 재작성 보존 (Must, oracle)
Given `pipeline.jsonl`에 최신(age 0) offset `+0900`(콜론 없음) 생존 엔트리 1개, 60일 전 timestamp의 정상 엔트리 1개(age-out 대상), 파싱 불가한 손상 줄 1개(원문 `GARBAGE-LINE-XYZ`)가 있다.
When `auto learn prune --max-age 30`을 실행한다(60일 전 엔트리가 실제로 age-out되어 `pruned>=1`로 `rewriteStore`가 호출됨; `prune.go`의 `pruned==0` short-circuit을 피한다).
Then prune 제거 수 예상 값은 `1`이고, 생존한 `+0900` 엔트리의 timestamp가 정규식 `T[0-9:.]+(Z|[+-][0-9]{2}:[0-9]{2})"`을 만족하며 콜론 없는 `+0900` 문자열은 파일에서 예상 값 `0`회 등장하고(`+09:00`으로 정규화), 손상 줄 `GARBAGE-LINE-XYZ`는 재작성 후에도 원본 상대 순서를 유지한 채 `1`회 보존된다.

### S4: 재작성이 손상 줄을 보존한다 (Must, oracle)
Given `pipeline.jsonl`에 정상 엔트리 2개와 파싱 불가한 손상 줄 1개(원문 `GARBAGE-LINE-XYZ` 포함)가 있다.
When 정상 엔트리 하나를 age-out시키는 `auto learn prune`을 실행한다.
Then 재작성 후 파일에서 `GARBAGE-LINE-XYZ`를 담은 원본 줄의 등장 횟수 예상 값은 `1`이며(보존됨, 원본 상대 순서 유지), 삭제 대상이던 정상 엔트리만 사라진다.

### S5: `--spec` 정확 필터 (Must, oracle)
Given `pipeline.jsonl`에 SpecID `SPEC-A` 엔트리 2개, `SPEC-B` 엔트리 1개, spec_id 없는 엔트리 1개가 있다.
When `auto learn query --spec SPEC-A`를 실행한다.
Then 반환 행 수 예상 값은 `2`이고 두 행 모두 SpecID `SPEC-A`이며 `SPEC-B`와 빈 spec_id 엔트리는 결과에 없다.

### S6: Doctor 신선도 — fresh (Must, oracle)
Given 워크스페이스에 최신 timestamp(age 0일) learnings 엔트리, `.autopus/canary/latest.json`(방금 시각), `DefaultIndexPath` 인덱스 파일(방금 mtime)이 모두 있다.
When `auto doctor --json`을 실행한다.
Then `doctor.evidence.learnings`·`doctor.evidence.canary`·`doctor.evidence.memindex` 세 check의 status 예상 값은 모두 `pass`이고 `data.overall_ok` 예상 값은 `true`다.

### S7: Doctor 신선도 — stale + 힌트 (Must, oracle)
Given 세 anchor가 모두 40일 전(30일 임계 초과)으로 세팅됐다.
When `auto doctor --json`을 실행한다.
Then 세 check의 status 예상 값은 모두 `warn`, severity 예상 값은 `warning`이며, learnings detail은 `auto learn record`, canary detail은 `auto canary`, memindex detail은 `auto mem rebuild`를 각각 포함하고, advisory이므로 `data.overall_ok` 예상 값은 여전히 `true`다.

### S8: Doctor 신선도 — 부재 루프 조용한 스킵 (Must, oracle)
Given 워크스페이스에 learnings 저장 파일도, `.autopus/project/canary.md`도, memindex 인덱스도 없다.
When `auto doctor --json`을 실행한다.
Then `doctor.evidence.learnings`·`doctor.evidence.canary`·`doctor.evidence.memindex` 세 check는 결과에서 빠진다(예상 값: 세 check 부재)이고 신선도 관련 경고는 없다.

### S9: Canary 구성됐으나 미실행 (Must, oracle)
Given `.autopus/project/canary.md`는 있으나 `.autopus/canary/latest.json` receipt가 없다.
When `auto doctor --json`을 실행한다.
Then `doctor.evidence.canary` check의 status 예상 값은 `warn`이고 detail은 미실행 취지 문구와 `auto canary` 힌트를 포함하며 `data.overall_ok` 예상 값은 `true`다.

### S10: 규칙 문서 신선도 + `--spec` 언급 (Should, oracle)
Given source 규칙 파일 `content/rules/doc-storage.md`가 신선도 가드와 `--spec` 필터를 문서화한다.
When 해당 규칙 파일 본문을 검사한다.
Then 언급 블록에 expected 부분 문자열 `freshness`(또는 `신선도`)와 `auto doctor`가 함께 포함되고(신선도 절), 동일 블록에 `--spec` 부분 문자열도 포함된다(REQ-010의 두 문서화 대상을 모두 추적).

## Oracle Acceptance Notes

concrete expected output로 닫으며 구조 신호(heading/파일 존재/exit 0)만으로 Must를 충족하지 않는다.

- S1 oracle(INV-002): 입력 `+09:00`,`+00:00`,`+0900` 3엔트리 → 반환 `3`, skip `0`. `+0900`이 fallback으로 수용되어 유지됨이 concrete 증거. Outcome Lock item (a)의 "fallback 수용"을 정면 검증.
- S2 oracle(INV-001): 입력 정상 2 + 잘린 JSON 1(3번째 줄) → 반환 `2`, skip `1`, 경고에 줄번호 `3`. fail-all→tolerant 전환 증거.
- S3 oracle(INV-003·INV-004): 60일 전 엔트리를 `--max-age 30`으로 실제 age-out(`pruned==1`)시켜 `rewriteStore`를 강제한 뒤, 생존 `+0900` 엔트리가 `+09:00`으로 정규화(콜론 없는 `+0900` `0`회)되고 손상 줄이 상대 순서 유지한 채 보존됨을 함께 단정. no-op prune short-circuit(F-001) 회피.
- S4 oracle(INV-004): 손상 줄 `GARBAGE-LINE-XYZ`가 prune 재작성 후에도 원본 상대 순서를 유지한 채 `1`회 잔존 → 데이터 무결성(조용한 삭제·재배열 방지) 증거. reuse-count 부활이 데이터 손실로 바뀌지 않음을 보장(F-005).
- S5 oracle(INV-005): 입력 SpecID `{SPEC-A×2, SPEC-B×1, ""×1}` → `--spec SPEC-A` 반환 정확히 `2`(둘 다 SPEC-A). 정확 일치 필터.
- S6/S7 oracle(INV-006·007): 세 anchor age 0일 → 전부 `pass`; age 40일 → 전부 `warn`+각 힌트, 두 경우 모두 `overall_ok==true`. 30일 임계 경계와 advisory 불변식의 concrete 검증.
- S8 oracle(INV-006): 존재 신호 없는 워크스페이스 → 세 check 부재(무경고). 최종 사용자 설치 노이즈 방지.
- S9 oracle(INV-006): canary.md 존재 + receipt 부재 → `doctor.evidence.canary` `warn`+`auto canary`. 조용한 사망(never-run)의 표면화.
- S10 oracle(INV-007): source 규칙 파일 `content/rules/doc-storage.md` 언급에 `freshness`/`신선도`+`auto doctor`(신선도)와 `--spec`(필터) 부분 문자열이 모두 포함 — REQ-010의 두 문서화 대상을 추적(F-003).
- REQ-009 패리티: S7의 `auto learn record`·`auto canary`·`auto mem rebuild`는 플랫폼 무관 동일 명령이라 4플랫폼 사용자 모두에게 유효하다.
