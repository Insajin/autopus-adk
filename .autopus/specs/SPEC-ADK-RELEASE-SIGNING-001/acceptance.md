# SPEC-ADK-RELEASE-SIGNING-001 수락 기준

각 Must 시나리오는 oracle-first다. 파일 존재/heading/exit success 같은 구조 신호만으로 닫지 않고 concrete 기대값(검증 bool, exit code, error 문자열, 설치물 유무)과 정확 일치를 요구한다. fixture는 합성 `ecdsa.GenerateKey(elliptic.P256(), rand.Reader)` 키페어(publisher K, 공격자 A)와 임시 `checksums.txt`로 구성한다. 실제 배포 키는 쓰지 않는다.

## Test Scenarios

### S1: 정상 서명 경로 — selfupdate (Must, oracle)
Given publisher 키 K로 `sig = ecdsa.SignASN1(K_priv, sha256(checksums.txt))`이고, 임베드 키 집합은 {K_pub(P-256, 미만료)}다.
When `VerifyReleaseSignature(checksums.txt, sig)`를 호출하고 이어서 아카이브 sha256을 비교한다.
Then 서명 검증 반환 값의 예상은 `error == nil`이고, 이후 checksum 단계에 진입하며, 정상 아카이브에서 최종 결과는 성공(설치/교체 진행)이다.

### S2: 변조된 checksums.txt — selfupdate (Must, oracle)
Given `sig`는 원본 `checksums.txt`에 대한 서명이나 다운로드된 바이트는 한 hex 문자가 바뀐 `checksums.txt'`다.
When `VerifyReleaseSignature(checksums.txt', sig)`를 호출한다.
Then 모든 비만료 임베드 키에 대한 `ecdsa.VerifyASN1(pub, sha256(checksums.txt'), sig)` 예상 반환은 `false`이고 함수는 `no trusted release signing key verified` error를 반환하며, checksum 단계에 **진입하지 않고** 업데이트는 non-zero로 중단된다.

### S3: 서명 자산 부재 — selfupdate (Must, oracle)
Given 릴리스에 `checksums.txt.sig`가 없어 `ReleaseInfo.SignatureURL`이 빈 문자열이다.
When `auto update --self` 경로에서 `DownloadAndVerify`가 실행된다.
Then 예상 결과는 `릴리스 서명을 찾을 수 없습니다` 취지의 error 반환 + checksum-only 폴백 없음 + `runSelfUpdate` non-zero 종료다.

### S4: 공격자 재서명 — install.sh (Must, oracle)
Given 공격자가 `checksums.txt`·아카이브·`checksums.txt.sig`를 모두 자기 키 A로 교체했고(A는 임베드 집합 밖), install.sh 임베드 키 집합은 {K_pub(PEM SPKI)}다.
When `verify_signature()`가 비만료 임베드 키마다 `openssl dgst -sha256 -verify pub.pem -signature checksums.txt.sig checksums.txt`를 시도한다.
Then 모든 시도의 예상 출력은 `Error Verifying Data`·종료 non-zero이고 스크립트는 `no trusted release signing key verified` 후 exit code `1`을 반환하며 `INSTALL_DIR`에 바이너리가 생성되지 않는다.

### S5: 아카이브 무결성 — 서명 통과 후 checksum 불일치 (Must, oracle)
Given `sig`는 유효(임베드 K_pub로 통과)하나 다운로드된 아카이브 바이트가 변조되어 sha256이 `checksums.txt` 항목과 다르다.
When 서명 검증 후 아카이브 sha256을 비교한다.
Then 서명 검증 예상은 통과이나 checksum 비교에서 `checksum mismatch` 취지 error/`exit 1`로 중단되고 설치/교체가 일어나지 않는다.

### S6: 동일-origin 전체 교체 위협 — 두 소비자 (Must, oracle)
Given 공격자가 세 자산(아카이브+checksums+sig)을 A 키로 일관되게 재구성(임의 KeyID 힌트 포함)했고, selfupdate·install.sh 임베드 키 집합은 모두 {K_pub}다.
When selfupdate `VerifyReleaseSignature`와 install.sh `verify_signature`를 각각 실행한다.
Then A가 임베드 집합에 없어 어떤 trial도 통과하지 못하므로 두 경로 모두 `no trusted release signing key verified`로 실패(selfupdate non-zero, install.sh exit `1`)한다. 와이어 KeyID는 신뢰 판정에 사용되지 않는다.

### S7: 검증 도구 부재 — install.sh fail-closed (Must, oracle)
Given install.sh 실행 환경에 openssl 바이너리가 탐지되지 않는다.
When main이 `verify_signature()`를 호출한다.
Then 예상 결과는 exit code `1` + `서명을 검증할 수 없습니다` 취지 안내(openssl 설치 또는 대체 검증 경로) 출력 + 바이너리 미설치다(기존 `install.sh:131` fail-open `return 0`과 대비되는 fail-closed).

### S8: 만료 게이트·multi-trial·회전창 — 두 소비자 (Must, oracle)
Given 세 픽스처 — (a) 임베드 집합 밖 공격자 키 A로 만든 서명, (b) 유일 임베드 키의 ExpiresAt를 과거로 설정하고 그 키로 만든 유효 서명, (c) 임베드 키 2개(K1, 신규 K2) 중 K2로 만든 서명.
When 각 픽스처에 대해 selfupdate `VerifyReleaseSignature`와 install.sh `verify_signature`를 실행한다.
Then (a) 예상은 두 소비자 모두 `no trusted release signing key verified`·non-zero/exit 1; (b) 예상은 만료 키가 시도 이전 제외되어 유효 서명이라도 `all embedded keys expired`·non-zero/exit 1; (c) 예상은 K2가 시도 집합에 있어 검증 통과·진위 인정·정상 진행(회전창 동작).

### S9: 정상 경로 UX 불변 (Must, oracle)
Given 서명·checksum·아카이브가 모두 유효하다.
When `auto update --self`가 완료된다.
Then 출력의 예상은 서명 도입 이전과 동일한 `v{old} → {new} 업데이트 완료` 라인이고 exit code는 `0`이며, 서명 검증은 사용자에게 추가 프롬프트 없이 투명하게 수행된다.

## Oracle Acceptance Notes

이 SPEC의 oracle acceptance는 concrete expected output으로 닫는다. 검증 bool·exit code·error 문자열·설치물 유무의 정확 일치를 요구하며, 파일 존재·heading·exit success 같은 구조 신호만으로는 Must를 충족하지 않는다. fixture는 합성 `ecdsa.GenerateKey(P256)`(publisher K, 공격자 A)만 쓰고 실제 배포 키를 노출하지 않는다. install.sh oracle은 stock `/usr/bin/openssl`(LibreSSL 3.3.6, 실증 통과)로 `dgst -sha256 -sign/-verify`와 `date -u +%Y-%m-%d` 사전식 만료 비교를 구동한다.

- S1 oracle(INV-001 결합): 입력 `sig=SignASN1(K_priv,sha256(checksums.txt))` + 임베드={K_pub} → 예상 `error==nil`, checksum 진입, 최종 성공.
- S2 oracle(INV-001/002 변조): 1 hex 변경 `checksums.txt'` → 모든 trial `VerifyASN1` 예상 `false`, `no trusted release signing key verified`, checksum 미진입, non-zero.
- S3 oracle(INV-002 부재): `SignatureURL==""` → `릴리스 서명을 찾을 수 없습니다`, 폴백 없음, non-zero.
- S4 oracle(INV-001 공격자키): install.sh, A∉임베드 → openssl `Error Verifying Data`·exit `1`, 바이너리 미생성.
- S5 oracle(INV-003 순서): 서명 통과 후 아카이브 sha256 불일치 → `checksum mismatch`/exit `1`, 설치 없음.
- S6 oracle(INV-004 전체교체): 세 자산 A키 재구성(+KeyID 힌트) → 두 소비자 모두 임베드 집합 불일치로 `no trusted…`(selfupdate non-zero, install.sh exit `1`).
- S7 oracle(INV-002 도구부재): openssl 미탐지 → exit `1` + 안내, 미설치.
- S8 oracle(INV-005 만료·multi-trial·회전): (a) 공격자키→`no trusted…`; (b) 유일 키 만료→`all embedded keys expired`(유효 서명이라도 시도 이전 제외); (c) 2키 중 K2 서명→통과. 두 소비자 대칭.
- S9 oracle(INV-001 UX): 전부 유효 → `v{old} → {new} 업데이트 완료` + exit `0`, 추가 프롬프트 없음.

paired-matching·fail-closed·ordering·trust-anchor·multi-trial 계열 invariant(INV-001~005)는 concrete 기대 error·bool·exit·문자열로 검증되며 structural-only 신호를 쓰지 않는다. Go↔openssl ECDSA 상호운용(Go 서명↔openssl 검증, 반대)도 T6에서 양방향 통과를 요구한다.
