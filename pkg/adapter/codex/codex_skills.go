package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

// renderSkillTemplates reads Codex skill templates from embedded FS,
// renders them, and writes to .codex/skills/.
func (a *Adapter) renderSkillTemplates(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir("codex/skills")
	if err != nil {
		return nil, fmt.Errorf("코덱스 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		name := entry.Name()
		skillFile := strings.TrimSuffix(name, ".tmpl")

		tmplContent, err := templates.FS.ReadFile("codex/skills/" + name)
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 읽기 실패 %s: %w", name, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 렌더링 실패 %s: %w", name, err)
		}

		targetPath := filepath.Join(a.root, ".codex", "skills", skillFile)
		if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
			return nil, fmt.Errorf("코덱스 스킬 파일 쓰기 실패 %s: %w", targetPath, err)
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "skills", skillFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	// Extended skills from content/skills/ via transformer
	extFiles, err := a.renderExtendedSkills()
	if err != nil {
		return nil, fmt.Errorf("extended skill rendering failed: %w", err)
	}
	for _, ef := range extFiles {
		targetPath := filepath.Join(a.root, ef.TargetPath)
		if err := os.WriteFile(targetPath, ef.Content, 0644); err != nil {
			return nil, fmt.Errorf("extended skill write failed %s: %w", targetPath, err)
		}
	}
	files = append(files, extFiles...)

	return files, nil
}

// agentsMDTemplate is the AGENTS.md AUTOPUS section template.
// Kept slim — detailed rules and agent definitions live in separate files.
const agentsMDTemplate = `# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: {{.ProjectName}}
- **모드**: {{.Mode}}

## Installed Components

- Rules: .codex/rules/autopus/
- Skills: .codex/skills/
- Agents: .codex/agents/

## Language Policy

IMPORTANT: Follow these language settings strictly for all work in this project.

- **Code comments**: {{.Language.Comments}}
- **Commit messages**: {{.Language.Commits}}
- **AI responses**: {{.Language.AIResponses}}

## Core Guidelines

See .codex/rules/autopus/ for detailed rule definitions.
See .codex/agents/ for agent definitions.
`
