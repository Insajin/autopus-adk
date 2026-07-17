# SPEC-ADK-RELEASE-SIGNING-001: 배포 아티팩트 Publisher 서명 검증

**Status**: completed
**Created**: 2026-07-17
**Domain**: RELEASE-SIGNING
**Module**: autopus-adk

## 목적

현재 배포 신뢰 체인은 checksum-only다. 두 소비자 모두 GitHub release assets라는 **동일 origin**에서 `checksums.txt`를 받아 그대로 신뢰한다: `pkg/selfupdate/downloader.go:36`의 `DownloadAndVerify`는 `ParseChecksums` 후 sha256 비교만 하고(서명 검증 없음), `install.sh:122`의 `verify_checksum()`도 같은 패턴이다. release asset을 교체할 수 있는 공격자(계정 탈취, 악성 릴리스)는 아카이브와 `checksums.txt`를 함께 바꾸면 무결성 검증을 통과한다. checksum은 전송 오류만 잡고 위조는 못 잡는다.

실측으로 producer 측은 이미 서명한다: `.goreleaser.yaml:65`의 `signs:` 블록이 `cosign sign-blob`(keyless, `id-token: write` + `COSIGN_EXPERIMENTAL=1`)으로 `checksums.txt.bundle`을 생성해 release asset으로 올린다. 그러나 두 소비자 중 누구도 이 서명을 검증하지 않는다. 또한 `install.sh:131`은 sha256 도구가 없으면 검증을 **건너뛰고**(`return 0`) 설치를 진행하는 fail-open 결함이 있다.

이 SPEC은 `checksums.txt`에 대한 publisher **ECDSA P-256 (prime256v1, SHA-256, ASN.1/DER) detached 서명**을 도입하고(`checksums.txt.sig`), 두 소비자가 클라이언트에 **임베드된 pinned public key 집합**으로 이 서명을 fail-closed 검증하도록 한다. 알고리즘을 ECDSA P-256으로 택한 근거는 이식성 실증이다: stock macOS의 `/usr/bin/openssl`은 LibreSSL 3.3.6로 ed25519 CLI(`genpkey -algorithm ed25519`, `pkeyutl -rawin`)를 지원하지 않아 ed25519 검증은 homebrew openssl 없는 모든 macOS에서 원라이너 설치를 파탄낸다. 반면 고전 `openssl dgst -sha256 -sign/-verify` 인터페이스의 ECDSA P-256은 LibreSSL 3.3.6에서 정상 동작한다(research 실증 참조). Go 측은 stdlib `crypto/ecdsa`(`VerifyASN1`)로 신규 의존성 0이다. 서명 키는 companion manifest의 ed25519 키와 **분리된 전용 release-signing ECDSA 키**를 쓰고, 회전·핀은 `pkg/companionmanifest`의 `PublicKeyReceipt`·`PinnedKey` **패턴**을 참조한다(ed25519 서명 코드 재사용이 아니라 설계 패턴 재사용).

**KeyID는 와이어(`.sig`)에 싣지 않는다.** bare 서명 파일은 in-band KeyID가 없고, release asset에 실린 KeyID는 공격자 통제 힌트일 뿐 보안 통제가 아니다. 대신 클라이언트는 임베드된 pinned key 집합 중 **비만료 키 전부로 검증을 시도(multi-trial)** 하고, 어느 키로도 검증되지 않으면 거부한다. KeyID·ExpiresAt는 임베드 상수의 감사·회전 북키핑 용도로만 존재한다.

## Outcome Boundary

- **User-visible outcome**: (1) 릴리스 자산에 publisher 서명 `checksums.txt.sig`가 포함된다. (2) `auto update --self`가 서명 검증 실패·부재 시 업데이트를 중단하고 사유를 출력한다. (3) `install.sh`가 서명 검증 실패·부재·검증 도구 부재 시 설치를 exit 1로 중단한다. (4) 서명·checksum·아카이브가 모두 유효한 정상 경로의 UX는 서명 도입 이전과 동일하다.
- **Mandatory requirements**: REQ-001 ~ REQ-009.
- **Explicit non-goals**: homebrew-tap formula/cask 서명 체인(별도 repo, 현행 sha256 핀 유지), Apple codesign/notarization(현행 유지), companion manifest 서명(이미 ed25519로 서명됨, 별개 신뢰 도메인), 기존 cosign keyless bundle 제거(defense-in-depth로 존치), 와이어 KeyID/envelope 전달, 서명 도입 이전 배포된 **구버전 바이너리의 소급 검증**(코드가 없어 불가 — 한계로 명시).
- **Completion evidence**: 합성 ECDSA P-256 키페어 기반 oracle 테스트 — selfupdate 검증기(정상/변조-checksums/변조-sig/서명부재/공격자키/만료키/회전창 각 concrete 기대 error·bool)와 `install.sh` 검증 함수(정상/변조/부재/검증도구-부재/만료 각 concrete exit·메시지), 그리고 동일-origin 전체 교체 위협(공격자 키로 재서명)이 임베드 키 집합 불일치로 두 소비자에서 모두 실패하는 oracle.

## 신뢰 경계

| 표면 | 신뢰 등급 | 근거 |
|------|-----------|------|
| GitHub release assets (아카이브, `checksums.txt`, `checksums.txt.sig`) | **untrusted** | 계정 탈취/악성 릴리스로 교체 가능. 서명 검증 대상 입력. 와이어 KeyID는 attacker-controlled 힌트라 미사용. |
| 바이너리에 임베드된 pinned ECDSA pubkey 집합 (selfupdate) | trusted anchor | `auto` 빌드 시 컴파일 인입, 공격자가 release asset만으로 바꿀 수 없음. |
| `install.sh`에 임베드된 pinned ECDSA pubkey 집합 (PEM SPKI + EXPIRES_n) | trusted anchor | `get.autopus.co` TLS로 배포(release assets와 다른 origin). |
| publisher release-signing ECDSA private key | secret | CI secret(placeholder `ADK_RELEASE_ECDSA_PRIVATE_KEY`), umask 077 + mktemp로 임시 materialize 후 즉시 삭제, 문서엔 placeholder만. |

## Requirements

### REQ-001: Publisher 서명 아티팩트 생성
THE SYSTEM SHALL 릴리스 파이프라인에서 publisher release-signing ECDSA P-256 private key로 `checksums.txt`의 SHA-256에 대한 detached ECDSA(ASN.1/DER) 서명을 만들고 이를 `checksums.txt.sig` release asset으로 게시해야 한다.
- EARS type: Ubiquitous
- Priority: Must
- 관측 지점: release assets 목록의 `checksums.txt.sig` 존재 + 검증기 통과

### REQ-002: selfupdate 서명 페치 및 검증
WHEN `auto update --self`가 릴리스를 다운로드할 때, THE SYSTEM SHALL `checksums.txt.sig`를 페치하고 바이너리에 임베드된 pinned ECDSA public key 집합으로 `checksums.txt`의 SHA-256에 대해 `ecdsa.VerifyASN1` multi-trial 검증(REQ-008)한 뒤에만 그 안의 checksum을 신뢰해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 검증 통과 시 checksum 단계 진입, 실패 시 진입 차단

### REQ-003: selfupdate fail-closed
WHEN 서명 검증이 실패하거나 `checksums.txt.sig` 자산이 부재할 때, THE SYSTEM SHALL 업데이트를 non-zero 결과로 중단하고 진단 메시지를 출력하며 checksum-only 경로로 폴백하지 않아야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: `DownloadAndVerify` error 반환 + `runSelfUpdate` 비정상 종료

### REQ-004: install.sh 서명 페치 및 검증
WHEN `install.sh`가 릴리스를 설치할 때, THE SYSTEM SHALL `checksums.txt.sig`를 페치하고 스크립트에 임베드된 pinned ECDSA public key 집합(PEM SPKI)으로 비만료 키마다 `openssl dgst -sha256 -verify`를 시도(REQ-008)한 뒤에만 그 안의 checksum을 신뢰해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 검증 통과 시에만 아카이브 checksum·설치 진행

### REQ-005: install.sh fail-closed (검증 도구 부재 포함)
WHEN `install.sh`가 openssl 부재로 서명을 검증할 수 없거나 서명이 부재·불일치할 때, THE SYSTEM SHALL 설치를 exit code 1로 중단하고 바이너리를 설치하지 않아야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: exit 1 + `INSTALL_DIR`에 바이너리 미생성 (기존 `install.sh:131` fail-open 제거)

### REQ-006: 아카이브 무결성은 인증된 checksum에 종속
WHILE 릴리스를 검증하는 동안, THE SYSTEM SHALL 아카이브의 sha256을 서명으로 인증된 `checksums.txt` 항목과 비교하고 불일치 시 중단해야 한다.
- EARS type: State
- Priority: Must
- 관측 지점: 서명 통과 후 checksum 비교, 불일치 시 error/exit 1

### REQ-007: 신뢰 앵커 출처
THE SYSTEM SHALL GitHub release assets를 untrusted 입력으로 취급하고, 클라이언트에 임베드된 pinned ECDSA public key 집합(바이너리 임베드 및 `get.autopus.co` 배포 스크립트 임베드)을 릴리스 진위의 유일한 신뢰 앵커로 사용해야 한다.
- EARS type: Ubiquitous
- Priority: Must
- 관측 지점: 검증 키가 release asset이 아닌 클라이언트 임베드 상수에서 로드됨

### REQ-008: 만료 게이트·multi-trial 검증·회전
THE SYSTEM SHALL 검증 이전에 ExpiresAt가 지난 임베드 pinned key를 시도 집합에서 제외하고, 남은 비만료 임베드 key 각각으로 서명 검증을 시도하며, 하나 이상의 key가 검증하면 서명을 진위로 인정하고, 어느 key도 검증하지 못하면 `no trusted release signing key verified`로, 모든 임베드 key가 만료면 `all embedded keys expired`로 거부해 두 소비자에서 대칭적으로 업데이트/설치를 중단해야 한다.
- EARS type: Ubiquitous
- Priority: Must
- 관측 지점: multi-trial 결과(통과 key 유무)·만료 제외 판정 + 임베드 상수의 KeyID·ExpiresAt; 회전은 과도기 2-key 임베드로 수행

### REQ-009: 정상 경로 UX 불변
WHEN 서명·checksum·아카이브 검증이 모두 통과할 때, THE SYSTEM SHALL 서명 도입 이전과 동일한 출력·단계로 업데이트/설치를 완료해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 기존 성공 메시지(`업데이트 완료` / `설치 완료`)와 exit 0 유지

## 생성 파일 상세

| 파일 | 역할 | 구분 |
|------|------|------|
| `pkg/selfupdate/signature.go` | ECDSA P-256 multi-trial 서명 검증(`VerifyReleaseSignature`), stdlib `crypto/ecdsa`·`crypto/sha256`·`crypto/x509` | [NEW] |
| `pkg/selfupdate/pinnedkey.go` | 임베드된 pinned key 집합(각: PEM SPKI pubkey·KeyID·ExpiresAt) + 비만료 시도 집합 조회 | [NEW] |
| `pkg/selfupdate/downloader.go` | `DownloadAndVerify`에 서명 검증 선행 단계 삽입 | 기존 수정 |
| `pkg/selfupdate/checker.go` | `FetchLatest` asset switch에 `checksums.txt.sig` 인식 추가 | 기존 수정 |
| `pkg/selfupdate/types.go` | `ReleaseInfo`에 `SignatureURL` 필드 추가 | 기존 수정 |
| `internal/cli/update_self.go` | `DownloadAndVerify` 호출에 signature URL 전달 | 기존 수정 |
| `install.sh` | `verify_signature()`(비만료 키 multi-trial dgst) + `EXPIRES_n` 만료 게이트, checksum fail-open 제거, pinned PEM 집합 임베드 | 기존 수정 |
| `.goreleaser.yaml` | `checksums.txt.sig` 서명 산출 배선 | 기존 수정 |
| producer signer 호출 | `checksums.txt` ECDSA 서명(umask 077+mktemp materialize→`openssl dgst -sha256 -sign`→trap 삭제) | [NEW] |

## Related SPECs

None (Primary SPEC, Outcome Lock 자기완결). 참조: `SPEC-DESKTOP-DEVICE-SETUP-001` 계열이 pinned-key·public-key-receipt 회전 **패턴**의 원천 `pkg/companionmanifest`를 산출했다(sibling 아님, 알고리즘은 ed25519로 다르고 in-band KeyID 조회라 검증 의미론도 다름 — 패턴 참조).

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1 | S1, S6 | INV-001, INV-004 |
| REQ-002 | T2, T3 | S1, S2 | INV-001 |
| REQ-003 | T2 | S2, S3 | INV-002 |
| REQ-004 | T4 | S1, S4 | INV-001 |
| REQ-005 | T4 | S4, S7 | INV-002 |
| REQ-006 | T2, T4 | S5 | INV-003 |
| REQ-007 | T3, T4 | S6 | INV-004 |
| REQ-008 | T3, T4 | S8 | INV-005 |
| REQ-009 | T2, T4, T5 | S9 | INV-001 |
