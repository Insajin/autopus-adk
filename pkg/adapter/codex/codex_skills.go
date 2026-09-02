package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
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

// agentsMDTemplate is the AGENTS.md AUTOPUS section template used when codex
// owns the root document, i.e. when opencode is not installed (see
// codexOwnsRootDoc). The platform-independent policy comes from the shared
// fragments so the codex and opencode markers cannot drift apart; only the
// installed path list, execution model, and native routing are codex-specific.
var agentsMDTemplate = `# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: {{.ProjectName}}
- **모드**: {{.Mode}}
- **플랫폼**: {{join ", " .Platforms}}

` + templates.RootDocInstalledComponents() + `

` + templates.RootDocPolicy() + `

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

` + templates.RootDocGuidelines() + `
{{if contains (join ", " .Platforms) "codex"}}
### Native Skill Routing

Use ` + "`@auto`" + ` or ` + "`$codex-auto`" + ` for routing. Load detailed ` + "`$codex-auto-<route>`" + ` and ` + "`$codex-<skill>`" + ` contracts before execution. Agent definitions live in ` + "`.codex/agents/`" + `.
{{end}}{{if contains (join ", " .Platforms) "opencode"}}
See .opencode/rules/autopus/ for OpenCode rule definitions.
{{end}}
`
