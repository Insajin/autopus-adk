package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

// renderExtendedSkills transforms embedded content skills for the Codex platform
// and returns file mappings for .codex/skills/{skill-name}.md.
func (a *Adapter) renderExtendedSkills(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	catalog, err := pkgcontent.LoadSkillCatalogFromFS(content.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("skill catalog init: %w", err)
	}
	transformer, err := pkgcontent.NewSkillTransformerFromFS(content.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("skill transformer init: %w", err)
	}

	skills, report, err := transformer.TransformForPlatformWithOptions("codex", pkgcontent.SkillTransformOptions{
		ResolveSkillRef: func(name string) string {
			return pkgcontent.ResolveCatalogSkillRefPath(catalog, name, "codex", cfg)
		},
		AllowSkill: func(meta pkgcontent.SkillMeta) bool {
			return meta.Visibility != pkgcontent.SkillVisibilityExplicitOnly ||
				skillCompilerExplicitlySelects(cfg, meta.Name)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("skill transform for codex: %w", err)
	}

	logTransformReport(report)

	var files []adapter.FileMapping
	for _, s := range skills {
		entry, ok := catalog.Get(s.Name)
		if !ok {
			continue
		}
		state := pkgcontent.ResolveCatalogSkillState(entry, "codex", cfg)
		if !state.Compiled || state.TargetPath == "" {
			continue
		}
		content := normalizeCodexInvocationBody(s.Content)
		content = normalizeCodexHelperPaths(content)
		content = normalizeCodexToolingBody(content)
		content = normalizeCodexExtendedSkill(s.Name, content)
		relPath := filepath.FromSlash(state.TargetPath)
		content = ensureCodexSkillFrontmatter(relPath, s.Name, s.Description, content)
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(content),
			Content:         []byte(content),
		})
	}

	return files, nil
}

func ensureCodexSkillFrontmatter(targetPath, name, description, body string) string {
	if filepath.Base(targetPath) != "SKILL.md" {
		return body
	}
	frontmatter, parsedBody := splitSkillFrontmatter(body)
	if frontmatter != "" {
		return frontmatter + "\n\n" + strings.TrimSpace(parsedBody) + "\n"
	}
	if strings.TrimSpace(description) == "" {
		description = name
	}
	return fmt.Sprintf("---\nname: %s\ndescription: >\n  %s\n---\n\n%s\n",
		name,
		description,
		strings.TrimSpace(body),
	)
}

// logTransformReport prints a summary of skill transformation results.
func logTransformReport(report *pkgcontent.TransformReport) {
	summary := pkgcontent.FormatTransformReport(report)
	if summary == "" {
		return
	}
	// Diagnostics go to stderr so JSON consumers reading stdout stay parseable.
	fmt.Fprintln(os.Stderr, summary)
}

func skillCompilerExplicitlySelects(cfg *config.HarnessConfig, name string) bool {
	if cfg == nil {
		return false
	}
	for _, selected := range cfg.Skills.Compiler.ExplicitSkills {
		if selected == name {
			return true
		}
	}
	return false
}
