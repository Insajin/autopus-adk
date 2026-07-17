# SPEC-ADK-DRIFT-GATE-001: Doctor Installed-Surface Drift Observability

**Status**: completed
**Created**: 2026-07-17
**Domain**: DRIFT-GATE
**Module**: autopus-adk

## 목적

`auto doctor`의 Hygiene 검사는 git 추적 위생만 관측한다(`internal/cli/status_hygiene.go`의 `collectStatusHygiene`은 `git status`/`git ls-files`만 읽는다). 그래서 세 가지 맹점이 "drift: none observed"로 조용히 통과한다.

1. **내용 스테일**: 설치된 generated 표면 내용이 현재 바이너리가 embedded 템플릿으로 생성할 내용과 다른데도(예: 구 바이너리로 재생성된 `route_team.workflow.js`에 구 모델 id가 잔존) 관측되지 않는다.
2. **고아 manifest**: `autopus.yaml` platforms에 없는 플랫폼의 `.autopus/<platform>-manifest.json`이 잔존해도 보이지 않는다.
3. **소스 repo 스테일**: ADK 소스 repo에서 `content/**` 변경이 `templates/**`에 미재생성이거나, 실행 중 바이너리 빌드 커밋이 repo HEAD보다 오래돼도 관측되지 않는다.

이 SPEC은 세 맹점을 `auto doctor`의 **비차단 advisory** 검사로 관측 가능하게 만든다. 자동 수리는 하지 않고 `auto update`/`generate-templates`/rebuild 힌트만 제시한다.

## Outcome Boundary

- **User-visible outcome**: `auto doctor`(text + `--json`)가 (a) 설치 표면 내용 드리프트를 플랫폼별 count와 대표 경로로, (b) 고아 manifest를 제거 힌트와 함께, (c) 소스 repo에서 템플릿 미재생성과 바이너리 스테일함을 보고한다. 전부 비차단이며 `overall_ok`를 뒤집지 않는다.
- **Mandatory requirements**: REQ-001~REQ-010.
- **Explicit non-goals**: 자동 수리(auto update 자동 실행), 서명/무결성 검증(SPEC-ADK-RELEASE-SIGNING-001 소관), 루트 워크스페이스 특화 로직, manifest 스키마 변경.
- **Completion evidence**: 변조 설치 파일·고아 manifest·미재생성 템플릿·스테일 커밋 fixture별 WARN oracle 테스트 + 무드리프트 fixture 무경고 + 실 워크스페이스 라이브 확인.

## Requirements

### REQ-001: 내용 드리프트 검사
WHEN `auto doctor`가 설치된 하네스 프로젝트에서 실행될 때, THE SYSTEM SHALL 각 구성 플랫폼의 결정적 생성 파일(embedded 템플릿과 cfg만의 순수 함수로 절대 root 경로·기존 사용자 상태에 독립인 `OverwriteAlways` 표면)의 내용을 현재 바이너리 생성 내용과 해시 비교하여 플랫폼별 드리프트 count와 대표 경로를 비차단 경고로 보고해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: `doctor.drift.content.<platform>` check(JSON) + Drift 섹션(text)

### REQ-002: 업데이트 힌트
WHEN 내용 드리프트가 감지될 때, THE SYSTEM SHALL 자동 수리를 실행하지 않고 `auto update` 재생성 힌트만 표시해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: check detail 문자열의 `auto update`

### REQ-003: 결정성 기반 제외
WHEN 설치 내용을 비교할 때, THE SYSTEM SHALL 결정적 표면만 비교하고 marker(`CLAUDE.md`)·merge(`.mcp.json`, `settings.json`)·기존 root 상태 의존(`InspectStatusLine(a.root)` 유래 `statusline-user-command.txt` 등) 파일은 제외해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 드리프트 경로 목록에서 `CLAUDE.md`·`.mcp.json`·`statusline-user-command.txt` 부재

### REQ-004: 고아 manifest 검사
WHEN `.autopus/<platform>-manifest.json`이 `autopus.yaml` platforms에 없는 플랫폼을 가리킬 때, THE SYSTEM SHALL 이를 고아 manifest로 제거 힌트와 함께 보고해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: `doctor.drift.orphan_manifest` check의 paths + count

### REQ-005: 소스 repo 템플릿 재생성 드리프트
WHERE 작업 디렉토리가 ADK 소스 repo(`content/`, `templates/`, `cmd/generate-templates` 존재)일 때, THE SYSTEM SHALL content 소스가 다르게 재생성할 템플릿을 감지하고 `generate-templates` 힌트와 함께 보고해야 한다.
- EARS type: State
- Priority: Should
- 관측 지점: `doctor.drift.template_regen` check

### REQ-006: 바이너리 스테일함 검사
WHERE 작업 디렉토리가 ADK 소스 repo이고 빌드 커밋이 `none`이나 빈 값이 아닐 때, THE SYSTEM SHALL 7자로 truncate된 `version.Commit()`을 repo HEAD 전체 해시의 접두사로 read-only git 비교하고 git 미가용·비 git repo면 경고 없이 스킵하며 접두사 불일치 시에만 rebuild 힌트를 보고해야 한다.
- EARS type: State
- Priority: Should
- 관측 지점: `doctor.drift.binary_stale` check의 빌드 커밋 + HEAD 접두사

### REQ-007: JSON 미러 + advisory
WHEN `auto doctor --json`이 실행될 때, THE SYSTEM SHALL 모든 드리프트 check를 doctor check 계약으로 JSON checks 배열에 미러하고, 드리프트가 advisory이므로 `overall_ok`를 true로 유지해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: JSON `checks[]` + `data.overall_ok`

### REQ-008: 조용한 스킵
WHEN 플랫폼 manifest·생성 표면이 없거나 디렉토리가 소스 repo가 아닐 때, THE SYSTEM SHALL 해당 검사를 경고 없이 조용히 스킵해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 해당 check의 부재

### REQ-009: 플랫폼 패리티 안내
THE SYSTEM SHALL 드리프트 안내를 claude-code·codex·antigravity-cli·opencode 사용자 모두에게 유효한 플랫폼 중립 힌트로 제시해야 한다.
- EARS type: Ubiquitous
- Priority: Must
- 관측 지점: check detail의 플랫폼 중립 문구(`auto update`/`rm`)

### REQ-010: 규칙 문서 언급
THE SYSTEM SHALL 드리프트 게이트의 존재를 content 규칙 참조 한 곳에 1~2줄로 문서화해야 한다.
- EARS type: Ubiquitous
- Priority: Should
- 관측 지점: `[NEW]` content 규칙 파일의 드리프트 게이트 언급 라인

## 생성 파일 상세

- `[NEW] internal/cli/doctor_drift_content.go` — 플랫폼별 결정성 게이트(두 시드 temp root 차분) 후 결정적 `OverwriteAlways` 설치본 vs 생성본 해시 비교(REQ-001~003).
- `[NEW] internal/cli/doctor_drift_orphan.go` — `.autopus/*-manifest.json` 집합 − 구성 platforms 집합 = 고아(REQ-004).
- `[NEW] internal/cli/doctor_drift_source.go` — 소스 repo 게이트 + 템플릿 재생성 비교 + 빌드 커밋 접두사 vs HEAD 전체 해시(REQ-005·006).
- `[NEW] internal/cli/doctor_drift_output.go` — text 렌더 + JSON 미러(advisory, envelope 미변경)(REQ-007~009).
- `[NEW]` 대응 `_test.go` 4종 + `internal/cli/doctor_json.go`·`doctor.go` 배선.
- `[NEW]` content 규칙 파일 1~2줄 언급(REQ-010).

## Related SPECs

None (Primary SPEC이 Outcome Lock을 단독으로 닫는다). 서명/무결성은 SPEC-ADK-RELEASE-SIGNING-001의 별도 소관이며 의존 관계가 아니다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 (Must) | T1, T7 | S1, S2 | INV-001 |
| REQ-002 (Must) | T1, T4, T7 | S1 | INV-001 |
| REQ-003 (Must) | T1, T7 | S2, S3 | INV-002 |
| REQ-004 (Must) | T2, T8 | S4 | INV-003 |
| REQ-005 (Should) | T3, T9 | S5 | INV-004 |
| REQ-006 (Should) | T3, T9 | S6 | INV-005 |
| REQ-007 (Must) | T4, T5, T10 | S7 | INV-006 |
| REQ-008 (Must) | T1, T2, T3 | S5 | INV-002, INV-003 |
| REQ-009 (Must) | T4, T6 | S1, S4 | INV-001, INV-003 |
| REQ-010 (Should) | T6 | S8 | INV-006 |
