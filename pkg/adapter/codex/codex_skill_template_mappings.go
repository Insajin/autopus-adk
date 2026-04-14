package codex

import (
	"fmt"
	"path/filepath"
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

		rendered = normalizeCodexInvocationBody(rendered)
		rendered = normalizeCodexHelperPaths(rendered)
		rendered = normalizeCodexToolingBody(rendered)
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "skills", skillFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}
