package codex

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

const agentsTemplateDir = "codex/agents"

// generateAgents renders TOML agent templates and writes to .codex/agents/.
func (a *Adapter) generateAgents(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir(agentsTemplateDir)
	if err != nil {
		return nil, fmt.Errorf("codex agent 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		name := entry.Name()
		agentFile := strings.TrimSuffix(name, ".tmpl")

		tmplContent, err := templates.FS.ReadFile(agentsTemplateDir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("codex agent 템플릿 읽기 실패 %s: %w", name, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("codex agent 템플릿 렌더링 실패 %s: %w", name, err)
		}
		rendered = normalizeCodexHelperPaths(rendered)
		rendered = normalizeCodexAgentContracts(rendered)

		targetPath := filepath.Join(a.root, ".codex", "agents", agentFile)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf(".codex/agents 디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
			return nil, fmt.Errorf("codex agent 파일 쓰기 실패 %s: %w", targetPath, err)
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "agents", agentFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

// prepareAgentFiles returns agent file mappings without writing to disk.
func (a *Adapter) prepareAgentFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir(agentsTemplateDir)
	if err != nil {
		return nil, fmt.Errorf("codex agent 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		name := entry.Name()
		agentFile := strings.TrimSuffix(name, ".tmpl")

		tmplContent, err := templates.FS.ReadFile(agentsTemplateDir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("codex agent 템플릿 읽기 실패 %s: %w", name, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("codex agent 템플릿 렌더링 실패 %s: %w", name, err)
		}
		rendered = normalizeCodexHelperPaths(rendered)
		rendered = normalizeCodexAgentContracts(rendered)

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "agents", agentFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

// renderAgentsSection renders embedded agent definitions as an inline section
// for AGENTS.md. Each agent becomes a subsection with its description.
func renderAgentsSection() (string, error) {
	var sb strings.Builder
	sb.WriteString("\n## Agents\n\n")
	sb.WriteString("The following specialized agents are available.\n\n")

	entries, err := contentfs.FS.ReadDir("agents")
	if err != nil {
		return "", fmt.Errorf("agents 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		data, err := fs.ReadFile(contentfs.FS, "agents/"+entry.Name())
		if err != nil {
			return "", fmt.Errorf("agent 파일 읽기 실패 %s: %w", entry.Name(), err)
		}

		name, desc := extractAgentMeta(string(data))
		if name == "" {
			name = strings.TrimSuffix(entry.Name(), ".md")
		}
		fmt.Fprintf(&sb, "### %s\n\n", name)
		if desc != "" {
			sb.WriteString(desc)
			sb.WriteString("\n\n")
		}
	}

	return sb.String(), nil
}

// extractAgentMeta extracts agent name and first paragraph description.
func extractAgentMeta(content string) (name, desc string) {
	content = stripFrontmatter(content)
	lines := strings.SplitN(content, "\n", -1)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			name = strings.TrimPrefix(trimmed, "# ")
			continue
		}
		if name != "" && desc == "" {
			desc = trimmed
			break
		}
	}
	return name, desc
}

func normalizeCodexAgentContracts(rendered string) string {
	if strings.Contains(rendered, "`owned_paths`") &&
		strings.Contains(rendered, "`changed_files`") &&
		strings.Contains(rendered, "`verification`") &&
		strings.Contains(rendered, "`blockers`") &&
		strings.Contains(rendered, "`next_required_step`") {
		return rendered
	}

	contract := `
## Supervisor Return Contract

When spawned by a supervisor, the final response MUST include these machine-readable fields:

- ` + "`owned_paths`" + `: exact files, directories, or modules the worker owned
- ` + "`changed_files`" + `: files actually changed, or ` + "`none`" + `
- ` + "`verification`" + `: commands, checks, or inspections run, including failures
- ` + "`blockers`" + `: unresolved blockers, or ` + "`none`" + `
- ` + "`next_required_step`" + `: the next gate, retry, handoff, or ` + "`none`" + `
`
	anchor := "\n'''"
	if idx := strings.LastIndex(rendered, anchor); idx >= 0 {
		return rendered[:idx] + contract + rendered[idx:]
	}
	return strings.TrimRight(rendered, "\n") + "\n" + strings.TrimLeft(contract, "\n")
}
