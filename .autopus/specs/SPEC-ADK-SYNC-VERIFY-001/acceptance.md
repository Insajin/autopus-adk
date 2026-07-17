# SPEC-ADK-SYNC-VERIFY-001 수락 기준

각 Must 시나리오는 oracle-first다. 구조 신호(파일 존재/제목/exit success)만으로 닫지 않고 concrete expected output(예상 파일 귀속·Phase 집합·경고 문자열·순서·exit code)과 정확 일치를 요구한다. fixture는 temp 루트 git repo + 가짜 nested git repo로 구성한다.

## Test Scenarios

### S1: repo 귀속 (Must, oracle)
Given temp 루트 git repo에 nested git repo `mod-a`가 있고, 루트에는 `ARCHITECTURE.md`가 dirty, `mod-a`에는 `pkg/x.go`가 dirty다.
When 워크스페이스 내부에서 `auto sync verify`를 실행한다.
Then `pkg/x.go`의 귀속 repo 예상 값은 `mod-a`(Phase A)이고 `ARCHITECTURE.md`의 귀속 repo 예상 값은 루트 `.`(Phase B)이며, 각 dirty 파일은 정확히 한 repo에만 나타난다(mod-a dirty count `1`, 루트 dirty count `1`).

### S2: Phase 분류 집합 (Must, oracle)
Given 루트 repo에 `.autopus/project/product.md`·`autopus.yaml`·`CHANGELOG.md`가 dirty이고 nested `mod-a`에 `src/app.ts`가 dirty다.
When `auto sync verify`를 실행한다.
Then Phase B 집합의 예상 값은 정확히 `{.autopus/project/product.md, autopus.yaml, CHANGELOG.md}`이고 Phase A `mod-a` 집합의 예상 값은 정확히 `{src/app.ts}`이며, 세 루트 파일 중 어느 것도 Phase A로 분류되지 않는다.

### S3: cross-boundary misplacement (Must, oracle)
Given 루트 `.autopus/specs/SPEC-FOO-001/plan.md`가 오직 `mod-a/pkg/foo.go` 경로만 참조하고 다른 모듈 경로는 참조하지 않는다.
When `auto sync verify`를 실행한다.
Then misplacement 경고가 출력되고 기대 문자열은 `SPEC-FOO-001`·현재 위치 루트·기대 위치 `mod-a/.autopus/specs/`를 포함하며, `--strict` 없이 exit code 예상 값은 `0`이다.

### S4: SPEC 위치-코드경로 모듈 불일치 (Must, oracle)
Given nested `mod-a/.autopus/specs/SPEC-BAR-001/plan.md`가 `mod-a/pkg/a.go`와 `mod-b/pkg/b.go` 두 모듈 경로를 함께 참조한다.
When `auto sync verify`를 실행한다.
Then location-mismatch 경고가 출력되고 기대 문자열은 `SPEC-BAR-001`·감지된 소유 `cross-module`·기대 위치 루트 `.autopus/specs/`를 포함한다(2+ 모듈 참조는 Module Detection상 루트 소관).

### S5: 무관 파일 혼입 (staged+unstaged) (Must, oracle)
Given 루트 repo에서 `ARCHITECTURE.md`는 staged, `.autopus/project/tech.md`는 unstaged다.
When `auto sync verify`를 실행한다.
Then 혼입 경고가 출력되고 기대 문자열은 `staged and unstaged`(또는 `스테이징`/`미스테이징`) 공존을 명시하며 두 파일 `ARCHITECTURE.md`·`.autopus/project/tech.md`를 나열한다.

### S6: --spec 소유/무관 분리 (Must, oracle)
Given `mod-a/.autopus/specs/SPEC-FOO-001/plan.md`가 `pkg/foo.go`를 소유로 명시하고, `mod-a` dirty 파일이 `pkg/foo.go`(소유)와 `pkg/unrelated.go`(무관) 둘이다.
When `auto sync verify --spec SPEC-FOO-001`을 실행한다.
Then "이 SPEC 커밋 대상" 집합의 예상 값은 workspace-relative `{mod-a/pkg/foo.go}`이고 "무관 dirty 파일" 집합의 예상 값은 `{mod-a/pkg/unrelated.go}`이며, 실제 Phase A 계획에는 `pkg/foo.go`만 남는다.

### S7: 결정적 계획 순서 (Must, oracle)
Given nested repo `mod-c`·`mod-a`·`mod-b`와 루트가 모두 dirty하다.
When `auto sync verify`를 실행한다.
Then 계획 블록의 Phase A 순서 예상 값은 정확히 `mod-a` → `mod-b` → `mod-c`이고 그 다음 Phase B 메타(`.`)가 마지막이며, 각 repo 라인은 `git -C <path> add --`로 시작하고 Lore 리마인더 라인을 동반한다.

### S8: read-only 불변 (Must, oracle)
Given dirty 파일이 여럿인 fixture 워크스페이스에서 실행 직전 각 repo의 `git status --porcelain`, `git rev-parse HEAD`, `.git/index` bytes/hash/mtime를 기록한다.
When default, 실제 host를 가진 `--spec`, `--strict` 변형을 실행한다.
Then 실행 후 각 repo의 status·HEAD·index bytes/hash/mtime가 모두 실행 전과 동일하다.

### S9: exit 계약 (Must, oracle)
Given 위반(misplacement 또는 혼입)이 하나 존재하는 fixture와 위반이 없는 fixture를 각각 준비한다.
When 위반 fixture에서 플래그 없이, 위반 fixture에서 `--strict`로, 무위반 fixture에서 `--strict`로 각각 실행한다.
Then exit code 예상 값은 순서대로 `0`(경고만), `1`(strict+위반), `0`(strict+무위반)이다.

### S10: --spec traversal 거부 (Must, oracle)
Given `--spec` 인자로 `../../etc`와 `SPEC-FOO/../../x` 같은 경로 이탈 값을 준다.
When `auto sync verify --spec <값>`을 실행한다.
Then 두 입력 모두 SPEC-ID 패턴(`^SPEC-[A-Z0-9-]+$`) 위반으로 거부되고 기대 에러 문자열은 `invalid --spec`(또는 유효하지 않은 SPEC ID)를 포함하며, `.autopus/specs/` 트리 밖 파일은 읽지 않고 절대 경로를 출력하지 않는다.

### S11: 패리티 문서 언급 (Should, oracle)
Given `[NEW]` content 스킬/규칙의 sync 절차 문서가 커밋 전 단계를 문서화한다.
When 해당 문서 본문을 검사한다.
Then 4플랫폼 어댑터가 공유하는 source-of-truth `content/rules/doc-storage.md`에 `sync verify`와 canonical keep/drop 경계가 있고 generated workspace surface를 커밋하지 않는다.

### S12: 완전 분할·안전한 계획 (Must, oracle)
Given canonical root keep, generated/runtime, tracked-but-ignored, 미분류, unsafe shell filename, nested generated path가 함께 있는 fixture다.
When `auto sync verify --strict`를 실행한다.
Then inventory의 각 path는 Phase A/B·blocked·unclassified 중 정확히 하나에 속하고, blocked/unclassified는 warning에 남지만 `git -C ... add --` 계획에는 나타나지 않으며 exit code는 1이다. NUL rename fixture의 source/destination은 둘 다 보존된다.

### S13: SPEC host·Git 진단 fail-closed (Must, oracle)
Given 동일 ID의 duplicate host, symlink host, missing `plan.md`, Git stderr에 절대 경로와 secret이 있는 fixture를 각각 준비한다.
When `auto sync verify --spec` 또는 Git inventory를 실행한다.
Then host/document fixture는 계획 출력 전에 오류로 종료하고, Git 오류는 고정된 상대 repo label만 포함하며 stderr·절대 경로·secret은 포함하지 않는다. 모든 Git 호출의 첫 옵션은 `--no-optional-locks`다.

## Oracle Acceptance Notes

이 SPEC의 oracle acceptance는 concrete expected output으로 닫는다. 파일 분류·집합·순서·경고 문자열·exit code의 정확 일치를 요구하며 구조 신호만으로는 Must를 충족하지 않는다.

- S1 oracle(INV-001 귀속): 입력 루트 `ARCHITECTURE.md` + `mod-a/pkg/x.go` → 각 파일 정확히 한 repo, count 루트 `1`/mod-a `1`.
- S2 oracle(INV-002 Phase 함수): 루트 추적 3파일 → Phase B 집합 정확 일치, `src/app.ts` → Phase A. 루트 파일의 Phase A 오분류 `0`.
- S3 oracle(INV-003 misplacement): 루트 SPEC이 단일 모듈 경로만 참조 → 기대 위치 `mod-a/.autopus/specs/`, exit `0`.
- S4 oracle(INV-004 Module Detection): 모듈 SPEC이 2 모듈 참조 → `cross-module`, 기대 루트. 단일 모듈 명확 귀속이 아니면 무경고(false-positive 억제).
- S5 oracle(INV-005 혼입): 같은 repo staged(`ARCHITECTURE.md`)+unstaged(`tech.md`) 공존 → 두 파일 나열 경고.
- S6 oracle(INV-005 --spec): workspace-relative 소유 `{mod-a/pkg/foo.go}` vs 무관 `{mod-a/pkg/unrelated.go}` 정확 분리 + owned-only 계획.
- S7 oracle(INV-006 순서): Phase A `mod-a→mod-b→mod-c` 정확 시퀀스 후 Phase B. 비결정 출력 불가.
- S8 oracle(INV-007 read-only): 실행 전후 status·HEAD·index bytes/hash/mtime 동일 = 변이 0의 concrete 증거.
- S9 oracle(INV-007 exit): 위반 유무 × `--strict` 조합의 exit 0/1/0 정확 일치.
- S10 oracle(INV-007 안전): traversal 입력 2종 거부 + specs 트리 밖 미접근 + 절대 경로 미노출.
- S11 oracle: 공유 content rule에 `sync verify`와 canonical keep/drop 포함, generated surface 커밋 0.
- S12 oracle(INV-008 partition/shell safety): inventory cardinality와 네 partition cardinality 합 일치, blocked/unclassified plan 포함 0, rename 양쪽 보존.
- S13 oracle(INV-009 fail-closed diagnostic): duplicate/symlink/missing-doc 오류, optional-lock 차단, Git stderr·절대 경로·secret 노출 0.

set-partition·ordering·comparison 계열 invariant(INV-001·002·004·006)는 concrete expected 집합/순서로 검증되고, read-only/exit(INV-007)는 실행 전후 상태 동일성과 exit code로 검증된다.

## Verification Receipt (2026-07-17)

- `go test -race ./internal/cli -run 'TestSyncVerify' -count=1` — PASS
- `go vet ./internal/cli` — PASS
- `go test ./internal/cli/... -count=1` — PASS
- 실제 meta workspace에서 `auto sync verify --spec SPEC-ADK-SYNC-VERIFY-001` read-only 실행 — PASS(owned-only empty plan, unrelated/misplacement warning 관측)
- `git diff --check` — PASS
- production source files — 모두 300줄 미만(최대 `sync_verify_discover.go` 258줄)
- `git ls-files -c -i --exclude-standard` — 출력 없음
