# SPEC-ADK-RELEASE-SIGNING-001 수락 기준

구조 검사는 보조 증거일 뿐입니다. Must 시나리오는 합성 P-256 키 또는 실제 v0.50.73 자산을 사용해 구체적인 반환값, sentinel error, exit code, 생성 자산을 확인해야 합니다.

## Test Scenarios

## Stage 1: envelope와 Go consumer

### S1: 정상 단일·다중 서명 (Must)

Given 합성 P-256 K1, K2와 정확한 `checksums.txt`가 있습니다.
When V1 envelope에 K1만 넣거나 K1+K2를 넣어 검증합니다.
Then known active key의 서명이 하나 이상 통과하면 `error == nil`입니다.

### S2: strict envelope parser (Must)

다음 입력은 모두 `ErrMalformedReleaseSignatureEnvelope`이어야 하며 암호 검증을 시작해서는 안 됩니다.

- 빈 입력, 잘못된 header, header-only 입력
- CRLF, 마지막 LF 누락, BOM, NUL, 빈 레코드
- 4,096바이트 초과, 16레코드 초과, 256바이트 초과 레코드
- uppercase·짧은 fingerprint, 탭이 아닌 구분자
- invalid 또는 non-canonical base64
- invalid, trailing, non-canonical DER 또는 P-256 범위를 벗어난 `r`, `s`
- duplicate fingerprint

### S3: unknown fingerprint와 회전 (Must)

Given envelope에 구조적으로 유효한 unknown signature와 known valid signature가 함께 있습니다.
When 전체 envelope를 파싱하고 검증합니다.
Then unknown 레코드는 파싱 후 무시되고 known signature가 통과해 성공합니다.

Given unknown 레코드가 malformed입니다.
Then known valid 레코드가 뒤에 있어도 envelope 전체를 거부합니다.

### S4: 변조와 공격자 재서명 (Must)

- 원문 서명과 변조된 `checksums.txt` 조합은 `ErrNoTrustedReleaseSignature`입니다.
- 공격자 fingerprint와 공격자 서명만 있는 envelope는 `ErrNoTrustedReleaseSignature`입니다.
- known fingerprint에 공격자 서명을 붙여도 `ErrNoTrustedReleaseSignature`입니다.
- 어느 경우에도 checksum-only 검증으로 돌아가지 않습니다.

### S5: embedded key 오류 분류 (Must)

- malformed PEM/SPKI, P-384/RSA, full fingerprint 불일치, 잘못된 만료일, duplicate pin은 `errors.Is(err, ErrMalformedEmbeddedReleaseKey) == true`입니다.
- PEM 앞의 추가 바이트와 PEM header도 `ErrMalformedEmbeddedReleaseKey`입니다.
- 유효한 pin이 하나 이상 있으나 모두 만료된 경우에만 `errors.Is(err, ErrAllReleaseSigningKeysExpired) == true`입니다.
- 두 sentinel은 동시에 true가 아니어야 합니다.
- 빈 pin 집합은 malformed configuration입니다.
- production Go consumer에는 checked-in 파일과 byte-for-byte 일치하는 K1·K2 public PEM과 full fingerprint 두 쌍이 있어야 합니다. K1은 `2028-07-17`, K2는 `2030-07-17`까지 active입니다.

### S6: downloader API 호환과 검증 순서 (Must)

Given 기존 4인자 `DownloadAndVerify` 호출이 있습니다.
When checksum URL의 basename이 정확히 `checksums.txt`입니다.
Then sibling `checksums.txt.signatures` URL을 계산하고 explicit 메서드로 위임해 정상 아카이브를 추출합니다.

When scheme, host, userinfo, fragment, encoded basename 또는 basename이 안전하지 않습니다.
Then URL 파생 단계에서 실패하며 checksum-only로 진행하지 않습니다.

When explicit signature URL이 비어 있거나 envelope가 HTTP 크기 제한을 넘습니다.
Then 네트워크·파싱 경로가 fail-closed error를 반환합니다.

## Stage 1: producer와 workflow

### S7: multi-key producer (Must)

Given 합성 K1과 K2 private-key 파일을 반대 순서로 전달합니다.
When `sign-checksums.sh`를 실행합니다.
Then 자산 이름은 `checksums.txt.signatures`이며 header가 정확하고 레코드 fingerprint가 C locale 오름차순입니다. Go verifier가 두 서명을 검증할 수 있어야 합니다.

같은 키를 두 번 전달하거나 non-P256 키를 전달하면 non-zero로 실패하고 완성된 output을 남기지 않아야 합니다.

### S8: key materialization과 pair preflight (Must)

- raw private-key secret은 별도의 짧은 workflow step에만 주입됩니다.
- 해당 step은 0700 credential 디렉터리에 0600 key file을 만듭니다.
- preflight는 private key, checked-in K1 P-256 public PEM, full fingerprint가 같은 tuple일 때만 성공합니다.
- checked-in K2는 Go consumer에 선배포하지만 v0.50.73 workflow와 K1 pair preflight에는 사용하지 않습니다.
- GoReleaser step에는 raw secret이 없고 key-file path만 있습니다.
- cleanup은 `if: always()`이며 credential 디렉터리 제거 실패를 숨기지 않습니다.
- Stage 1의 최종 `README.md`와 `install.sh`는 A4 기준 `6948fde`와 byte-identical이어야 합니다. 이후 `main`에 조기 유입된 `.sig` 기반 installer Stage 2 surface와 `scripts/test-install-signing.sh`는 존재하지 않아야 합니다.

### S9: GoReleaser v2.17.0 executable oracle (Must)

Given 실제 GoReleaser v2.17.0 binary와 합성 P-256 private key가 있습니다.
When checksum signer wiring을 포함한 synthetic project를 `goreleaser release --snapshot --clean`으로 실행합니다.
Then `dist/checksums.txt.signatures`가 생성되고, 생성된 `dist/checksums.txt`와 envelope를 Stage 1 Go verifier에 넣었을 때 `error == nil`이어야 합니다.

production `.goreleaser.yaml`은 기존 cosign `checksums.txt.bundle` signer와 V1 envelope signer를 함께 유지해야 합니다.

## Release gate

### S10: v0.50.73 live asset (Must, PASS)

Given fresh K1·offline-next K2의 encrypted custody와 recovery 검증이 완료되고, GitHub Environment secret이 checked-in K1과 pair입니다.
When v0.50.73 release workflow가 완료됩니다.
Then release assets에 `checksums.txt`, `checksums.txt.bundle`, `checksums.txt.signatures`가 모두 존재해야 합니다. live envelope를 내려받아 K1 fingerprint와 서명을 독립 검증해야 합니다.

이 시나리오가 PASS하기 전에는 S11과 S12 코드를 raw-main installer에 배포해서는 안 됩니다.

Evidence: release run `29588526312`, release/tag `v0.50.73`, source commit `334b297f05942accbecdfa15b54e38e005c82f2d`, live fixture `scripts/release-signing/tests/fixtures/v0.50.73/`. 세 자산과 K1 signature를 독립 검증했습니다.

### v0.50.74 A5 하드닝 추적 증거 (PASS)

Release 실행 `29640813340`은 정확한 source commit `b27252cb1148192a8ae1a95195c50e5f221453a4`와 tree `6c3790ee668b2d1c9f2f44a272144dd1106507d9`를 보호 환경 변수와 대조했습니다. 이후 annotated tag object `c79f133f0108bf3f07cee0162c1abeecf9d379d1`에서 변경 불가능한 최종 릴리스를 게시했습니다.

독립 라이브 검증은 다음을 모두 통과했습니다.

- 11개 릴리스 자산의 로컬 SHA-256이 GitHub 서버 digest와 일치합니다. `checksums.txt` SHA-256 `48c79e1fb47444aa83909794cd041bdfed18bf263bf5c0209578540382824ad4`는 8개 플랫폼 아카이브를 모두 검증합니다.
- `checksums.txt.signatures` SHA-256은 `79c0afda3023a8f270e5386b88d87f7f219550f685c3fab8afe06cf5d70d1dc4`입니다. 활성 K1 fingerprint `e1fdfe066484c7eae8ff16fa4b1ee6237b8d06299c2b66ced485f029af77837f`의 서명이 유효했고, 변조된 checksum은 거부됐습니다. cosign bundle도 정확한 `release.yaml@refs/tags/v0.50.74` OIDC identity로 검증됐습니다.
- Darwin amd64·arm64 바이너리는 `Identifier=co.autopus.adk`, `TeamIdentifier=GP2PFA2PUV`, hardened runtime, secure timestamp와 notarized designated requirement를 모두 만족합니다. A0 receipt record SHA-256 `84ee9403223aabd1f60e5e55e79a5c7d6b2c764bc594435cbf7c4e997e2ce475`는 A5까지 같은 바이트를 유지합니다. amd64 manifest SHA-256은 `5b4381d3f2180b19c0da9d419ebc8452b9ba04c73c8d0921c2a74c09ab38b85c`, arm64 manifest SHA-256은 `62a9f78302ee000c16c1c73669282e955fc3abc82f850ff4a77d0e04069f4aed`입니다.
- 라이브 네이티브 바이너리는 `0.50.74`와 commit `b27252c`를 보고합니다. Darwin 두 아키텍처 아카이브에서 `auto`의 권한은 `0755`입니다. Homebrew commit `9e3b9b4076b47b85218b14632c79a3d796e6769c`는 `Casks/auto.rb`만 갱신했고 Formula blob `4ebc6c38925002dec00759823d4dd847a499818a`는 유지했습니다.

### v0.50.75 A6 계보 준비 (PENDING LIVE RELEASE)

A6는 immutable `v0.50.74`를 직접 선행 릴리스로 검증합니다. 고정된 A5 증거는 source commit `b27252cb1148192a8ae1a95195c50e5f221453a4`, annotated tag object `c79f133f0108bf3f07cee0162c1abeecf9d379d1`, checksums SHA-256 `48c79e1fb47444aa83909794cd041bdfed18bf263bf5c0209578540382824ad4`입니다. Darwin amd64·arm64 archive SHA-256은 각각 `aeb9d048579c77ab17f4a4ec3a1160778d16c627747c5af5f341e664e1417cb0`, `bc90e594c91de61dabc2982f60249b638d448fa3f6643004fe6d45cdd0cc5eab`이고, embedded manifest SHA-256은 각각 `5b4381d3f2180b19c0da9d419ebc8452b9ba04c73c8d0921c2a74c09ab38b85c`, `62a9f78302ee000c16c1c73669282e955fc3abc82f850ff4a77d0e04069f4aed`입니다.

Homebrew 전이는 A5 Cask blob `ceed648bfece4555e8310b6e894fedc847520960`을 prior CAS pin으로 사용하고 Formula blob `4ebc6c38925002dec00759823d4dd847a499818a`를 계속 동결합니다. 이 절은 코드·계보 준비 상태만 기록하며, `v0.50.75` 태그와 보호 환경 source pin, 실제 서명·공증·immutable release, Cask 게시의 라이브 증거는 릴리스 성공 후 별도로 동기화해야 합니다.

## Stage 2: installers

### S11: POSIX installer (Must, PASS)

Given v0.50.73 이상의 signed release가 있습니다.
When `install.sh`가 정상, 변조, unknown-only, malformed, duplicate, all-expired, openssl-absent fixture를 처리합니다.
Then 정상만 exit 0이며 나머지는 exit 1이고 바이너리를 설치하지 않아야 합니다. 파싱 한도와 error taxonomy는 Go consumer와 같아야 합니다.

Evidence: Ubuntu root oracle과 Stage 2 CI run `29618589360`의 macOS stock LibreSSL oracle이 정상, K2 rotation, 변조, unknown-only, malformed wire/base64/DER, duplicate, all-expired, helper drift, OpenSSL·checksum-tool 부재, checksum mismatch를 검증했습니다.

v0.50.74 POSIX oracle에서는 `umask 077`로 non-sudo와 강제 sudo 설치를 모두 실행해 새 설치 디렉터리와 최종 바이너리의 권한이 두 경로에서 정확히 `0755`인지 검증했습니다. Release 실행 `29640813340`의 macOS runtime과 전체 test gate, 라이브 아카이브 권한 확인도 모두 통과했습니다.

### S12: Windows installer PS5.1/7 (Must, PASS)

Given producer-shaped P-256 SPKI DER와 ASN.1/DER signature vector가 있습니다.
When Windows PowerShell 5.1과 PowerShell 7에서 SPKI를 CNG `ECS1` blob으로, DER를 canonical P1363 64바이트로 바꿔 검증합니다.
Then 두 runtime 모두 정상 vector는 true, checksum·signature·fingerprint 변조는 false여야 합니다. malformed/duplicate/all-expired/unknown-only 입력은 설치 전에 실패해야 합니다.

Evidence: Stage 2 CI run `29618589360`, job `88008858920`에서 Windows PowerShell 5.1과 PowerShell 7이 같은 live K1 vector, strict parser, ECS1/P1363 변환, 실패 전 설치 0건, 정상 Main 설치 경로를 모두 통과했습니다.

### S13: 정상 UX와 trust limitation (Must, PASS)

- 세 소비자의 정상 성공 메시지와 무인 설치 흐름은 기존과 같아야 합니다.
- 문서는 v0.50.73 floor와 v0.50.72 이하 unsigned 상태를 명시해야 합니다.
- 문서는 raw-main installer가 release assets와 독립된 trust anchor가 아니며, repository main이나 raw-main 전달 경로가 침해되면 release assets 장악 없이도 우회할 수 있음을 명시해야 합니다.

## 완료 판정

| Gate | 현재 상태 | 완료 조건 |
|---|---|---|
| Stage 1 code/test | PASS | race, vet, shell syntax, GoReleaser check/oracle PASS |
| K1·K2 local ceremony와 public pin | PASS | encrypted custody·recovery receipt, checked-in/embedded 2-pin parity |
| K1 GitHub Environment | PASS | fresh K1 secret 설정과 pair preflight PASS |
| v0.50.73 live release | PASS | S10 PASS |
| POSIX installer | PASS | S11 PASS |
| Windows installer | PASS | S12 PASS on PS5.1 and PS7 |
| SPEC completion | PASS | S1~S13 Must 전체 PASS |

## Oracle Acceptance Notes

| Scenario | Concrete expected output |
|---|---|
| S1 | `error == nil` |
| S2 | `errors.Is(err, ErrMalformedReleaseSignatureEnvelope) == true` |
| S3 | unknown+known valid은 `error == nil`; malformed unknown은 `ErrMalformedReleaseSignatureEnvelope` |
| S4 | `errors.Is(err, ErrNoTrustedReleaseSignature) == true`; checksum 단계 호출 0회 |
| S5 | malformed와 all-expired sentinel은 상호 배타적; checked-in/embedded pin count 2와 두 tuple exact match |
| S6 | 4인자 호출 성공 또는 unsafe URL에서 non-nil error; checksum-only fallback 0회 |
| S7 | 정상 helper exit 0과 sorted 1~16 records; duplicate/non-P256 exit 1과 output 부재 |
| S8 | pair 일치 exit 0, 불일치 exit 1; GoReleaser raw secret binding 0개 |
| S9 | GoReleaser v2.17.0 exit 0, `dist/checksums.txt.signatures` 존재, Go verifier `error == nil` |
| S10 | 세 live asset HTTP 200, K1 fingerprint 일치, live signature verify true |
| S11 | POSIX 정상 exit 0; 모든 실패 fixture exit 1과 설치 파일 부재 |
| S12 | PS5.1·PS7 정상 `VerifyHash == true`; 변조 false; 실패 fixture 설치 파일 부재 |
| S13 | 기존 성공 메시지 유지, floor/raw-main 한계 문서에 명시 |

허용 오차는 없습니다. envelope 바이트 규격, fingerprint, sentinel 분류, exit code는 exact match여야 합니다. ECDSA 서명 바이트 자체는 매 실행 달라질 수 있으므로 서명 값의 byte equality는 요구하지 않고 암호 검증 결과만 요구합니다.
