package codex

import (
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func (a *Adapter) prepareStandardSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	routerContent, err := a.renderRouterSkill(cfg)
	if err != nil {
		return nil, err
	}
	return []adapter.FileMapping{
		newSkillMapping(codexProjectSkillPath("auto"), routerContent),
	}, nil
}

func (a *Adapter) preparePluginMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files := make([]adapter.FileMapping, 0, 3)

	routerContent, err := a.renderRouterSkill(cfg)
	if err != nil {
		return nil, err
	}
	pluginRouterContent := strings.Replace(routerContent, "name: codex-auto", "name: auto", 1)
	files = append(files, newSkillMapping(
		filepath.Join(".autopus", "plugins", "auto", "skills", "auto", "SKILL.md"),
		pluginRouterContent,
	))

	pluginJSON, err := a.renderPluginManifestJSON(cfg, pluginRouterContent)
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
		OverwritePolicy: adapter.OverwriteMerge,
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
