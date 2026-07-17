# SPEC-ADK-RELEASE-SIGNING-001 구현 계획

## Tasks

- [x] **T1 — producer 서명 산출 + secret 위생** (REQ-001): `checksums.txt`의 SHA-256에 publisher ECDSA P-256 서명을 만들어 `checksums.txt.sig` release asset을 게시.
  - CI 서명 스텝(F-002): `umask 077` → `keyfile="$(mktemp)"` → `printf '%s' "$ADK_RELEASE_ECDSA_PRIVATE_KEY" > "$keyfile"` → `trap 'rm -f "$keyfile"' EXIT` → `openssl dgst -sha256 -sign "$keyfile" -out dist/checksums.txt.sig dist/checksums.txt`. env 이름(`ADK_RELEASE_ECDSA_PRIVATE_KEY`)과 파일 경로 변수(`keyfile`) 표기 일치. 대안: `[NEW] auto release sign-checksums`(Go `ecdsa.SignASN1`) — openssl 경로 선호.
  - `.goreleaser.yaml` `signs:`에 두 번째 엔트리 추가(`artifacts: checksum`, `signature: ${artifact}.sig`, `cmd: openssl`) 또는 release.yaml post 스텝. 기존 cosign 엔트리 존치.
  - **전용 release-signing ECDSA 키** 신규 프로비저닝: CI secret placeholder `ADK_RELEASE_ECDSA_PRIVATE_KEY`(companion ed25519 키와 분리). 소스 ≤300줄.

- [x] **T2 — selfupdate 검증 배선** (REQ-002, 003, 006, 009): `[NEW] pkg/selfupdate/signature.go`에 `VerifyReleaseSignature(checksums, sig []byte) error` — `sha256.Sum256(checksums)` 후 비만료 임베드 키 각각으로 `ecdsa.VerifyASN1(pub_i, hash, sig)` multi-trial, 하나라도 true면 nil, 전부 false면 `no trusted release signing key verified`, 시도 집합 공집합이면 `all embedded keys expired`. `downloader.go::DownloadAndVerify`에 signature URL 파라미터 추가, **checksum 비교(line 58) 이전** 서명 검증 선행(폴백 없음). `types.go::ReleaseInfo`에 `SignatureURL`, `checker.go::FetchLatest` switch에 `case "checksums.txt.sig"`, `internal/cli/update_self.go:81` 호출부에 `info.SignatureURL` 전달. 정상 경로 출력 유지.

- [x] **T3 — 임베드 pinned key 집합 + 만료 게이트 + 회전** (REQ-002, 007, 008): `[NEW] pkg/selfupdate/pinnedkey.go`에 pinned key **집합**(각 항목: PEM SPKI pubkey·KeyID·ExpiresAt) 상수 + `activeKeys(now)` 조회(ExpiresAt 지난 키 제외). 로드는 `x509.ParsePKIXPublicKey`→`*ecdsa.PublicKey` 단언(P-256 곡선 확인). 와이어 KeyID 미사용; KeyID는 감사·회전 북키핑. 회전 = 후속 릴리스에서 과도기 2키 병행 임베드(시도 집합 2개) 후 구키 제거(`PublicKeyReceipt` 패턴 참조 주석). 실제 키는 회전 시 생성, 문서엔 placeholder.

- [x] **T4 — install.sh 검증 + 만료 대칭 + fail-open 제거** (REQ-004, 005, 006, 007, 008, 009): `install.sh`에 pinned key 집합 임베드(키별 PEM 상수 + `EXPIRES_1="YYYY-MM-DD"` …). `verify_signature()` — `checksums.txt.sig` 다운로드, 만료 게이트(F-003): `now="$(date -u +%Y-%m-%d)"` 후 각 `EXPIRES_n`과 **사전식 문자열 비교**(`[ "$now" \> "$EXPIRES_n" ]`; ISO 날짜는 사전식=시간식, crypto 불필요·POSIX 포터블)로 만료 키 제외, 남은 키마다 임시 PEM 기록 후 `openssl dgst -sha256 -verify pub.pem -signature checksums.txt.sig checksums.txt`(유효 `Verified OK` exit 0, 변조 `Error Verifying Data` exit 1) 시도. 하나라도 통과 없으면 `no trusted release signing key verified`, 활성 키 0이면 `all embedded keys expired`, openssl 미탐지면 안내 후 각각 `err`(exit 1). 활성 1키면 루프가 단일 `dgst -verify` 호출과 동일. main(line 165-174)에서 checksum grep **이전** 호출. 기존 `verify_checksum` line 131-135 fail-open(`return 0`)을 fail-closed `err`로 교체.

- [x] **T5 — 롤아웃 순서·버전 경계** (REQ-009): 첫 서명 릴리스 tag를 signing floor로 기록(현행 `v0.50.71`, `pkg/version` 흐름). install.sh 배포는 첫 서명 릴리스 이후로 순서 고정(fail-closed가 미서명 최신 릴리스 차단). 구버전 소급 불가를 CHANGELOG/README에 한계로 명시.

- [x] **T6 — oracle 테스트** (전체 REQ): `[NEW] pkg/selfupdate/signature_test.go` — 합성 `ecdsa.GenerateKey(elliptic.P256(), rand.Reader)`로 S1~S9 각 concrete 기대. S8은 3서브케이스: (a) 임베드 밖 공격자 키 서명→`no trusted…`, (b) 유일 임베드 키 ExpiresAt 과거→`all embedded keys expired`(유효 서명이라도), (c) 2키 임베드·2번째 키 서명→통과(회전창). Go 서명↔openssl 검증 양방향(R3). `[NEW] install.sh` 검증 함수용 shell 테스트(stock LibreSSL 3.3.6): 정상/변조/부재/openssl부재/만료 exit·메시지. acceptance S1~S9와 1:1.

## Implementation Strategy

- **재사용 우선**: 신규 Go 의존성 0. 서명/검증 stdlib `crypto/ecdsa`·`x509`·`sha256`; 회전·핀은 `companionmanifest` `PublicKeyReceipt`/`PinnedKey` **패턴** 참조(in-band KeyID 조회는 bare 파일에 부적합→multi-trial로 대체).
- **변경 범위**: selfupdate 4개 기존 파일 소폭 수정 + 2개 신규, install.sh 1개, .goreleaser.yaml 1개 + 신규 서명 스텝. 신규 소스 ≤300줄(목표 ≤200), signature/pinnedkey 파일 분리.
- **fail-closed·대칭 원칙**: 서명 부재·불일치·공격자키·전만료·openssl 부재 전부 중단. 만료 게이트와 multi-trial을 두 소비자에 대칭 구현. checksum-only 폴백·skip 경로 신설 금지, 기존 install.sh skip 제거.
- **키 용도·와이어 분리**: release 서명은 companion ed25519와 별개 전용 ECDSA 키. KeyID는 와이어에 싣지 않고 임베드 북키핑용.
- **producer/consumer 순서**: T1(서명) → 첫 서명 릴리스 → T4/install.sh 배포. 검증기는 서명 릴리스만 대상.

## Visual Planning Brief

검증 data-flow(multi-trial + 만료 게이트)와 동일-origin 교체 방어 다이어그램은 `research.md`의 `## Visual Planning Brief` 참조. 핵심: 두 소비자 모두 비만료 임베드 키 집합으로 ECDSA 서명 multi-trial(Go `ecdsa.VerifyASN1` / sh `openssl dgst -sha256 -verify`) → pass → sha256(archive) 비교 → install/replace. 어느 실패도 fail-closed.

## Feature Completion Scope

- Primary SPEC이 Outcome Lock 4요소(서명 자산·selfupdate fail-closed·install.sh fail-closed·정상 UX 불변)를 T1~T6로 모두 닫는다. REQ-008(만료·multi-trial·회전)은 두 소비자 대칭으로 SPEC 내 종결.
- 승인된 sibling 의존성: 없음.
- 남은 Completion Debt: 없음.
- Evolution Ideas(homebrew 체인, Rekor 소비, SLSA)는 optional이며 완료를 막지 않는다.
