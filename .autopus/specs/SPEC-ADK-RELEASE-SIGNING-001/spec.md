# SPEC-ADK-RELEASE-SIGNING-001: 배포 아티팩트 게시자 서명 검증

**Status**: completed
**Review verdict**: APPROVE
**Created**: 2026-07-17
**Signing floor**: v0.50.73
**Domain**: RELEASE-SIGNING
**Module**: autopus-adk

## 목적

현재 릴리스 아카이브와 `checksums.txt`는 같은 GitHub Release에서 내려받습니다. 릴리스 자산을 바꿀 수 있는 공격자는 두 파일을 함께 교체해 checksum 검증을 통과할 수 있습니다. 기존 `checksums.txt.bundle`은 cosign keyless 투명성 증거이지만, 설치 경로에서 소비하지 않으므로 이 공격을 막지 못합니다.

이 SPEC은 `checksums.txt`에 대한 ECDSA P-256 게시자 서명을 `checksums.txt.signatures`로 배포하고, 클라이언트에 고정한 공개키로 검증합니다. 실제 소비자는 다음 세 가지입니다.

1. Go self-update: `auto update --self`
2. POSIX 설치기: `install.sh`
3. Windows 설치기: `install.ps1`

구현은 두 단계로 나눕니다. Stage 1은 producer와 Go consumer를 먼저 배포합니다. v0.50.73의 실제 자산을 검증한 뒤 Stage 2에서 두 설치기를 fail-closed로 전환합니다. 설치기를 먼저 전환하면 최신 릴리스에 서명 자산이 없는 동안 신규 설치가 모두 막히므로 순서를 바꿀 수 없습니다.

## Outcome Boundary

- **User-visible outcome**: v0.50.73부터 publisher envelope가 릴리스 자산에 포함되고, 최종적으로 세 소비자가 인증된 `checksums.txt`만 사용합니다. 정상 업데이트·설치 UX는 유지합니다.
- **Mandatory requirements**: REQ-001~REQ-010.
- **Explicit non-goals**: K2 온라인 서명 활성화, cosign 제거, Homebrew·Apple·companion 서명 변경, legacy asset 소급 서명, raw-main origin 이전, SLSA/Rekor 소비.
- **Completion evidence**: Stage 1 synthetic-key Go/openssl/GoReleaser v2.17.0 oracle, K1·K2 public pin과 encrypted custody·recovery receipt, K1 pair preflight, v0.50.73 live asset 검증, Stage 2 POSIX·PowerShell 5.1/7 oracle가 모두 필요합니다.

## 서명 envelope 규격

자산 이름은 `checksums.txt.signatures`입니다. 내용은 ASCII 기반 line-oriented envelope입니다.

```text
AUTOPUS-RELEASE-SIGNATURE-V1
<64 lowercase hex SPKI SHA-256 fingerprint>\t<canonical base64 ASN.1/DER ECDSA signature>
```

규격은 다음과 같습니다.

- 전체 크기는 1~4,096바이트입니다.
- 첫 줄은 `AUTOPUS-RELEASE-SIGNATURE-V1`과 정확히 같아야 합니다.
- 서명 레코드는 1~16개이며, 각 줄은 256바이트 이하여야 합니다.
- 모든 줄은 LF로 끝나며 파일도 마지막 LF로 끝납니다. CR, BOM, NUL, 빈 줄은 허용하지 않습니다.
- fingerprint는 공개키의 canonical SPKI DER에 대한 SHA-256 전체값인 lowercase hex 64자입니다.
- 구분자는 탭 한 개뿐입니다.
- 서명은 `checksums.txt` 원문 바이트의 SHA-256에 대한 ECDSA P-256 ASN.1/DER입니다. base64와 DER은 canonical encoding이어야 하며 `r`, `s`는 P-256 order 범위 안에 있어야 합니다.
- 같은 fingerprint가 두 번 나오면 envelope 전체를 거부합니다.
- 모든 레코드를 구조적으로 검증한 뒤 암호 검증을 시작합니다. 알 수 없는 fingerprint도 malformed 레코드이면 거부합니다.
- 구조 검증을 통과한 알 수 없는 fingerprint는 무시합니다. 비만료 known key의 서명 중 하나 이상이 통과해야 성공합니다.

이 형식은 K1과 K2가 함께 서명하는 회전 구간을 지원합니다. producer는 키 파일을 한 개 이상 받아 각 fingerprint를 직접 계산하고 fingerprint 오름차순으로 레코드를 출력합니다.

## 신뢰 경계

| 표면 | 신뢰 등급 | 계약 |
|---|---|---|
| 아카이브, `checksums.txt`, `checksums.txt.signatures` | untrusted | 전체 입력을 엄격하게 파싱하고 고정키로 검증합니다. |
| 바이너리와 설치기에 포함한 P-256 공개키·fingerprint·만료일 | trusted anchor | 릴리스 자산에서 키나 만료일을 받지 않습니다. |
| K1 release-signing private key | secret | GitHub Environment secret을 짧은 step에서 0600 임시 파일로 만들고, 배포 전 checked-in K1과 pair인지 확인합니다. |
| K2 offline-next private key | secret, offline | encrypted custody와 recovery 검증을 마쳤지만 GitHub secret이나 v0.50.73 producer 입력으로 사용하지 않습니다. |
| `raw.githubusercontent.com/.../main/install.{sh,ps1}` | 별도 신뢰 경계 | 신뢰된 설치기 바이트가 유지될 때 release-asset-only 공격을 막습니다. 저장소 main이나 raw-main 전달 경로가 침해되면 공격자는 pin·검증 코드·다운로드 대상을 바꿀 수 있어 릴리스 자산 장악 없이도 우회할 수 있습니다. |

## Requirements

### REQ-001: V1 envelope의 엄격한 파싱

THE SYSTEM SHALL 위 규격의 크기, 줄 수, 줄 길이, ASCII/LF, fingerprint, 중복, canonical base64, canonical P-256 DER 조건을 모두 확인한 뒤에만 암호 검증을 시작해야 합니다.

### REQ-002: 다중 서명 producer

WHEN 릴리스를 만들 때, THE SYSTEM SHALL 전용 helper에 P-256 private-key 파일을 한 개 이상 전달하고, 각 SPKI fingerprint를 계산해 fingerprint 오름차순의 `checksums.txt.signatures`를 생성해야 합니다. JSON이나 jq에 의존해서는 안 됩니다. 기존 `checksums.txt.bundle`은 유지해야 합니다.

### REQ-003: release workflow의 키 위생

WHEN release workflow가 private-key secret을 사용할 때, THE SYSTEM SHALL secret을 별도의 짧은 step에서 0600 파일로 만든 뒤 raw secret 대신 파일 경로만 GoReleaser에 전달해야 합니다. GoReleaser 실행 전 private key, checked-in P-256 public key, full fingerprint가 정확히 한 쌍인지 확인하고, 항상 실행되는 cleanup에서 임시 credential 디렉터리를 제거해야 합니다.

### REQ-004: Go consumer의 fail-closed 검증

WHEN self-update가 v0.50.73 이상의 릴리스를 처리할 때, THE SYSTEM SHALL `checksums.txt.signatures`를 내려받아 V1 envelope 전체를 파싱하고, 비만료 embedded key의 known fingerprint 서명 중 하나 이상을 검증한 뒤에만 `checksums.txt`를 신뢰해야 합니다. 서명 부재·변조·형식 오류·미신뢰 서명에서 checksum-only로 돌아가서는 안 됩니다.

### REQ-005: 기존 Go API 호환

THE SYSTEM SHALL 기존 `DownloadAndVerify(archiveURL, checksumURL, archiveName, destDir)` 호출을 source-compatible하게 유지해야 합니다. 이 메서드는 정확한 `checksums.txt` URL에서 sibling `checksums.txt.signatures` URL을 안전하게 계산해 explicit signature 메서드로 위임해야 하며, 안전하게 계산할 수 없으면 실패해야 합니다.

### REQ-006: trust-anchor 오류 분류

THE SYSTEM SHALL malformed PEM/SPKI, non-P-256 키, fingerprint 불일치, 잘못된 만료일, 중복 embedded fingerprint를 `ErrMalformedEmbeddedReleaseKey`로 거부해야 합니다. 유효한 embedded key가 모두 만료된 경우에는 `ErrAllReleaseSigningKeysExpired`로 구분해야 합니다. 빈 trust-anchor 집합을 “모두 만료”로 오인해서는 안 됩니다. Go consumer에는 checked-in 파일과 정확히 일치하는 fresh K1과 offline-next K2 두 pin을 포함해야 하며, v0.50.73 workflow는 K1 하나로만 서명해야 합니다.

### REQ-007: POSIX installer Stage 2

AFTER v0.50.73의 live signature asset을 검증한 뒤, THE SYSTEM SHALL `install.sh`에 같은 envelope 규격과 비만료 key 정책을 구현하고 openssl이 없거나 envelope·서명·checksum 검증이 실패하면 설치를 중단해야 합니다. Stage 1 릴리스 전에는 이 fail-closed 전환을 배포해서는 안 됩니다.

### REQ-008: Windows installer Stage 2

AFTER v0.50.73의 live signature asset을 검증한 뒤, THE SYSTEM SHALL Windows PowerShell 5.1과 PowerShell 7에서 같은 envelope를 검증해야 합니다. SPKI P-256 공개키를 CNG `ECS1` blob으로 변환하고 DER 서명을 P1363 `r||s` 64바이트로 canonical 변환한 뒤 SHA-256 digest를 검증해야 합니다.

### REQ-009: signing floor와 two-stage rollout

THE SYSTEM SHALL v0.50.73을 최초 signed release로 취급해야 합니다. v0.50.72 이하는 unsigned legacy release입니다. Stage 1 producer와 Go consumer를 v0.50.73으로 배포하고 live 자산을 검증하기 전에는 Stage 2 설치기를 fail-closed로 전환해서는 안 됩니다.

### REQ-010: 정상 경로와 한계의 명시

WHEN 서명, checksum, 아카이브 검증이 모두 통과할 때, THE SYSTEM SHALL 기존 성공 UX를 유지해야 합니다. 문서는 raw-main installer가 독립 trust origin이 아니라는 한계와 세 소비자의 적용 상태를 명시해야 합니다.

## 단계별 상태

| 단계 | 범위 | 상태 |
|---|---|---|
| Stage 1A | V1 envelope, multi-key producer, fresh K1·offline-next K2 pin, K1 preflight, Go verifier, 4인자 API 호환 | 완료 |
| Stage 1B | K1·K2 encrypted custody와 recovery 검증, public pin 선배포 | 완료 |
| Stage 1C | K1 GitHub Environment secret 설정, v0.50.73 release, live asset 검증 | 완료 |
| Stage 2A | `install.sh` V1 검증과 fail-closed 전환 | 완료 |
| Stage 2B | `install.ps1` PS5.1/7 V1 검증과 fail-closed 전환 | 완료 |

## 명시적 비목표

- K2 private key를 소스나 GitHub secret에 추가하거나 v0.50.73 서명에 사용하는 작업
- K2 온라인 서명 활성화와 K1+K2 overlap 시작
- 기존 cosign bundle 제거
- Homebrew cask/formula, Apple codesign/notarization, companion manifest 서명 체계 변경
- v0.50.72 이하 자산에 서명을 소급 추가하는 작업
- raw-main installer origin을 독립 호스트로 이전하는 작업
- SLSA provenance나 Rekor 소비를 이번 완료 조건에 추가하는 작업

## 변경 파일

| 파일 | 역할 |
|---|---|
| `pkg/selfupdate/signature.go` | V1 parser, P-256 검증, sentinel errors |
| `pkg/selfupdate/pinnedkey.go` | fresh K1·offline-next K2 full fingerprint, 만료·형식 검증 |
| `pkg/selfupdate/downloader.go` | signature-first 다운로드와 4인자 API 호환 |
| `pkg/selfupdate/checker.go`, `types.go` | `checksums.txt.signatures` asset discovery |
| `scripts/release-signing/sign-checksums.sh` | deterministic multi-key envelope producer |
| `scripts/release-signing/verify-key-pair.sh` | private/public/full-fingerprint preflight |
| `scripts/release-signing/release-k{1,2}-{public.pem,.fingerprint}` | 두 P-256 public pin의 checked-in 정본 |
| `.goreleaser.yaml` | checksum envelope signer wiring, cosign 존치 |
| `.github/workflows/release.yaml` | 짧은 secret materialization, preflight, cleanup |
| `scripts/release-signing/verify-checksums-v1.sh` | POSIX V1 parser, embedded K1·K2 pin, OpenSSL verification |
| `install.sh` | SHA-256 pinned verifier를 로드하고 signature → checksum → extraction/install 순서를 강제 |
| `install.ps1` | PS5.1/7 공통 strict parser, SPKI→ECS1, DER→P1363, ECDsaCng verification |
| `.github/workflows/ci.yaml`, `scripts/release-signing/tests/**` | Ubuntu·macOS·Windows cross-runtime oracle와 v0.50.73 live fixture |

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|---|---|---|---|
| REQ-001 | T1 | S1, S2 | INV-001, INV-002 |
| REQ-002 | T2 | S7, S9 | INV-003 |
| REQ-003 | T3 | S8 | INV-004, INV-010 |
| REQ-004 | T4 | S3, S4 | INV-001, INV-005 |
| REQ-005 | T4 | S6 | INV-005 |
| REQ-006 | T1, T4 | S5 | INV-006, INV-010 |
| REQ-007 | T7 | S11 | INV-001, INV-007 |
| REQ-008 | T8 | S12 | INV-001, INV-007 |
| REQ-009 | T6, T7, T8 | S10, S11, S12 | INV-008 |
| REQ-010 | T9 | S13 | INV-009 |

## 완료 판정과 증거

- K1 `ADK_RELEASE_ECDSA_PRIVATE_KEY`는 GitHub Environment `adk-companion-release`에만 설정했고, release run `29588526312`의 exact pair preflight가 checked-in K1과 일치함을 확인했습니다. K2는 GitHub secret과 signing input에 없습니다.
- annotated tag `v0.50.73`은 source commit `334b297f05942accbecdfa15b54e38e005c82f2d`를 가리키며, release에는 `checksums.txt`, 기존 cosign `checksums.txt.bundle`, 신규 `checksums.txt.signatures`가 함께 게시됐습니다.
- live checksum SHA-256은 `a30e0893f1565919e9e90dd7e1f2b19e5487024b0373f66de56729e1d747e7d1`, live envelope SHA-256은 `a1b1643f78995fea5b773d8e213d4e33e1d71ff98ec142c14a6361a4e20cccb3`이며 K1 fingerprint와 OpenSSL·Go 검증이 모두 통과했습니다.
- Stage 2 CI run `29618589360`에서 Windows PowerShell 5.1·7 CNG oracle과 macOS stock LibreSSL POSIX oracle이 통과했습니다. 독립 리뷰는 P0/P1/P2 finding 0으로 APPROVE했습니다.
- Outcome Lock은 만족했고 Mandatory requirements는 10/10, Must acceptance는 S1~S13 13/13, Completion Debt는 없습니다.

## 운영 잔여와 비목표

- encrypted local backup과 Keychain recovery는 검증했지만 별도의 off-device 독립 매체 보관은 확인하지 못했습니다. 이는 운영 복원력 한계이며 이 SPEC의 Completion Debt로 완료를 과장하지 않습니다.
- K2는 offline-next 상태입니다. K1+K2 overlap과 K1 retirement는 실제 회전 시점의 운영 절차입니다.
- 저장소 main 또는 raw-main 설치기 전달 경로의 침해는 여전히 범위 밖입니다. 독립 installer origin은 Evolution Idea이며 자동 후속 작업으로 예약하지 않습니다.
