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

// generateConfig renders the project-scoped Codex config template.
func (a *Adapter) generateConfig(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile("codex/config.toml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("codex config 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return nil, fmt.Errorf("codex config 템플릿 렌더링 실패: %w", err)
	}

	targetPath := filepath.Join(a.root, codexConfigRelPath)
	if existing, readErr := os.ReadFile(targetPath); readErr == nil {
		rendered = preserveUserCodexModelSettings(rendered, string(existing))
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, fmt.Errorf("codex config 디렉터리 생성 실패: %w", err)
	}
	if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
		return nil, fmt.Errorf("codex config.toml 쓰기 실패: %w", err)
	}

	return []adapter.FileMapping{{
		TargetPath:      codexConfigRelPath,
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(rendered),
		Content:         []byte(rendered),
	}}, nil
}

// prepareConfigFile returns the project-scoped Codex config mapping without writing to disk.
func (a *Adapter) prepareConfigFile(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile("codex/config.toml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("codex config 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return nil, fmt.Errorf("codex config 템플릿 렌더링 실패: %w", err)
	}
	if existing, readErr := os.ReadFile(filepath.Join(a.root, codexConfigRelPath)); readErr == nil {
		rendered = preserveUserCodexModelSettings(rendered, string(existing))
	}

	return []adapter.FileMapping{{
		TargetPath:      codexConfigRelPath,
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(rendered),
		Content:         []byte(rendered),
	}}, nil
}
