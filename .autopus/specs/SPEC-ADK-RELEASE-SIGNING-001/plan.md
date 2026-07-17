# SPEC-ADK-RELEASE-SIGNING-001 구현 계획

## 진행 원칙

- v0.50.73을 signing floor로 사용합니다. v0.50.72 이하는 unsigned legacy release입니다.
- Stage 1 producer와 Go consumer를 먼저 배포합니다.
- v0.50.73 live asset을 검증한 다음 Stage 2에서 POSIX와 Windows installer를 fail-closed로 전환합니다.
- fresh K1과 offline-next K2는 repository 밖 ceremony에서 생성하고 recovery를 검증했습니다. 소스에는 두 public pin만 포함합니다.
- v0.50.73 workflow는 K1만 사용합니다. K2 온라인 활성화는 이번 릴리스 범위가 아닙니다.
- 기존 cosign keyless bundle은 유지합니다.

## Implementation Strategy

- **RED**: envelope 형식, malformed pin, 4인자 API, producer·workflow wiring 기대값을 먼저 실패 테스트로 고정합니다.
- **GREEN**: Go 표준 라이브러리와 POSIX shell·openssl만 사용해 Stage 1 최소 구현을 완성합니다.
- **REFACTOR**: synthetic key 주입 seam을 package-private로 줄이고 format bounds를 세 소비자가 재사용할 공개 계약으로 고정합니다.
- **Release sequence**: Stage 1 code + K1·K2 public pin → K1 Environment secret → v0.50.73 live oracle → Stage 2 installers 순서를 hard gate로 유지합니다.

## Tasks

- [x] **T1 — V1 envelope와 trust-anchor 계약 구현** (REQ-001, REQ-006)
  - 전체 4,096바이트, 레코드 1~16개, 줄 256바이트 제한을 적용합니다.
  - exact header, LF 종료, CR/BOM/NUL/빈 줄 거부, full lowercase fingerprint, 탭 한 개, canonical base64·DER, duplicate fingerprint 거부를 구현합니다.
  - malformed embedded key와 all-expired를 서로 다른 sentinel error로 고정합니다.
  - fresh K1과 offline-next K2의 PEM·full fingerprint·만료일을 Go consumer와 checked-in 파일에 동일하게 고정합니다.

- [x] **T2 — deterministic multi-key producer 구현** (REQ-002)
  - `scripts/release-signing/sign-checksums.sh CHECKSUMS OUTPUT KEY_FILE...`을 구현합니다.
  - 각 private key에서 P-256 SPKI와 full fingerprint를 계산합니다.
  - fingerprint 오름차순으로 envelope를 생성하고 duplicate/non-P256 키를 거부합니다.
  - JSON과 jq를 사용하지 않습니다.

- [x] **T3 — release workflow 키 위생과 pair preflight 구현** (REQ-003)
  - raw secret을 별도의 짧은 step에서 0600 파일로 만듭니다.
  - checked-in K1 public PEM과 full fingerprint를 유지합니다.
  - K2 public pin은 선배포하되 workflow preflight와 GoReleaser 입력에는 추가하지 않습니다.
  - GoReleaser 전에 private/public/fingerprint tuple을 확인합니다.
  - GoReleaser에는 secret 값이 아니라 파일 경로만 전달합니다.
  - 기존 always-run credential cleanup을 재사용합니다.

- [x] **T4 — Go self-update consumer 구현** (REQ-004~006)
  - checker가 `checksums.txt.signatures`를 찾도록 합니다.
  - V1 envelope 검증을 checksum parsing보다 먼저 실행합니다.
  - 구조적으로 유효한 unknown fingerprint는 무시하되 known active signature가 하나 이상 통과해야 성공하도록 합니다.
  - 기존 4인자 `DownloadAndVerify`를 유지하고 exact sibling URL 파생에 실패하면 중단합니다.
  - synthetic key 주입 seam은 package-private로 제한합니다.

- [x] **T5 — Stage 1 oracle 구현** (REQ-001~006)
  - 합성 P-256 키로 정상, 변조, 공격자, unknown+known, duplicate, malformed DER/base64, 만료, malformed pin을 검증합니다.
  - helper의 dual-sign 정렬, duplicate와 non-P256 거부를 실행합니다.
  - GoReleaser v2.17.0 실행으로 실제 `checksums.txt.signatures`를 만든 뒤 같은 Go verifier가 통과하는지 확인합니다.

- [x] **T6 — K1·K2 ceremony와 v0.50.73 live release** (REQ-003, REQ-009)
  - [x] fresh K1과 offline-next K2를 생성하고 encrypted local custody와 Keychain 기반 recovery를 검증합니다.
  - [x] K1·K2 public PEM과 full fingerprint를 checked-in 정본과 Go consumer에 선배포합니다.
  - [x] K1 GitHub Environment secret을 설정하고 exact pair preflight를 통과합니다.
  - [x] K2는 offline-next로 유지하고 GitHub secret이나 v0.50.73 signing input에 추가하지 않습니다.
  - [x] v0.50.73을 릴리스하고 `checksums.txt`, `checksums.txt.bundle`, `checksums.txt.signatures`를 확인합니다.
  - [x] 내려받은 live envelope를 Go verifier와 openssl로 검증하고 fixture metadata에 release evidence를 보존합니다.

- [x] **T7 — POSIX installer Stage 2** (REQ-007, REQ-009, REQ-010)
  - T6 live evidence가 PASS인 경우에만 시작합니다.
  - `install.sh`에 동일한 4 KiB/16-record envelope parser와 active-key 검증을 구현합니다.
  - openssl 부재, malformed envelope, 서명 실패, checksum 실패를 모두 fail-closed로 처리합니다.
  - raw-main origin 한계를 문서와 테스트에 남깁니다.

- [x] **T8 — Windows installer Stage 2** (REQ-008~010)
  - T6 live evidence가 PASS인 경우에만 시작합니다.
  - PowerShell 5.1과 7 공통 코드로 SPKI DER를 CNG `ECS1` blob으로 바꿉니다.
  - canonical DER `r`, `s`를 고정 32바이트씩 채운 P1363 서명으로 바꾸고 `ECDsaCng.VerifyHash`를 실행합니다.
  - POSIX와 같은 파싱 한도·중복·unknown·만료 정책을 적용합니다.

- [x] **T9 — 전체 수렴과 lifecycle 종료** (REQ-010)
  - 세 소비자의 상태, signing floor, raw-main 한계, 키 회전 절차를 README/docs/CHANGELOG에 반영합니다.
  - full race/vet/shell/GoReleaser/release wiring gate를 실행합니다.
  - Stage 2까지 끝난 뒤에만 SPEC을 implemented로 전환하고 sync를 진행합니다.

## Gate 순서

```text
Stage 1 code/test
  -> fresh K1 + offline-next K2 public pin and recovery receipt
  -> K1 GitHub Environment secret + exact pair preflight
  -> v0.50.73 release
  -> live envelope verification
  -> install.sh Stage 2
  -> install.ps1 Stage 2
  -> three-consumer regression + sync
```

앞 gate가 실패하면 다음 gate로 진행하지 않습니다. 특히 live envelope 검증 전에 installer를 fail-closed로 배포하지 않습니다.

## Visual Planning Brief

```text
KEY_FILE... -> deterministic envelope producer -> checksums.txt.signatures
                                                    |
archive + checksums + envelope (untrusted)           v
      -> strict full parse -> active known key verify -> checksum verify -> replace/install
                               | failure
                               +-> fail closed
```

producer와 세 consumer가 같은 4 KiB/16-record/256-byte-line 계약을 사용해야 합니다. 회전 중에는 K1+K2 레코드를 함께 싣고, 각 consumer는 자신이 아는 active fingerprint 중 하나 이상을 검증합니다.

## Feature Completion Scope

- Primary SPEC 안에서 Stage 1 producer·Go consumer, v0.50.73 live gate, Stage 2 POSIX·Windows consumer까지 닫습니다.
- sibling SPEC은 만들지 않습니다.
- Completion Debt는 모두 해소됐습니다. v0.50.73 live release, 두 installer, cross-runtime CI, 최종 문서 동기화까지 Primary SPEC 안에서 완료했습니다.
- raw-main 독립 trust origin, Homebrew 서명, SLSA/Rekor는 Evolution Idea이며 완료를 막지 않습니다.

## 완료 증거

1. Release run `29588526312`: K1 Environment pair preflight, K1-only envelope, cosign bundle, credential cleanup PASS.
2. v0.50.73 live fixture: source `334b297f05942accbecdfa15b54e38e005c82f2d`, checksum/envelope exact hash와 K1 signature PASS.
3. Stage 2 CI run `29618589360`: Windows PS5.1·PS7, macOS POSIX, static contracts, lint, race/coverage/vet gate PASS.
4. Independent Stage 2 review: P0/P1/P2 0, APPROVE.

## 운영 한계

- K1·K2 encrypted local custody와 Keychain 기반 recovery 검증은 완료했습니다.
- 별도의 off-device 독립 매체 보관은 현 환경에서 확인하지 못했습니다. 이 문서는 그 수준의 재해 복구를 완료했다고 주장하지 않습니다.
- K2 온라인 signing activation과 K1+K2 overlap은 후속 회전 단계입니다.
