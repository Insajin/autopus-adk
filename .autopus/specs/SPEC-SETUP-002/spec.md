# SPEC-SETUP-002: Multi-Repo Workspace Detection and Cross-Repo Dependency Mapping

**Status**: completed (2026-04-21)
**Created**: 2026-03-25
**Domain**: SETUP
**Scope**: Module (autopus-adk)

**Sync Note (2026-04-21)**:
- 현재 `pkg/setup/workspace.go` 는 `go.work`, npm/pnpm/yarn, Cargo 기반 **monorepo workspace** 감지는 이미 지원한다.
- 이 SPEC은 그 기능을 대체하지 않는다. 대신 nested git repo를 1급 모델로 추가해 **multi-repo workspace** 를 인식하도록 확장한다.
- 따라서 구현 시 `ProjectInfo.Workspaces` 는 유지하고, 별도 `MultiRepoInfo` 계층을 추가해야 한다.

**Sync Summary (2026-04-21)**:
- `ProjectInfo.MultiRepo` / `MultiRepoInfo` / `RepoComponent` / `RepoDependency` 가 추가되어 single-repo, monorepo, multi-repo를 병행 표현할 수 있게 됐다.
- `DetectMultiRepo()` 는 루트 repo 유무와 무관하게 immediate child repo topology를 감지하고, 기존 `DetectWorkspaces()` 경로는 그대로 유지한다.
- Go `require`/`replace` 와 npm `package`/`file:` sibling reference를 `multirepo_deps.go` 에서 크로스 레포 edge 로 매핑한다.
- setup renderer/scenario 경로는 repository list, development workflow, repo boundary, language-specific cross-repo command 생성까지 포함하도록 확장됐다.

## 목적

현재 `auto setup generate` 및 `auto arch generate` 명령은 단일 git 레포 또는 go.work/npm workspace 기반 모노레포만 인식한다. 그러나 `autopus-co`처럼 루트 repo와 nested repo가 공존하거나, 루트가 메타 workspace 역할을 하고 각 서브디렉토리가 독립 git repo인 "멀티레포 워크스페이스" 구조는 아직 1급 모델로 감지하지 못한다.

이 SPEC은 setup 바이너리에 멀티레포 워크스페이스 감지, 크로스 레포 의존성 매핑, 워크스페이스 수준 문서 생성 기능을 추가하여 AI 에이전트가 레포 경계와 컴포넌트 관계를 정확히 이해할 수 있게 한다.

## 현재 구현과의 관계

- 이미 있는 것:
  - `DetectWorkspaces()` 기반 monorepo workspace 감지
  - generated docs의 stale/fresh 검증과 drift score 계산
- 아직 없는 것:
  - nested git repo 감지
  - repo 간 의존성 그래프
  - root meta repo + nested repo 공존 모델
  - repo boundary / remote / ownership 기반 문서 렌더링

## 요구사항

### R1: Multi-Repo Workspace Detection (P0)
WHEN setup scans a project root, THE SYSTEM SHALL inspect immediate subdirectories for nested `.git` directories regardless of whether the root directory itself is also a git repository. Each detected repository SHALL be recorded as a `RepoComponent` with name, path, git remote URL, primary language, and module path.

### R2: Backward Compatibility (P0)
WHEN the project root contains a `.git` directory and no nested git repositories are detected, THE SYSTEM SHALL behave exactly as before for single-repo scanning. WHEN the root contains both its own `.git` directory and nested git repositories, THE SYSTEM SHALL treat the project as a multi-repo workspace instead of skipping nested-repo detection.

### R3: Cross-Repo Dependency Mapping (P0)
WHEN a multi-repo workspace is detected, THE SYSTEM SHALL parse each repository's `go.mod` for `replace` directives and `require` statements that reference sibling repositories. The result SHALL be a directed dependency graph stored as `[]RepoDependency` with source repo, target repo, dependency type (replace/require), and module version.

### R4: NPM/Package Cross-Reference Detection (P1)
WHEN a multi-repo workspace is detected, THE SYSTEM SHALL also parse `package.json` files for cross-references between sibling repositories using package names or file: protocol links.

### R5: Workspace Section in Architecture Document (P0)
WHEN a multi-repo workspace is detected, THE SYSTEM SHALL generate a "Workspace" section in `architecture.md` containing: workspace type (multi-repo), repository list with roles, dependency graph in text format, and deploy target mapping.

### R6: Development Workflow Section (P0)
WHEN a multi-repo workspace is detected, THE SYSTEM SHALL generate a "Development Workflow" section in `architecture.md` documenting: which repository handles which concern, cross-repo change coordination strategy, and local development setup using replace directives.

### R7: Repository Boundaries in Structure Document (P0)
WHEN a multi-repo workspace is detected, THE SYSTEM SHALL add repository boundary indicators and git remote information to `structure.md`, marking each top-level directory as `[git repo]` with its remote URL.

### R8: Cross-Component Scenarios (P1)
WHEN a multi-repo workspace is detected, THE SYSTEM SHALL generate cross-component end-to-end scenarios in `scenarios.md` that span multiple repositories, based on the detected dependency graph.

### R9: ProjectInfo Extension (P0)
THE SYSTEM SHALL extend the `ProjectInfo` struct with a `MultiRepo *MultiRepoInfo` field. `MultiRepoInfo` SHALL contain: `IsMultiRepo bool`, `Components []RepoComponent`, `Dependencies []RepoDependency`, and `WorkspaceRoot string`. This field SHALL coexist with the existing `Workspaces []Workspace` field rather than replacing it.

### R10: Scan Aggregation (P0)
WHEN a multi-repo workspace is detected, THE SYSTEM SHALL scan each component repository independently and aggregate results. The aggregated `ProjectInfo` SHALL include languages, frameworks, build files, and entry points from all component repositories while preserving existing monorepo-workspace detection for individual repos where applicable.

## 생성 파일 상세

### `pkg/setup/multirepo.go` (신규)
멀티레포 감지 핵심 로직: `DetectMultiRepo(dir string) *MultiRepoInfo`, `ScanRepoComponent(dir string) (*RepoComponent, error)`, root + immediate child repository scan.

### `pkg/setup/multirepo_deps.go` (신규)
크로스 레포 의존성 매핑: `MapCrossRepoDeps(components []RepoComponent) []RepoDependency`, Go `require`/`replace`, npm `package`/`file:` sibling reference 파싱.

### `pkg/setup/multirepo_types.go` (신규)
멀티레포 관련 타입 정의: `MultiRepoInfo`, `RepoComponent`, `RepoDependency`.

### `pkg/setup/multirepo_render.go` (신규)
문서 렌더링 헬퍼: `renderWorkspaceSection(info *MultiRepoInfo) string`, `renderDevWorkflow(info *MultiRepoInfo) string`, `renderRepoBoundaries(info *MultiRepoInfo) string`.

### 기존 파일 수정
- `pkg/setup/types.go` — `ProjectInfo`에 `MultiRepo *MultiRepoInfo` 필드 추가
- `pkg/setup/workspace.go` — 기존 monorepo workspace 감지는 유지하고, multi-repo detection과 의미가 섞이지 않도록 역할을 분리
- `pkg/setup/scanner.go` — `Scan()`에서 멀티레포 감지 호출, 컴포넌트별 스캔 집계
- `pkg/setup/renderer_arch.go` — `renderArchitecture()`에 Workspace / Development Workflow 섹션 삽입
- `pkg/setup/renderer_docs.go` — `Workspaces`(monorepo) 와 `Repositories`(multi-repo) 섹션을 구분 렌더링
- `pkg/setup/scenarios.go` — 번호 offset 유지, repo path 기반 subshell, language-specific cross-component 시나리오 생성 로직 추가
