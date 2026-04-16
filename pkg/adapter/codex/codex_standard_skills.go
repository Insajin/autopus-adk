package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func (a *Adapter) renderStandardSkills(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	mappings, err := a.prepareStandardSkillMappings(cfg)
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		destPath := filepath.Join(a.root, m.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("codex standard skill dir 생성 실패 %s: %w", filepath.Dir(destPath), err)
		}
		if err := os.WriteFile(destPath, m.Content, 0644); err != nil {
			return nil, fmt.Errorf("codex standard skill 쓰기 실패 %s: %w", destPath, err)
		}
	}

	return mappings, nil
}

func (a *Adapter) prepareStandardSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files := make([]adapter.FileMapping, 0, len(workflowSpecs)*2)

	routerContent, err := a.renderRouterSkill(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, newSkillMapping(filepath.Join(".agents", "skills", "auto", "SKILL.md"), routerContent))

	for _, spec := range workflowSpecs {
		if spec.Name == "auto" {
			continue
		}

		content, err := a.renderWorkflowSkill(cfg, spec)
		if err != nil {
			return nil, err
		}
		files = append(files, newSkillMapping(filepath.Join(".agents", "skills", spec.Name, "SKILL.md"), content))
	}

	return files, nil
}

func (a *Adapter) renderPluginFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	mappings, err := a.preparePluginMappings(cfg)
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		destPath := filepath.Join(a.root, m.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("codex plugin dir 생성 실패 %s: %w", filepath.Dir(destPath), err)
		}
		if err := os.WriteFile(destPath, m.Content, 0644); err != nil {
			return nil, fmt.Errorf("codex plugin 파일 쓰기 실패 %s: %w", destPath, err)
		}
	}

	return mappings, nil
}

func (a *Adapter) preparePluginMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files := make([]adapter.FileMapping, 0, len(workflowSpecs)*2+2)

	routerContent, err := a.renderRouterSkill(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, newSkillMapping(filepath.Join(".autopus", "plugins", "auto", "skills", "auto", "SKILL.md"), routerContent))

	for _, spec := range workflowSpecs {
		if spec.Name == "auto" {
			continue
		}

		content, err := a.renderPluginWorkflowShim(cfg, spec)
		if err != nil {
			return nil, err
		}
		files = append(files, newSkillMapping(filepath.Join(".autopus", "plugins", "auto", "skills", spec.Name, "SKILL.md"), content))
	}

	pluginJSON, err := a.renderPluginManifestJSON()
	if err != nil {
		return nil, err
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      filepath.Join(".autopus", "plugins", "auto", ".codex-plugin", "plugin.json"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(pluginJSON),
		Content:         []byte(pluginJSON),
	})

	marketplaceJSON, err := a.renderMarketplaceJSON()
	if err != nil {
		return nil, err
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      filepath.Join(".agents", "plugins", "marketplace.json"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(marketplaceJSON),
		Content:         []byte(marketplaceJSON),
	})

	return files, nil
}

func newSkillMapping(targetPath, content string) adapter.FileMapping {
	return adapter.FileMapping{
		TargetPath:      targetPath,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(content),
		Content:         []byte(content),
	}
}
