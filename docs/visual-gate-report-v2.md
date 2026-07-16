# Visual gate report v2 migration

`auto verify`의 상세 Playwright snapshot 증거는 기존 보고서를 덮어쓰지 않고 별도 v2 파일로 추가됩니다.

## 호환 계약

- `.autopus/design/verify/latest.json`은 기존 `version: 1` 필드와 artifact shape를 유지합니다.
- `.autopus/design/verify/latest.v2.json`은 assertion, required/executed project, Playwright version, snapshot update mode와 `latest.json`의 `legacy_sha256`을 포함합니다.
- 기존 Go API인 `VisualGateInput`, `VisualGateReport`, `VisualArtifact`는 v1 shape를 유지합니다. 새 소비자는 `*V2` 타입을 명시적으로 사용합니다.
- 두 보고서는 임시 파일에 완성된 뒤 v1, v2 순서로 교체합니다. v2가 마지막 commit marker 역할을 하며, symlink가 섞인 출력 경로에는 쓰지 않습니다.
- v2 소비자는 파일을 직접 따로 읽지 않고 `ReadVisualGateReportBundle`을 사용해야 합니다. 이 함수는 v1을 앞뒤로 읽어 도중 변경이 없었는지 확인하고, v2의 `legacy_sha256`이 v1의 정확한 바이트와 일치할 때만 v2를 반환합니다.

## 실행 정책

- 기본 실행은 snapshot proof가 없거나 불충분해도 보고서에 finding을 남기고 종료 코드를 바꾸지 않습니다.
- `--strict-visual-gate`를 명시하면 proof 누락, 비활성 comparison, unsafe snapshot update mode, required project의 증거 누락을 차단합니다.
- 실제 Playwright 프로세스 실패, 임시 파일 처리 실패, 출력 한도 초과는 strict 여부와 관계없이 실패합니다.
- `auto verify`는 승인 baseline을 새로 만들거나 갱신하지 않도록 `--update-snapshots=none`을 강제합니다.

## Playwright와 프로젝트 호환성

- Playwright 1.59 이상은 reporter에 공개된 `FullProject.ignoreSnapshots`를 사용합니다.
- Playwright 1.58 계열에는 같은 공개 필드가 없으므로 기본 모드에서는 `unproven`, strict 모드에서는 실패로 처리합니다. private `_fullProject`에는 의존하지 않습니다.
- 기존 기본값인 `--viewport desktop`은 이전 버전과 마찬가지로 project filter 없이 설정된 프로젝트를 실행합니다. 그 밖의 custom project 이름은 예약된 viewport 열거형으로 변환하지 않습니다. 명시한 required project마다 최종 retry의 screenshot assertion PASS가 필요합니다.

## 채택과 롤백

- 기존 프로젝트는 변경 없이 v1 보고서와 비차단 기본 동작을 계속 사용합니다.
- Desktop처럼 엄격한 시각 증명이 필요한 프로젝트만 검증 명령에 `--strict-visual-gate`와 실제 Playwright project selector를 추가합니다.
- 문제가 있으면 strict 플래그를 제거하고 v1 `latest.json`만 소비하면 됩니다. 별도 설정 마이그레이션이나 하네스 재생성은 필요하지 않습니다.

## QAMESH 경계

보고서는 QAMESH handoff 후보 metadata일 뿐입니다. v1의 `qamesh_handoff` check ID·상태·순서는 기존 소비자 호환성을 위해 그대로 유지하지만, ingestion 성공의 근거로 해석해서는 안 됩니다. v2는 이 경계를 `WARN`으로 명시합니다. 현재 ADK에는 visual report를 실제로 수집하는 QAMESH decoder가 없습니다.
