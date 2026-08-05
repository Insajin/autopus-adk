package omp

import (
	"fmt"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

var ompNativeExtendedSkillTemplates = map[string]string{
	"agent-pipeline":     "shared/omp-agent-pipeline.md.tmpl",
	"worktree-isolation": "shared/omp-worktree-isolation.md.tmpl",
}

func (a *Adapter) prepareSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	if !ompOwnsSharedSkillSurface(cfg) {
		return nil, nil
	}

	workflow, err := a.prepareWorkflowSkillMappings(cfg)
	if err != nil {
		return nil, err
	}
	extended, err := a.prepareExtendedSkillMappings(cfg)
	if err != nil {
		return nil, err
	}
	return append(workflow, extended...), nil
}

func (a *Adapter) prepareWorkflowSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files := make([]adapter.FileMapping, 0, len(workflowSpecs))
	for _, spec := range workflowSpecs {
		rendered, err := a.renderWorkflowSkill(spec, cfg)
		if err != nil {
			return nil, err
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".agents", "skills", spec.Name, "SKILL.md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(rendered),
			Content:         []byte(rendered),
		})
	}
	return files, nil
}

func (a *Adapter) renderWorkflowSkill(spec workflowSpec, cfg *config.HarnessConfig) (string, error) {
	if spec.Name == "auto" {
		return a.renderRouterSkill()
	}
	if spec.SkillPath == "" {
		return renderOMPCompactWorkflowSkill(spec), nil
	}
	return a.renderTemplateWorkflowSkill(spec, cfg)
}

func (a *Adapter) renderRouterSkill() (string, error) {
	body := thinRouterSkillBody()
	frontmatter := ompSkillFrontmatter("auto", "Autopus 명령 라우터 — oh-my-pi helper")
	return buildMarkdown(frontmatter, body), nil
}

func thinRouterSkillBody() string {
	return ompRouterBody("# Autopus 명령 라우터\n\n")
}

func (a *Adapter) renderTemplateWorkflowSkill(spec workflowSpec, cfg *config.HarnessConfig) (string, error) {
	raw, err := templates.FS.ReadFile(spec.SkillPath)
	if err != nil {
		return "", fmt.Errorf("workflow 템플릿 읽기 실패 %s: %w", spec.SkillPath, err)
	}
	rendered, err := a.engine.RenderString(string(raw), cfg)
	if err != nil {
		return "", fmt.Errorf("workflow 템플릿 렌더링 실패 %s: %w", spec.SkillPath, err)
	}
	_, body := splitOMPFrontmatter(rendered)
	if strings.TrimSpace(body) == "" {
		body = rendered
	}
	body = normalizeOMPWorkflowBody(body)
	body = injectOMPInvocation(body, spec.Name)
	return buildMarkdown(ompSkillFrontmatter(spec.Name, spec.Description), body), nil
}

func buildMarkdown(frontmatter, body string) string {
	return fmt.Sprintf("---\n%s\n---\n\n%s\n", strings.TrimSpace(frontmatter), strings.TrimSpace(body))
}

func (a *Adapter) prepareExtendedSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	catalog, err := pkgcontent.LoadSkillCatalogFromFS(contentfs.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("skill catalog init 실패: %w", err)
	}
	transformer, err := pkgcontent.NewSkillTransformerFromFS(contentfs.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("skill transformer init 실패: %w", err)
	}
	skills, _, err := transformer.TransformForPlatformWithOptions("omp", pkgcontent.SkillTransformOptions{
		ResolveSkillRef: func(name string) string {
			return pkgcontent.ResolveCatalogSkillRefPath(catalog, name, "omp", cfg)
		},
		AllowSkill: func(meta pkgcontent.SkillMeta) bool {
			return meta.Visibility != pkgcontent.SkillVisibilityExplicitOnly ||
				skillCompilerExplicitlySelects(cfg, meta.Name)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("omp skill transform 실패: %w", err)
	}

	files := make([]adapter.FileMapping, 0, len(skills))
	for _, skill := range skills {
		if isWorkflowSkillName(skill.Name) {
			continue
		}
		entry, ok := catalog.Get(skill.Name)
		if !ok {
			continue
		}
		state := pkgcontent.ResolveCatalogSkillState(entry, "omp", cfg)
		if !state.Compiled || state.TargetPath == "" {
			continue
		}
		body := skill.Content
		if templatePath, native := ompNativeExtendedSkillTemplates[skill.Name]; native {
			raw, readErr := templates.FS.ReadFile(templatePath)
			if readErr != nil {
				return nil, fmt.Errorf("read OMP native skill %s: %w", skill.Name, readErr)
			}
			body = pkgcontent.NormalizeOMPSemanticReferences(string(raw))
		}
		content := buildMarkdown(
			ompSkillFrontmatter(skill.Name, skill.Description),
			body,
		)
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.FromSlash(state.TargetPath),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}

func isWorkflowSkillName(name string) bool {
	for _, spec := range workflowSpecs {
		if spec.Name == name {
			return true
		}
	}
	return false
}

func skillCompilerExplicitlySelects(cfg *config.HarnessConfig, name string) bool {
	if cfg == nil {
		return false
	}
	return containsString(cfg.Skills.Compiler.ExplicitSkills, name)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ompSkillFrontmatter builds skill frontmatter with every value escaped. A name
// or description carrying a newline or ": " would otherwise close its field and
// inject sibling keys such as a second compatibility line.
func ompSkillFrontmatter(name, description string) string {
	return fmt.Sprintf("name: %s\ndescription: %s\ncompatibility: omp",
		pkgcontent.OMPYAMLScalar(name), pkgcontent.OMPYAMLScalar(description))
}
