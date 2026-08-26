package codex

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

func (a *Adapter) prepareSkillTemplateMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	entries, err := templates.FS.ReadDir("codex/skills")
	if err != nil {
		return nil, fmt.Errorf("코덱스 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}

	var files []adapter.FileMapping
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		skillFile := strings.TrimSuffix(entry.Name(), ".tmpl")
		skillName := strings.TrimSuffix(skillFile, ".md")
		emit, err := shouldEmitCodexRepoSkillTemplate(skillFile, cfg)
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 대상 판정 실패 %s: %w", entry.Name(), err)
		}
		if !emit {
			continue
		}

		if spec, ok := codexWorkflowSpecByName(skillName); ok {
			rendered, renderErr := a.renderWorkflowSkill(cfg, spec)
			if renderErr != nil {
				return nil, renderErr
			}
			files = append(files, adapter.FileMapping{
				TargetPath:      codexProjectSkillPath(spec.Name),
				OverwritePolicy: adapter.OverwriteAlways,
				Checksum:        checksum(rendered),
				Content:         []byte(rendered),
			})
			continue
		}

		tmplContent, err := templates.FS.ReadFile("codex/skills/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 읽기 실패 %s: %w", entry.Name(), err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			if strings.HasPrefix(skillFile, "auto-") {
				return nil, fmt.Errorf("코덱스 스킬 템플릿 렌더링 실패 %s: %w", entry.Name(), err)
			}
			rendered = string(tmplContent)
		}

		rendered = normalizeCodexExtendedSkill(skillName, rendered)
		rendered = ensureCodexSkillFrontmatter(
			codexProjectSkillPath(skillName),
			skillName,
			skillName,
			normalizeCodexToolingBody(normalizeCodexHelperPaths(normalizeCodexInvocationBody(rendered))),
		)
		files = append(files, adapter.FileMapping{
			TargetPath:      codexProjectSkillPath(skillName),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	for _, spec := range workflowSpecs {
		if spec.Name == "auto" || spec.SkillPath != "" {
			continue
		}

		rendered, err := a.renderWorkflowSkill(cfg, spec)
		if err != nil {
			return nil, err
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      codexProjectSkillPath(spec.Name),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

func codexWorkflowSpecByName(name string) (workflowSpec, bool) {
	for _, spec := range workflowSpecs {
		if spec.Name == name && spec.Name != "auto" {
			return spec, true
		}
	}
	return workflowSpec{}, false
}
