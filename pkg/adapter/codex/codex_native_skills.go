package codex

import (
	"path/filepath"
	"strings"

	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

func codexNativeSkillName(name string) string {
	return pkgcontent.ResolveCatalogSkillOutputName(name, "codex")
}

func codexProjectSkillPath(name string) string {
	return filepath.Join(".codex", "skills", codexNativeSkillName(name), "SKILL.md")
}

func isCodexWorkflowSkill(name string) bool {
	for _, spec := range workflowSpecs {
		if spec.Name == name {
			return true
		}
	}
	return false
}

func hasCodexSkillTemplate(name string) bool {
	_, err := templates.FS.ReadFile("codex/skills/" + strings.TrimSpace(name) + ".md.tmpl")
	return err == nil
}

func normalizeCodexNativeSkillReferences(body string) string {
	names := make(map[string]bool)
	if entries, err := templates.FS.ReadDir("codex/skills"); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md.tmpl") {
				continue
			}
			names[strings.TrimSuffix(entry.Name(), ".md.tmpl")] = true
		}
	}
	for _, spec := range workflowSpecs {
		names[spec.Name] = true
	}
	for name := range names {
		nativeName := codexNativeSkillName(name)
		body = strings.ReplaceAll(body, "$"+name, "$"+nativeName)
		// Both layouts appear in sources: the legacy flat file and the
		// directory form that every platform now installs. Rewriting only the
		// flat one left .codex/skills/<name>/SKILL.md without its codex-
		// prefix, so it named a directory the installer never creates.
		for _, authored := range []string{
			filepath.Join(".codex", "skills", name+".md"),
			filepath.Join(".codex", "skills", name, "SKILL.md"),
		} {
			body = strings.ReplaceAll(
				body,
				filepath.ToSlash(authored),
				filepath.ToSlash(codexProjectSkillPath(name)),
			)
		}
	}
	return body
}
