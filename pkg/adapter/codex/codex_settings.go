package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

const codexConfigRelPath = ".codex/config.toml"

// prepareConfigFile returns the project-scoped Codex config mapping without writing to disk.
func (a *Adapter) prepareConfigFile(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	return a.prepareConfigFileWithManifest(cfg, nil)
}

func (a *Adapter) prepareConfigFileWithManifest(
	cfg *config.HarnessConfig,
	oldManifest *adapter.Manifest,
) ([]adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile("codex/config.toml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("codex config 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), a.codexRenderData(cfg))
	if err != nil {
		return nil, fmt.Errorf("codex config 템플릿 렌더링 실패: %w", err)
	}
	targetPath := filepath.Join(a.root, codexConfigRelPath)
	if existing, readErr := os.ReadFile(targetPath); readErr == nil {
		legacyPolicy := cfg == nil || cfg.Quality.SupervisorModelPolicy == ""
		preservation := codexModelSettingsToPreserve(existing, oldManifest, legacyPolicy)
		rendered, err = mergeCodexConfig(string(existing), rendered)
		if err != nil {
			return nil, err
		}
		rendered = preserveUserCodexModelSettings(rendered, preservation)
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("existing Codex config 읽기 실패: %w", readErr)
	}

	return []adapter.FileMapping{{
		TargetPath:      codexConfigRelPath,
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(rendered),
		Content:         []byte(rendered),
	}}, nil
}
