package opencode

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

var markerRe = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(markerBegin) + `.*?` + regexp.QuoteMeta(markerEnd))

const agentsMDTemplate = `# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: {{.ProjectName}}
- **모드**: {{.Mode}}
- **플랫폼**: opencode

## Installed Components

- Rules: .opencode/rules/autopus/
- Skills: .agents/skills/
- Commands: .opencode/commands/
- Agents: .opencode/agents/
- Plugins: .opencode/plugins/

## Language Policy

IMPORTANT: Follow these language settings strictly for all work in this project.

- **Code comments**: {{.Language.Comments}}
- **Commit messages**: {{.Language.Commits}}
- **AI responses**: {{.Language.AIResponses}}

## OpenCode Notes

- The generated rules are loaded through opencode.json instructions.
- Use /auto <subcommand> ... or direct aliases like /auto-plan ... .
- Project skills are published under .agents/skills/ so OpenCode can load them through the native skill tool.
`

func (a *Adapter) prepareAgentsMapping(cfg *config.HarnessConfig) (adapter.FileMapping, error) {
	content, err := a.injectMarkerSection(cfg)
	if err != nil {
		return adapter.FileMapping{}, err
	}
	return adapter.FileMapping{
		TargetPath:      "AGENTS.md",
		OverwritePolicy: adapter.OverwriteMarker,
		Checksum:        adapter.Checksum(content),
		Content:         []byte(content),
	}, nil
}

func (a *Adapter) injectMarkerSection(cfg *config.HarnessConfig) (string, error) {
	path := filepath.Join(a.root, "AGENTS.md")
	var existing string
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	section, err := a.engine.RenderString(agentsMDTemplate, cfg)
	if err != nil {
		return "", fmt.Errorf("AGENTS.md 템플릿 렌더링 실패: %w", err)
	}
	agentsSection, err := renderAgentsSection()
	if err != nil {
		return "", err
	}
	section += agentsSection
	section += "\n## Rules\n\nSee .opencode/rules/autopus/ for detailed guidance.\n"
	newSection := markerBegin + "\n" + section + "\n" + markerEnd

	if strings.Contains(existing, markerBegin) && strings.Contains(existing, markerEnd) {
		return markerRe.ReplaceAllString(existing, newSection), nil
	}
	if existing == "" {
		return newSection + "\n", nil
	}
	return existing + "\n\n" + newSection + "\n", nil
}

func renderAgentsSection() (string, error) {
	entries, err := contentfs.FS.ReadDir("agents")
	if err != nil {
		return "", fmt.Errorf("agents 디렉터리 읽기 실패: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("\n## Agents\n\n")
	sb.WriteString("The following specialized agents are available.\n\n")
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, readErr := fs.ReadFile(contentfs.FS, filepath.Join("agents", entry.Name()))
		if readErr != nil {
			return "", fmt.Errorf("agent 파일 읽기 실패 %s: %w", entry.Name(), readErr)
		}
		name, desc := extractAgentMeta(string(data), entry.Name())
		fmt.Fprintf(&sb, "### %s\n\n", name)
		if desc != "" {
			sb.WriteString(desc)
			sb.WriteString("\n\n")
		}
	}
	return sb.String(), nil
}

func extractAgentMeta(content string, fallback string) (string, string) {
	_, body := splitFrontmatter(content)
	if strings.TrimSpace(body) == "" {
		body = content
	}
	name := strings.TrimSuffix(fallback, filepath.Ext(fallback))
	var desc string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			name = strings.TrimPrefix(trimmed, "# ")
			continue
		}
		desc = trimmed
		break
	}
	return name, desc
}

func removeMarkerSection(content string) string {
	cleaned := strings.TrimSpace(markerRe.ReplaceAllString(content, ""))
	if cleaned == "" {
		return ""
	}
	return cleaned + "\n"
}
