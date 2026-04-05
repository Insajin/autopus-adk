package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validateName rejects names containing path separators or traversal sequences.
func validateName(name string) error {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid name %q: contains path separators or traversal", name)
	}
	return nil
}

// GenerateAllTemplates reads content sources and writes platform templates.
// contentDir is the path to the content/ directory (agents/, skills/ subdirs).
// templateDir is the path to the templates/ directory (codex/, gemini/ subdirs).
func GenerateAllTemplates(contentDir, templateDir string) error {
	if err := generateAgentTemplates(contentDir, templateDir); err != nil {
		return fmt.Errorf("agent templates: %w", err)
	}
	if err := generateSkillTemplates(contentDir, templateDir); err != nil {
		return fmt.Errorf("skill templates: %w", err)
	}
	return nil
}

// generateAgentTemplates transforms agent sources into Codex TOML and Gemini MD.
func generateAgentTemplates(contentDir, templateDir string) error {
	sources, err := LoadAgentSources(filepath.Join(contentDir, "agents"))
	if err != nil {
		return fmt.Errorf("load agent sources: %w", err)
	}

	codexDir := filepath.Join(templateDir, "codex", "agents")
	geminiDir := filepath.Join(templateDir, "gemini", "agents")

	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return fmt.Errorf("create codex agents dir: %w", err)
	}
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		return fmt.Errorf("create gemini agents dir: %w", err)
	}

	for _, src := range sources {
		if err := validateName(src.Meta.Name); err != nil {
			return fmt.Errorf("agent %s: %w", src.Meta.Name, err)
		}

		// Codex TOML
		toml := TransformAgentForCodex(src)
		path := filepath.Join(codexDir, src.Meta.Name+".toml.tmpl")
		if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
			return fmt.Errorf("write codex agent %s: %w", src.Meta.Name, err)
		}

		// Gemini MD
		md := TransformAgentForGemini(src)
		path = filepath.Join(geminiDir, src.Meta.Name+".md.tmpl")
		if err := os.WriteFile(path, []byte(md), 0644); err != nil {
			return fmt.Errorf("write gemini agent %s: %w", src.Meta.Name, err)
		}
	}

	return nil
}

// generateSkillTemplates transforms skills into Codex and Gemini templates.
// Existing auto-* command skill templates are preserved (not overwritten).
func generateSkillTemplates(contentDir, templateDir string) error {
	transformer, err := NewSkillTransformer(filepath.Join(contentDir, "skills"))
	if err != nil {
		return fmt.Errorf("create skill transformer: %w", err)
	}

	for _, platform := range []string{"codex", "gemini"} {
		transformed, _, err := transformer.TransformForPlatform(platform)
		if err != nil {
			return fmt.Errorf("transform skills for %s: %w", platform, err)
		}

		for _, skill := range transformed {
			// Skip auto-* names to preserve existing command skill templates
			if strings.HasPrefix(skill.Name, "auto-") {
				continue
			}

			if err := validateName(skill.Name); err != nil {
				return fmt.Errorf("skill %s: %w", skill.Name, err)
			}

			body := ReplacePlatformReferences(skill.Content, platform)
			output := buildSkillTemplate(skill.Name, body, platform)

			path, err := writeSkillTemplate(templateDir, platform, skill.Name, output)
			if err != nil {
				return fmt.Errorf("write skill %s/%s: %w", platform, skill.Name, err)
			}
			_ = path
		}
	}

	return nil
}

// writeSkillTemplate writes a skill template to the correct platform path.
// Codex: templates/codex/skills/{name}.md.tmpl
// Gemini: templates/gemini/skills/{name}/SKILL.md.tmpl (subdirectory structure)
func writeSkillTemplate(templateDir, platform, name, output string) (string, error) {
	var path string

	switch platform {
	case "gemini", "gemini-cli":
		dir := filepath.Join(templateDir, "gemini", "skills", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create gemini skill dir: %w", err)
		}
		path = filepath.Join(dir, "SKILL.md.tmpl")
	default:
		dir := filepath.Join(templateDir, platform, "skills")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create %s skill dir: %w", platform, err)
		}
		path = filepath.Join(dir, name+".md.tmpl")
	}

	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// buildSkillTemplate creates platform-formatted skill content.
func buildSkillTemplate(name, body, platform string) string {
	var sb strings.Builder

	switch platform {
	case "gemini", "gemini-cli":
		sb.WriteString("---\n")
		fmt.Fprintf(&sb, "name: auto-%s\n", name)
		sb.WriteString("---\n\n")
	default:
		fmt.Fprintf(&sb, "# auto-%s\n\n", name)
	}

	if body != "" {
		sb.WriteString(body)
		sb.WriteString("\n")
	}
	return sb.String()
}
