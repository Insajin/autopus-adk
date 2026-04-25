package content

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentSourceMeta holds the richer frontmatter from content/agents/*.md source files.
// This is distinct from AgentDefinition which uses a simpler schema.
type AgentSourceMeta struct {
	Name           string   `yaml:"name"`
	Description    string   `yaml:"description"`
	Model          string   `yaml:"model"`
	Effort         string   `yaml:"effort"`
	Tools          string   `yaml:"tools"`
	PermissionMode string   `yaml:"permissionMode"`
	MaxTurns       int      `yaml:"maxTurns"`
	Skills         []string `yaml:"skills"`
}

// AgentSource holds a parsed agent source file with metadata and body.
type AgentSource struct {
	Meta AgentSourceMeta
	Body string
}

// AgentTransformer loads agent source .md files and transforms them for target platforms.
type AgentTransformer struct {
	sources []AgentSource
}

// LoadAgentSources loads agent source files from a directory on disk.
func LoadAgentSources(dir string) ([]AgentSource, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read agent source dir %s: %w", dir, err)
	}

	var sources []AgentSource
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read agent source %s: %w", entry.Name(), err)
		}
		src, err := parseAgentSource(data, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("parse agent source %s: %w", entry.Name(), err)
		}
		sources = append(sources, src)
	}
	return sources, nil
}

// LoadAgentSourcesFromFS loads agent source files from an embedded filesystem.
func LoadAgentSourcesFromFS(fsys fs.FS, dir string) ([]AgentSource, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read agent source dir %s: %w", dir, err)
	}

	var sources []AgentSource
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := fs.ReadFile(fsys, dir+"/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read agent source %s: %w", entry.Name(), err)
		}
		src, err := parseAgentSource(data, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("parse agent source %s: %w", entry.Name(), err)
		}
		sources = append(sources, src)
	}
	return sources, nil
}

// NewAgentTransformer creates a transformer from pre-loaded sources.
func NewAgentTransformer(sources []AgentSource) *AgentTransformer {
	return &AgentTransformer{sources: sources}
}

// Sources returns the loaded agent sources.
func (t *AgentTransformer) Sources() []AgentSource {
	return t.sources
}

// TransformAgentForCodex produces a Codex TOML template from an agent source.
// The output contains Go template variables ({{.ProjectName}}, etc.) for later rendering.
func TransformAgentForCodex(src AgentSource) string {
	var sb strings.Builder

	model := MapModel(src.Meta.Model, "codex")
	effort := MapEffort(src.Meta.Effort, "codex")
	body := NormalizeAgentReferences(src.Body, "codex")

	fmt.Fprintf(&sb, "name = %q\n", src.Meta.Name)
	fmt.Fprintf(&sb, "description = %q\n", src.Meta.Description)
	fmt.Fprintf(&sb, "model = %q\n", model)
	sb.WriteString(formatCodexEffortLine(effort))
	sb.WriteString("\n")

	// Preserve the source structure so Codex keeps the phase contracts and I/O shape.
	instructions := buildCodexInstructions(src.Meta, body)
	sb.WriteString("developer_instructions = ")
	sb.WriteString(formatTOMLMultilineString(instructions))
	sb.WriteString("\n")

	return sb.String()
}

func formatCodexEffortLine(effort string) string {
	if effort == "xhigh" {
		return `model_reasoning_effort = "xhigh"`
	}
	return fmt.Sprintf(`model_reasoning_effort = "{{if eq .Quality.Default "ultra"}}xhigh{{else}}%s{{end}}"`, effort)
}

// TransformAgentForGemini produces a Gemini MD template from an agent source.
func TransformAgentForGemini(src AgentSource) string {
	var sb strings.Builder

	body := NormalizeAgentReferences(src.Body, "gemini")

	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "name: auto-agent-%s\n", src.Meta.Name)
	fmt.Fprintf(&sb, "description: %s\n", src.Meta.Description)
	if len(src.Meta.Skills) > 0 {
		sb.WriteString("skills:\n")
		for _, s := range src.Meta.Skills {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	sb.WriteString("\n")

	return sb.String()
}

// parseAgentSource parses an agent source .md file into AgentSource.
func parseAgentSource(data []byte, filename string) (AgentSource, error) {
	raw := string(data)
	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		return AgentSource{}, fmt.Errorf("frontmatter split: %w", err)
	}

	var meta AgentSourceMeta
	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
			return AgentSource{}, fmt.Errorf("yaml parse: %w", err)
		}
	}

	if meta.Name == "" {
		meta.Name = strings.TrimSuffix(filename, ".md")
	}

	return AgentSource{
		Meta: meta,
		Body: strings.TrimSpace(body),
	}, nil
}

// buildCodexInstructions creates structured developer_instructions text from agent metadata and body.
func buildCodexInstructions(meta AgentSourceMeta, body string) string {
	parts := []string{
		fmt.Sprintf("You are the %s agent for project {{.ProjectName}}.", meta.Name),
	}

	if meta.Description != "" {
		parts = append(parts, meta.Description)
	}

	parts = append(parts,
		"Keep the task scoped to this role. Follow the explicit ownership and completion contract from the parent prompt when it is provided.",
	)

	if strings.TrimSpace(body) != "" {
		parts = append(parts, strings.TrimSpace(body))
	}

	var ops []string
	if len(meta.Skills) > 0 {
		ops = append(ops, fmt.Sprintf("- Skills reference: %s", strings.Join(meta.Skills, ", ")))
	}
	if meta.Tools != "" {
		ops = append(ops, fmt.Sprintf("- Source tool contract: %s", meta.Tools))
	}
	if meta.Effort != "" {
		ops = append(ops, fmt.Sprintf("- Source effort default: %s", meta.Effort))
	}
	if meta.PermissionMode != "" {
		ops = append(ops, fmt.Sprintf("- Source permission intent: %s", meta.PermissionMode))
	}
	if meta.MaxTurns > 0 {
		ops = append(ops, fmt.Sprintf("- Source max turns: %d", meta.MaxTurns))
	}
	ops = append(ops,
		"- File size limit: 300 lines per source file.",
		"- Test coverage target: {{if .IsFullMode}}85{{else}}80{{end}}%+.",
	)

	parts = append(parts, "## Operational Defaults\n"+strings.Join(ops, "\n"))

	return strings.Join(parts, "\n\n")
}

func normalizeTOMLMultiline(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return s
}

func escapeTOMLBasicMultiline(s string) string {
	s = normalizeTOMLMultiline(s)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"\"\"", "\\\"\\\"\\\"")
	return s
}

func formatTOMLMultilineString(s string) string {
	s = normalizeTOMLMultiline(s)
	if !strings.Contains(s, "'''") {
		return "'''\n" + s + "\n'''"
	}
	return "\"\"\"\n" + escapeTOMLBasicMultiline(s) + "\n\"\"\""
}
