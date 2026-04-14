package opencode

import (
	"fmt"
	"path/filepath"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

func (a *Adapter) prepareSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	workflow, err := a.prepareWorkflowSkillMappings(cfg)
	if err != nil {
		return nil, err
	}
	extended, err := a.prepareExtendedSkillMappings()
	if err != nil {
		return nil, err
	}
	return append(workflow, extended...), nil
}

func (a *Adapter) prepareWorkflowSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files := make([]adapter.FileMapping, 0, len(workflowSpecs))
	for _, spec := range workflowSpecs {
		rendered, err := a.renderWorkflowPrompt(spec.PromptPath, cfg)
		if err != nil {
			return nil, err
		}
		_, body := splitFrontmatter(rendered)
		body = normalizeOpenCodeMarkdown(body)
		body = skillInvocationNote(spec.Name) + "\n" + body
		content := buildMarkdown(fmt.Sprintf("name: %s\ndescription: %q\ncompatibility: opencode", spec.Name, spec.Description), body)
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".agents", "skills", spec.Name, "SKILL.md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}

func (a *Adapter) prepareExtendedSkillMappings() ([]adapter.FileMapping, error) {
	transformer, err := pkgcontent.NewSkillTransformerFromFS(contentfs.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("skill transformer init 실패: %w", err)
	}
	skills, _, err := transformer.TransformForPlatform("opencode")
	if err != nil {
		return nil, fmt.Errorf("opencode skill transform 실패: %w", err)
	}

	files := make([]adapter.FileMapping, 0, len(skills))
	for _, skill := range skills {
		content := buildMarkdown(
			fmt.Sprintf("name: %s\ndescription: %q\ncompatibility: opencode", skill.Name, skill.Description),
			skill.Content,
		)
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".agents", "skills", skill.Name, "SKILL.md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}

func (a *Adapter) renderWorkflowPrompt(templatePath string, cfg *config.HarnessConfig) (string, error) {
	tmplContent, err := templates.FS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("workflow 템플릿 읽기 실패 %s: %w", templatePath, err)
	}
	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return "", fmt.Errorf("workflow 템플릿 렌더링 실패 %s: %w", templatePath, err)
	}
	return rendered, nil
}
