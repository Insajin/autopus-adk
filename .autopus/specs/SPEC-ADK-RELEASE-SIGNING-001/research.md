# SPEC-ADK-RELEASE-SIGNING-001 리서치

## 기존 상태

- Go self-update, `install.sh`, `install.ps1`은 같은 GitHub Release의 `checksums.txt`를 신뢰합니다. 실제 소비자는 셋이며 cosign bundle을 검증하지 않습니다.
- 초안의 bare `checksums.txt.sig`는 signer 식별과 K1+K2 동시 전달이 불가능합니다. 또한 v0.50.71·v0.50.72에는 publisher signature asset이 없으므로 실제 floor는 v0.50.73입니다.
- 초안의 `get.autopus.co` 가정과 달리 공식 installer는 현재 `raw.githubusercontent.com/.../main/install.{sh,ps1}`입니다.

## Outcome Lock

- **User-visible outcome**: signed checksum envelope 게시와 세 소비자의 fail-closed 검증, 정상 UX 유지.
- **Mandatory requirements**: REQ-001~REQ-010.
- **Explicit non-goals**: K2 온라인 signing activation, cosign 제거, Homebrew/Apple/companion 변경, legacy 자산 소급, raw-main 이전, SLSA/Rekor.
- **Completion evidence**: synthetic Stage 1 oracle만으로는 부족하며 K1·K2 public pin과 custody·recovery receipt, K1 Environment pair preflight, v0.50.73 live oracle, POSIX·PS5.1·PS7 oracle가 모두 필요합니다.

## 결정 1: versioned multi-signature envelope

bare DER 대신 다음 V1 envelope를 사용합니다.

```text
AUTOPUS-RELEASE-SIGNATURE-V1
fingerprint<TAB>base64-DER
```

fingerprint는 public key의 canonical SPKI DER에 대한 SHA-256 전체 lowercase hex입니다. release asset에서 별도 KeyID를 받아 키를 고르는 방식이 아닙니다. 클라이언트가 가진 pin과 fingerprint가 일치할 때만 해당 서명을 시도합니다.

K1과 K2를 함께 실을 수 있고 구·신 클라이언트가 자신이 아는 레코드를 검증할 수 있습니다. producer는 fingerprint 순으로 정렬하며, unknown fingerprint는 전체 구조 검증을 통과한 뒤에만 무시합니다.

## 결정 2: 세 소비자가 공유하는 strict bounds

| 항목 | 값 | 근거 |
|---|---:|---|
| 전체 envelope | 4,096 bytes | 16개 P-256 서명과 header에 충분하며 다운로드 메모리 상한이 작습니다. |
| record count | 1~16 | 통상 회전은 2개지만 긴급 다중 서명을 허용하면서 무제한 trial을 막습니다. |
| line length | 256 bytes | full fingerprint 64 + tab + P-256 DER base64 약 96바이트보다 넉넉합니다. |
| line ending | ASCII LF, final LF required | Go, POSIX, PowerShell parser가 같은 wire bytes를 보게 합니다. |
| forbidden | CR, BOM, NUL, blank line, duplicate fingerprint | parser 차이와 모호성을 제거합니다. |

파서는 모든 레코드의 fingerprint, base64, DER, P-256 scalar 범위를 먼저 확인합니다. 그 뒤 unknown fingerprint를 무시하고 known active key만 암호 검증합니다. 이 순서를 바꾸면 malformed unknown 레코드를 소비자마다 다르게 처리할 수 있습니다.

## 결정 3: ECDSA P-256

Go는 표준 라이브러리 `crypto/ecdsa`, `crypto/x509`, `crypto/sha256`만 사용합니다. producer와 POSIX installer는 `openssl dgst -sha256`을 사용할 수 있습니다. 기존 조사에서 stock macOS LibreSSL 3.3.6은 P-256 `dgst -sign/-verify`를 지원했지만 ed25519 CLI 경로는 지원하지 않았습니다.

서명 값은 ASN.1/DER `SEQUENCE(INTEGER r, INTEGER s)`입니다. Go parser는 canonical DER 재인코딩 일치, 양수 scalar, P-256 order 미만을 확인합니다. 이 조건은 Stage 2 PowerShell의 DER→P1363 변환에도 그대로 적용할 수 있습니다.

## Windows PowerShell 5.1/7 경로

PowerShell 5.1에는 DER signature-format overload가 없습니다. P-256 SPKI DER 91바이트에서 `X`, `Y`를 추출해 `45 43 53 31 20 00 00 00 || X || Y` CNG `ECS1` blob을 만들고, canonical DER의 positive `r`, `s`를 각각 32바이트로 채워 P1363 `r||s`로 바꾼 뒤 `ECDsaCng.VerifyHash`를 호출합니다. Stage 2는 같은 vector를 PS5.1과 PS7에서 검증해야 합니다.

## K1·K2 trust anchors와 ceremony evidence

2026-07-17 ceremony에서 fresh P-256 K1과 offline-next K2를 생성했습니다. public pin 정본은 다음과 같습니다.

| 역할 | 만료일 | full SPKI SHA-256 fingerprint |
|---|---|---|
| K1 active signer | `2028-07-17` | `e1fdfe066484c7eae8ff16fa4b1ee6237b8d06299c2b66ced485f029af77837f` |
| K2 offline-next | `2030-07-17` | `93d9f681d829f2d0bdba7e1853e6acf9ae2ffd2c760355853218e920c35cc5ff` |
두 public PEM과 fingerprint는 `scripts/release-signing/release-k{1,2}-{public.pem,.fingerprint}`와 Go consumer에 동일하게 고정합니다. ceremony receipt는 두 private key의 encrypted local custody와 Keychain 기반 recovery 검증이 완료됐다고 기록합니다. 별도의 off-device 독립 매체 보관은 현 환경에서 확인하지 못했으므로 독립 재해 복구까지 완료했다고 주장하지 않습니다.

v0.50.73 workflow는 K1 하나로만 서명합니다. K2는 GitHub secret에 올리지 않고 offline-next 상태를 유지합니다. 후속 회전 순서는 K2 public pin 선배포(이번 단계) → K1+K2 동시 서명 활성화 → overlap 뒤 K1 제거입니다.

## Secret materialization

별도의 짧은 step이 K1 GitHub Environment secret을 0700 credential directory의 0600 파일로 만들고, 다음 step이 private/public/fingerprint tuple을 확인합니다. GoReleaser에는 파일 경로만 전달하며 `if: always()` cleanup이 제거 실패를 숨기지 않습니다. K1 Environment secret 설정과 exact pair 검증은 아직 외부 blocker입니다. K2는 이 경로에 들어가지 않습니다.

## GoReleaser wiring과 실행 증거

`.goreleaser.yaml`은 cosign `checksums.txt.bundle`과 helper의 `checksums.txt.signatures`를 함께 만듭니다. helper는 `CHECKSUMS OUTPUT KEY_FILE...`을 받아 1~16개 P-256 key를 self-check하며 JSON/jq 없이 duplicate를 거부합니다. 2026-07-17에 공식 checksum을 확인한 GoReleaser v2.17.0 Darwin arm64 binary로 synthetic project oracle을 실행했고, 생성 envelope를 production Go parser·verifier가 통과했습니다. production config는 `goreleaser check`와 companion wiring test로 검증합니다.

## API 호환 결정

기존 4인자 `DownloadAndVerify(archiveURL, checksumURL, archiveName, destDir)`를 유지합니다. exact `checksums.txt` URL의 sibling `.signatures` URL을 계산해 explicit 메서드에 위임하며 userinfo, fragment, encoded path, 다른 basename, 비 HTTP(S) scheme은 거부합니다. 계산 실패 시 checksum-only로 돌아가지 않습니다.

합성 키와 clock을 주입하는 seam은 package-private입니다. 외부에 테스트 전용 mutable key set을 노출하지 않습니다. 공개 error sentinel은 호출자가 trust-anchor 구성 오류, 전 만료, envelope 형식 오류, 미신뢰 서명을 구분하는 production contract입니다.

## two-stage rollout

순서는 `v0.50.72 이하 unsigned → Stage 1 + K1·K2 public pin → K1 Environment secret → v0.50.73 signed release → live oracle → Stage 2 installers`입니다. live evidence가 나오기 전에 installer가 `.signatures`를 요구하지 않도록 이 순서를 hard gate로 둡니다.

## Visual Planning Brief

```text
GitHub Release assets (untrusted)
  checksums.txt -----------+                    embedded pins (trusted)
  checksums.txt.signatures +-> strict parse -> active fingerprint map
  archive -----------------+                         |
                                                     v
                          >=1 known signature verifies?
                                  | yes                 | no
                                  v                     v
                         archive SHA-256 check       abort
                                  |
                            replace/install
```

raw-main installer 자체는 오른쪽 trusted anchor를 담는 전달체이지만 GitHub repository와 독립된 origin은 아닙니다.

## raw-main installer 한계

`install.sh`와 `install.ps1`은 현재 GitHub raw main에서 직접 실행하도록 안내합니다. release assets만 교체한 공격자는 스크립트에 고정된 K1·K2 pin을 바꿀 수 없으므로 envelope 검증에 실패합니다. 그러나 repository main과 release assets를 함께 장악한 공격자는 스크립트의 공개키도 바꿀 수 있습니다.

따라서 Stage 2가 완료돼도 다음 주장은 할 수 없습니다.

- installer가 GitHub repository와 독립된 trust origin이다.
- raw-main script 자체가 version-pinned 또는 immutable하다.
- repository compromise 전체를 방어한다.

독립 도메인, version-pinned installer, 별도 bootstrap trust는 후속 hardening 후보이며 이번 완료 조건은 아닙니다. 문서에는 이 한계를 숨기지 않습니다.

## 위험과 완화

| 위험 | 완화 |
|---|---|
| GitHub secret이 K1과 다른 key | release 전 exact pair preflight로 차단 |
| private key가 env에 오래 남음 | 짧은 materialization step 후 file path만 전달 |
| 회전 전 K2 전달 누락 | v0.50.73 Go consumer에 K2 public pin 선배포 |
| 회전 중 구·신 클라이언트 단절 | 후속 K1+K2 multi-sign envelope와 overlap |
| K2 조기 온라인 노출 | K2는 GitHub secret과 v0.50.73 producer 입력에서 제외 |
| 로컬 장비 상실 | encrypted local recovery는 검증했지만 off-device 독립 매체 미확인은 운영 한계로 명시 |
| unknown record parser 차이 | full parse before fingerprint lookup |
| oversized/duplicate input DoS | 4 KiB, 16-record, 256-byte line 상한 |
| installer 조기 fail-closed | v0.50.73 live evidence를 Stage 2 hard gate로 사용 |
| raw-main까지 함께 침해 | 명시적 threat-model 한계로 기록; 독립 bootstrap은 후속 작업 |

## Semantic Invariant Inventory

| ID | Source clause | Invariant type | Affected outputs | Acceptance IDs |
|---|---|---|---|---|
| INV-001 | 서명 통과 뒤에만 checksum 신뢰 | ordering | checksum 진입, 설치/교체 | S1, S4, S11, S12 |
| INV-002 | V1 strict wire format | canonical parsing | parser error taxonomy | S2 |
| INV-003 | K1+K2 회전 지원 | deterministic multi-sign membership | record ordering, producer output | S7, S9 |
| INV-004 | raw secret 최소 노출과 exact pair | credential provenance | workflow gate | S8, S10 |
| INV-005 | checksum-only fallback 금지 | fail-closed | downloader result | S4, S6 |
| INV-006 | malformed pin과 all-expired 구분 | error partition | sentinel error | S5 |
| INV-007 | 세 소비자 format parity | cross-runtime equivalence | Go/POSIX/PS 결과 | S11, S12 |
| INV-008 | v0.50.73 live gate 선행 | release sequencing | deployment order | S10, S11, S12 |
| INV-009 | raw-main 한계 비은폐 | trust-boundary disclosure | 문서·운영 판단 | S13 |
| INV-010 | K1·K2 public pin parity와 K1-only activation | trust-anchor provenance | checked-in files, Go consumer, workflow | S5, S8 |

## Feature Coverage Map

| Outcome slice | Coverage | Status |
|---|---|---|
| V1 envelope parser·Go verifier | T1, T4 / S1~S6 | implemented, gates running |
| multi-key producer·workflow preflight | T2, T3 / S7~S9 | implemented, gates running |
| K1·K2 local ceremony·public pin | T1, T6 / S5, S8 | completed |
| K1 Environment·v0.50.73 live asset | T6 / S10 | blocked |
| POSIX installer | T7 / S11 | pending live gate |
| Windows PS5.1/7 installer | T8 / S12 | pending live gate |
| 정상 UX·한계 문서 | T9 / S13 | pending Stage 2 |

## Evolution Ideas

| Idea | Why not required now | Promotion trigger |
|---|---|---|
| version-pinned 또는 독립 installer origin | 현재 Outcome Lock은 release-asset-only 공격 방어 | 별도 bootstrap trust 요청 |
| Homebrew provenance 검증 | 별도 repository와 배포 체인 | Homebrew trust-chain SPEC |
| cosign/Rekor consumer | 현재 pinned P-256 검증과 별도 기능 | transparency 검증 요청 |
| SLSA provenance | 공급망 확장 범위 | 별도 release-hardening SPEC |

## Reference Discipline

| Reference | Type | Verification |
|---|---|---|
| `pkg/selfupdate/downloader.go::DownloadAndVerify` | existing public API | A4 기준 `6948fde`의 4인자 signature 확인 |
| `pkg/selfupdate/checker.go::FetchLatest` | existing | release asset switch 확인 |
| `.goreleaser.yaml::signs` cosign | existing | checksum bundle signer 존치 확인 |
| `install.sh`, `install.ps1` | existing consumer | checksums 다운로드 경로 확인; Stage 1에서는 조기 `.sig` 구현 제거 외 서명 검증을 추가하지 않음 |
| `pkg/selfupdate/signature.go` | Stage 1 source | strict parser·sentinel oracle 확인 |
| `scripts/release-signing/sign-checksums.sh` | Stage 1 source | OpenSSL·GoReleaser executable oracle 확인 |
| `scripts/release-signing/release-k1-public.pem` | fresh active K1 | P-256 SPKI와 full fingerprint 확인 |
| `scripts/release-signing/release-k2-public.pem` | offline-next K2 | P-256 SPKI, full fingerprint, Go embedded pin parity 확인 |

## Reviewer Brief

- **Intended Stage 1 scope**: V1 multi-sign envelope, deterministic producer, fresh K1·offline-next K2 public pin, K1 workflow key-file preflight, Go self-update verifier, API 호환, SPEC 교정.
- **Do not expand here**: 조기 `.sig` 구현 제거를 넘는 installer 서명 검증, K2 online signing activation, tag/A4 값, Homebrew/Apple/companion lineage.
- **Security focus**: full parse before unknown ignore, DER canonicality, fingerprint provenance, malformed/all-expired partition, raw secret scope, two-stage order.
- **Completion status**: Stage 1 code can pass review, but 전체 SPEC verdict는 K1/live/installer debt 때문에 REVISE입니다.

## Self-Verify Summary

- Q-CORR-04 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 기존 3소비자와 신규 Stage 1 경로를 분리하고 `[NEW]`/pending 범위를 명시함
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, acceptance.md | reason: INV-001~010을 concrete sentinel·exit·bool oracle S1~S13에 연결함
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md | reason: Traceability Matrix가 REQ·Task·Scenario·Invariant를 모두 연결함
- Q-COMP-07 | status: PASS | attempt: 2 | files: spec.md, research.md | reason: K1·K2 local ceremony와 pin 선배포 완료를 증거에 반영하고 K1 Environment/live/installers만 Completion Debt로 남김

## Completion Debt

| 부채 | 상태 | 해소 조건 |
|---|---|---|
| K1·K2 encrypted local custody와 public pin | RESOLVED | ceremony recovery receipt와 checked-in/embedded 2-pin parity |
| K1 GitHub Environment secret | BLOCKED | fresh K1 설정과 exact pair preflight |
| v0.50.73 live asset | BLOCKED | 실제 release와 독립 검증 |
| POSIX installer | PENDING | live gate 뒤 V1 parser·oracle PASS |
| Windows installer | PENDING | PS5.1/PS7 CNG oracle PASS |
| 세 소비자 문서 동기화 | PENDING | Stage 2 뒤 README/docs/CHANGELOG 반영 |

Completion Debt가 남아 있으므로 SPEC 상태는 `in-progress`, review verdict는 `REVISE`입니다. 운영 한계로 encrypted local backup과 Keychain 기반 recovery는 검증했지만, 별도의 off-device 독립 매체 보관은 현 환경에서 확인하지 못했습니다. 이 제한은 K1 Environment/live release gate와 별도로 계속 명시합니다.
