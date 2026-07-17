# SPEC-ADK-RELEASE-SIGNING-001 리서치

## 기존 코드 분석

- `pkg/selfupdate/downloader.go:36` `DownloadAndVerify(archiveURL, checksumURL, archiveName, destDir)`: checksum 다운로드 → `ParseChecksums`(line 120) → sha256 비교(line 58-61)만. 서명 검증 없음. 서명 삽입 지점.
- `pkg/selfupdate/checker.go:74-98` `FetchLatest`: asset switch가 `expectedArchive`와 `"checksums.txt"`만 인식. `checksums.txt.sig` 미포착 → `ReleaseInfo`(`types.go:4`)에 `SignatureURL` 추가 필요.
- `internal/cli/update_self.go:81` `dl.DownloadAndVerify(info.ArchiveURL, info.ChecksumURL, ...)`: 검증 호출 지점, error를 `다운로드/검증 실패: %w`로 전파(line 83) → 프로세스 non-zero.
- `install.sh:122-141` `verify_checksum()`: sha256sum/shasum 비교. **line 131-135는 도구 부재 시 `return 0`(fail-open)** — 이번에 fail-closed로 교체.
- `install.sh:165-174` main: `checksums.txt` 다운로드 → grep → `verify_checksum`. 서명 페치/검증 없음.
- `.goreleaser.yaml:62-76`: `checksum: name_template: checksums.txt` + `signs: cosign sign-blob --bundle → checksums.txt.bundle`(keyless). producer는 이미 cosign 서명하나 소비자가 검증 안 함.
- `.github/workflows/release.yaml:25,95,163` CI가 ed25519 companion 키(`ADK_COMPANION_ED25519_PRIVATE_KEY`, `COMPANION_KEY_ID`, receipt vars) 프로비저닝 + `./cmd/auto`를 서명기로 빌드(line 77). 이번 release-signing 키는 **여기에 신규 ECDSA secret을 추가**한다(용도 분리).
- 패턴 참조 machinery(알고리즘·검증 의미론 모두 다름 — 코드 재사용 아님, 설계 참조): `pkg/companionmanifest/verify.go:39` `Verify`는 `policy.PinnedKeys[manifest.KeyID]`로 **in-band KeyID 조회**(서명 대상이 KeyID 필드를 가진 struct)한다. release `.sig`는 bare 파일이라 in-band KeyID가 없으므로 그 조회 패턴을 쓸 수 없다 → multi-trial로 대체. `public_key_receipt.go:85` `IssuePublicKeyReceipt`(KeyID·expiry = 회전 북키핑 앵커)만 개념 참조. ECDSA 서명/검증은 stdlib `crypto/ecdsa`(`SignASN1`/`VerifyASN1`).
- pinned pubkey `go:embed` 상수: `pkg/selfupdate`·`internal/`에 부재(grep 확인) → 임베드 앵커는 `[NEW]`.

## Outcome Lock

- **User-visible outcome**: `checksums.txt.sig` 자산 게시 + `auto update --self`/`install.sh`가 서명 실패·부재 시 중단·사유출력 + 정상 경로 UX 불변.
- **Mandatory requirements**: REQ-001~REQ-009.
- **Explicit non-goals**: homebrew-tap 체인, Apple notarization, companion manifest 서명, cosign bundle 제거, 와이어 KeyID/envelope, 구버전 바이너리 소급 검증.
- **Completion evidence**: 합성 ECDSA P-256 키페어 oracle 테스트(Go 검증기 + install.sh 함수) 정상/변조/서명부재/공격자키/만료키/회전창/검증도구부재/동일-origin-전체교체 각 concrete 기대값.

## Visual Planning Brief (검증 data-flow)

```
release assets (UNTRUSTED, 와이어 KeyID 없음)   client (TRUSTED anchor)
 checksums.txt ─┐                        임베드 pinned key 집합 {PEM_i, KeyID_i, ExpiresAt_i}
 checksums.txt.sig ─┐                            │ 만료 키 제외(사전 게이트)
 archive.tar.gz     │                            ▼
        │           └─► for k in non-expired: VerifyASN1/dgst(k, sha256(checksums.txt), sig)
        │                    │ 하나라도 pass ─► authentic
        │                    │ 전부 fail ─► "no trusted release signing key verified" ABORT
        │                    │ 전부 만료 ─► "all embedded keys expired" ABORT
        │                    ▼ pass
        └────────► sha256(archive) == checksums.txt[archive]?  mismatch ─► ABORT / match ─► install
```
동일-origin 공격자가 셋을 모두 교체(+임의 KeyID 힌트)해도 임베드 키 집합에 없는 키라 어떤 trial도 통과 못 함.

## 설계 결정 — 서명 메커니즘 (검증-측 의존성 + 이식성 실증)

**Stock macOS 실증(2026-07-17, `/usr/bin/openssl`)**: `LibreSSL 3.3.6`. `genpkey -algorithm ed25519` → `Algorithm ed25519 not found`, `pkeyutl -help`에 `-rawin` 없음 → **ed25519 CLI 검증 불가**. 반면 `ecparam -name prime256v1 -genkey` → `dgst -sha256 -sign` → `dgst -sha256 -verify` → 유효 `Verified OK`(exit 0), 변조 `Error Verifying Data`(exit 1). 고전 dgst 인터페이스 ECDSA는 LibreSSL·OpenSSL 전 계열 동작.

| 옵션 | selfupdate(Go) 검증 | install.sh(POSIX sh) 검증 | stock macOS 통과 | 판정 |
|------|---------------------|---------------------------|-----------------|------|
| ed25519 + `pkeyutl -rawin` | stdlib `crypto/ed25519`(신규 0) | LibreSSL 3.3.6 미지원 → homebrew 필수 | **불가**(원라이너 파탄) | 반려 |
| **ECDSA P-256 + `dgst -verify`** | stdlib `crypto/ecdsa`(신규 0) | LibreSSL 3.3.6 동작(실증) | **통과** | **채택** |
| cosign keyless bundle | sigstore-go(대형)/CLI(부재) | cosign CLI(부재) | 불가 | 반려 |
| minisign | 신규 dep/CLI(부재) | minisign CLI(부재) | 불가 | 반려 |

**결정(신뢰도 高)**: ECDSA P-256(prime256v1, SHA-256, ASN.1/DER). Go는 stdlib `crypto/ecdsa`로 신규 의존성 0, install.sh는 stock macOS 포함 openssl 전 계열 동작 `dgst -sha256 -verify`. ed25519(companion)와 알고리즘이 달라 **전용 release-signing ECDSA 키 신규 프로비저닝**(용도 분리). 기존 cosign bundle은 Rekor 투명성 가치로 존치. 서명 대상은 `checksums.txt` 1개로 전체 아카이브 커버.

**키 식별 = multi-trial(F-001)**: bare `.sig`엔 in-band KeyID가 없고 release-asset KeyID는 attacker-controlled 힌트라 신뢰 불가. 따라서 클라이언트는 임베드 키 집합 중 비만료 키 전부로 검증 시도, 하나라도 통과하면 진위, 전부 실패면 `no trusted release signing key verified`. 만료는 시도 이전 클라이언트 게이트(전부 만료 → `all embedded keys expired`). 회전 = 과도기 2키 임베드(시도 집합 2개) 후 구키 제거 — multi-trial이 곧 회전 메커니즘. companion `verify.go:39`의 in-band KeyID 조회와 다른 이유는 서명 대상이 struct가 아니라 bare 파일이기 때문.

- 서명 포맷: `checksums.txt.sig` = ECDSA-over-SHA256 ASN.1/DER(가변 ~70-72B). Go: `ecdsa.VerifyASN1(pub, sha256(bytes), sig)`. install.sh: 표준 PEM SPKI 임베드 후 `openssl dgst -sha256 -verify pub.pem -signature checksums.txt.sig checksums.txt`(키 포맷 변환 불필요).
- 롤아웃: 새 검증기는 서명 필수(fail-closed 기본). 구버전 바이너리는 코드 부재로 소급 불가(한계). install.sh 배포는 첫 서명 릴리스 이후로 순서 고정.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | stdlib `crypto/ecdsa`·`crypto/x509` (신규 dep 0) | go 1.26 (`go.mod:3`) | 로컬 `go.mod` | 2026-07-17 | crypto/ed25519(LibreSSL CLI 미지원), sigstore-go(대형) |
| brownfield(runtime tool) | install.sh 검증기 `openssl dgst` + `date -u`(만료 사전식 비교) | LibreSSL 3.3.6 동작 실증(stock macOS) | 로컬 `/usr/bin/openssl` | 2026-07-17 | ed25519 pkeyutl(LibreSSL 미지원), cosign/minisign CLI(부재) |
| brownfield | 기존 cosign keyless(존치) | cosign v3.5.0 (`release.yaml:63` `@59acb62`) | 로컬 workflow | 2026-07-17 | cosign 제거(투명성 로그 상실) |
| brownfield | goreleaser(존치) | v2.17.0 (`release.yaml:59`) | 로컬 workflow | 2026-07-17 | — |

major 버전 변경·마이그레이션 없음. 신규 Go 의존성 없음. 신규 CI secret(release-signing ECDSA key)만 추가.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | Outcome Lock: 동일-origin asset 교체 방어 = 서명 검증 | proceed | checksums.txt publisher 서명 검증 |
| existing code/helper/pattern | `companionmanifest` PinnedKey·PublicKeyReceipt(verify.go:39, public_key_receipt.go:85) rg 확인 | reuse-pattern | 회전·핀 개념(in-band KeyID 조회는 부적합→multi-trial) |
| stdlib/native | Go `crypto/ecdsa` `VerifyASN1`; install.sh는 openssl dgst + date -u(native, LibreSSL 3.3.6 실증) | use | Go 검증 stdlib, sh 검증 openssl+date |
| existing dependency | cosign bundle(goreleaser.yaml:65) 존재하나 소비자 검증 의존성 과중 | not sufficient(consumer) | cosign 존치·소비자 미채택 |
| new dependency or new abstraction | 신규 Go dep 0; `[NEW]` 최소 코드 2파일+install.sh 함수; 신규 CI ECDSA 키 | accepted | multi-trial 검증기·임베드 키 집합·전용 키 |
| minimum sufficient verification | 합성 ECDSA 키 oracle: 정상/변조/부재/공격자키/만료/회전창/도구부재/전체교체 | required checks | S1~S9 oracle(구조검사 미사용) |

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "publisher 서명 포함" + "서명 검증 실패 시 중단" | paired matching (sig↔checksums SHA-256↔임베드 키) | 검증 bool, checksum 단계 진입 | S1, S2, S4 |
| INV-002 | "서명 부재·불일치 → 설치/업데이트 중단" | fail-closed guard | exit code, error 문자열 | S2, S3, S4, S7 |
| INV-003 | "서명 없는 checksums.txt를 그대로 신뢰" 결함 제거 | ordering (auth-then-checksum) | 아카이브 checksum 결과 | S5 |
| INV-004 | "release asset을 바꿀 수 있는 공격자" 방어 | trust-anchor provenance | 키 로드 출처, 전체교체 거부 | S6 |
| INV-005 | "회전은 public-key receipt 패턴 참조" + 와이어 KeyID 부재 | multi-trial membership + pre-trial expiry gate | 통과 key 유무, 만료 제외, 회전창 통과 | S8 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| producer 서명 산출(`checksums.txt.sig`) | Primary T1 | covered |
| selfupdate multi-trial 검증 + fail-closed | Primary T2, T3 | covered |
| install.sh multi-trial 검증 + 만료 게이트 + fail-closed | Primary T4 | covered |
| 롤아웃 순서·버전 경계 | Primary T5 | covered |
| oracle 테스트 | Primary T6 | covered |
| homebrew-tap 서명 | — | non-goal (Outcome Lock 제외) |

## Completion Debt

REQ-008이 multi-trial+만료 게이트로 SPEC 내에서 두 소비자 대칭 종결되어 Completion Debt 없음.

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

optional 개선이며 sync completion을 막지 않는다. SPEC/task/acceptance ID를 부여하지 않는다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| homebrew-tap cask 서명 검증 체인 | Outcome Lock non-goal(별도 repo) | 사용자가 명시 요청 |
| cosign bundle을 Rekor 투명성 검증까지 소비 | 현 위협모델은 임베드 키로 충분 | 사용자가 명시 요청 |
| SLSA provenance attestation | Outcome Lock 밖 공급망 강화 | 사용자가 명시 요청 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC이 Outcome Lock을 닫음(단일 응집 변경) | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/selfupdate/downloader.go::DownloadAndVerify` | existing | Read 확인(line 36) |
| `pkg/selfupdate/checker.go::FetchLatest` | existing | Read 확인(line 38-106) |
| `pkg/selfupdate/types.go::ReleaseInfo` | existing | Read 확인(line 4) |
| `internal/cli/update_self.go::runSelfUpdate` | existing | Read 확인(line 19,81) |
| `install.sh::verify_checksum` (fail-open line 131) | existing | Read 확인 |
| `.goreleaser.yaml` signs/cosign | existing | Read 확인(line 65) |
| `pkg/companionmanifest/verify.go::Verify` in-band KeyID 조회(line 39) | existing | Read 확인(패턴만, 의미론 다름) |
| stock `/usr/bin/openssl` LibreSSL 3.3.6 ed25519 미지원 / ECDSA dgst 동작 | existing(env) | 로컬 실행 실증(2026-07-17) |
| `pkg/selfupdate/signature.go`, `pinnedkey.go` | [NEW] planned addition | 미존재(grep 확인) |
| `checksums.txt.sig` 자산, 전용 ECDSA CI 키 | [NEW] planned addition | 미존재 |

## Reviewer Brief

- **Intended scope**: `checksums.txt` publisher ECDSA P-256 서명 도입 + 두 소비자(selfupdate, install.sh) fail-closed multi-trial 검증. 신뢰 앵커 = 클라이언트 임베드 pinned key 집합. 와이어 KeyID 미사용.
- **Explicit non-goals**: homebrew-tap, Apple notarization, cosign 제거, 와이어 KeyID/envelope, 구버전 소급, SLSA — 리뷰 scope 확장 금지.
- **Self-verified**: Traceability Matrix, Semantic Invariant Inventory, oracle acceptance, existing/[NEW] Reference Discipline, Technology Stack Decision, LibreSSL 이식성 실증.
- **Reviewer should focus on**: correctness(서명↔SHA256↔임베드 키 결합, DER 파싱), multi-trial/만료 게이트 대칭성(두 소비자), fail-closed 완전성(부재/변조/공격자키/전만료/openssl부재), 신뢰 앵커 출처, Go↔openssl 상호운용, regression risk. Completion Debt 없음.

## Risks

- **R1 (openssl 자체 부재, 低)**: openssl 바이너리 자체가 없는 드문 환경에서 install.sh fail-closed로 차단 → 설치 안내 출력. stock macOS LibreSSL 3.3.6은 통과(실증)라 기본 경로 아님. S7 고정.
- **R2 (롤아웃 순서, 低)**: install.sh 배포가 첫 서명 릴리스보다 앞서면 `.sig` 부재로 설치 막힘 → T5가 순서 고정.
- **R3 (Go↔openssl 상호운용, 低)**: producer `openssl dgst -sha256 -sign` ↔ Go `ecdsa.VerifyASN1(pub, sha256(...), sig)` 동일 ECDSA-over-SHA256 DER. T6 oracle이 양방향 고정.

## Plan Intent Ledger

Clarification Ledger unavailable — team-lead 프롬프트로 착수. 리비전1에서 stock macOS LibreSSL 3.3.6 실증으로 ed25519→ECDSA P-256 확정(로컬 재현). 리비전2(리뷰 REVISE)에서 F-001(와이어 KeyID 부재→multi-trial 의미론), F-002(CI secret materialize), F-003(install.sh 만료 대칭화) 반영.

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 3 | files: spec.md, research.md | reason: 기존 경로·심볼 Read 확인 + LibreSSL 3.3.6 실증 재현 + companion verify.go:39 in-band KeyID 조회 확인
- Q-CORR-04 | status: PASS | attempt: 3 | files: research.md | reason: existing과 [NEW] 분리, companionmanifest는 패턴-참조(의미론 다름)로 명시
- Q-COMP-04 | status: PASS | attempt: 3 | files: spec.md, plan.md, acceptance.md | reason: Outcome Lock 4요소를 REQ/plan/Must acceptance가 닫음
- Q-COMP-05 | status: PASS | attempt: 3 | files: research.md, acceptance.md | reason: INV-005를 multi-trial/만료 게이트로 재정의, S8 3서브케이스로 추적
- Q-COMP-06 | status: PASS | attempt: 3 | files: spec.md, research.md | reason: Traceability Matrix(REQ-008→T3,T4→S8) + Reviewer Brief로 범위 제한
- Q-COMP-07 | status: PASS | attempt: 3 | files: research.md | reason: REQ-008 SPEC 내 종결로 Completion Debt None 정당, Evolution Ideas ID 미부여
- Q-FEAS-02 | status: PASS | attempt: 3 | files: plan.md | reason: 모든 대상 경로가 autopus-adk 모듈 내부, generated/source 구분
- Q-FEAS-03 | status: PASS | attempt: 3 | files: acceptance.md | reason: openssl dgst 검증·date -u 만료 비교를 stock LibreSSL 3.3.6에서 실행 확인
- Q-SEC-01 | status: PASS | attempt: 3 | files: spec.md, research.md | reason: 신뢰 경계표 + 와이어 KeyID를 attacker-controlled로 명시(F-001)
- Q-SEC-02 | status: PASS | attempt: 3 | files: spec.md, plan.md | reason: private key umask 077+mktemp materialize→trap 삭제(F-002), placeholder만
- Q-SEC-03 | status: PASS | attempt: 3 | files: acceptance.md | reason: 별도 로그 artifact 미생성, 서명/키 값 로그 미노출
