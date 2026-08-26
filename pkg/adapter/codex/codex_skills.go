package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// renderSkillTemplates reads Codex skill templates from embedded FS,
// renders them, and writes to .codex/skills/.
func (a *Adapter) renderSkillTemplates(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files, err := a.prepareSkillTemplateMappings(cfg)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		targetPath := filepath.Join(a.root, file.TargetPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf("코덱스 스킬 디렉터리 생성 실패 %s: %w", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, file.Content, 0644); err != nil {
			return nil, fmt.Errorf("코덱스 스킬 파일 쓰기 실패 %s: %w", targetPath, err)
		}
	}

	// Extended skills from content/skills/ via transformer
	extFiles, err := a.renderExtendedSkills(cfg)
	if err != nil {
		return nil, fmt.Errorf("extended skill rendering failed: %w", err)
	}
	for _, ef := range extFiles {
		targetPath := filepath.Join(a.root, ef.TargetPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf("extended skill dir 생성 실패 %s: %w", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, ef.Content, 0644); err != nil {
			return nil, fmt.Errorf("extended skill write failed %s: %w", targetPath, err)
		}
	}
	files = append(files, extFiles...)

	return files, nil
}

// agentsMDTemplate is the AGENTS.md AUTOPUS section template.
const agentsMDTemplate = `# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: {{.ProjectName}}
- **모드**: {{.Mode}}
- **플랫폼**: {{join ", " .Platforms}}

## Installed Components

{{if contains (join ", " .Platforms) "codex"}}- Codex Skills: .codex/skills/codex-*/SKILL.md
- Codex Agents: .codex/agents/*.toml
- Codex Hooks: .codex/hooks.json
- Codex Config: .codex/config.toml
- Plugin Marketplace: .agents/plugins/marketplace.json
{{end}}{{if contains (join ", " .Platforms) "opencode"}}- OpenCode Rules: .opencode/rules/autopus/
- OpenCode Commands: .opencode/commands/
- OpenCode Agents: .opencode/agents/
- OpenCode Plugins: .opencode/plugins/
- OpenCode Skills: .agents/skills/
{{end}}

## Language Policy

IMPORTANT: Follow these language settings strictly for all work in this project.

- **Code comments**: {{.Language.Comments}}
- **Commit messages**: {{.Language.Commits}}
- **AI responses**: {{.Language.AIResponses}}

## Execution Model

{{if contains (join ", " .Platforms) "codex"}}- **Codex Invocation**: use ` + "`@auto <route> ...`" + ` or ` + "`$codex-auto <route> ...`" + `; load detailed ` + "`$codex-auto-<route>`" + ` and ` + "`$codex-<skill>`" + ` skills.
- **Codex V2**: use only spawn_agent, send_message, followup_task, target-less wait_agent, interrupt_agent, and list_agents.
- **Codex Shared Workspace**: every worker uses the same cwd/filesystem. Parallel writers require disjoint write ownership; overlapping writers run sequentially.
- **Codex --auto**: ` + "`@auto ... --auto`" + ` explicitly approves the default subagent pipeline.
- **Codex /goal**: use the native Codex goals feature; ` + "`@auto goal`" + ` is only a thin wrapper.
- **Codex --team**: use the Multi-Agent V2 Lead/Builder/Guardian profile.
{{end}}{{if contains (join ", " .Platforms) "opencode"}}- **OpenCode**: 기본 실행 모델은 task(...) 기반 subagent-first 입니다.
- **OpenCode Invocation**: /auto <subcommand> ... 또는 /auto-<subcommand> ... alias를 사용합니다.
{{end}}

## Core Guidelines

{{if contains (join ", " .Platforms) "codex"}}### Supervisor Contract

IMPORTANT: 메인 세션은 얇은 라우터가 아니라 phase/gate를 관리하는 supervisor입니다. 각 단계마다 필수 단계, skip 조건, retry 한도, 다음 필수 단계를 명확히 유지하세요.

### Subagent Delegation

IMPORTANT: 3개 이상 파일 수정, 다중 도메인 변경, 또는 신규 코드 200줄 초과가 예상되면 기본적으로 서브에이전트를 사용하세요. 단, 읽기 위주 탐색/리서치/테스트 분석은 병렬 fan-out을 우선하고, 쓰기 위주 구현은 파일 소유권이 겹치면 순차 실행으로 전환하세요.

### Worker Contracts

IMPORTANT: 각 worker 프롬프트에는 반드시 소유 파일/모듈, 수정 금지 범위, 완료 기준, 반환 형식을 포함하세요. 최소 반환 필드는 ` + "`owned_paths`, `changed_files`, `verification`, `blockers`, `next_required_step`" + ` 입니다.

### Review Convergence

IMPORTANT: 리뷰는 discovery와 verification을 분리하세요. 첫 리뷰는 finding discovery에 집중하고, 재시도는 열린 finding 해결 여부만 diff 기준으로 확인하세요. 같은 범위를 무한 재탐색하지 마세요.

### File Size Limit

IMPORTANT: 300줄 제한은 소스 코드 파일에만 적용합니다. SPEC Markdown files under .autopus/specs/** are documentation and exempt from the 300-line source code limit. prd.md, spec.md, plan.md, acceptance.md, research.md, review.md는 300줄 초과만으로 분할하거나 거절하지 마세요.

### Mandatory Compact Policy

IMPORTANT: Write tests before implementation when behavior changes. New code comments are English. Source and test files MUST stay at or below 300 lines. Do not edit outside assigned ownership. Every spawned worker returns exactly ` + "`owned_paths`, `changed_files`, `verification`, `blockers`, `next_required_step`" + `.

### Native Skill Routing

Use ` + "`@auto`" + ` or ` + "`$codex-auto`" + ` for routing. Load detailed ` + "`$codex-auto-<route>`" + ` and ` + "`$codex-<skill>`" + ` contracts before execution. Agent definitions live in ` + "`.codex/agents/`" + `.
{{end}}{{if contains (join ", " .Platforms) "opencode"}}See .opencode/rules/autopus/ for OpenCode rule definitions.
{{end}}
`
