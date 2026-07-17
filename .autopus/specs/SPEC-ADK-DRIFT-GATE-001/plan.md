# SPEC-ADK-DRIFT-GATE-001 구현 계획

## Tasks

- [x] T1: `[NEW] internal/cli/doctor_drift_content.go` — 구성 플랫폼별 결정성 게이트. 서로 다른 두 격리 temp root(A=빈 root, B=대표 pre-existing 사용자 상태 시드: `.claude/settings.json` statusLine 명령·user CLAUDE.md 본문)에 실제 로드된 cfg로 각각 `adapter.Generate`(부작용은 temp에만) 호출. A·B에서 바이트가 동일한 `OverwriteAlways` 파일만 결정적 부분집합으로 채택(root-path·기존 상태 의존 파일 자동 제외; 예: `statusline-user-command.txt`는 `InspectStatusLine(a.root)` 유래라 B에서 달라져 제외). 그 부분집합만 설치 `dir` 바이트를 읽어 `adapter.Checksum` 비교. 플랫폼별 count + 대표 3개 경로 반환. `LoadManifest(dir, platform)`가 nil이면 스킵(REQ-001·002·003·008).
- [x] T2: `[NEW] internal/cli/doctor_drift_orphan.go` — `os.ReadDir(filepath.Join(dir, ".autopus"))`로 `-manifest.json` 파일 수집, `strings.TrimSuffix(base, "-manifest.json")`로 platform token 추출, `cfg.Platforms` 집합에 없으면 고아. `isRootAutopusManifestPath` 형태 규칙 재사용. 알려진 legacy alias(gemini-cli↔antigravity-cli)는 여전히 고아로 보고하되 hint에 superseded 명시(REQ-004·008).
- [x] T3: `[NEW] internal/cli/doctor_drift_source.go` — `content/`+`templates/`+`cmd/generate-templates` 동시 존재 게이트. 통과 시 (a) `content.GenerateAllTemplates(contentDir, tempTemplateDir)` 후 `templates/` 커밋본과 파일별 비교 → 다른 템플릿 목록, (b) `version.Commit()`(7자 truncate)를 `git rev-parse HEAD`(전체 해시)의 접두사로 비교(`strings.HasPrefix(headFull, version.Commit())`; F-001 길이 불일치 오탐 방지). 커밋이 `none`이나 빈 값이면 (b) 스킵(`dev`는 `Version()` 폴백이라 가드 대상 아님; F-003). `hygieneGitLines` 패턴으로 read-only git 실행하고 git 미가용·비 git repo 에러는 경고 없이 graceful skip(F-005)(REQ-005·006·008).
- [x] T4: `[NEW] internal/cli/doctor_drift_output.go` — text `renderDriftText`(Hygiene 섹션 미러)와 JSON `collectDriftGateChecks`. `jsonCheck{ID,Severity,Status,Detail}` 계약 사용, `collectContextWeightChecks` advisory 패턴대로 `warn` check를 append하되 `r.status`는 건드리지 않음. 힌트(`auto update`/`rm`)는 플랫폼 중립(REQ-007·009).
- [x] T5: `internal/cli/doctor_json.go`의 `collectDoctorJSONReport`와 `doctor.go`의 `runDoctorText`에 드리프트 검사 배선(Hygiene 뒤). cfg 로드 실패 경로에서도 안전하게 스킵(REQ-007).
- [x] T6: `[NEW]` content 규칙 파일(예: `content/rules/autopus/doc-storage.md`)에 드리프트 게이트 존재를 1~2줄로 언급(`drift`/`드리프트`+`auto doctor` 포함)하고 `generate-templates`로 4플랫폼 템플릿 재생성(REQ-009·010).
- [x] T7: `[NEW] internal/cli/doctor_drift_content_test.go` — S1(변조 `route_team.workflow.js`→warn count 1), S2(무드리프트+사용자 statusline→pass count 0, 환경 의존 파일 제외), S3(marker/merge 제외→0) oracle 테스트.
- [x] T8: `[NEW] internal/cli/doctor_drift_orphan_test.go` — S4(gemini-cli-manifest 고아, count 1, 정확 경로) oracle 테스트.
- [x] T9: `[NEW] internal/cli/doctor_drift_source_test.go` — S5(템플릿 미재생성→warn, 비소스 repo→check 부재), S6(빌드 커밋 7자가 HEAD 전체 해시 접두사면 pass, 불일치면 warn, git 미가용면 check 부재) oracle 테스트.
- [x] T10: `[NEW] internal/cli/doctor_drift_gate_test.go` — S7(advisory: 두 warn 존재해도 `overall_ok==true`) + S8(규칙 파일 언급 라인) + JSON/text 미러 패리티 + `go test ./internal/cli/...` green + 실 워크스페이스 `auto doctor --json` 라이브 확인.

## Implementation Strategy

- **재사용 우선**: 새 의존성 0. `adapter.PlatformAdapter.Generate`, `adapter.Checksum`, `adapter.LoadManifest`, `adapter.OverwriteAlways`, `version.Commit`, `content.GenerateAllTemplates`, `pkg/config` platforms, `jsonCheck` 계약, context-weight advisory 패턴을 그대로 재사용한다.
- **부작용 격리**: `Generate`는 `os.WriteFile`+`manifest.Save`로 디스크에 쓴다. 따라서 반드시 `os.MkdirTemp` 격리 root에 대해 호출하고 `defer os.RemoveAll`로 정리한다. 설치 표면과 실 manifest는 절대 변경하지 않는다(읽기만).
- **결정성 게이트(F-002)**: `OverwriteAlways`라도 일부 파일은 pre-existing root 상태에 의존한다(`statusline-user-command.txt`←`InspectStatusLine(a.root)`, `settings.json`←`DetectPermissions`는 merge). 그래서 두 시드 temp root 차분으로 "template+cfg 순수 함수" 파일만 비교 집합으로 남긴다. 이것이 무드리프트 프로젝트에서 S2 count 0을 보장한다.
- **성능**: 플랫폼당 temp Generate 2회(차분용). 작은 파일 다수 write, 해시 비교 전 크기 선비교로 대형 파일 조기 단락. doctor 총 실행시간 영향 최소.
- **비차단**: 모든 드리프트 check는 advisory. `r.status`/`overall_ok`를 뒤집지 않아 pending update가 있는 프로젝트도 doctor가 실패로 보고되지 않는다.
- **TDD**: 각 검출기(T1~T3)는 대응 oracle 테스트(T7~T9)와 짝. 파일당 ≤300줄, 검출·렌더 분리로 라인 예산 준수.

## Visual Planning Brief (data-flow)

```mermaid
flowchart TD
  A[auto doctor / --json] --> B[loadHarnessConfigForDir → cfg.Platforms]
  B --> C1[content drift: per platform]
  C1 --> C2[Generate into temp root A empty + temp root B seeded]
  C2 --> C3[keep OverwriteAlways files byte-identical across A,B = deterministic set]
  C3 --> C4[read installed dir/TargetPath bytes]
  C4 --> C5{Checksum installed == FileMapping.Checksum?}
  C5 -- differ --> C6[drift: count + repr paths + auto update hint]
  C5 -- equal --> C7[pass: none observed]
  B --> D1[orphan: ReadDir .autopus/*-manifest.json]
  D1 --> D2[platform token - cfg.Platforms set-diff]
  D2 --> D3[orphan paths + count + rm hint]
  A --> E0{content/+templates/+cmd/generate-templates?}
  E0 -- yes --> E1[GenerateAllTemplates → temp vs committed templates/]
  E0 -- yes --> E2[version.Commit prefix-of git rev-parse HEAD full; graceful skip on git error]
  E0 -- no --> E3[silent skip]
  C6 --> F[collectDriftGateChecks: jsonCheck warn, advisory]
  C7 --> F
  D3 --> F
  E1 --> F
  E2 --> F
  F --> G[JSON checks[] + text Drift section; overall_ok unchanged]
```

## Feature Completion Scope

Primary SPEC이 Outcome Lock을 단독으로 닫는다. (a) 내용 드리프트=T1/T4/T5/T7, (b) 고아 manifest=T2/T4/T5/T8, (c) 소스 repo 템플릿·바이너리=T3/T4/T5/T9, advisory·패리티·문서=T4/T6/T10. 승인된 sibling 의존성 없음. Completion Debt 없음(모든 mandatory requirement가 태스크로 커버됨).
