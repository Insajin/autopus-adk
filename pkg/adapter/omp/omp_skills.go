package omp

import (
	"fmt"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

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
	var files []adapter.FileMapping
	for _, spec := range workflowSpecs {
		if spec.Name != "auto" {
			continue
		}
		rendered, err := a.renderRouterSkill()
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

// renderRouterSkill builds the only workflow skill omp emits; the caller filters
// workflowSpecs down to the router before reaching here.
func (a *Adapter) renderRouterSkill() (string, error) {
	body := thinRouterSkillBody()
	frontmatter := ompSkillFrontmatter("auto", "Autopus 명령 라우터 — oh-my-pi helper")
	return buildMarkdown(frontmatter, body), nil
}

func thinRouterSkillBody() string {
	return `# Autopus 명령 라우터

이 스킬은 oh-my-pi 세션에서 Autopus CLI 도구를 호출하는 라우터 역할을 합니다.
`
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
		if skill.Name == "auto" {
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
		content := buildMarkdown(
			ompSkillFrontmatter(skill.Name, skill.Description),
			skill.Content,
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
