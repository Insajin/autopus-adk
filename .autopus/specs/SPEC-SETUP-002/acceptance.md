# SPEC-SETUP-002 수락 기준

## 시나리오

### S1: 멀티레포 워크스페이스 감지
- Given: 루트 디렉토리에 `.git`이 없고, 서브디렉토리 6개 중 5개에 `.git`이 존재
- When: `Scan(rootDir)` 호출
- Then: `ProjectInfo.MultiRepo` != nil, `MultiRepo.IsMultiRepo` == true, `len(MultiRepo.Components)` == 5

### S2: 단일 레포 하위 호환성
- Given: 루트 디렉토리에 `.git`이 존재하는 일반 Go 프로젝트
- When: `Scan(rootDir)` 호출
- Then: `ProjectInfo.MultiRepo` == nil, 기존 동작과 동일한 결과

### S3: Go replace 의존성 매핑
- Given: autopus-bridge/go.mod에 `replace github.com/insajin/autopus-agent-protocol => ./third_party/autopus-agent-protocol`이 존재
- When: `MapCrossRepoDeps(components)` 호출
- Then: `RepoDependency{Source: "autopus-bridge", Target: "autopus-agent-protocol", Type: "replace"}` 포함

### S4: Architecture 문서에 Workspace 섹션 생성
- Given: 멀티레포 워크스페이스가 감지된 상태
- When: `Render(info, nil)` 호출
- Then: `DocSet.Architecture`에 "## Workspace" 섹션 포함, 각 레포의 이름/역할/의존 관계 기술

### S5: Architecture 문서에 Development Workflow 섹션 생성
- Given: 멀티레포 워크스페이스가 감지된 상태
- When: `Render(info, nil)` 호출
- Then: `DocSet.Architecture`에 "## Development Workflow" 섹션 포함, 크로스 레포 변경 조율 가이드 포함

### S6: Structure 문서에 레포 경계 표시
- Given: 멀티레포 워크스페이스가 감지된 상태
- When: `Render(info, nil)` 호출
- Then: `DocSet.Structure`에 각 서브디렉토리가 `[git repo]`로 표시됨, git remote URL 포함

### S7: 크로스 컴포넌트 시나리오 생성
- Given: autopus-bridge → autopus-agent-protocol 의존성이 감지된 상태
- When: `generateScenarios(projectDir, info)` 호출
- Then: scenarios.md에 두 레포를 아우르는 E2E 시나리오 포함

### S8: 빈 워크스페이스 (git 없는 디렉토리)
- Given: 루트 디렉토리에 `.git`이 없고, 서브디렉토리에도 `.git`이 없는 상태
- When: `Scan(rootDir)` 호출
- Then: `ProjectInfo.MultiRepo` == nil (멀티레포로 분류하지 않음)

### S9: 파일 크기 제한 준수
- Given: 모든 신규/수정 파일
- When: 구현 완료 후 라인 수 측정
- Then: 모든 소스 파일이 300줄 이하

## Sync Verification (2026-04-21)

- S1 PASS — `TestDetectMultiRepo_NestedReposWithoutRootRepo`, `TestScan_MultiRepoAggregatesComponentSignals`
- S2 PASS — `TestScan_SingleRepoDoesNotSetMultiRepo`
- S3 PASS — `TestMapCrossRepoDeps_GoAndNPMReferences`
- S4 PASS — `TestRender_MultiRepoSections`
- S5 PASS — `TestRender_MultiRepoSections`
- S6 PASS — `TestRender_MultiRepoSections`
- S7 PASS — `TestGenerateScenarios_AddsCrossRepoScenario`, `TestGenerateScenarios_CrossRepoScenarioNumbersFollowCobraScenarios`, `TestGenerateScenarios_UsesLanguageSpecificCommandsAndPaths`
- S8 PASS — `TestScan_EmptyWorkspaceDoesNotSetMultiRepo`
- S9 PASS — `wc -l` 확인 결과 신규/수정 소스 파일 최대 길이는 `pkg/setup/multirepo.go` 225줄

## Command Verification

- `go test ./pkg/setup/...`
- `go test -cover ./pkg/setup/...` → 86.9%
- `go vet ./pkg/setup/...`
