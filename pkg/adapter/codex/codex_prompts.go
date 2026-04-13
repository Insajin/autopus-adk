package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

const promptsTemplateDir = "codex/prompts"

// renderPromptTemplates reads Codex prompt templates from embedded FS,
// renders them, and writes to .codex/prompts/.
func (a *Adapter) renderPromptTemplates(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir(promptsTemplateDir)
	if err != nil {
		return nil, fmt.Errorf("codex prompt 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		name := entry.Name()
		promptFile := strings.TrimSuffix(name, ".tmpl")

		tmplContent, err := templates.FS.ReadFile(promptsTemplateDir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("codex prompt 템플릿 읽기 실패 %s: %w", name, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("codex prompt 템플릿 렌더링 실패 %s: %w", name, err)
		}
		rendered = normalizeCodexInvocationBody(rendered)
		rendered = normalizeCodexHelperPaths(rendered)

		targetPath := filepath.Join(a.root, ".codex", "prompts", promptFile)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf("codex prompts 디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
			return nil, fmt.Errorf("codex prompt 파일 쓰기 실패 %s: %w", targetPath, err)
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "prompts", promptFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

// preparePromptFiles prepares prompt file mappings without writing to disk.
func (a *Adapter) preparePromptFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir(promptsTemplateDir)
	if err != nil {
		return nil, fmt.Errorf("codex prompt 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		name := entry.Name()
		promptFile := strings.TrimSuffix(name, ".tmpl")

		tmplContent, err := templates.FS.ReadFile(promptsTemplateDir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("codex prompt 템플릿 읽기 실패 %s: %w", name, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("codex prompt 템플릿 렌더링 실패 %s: %w", name, err)
		}
		rendered = normalizeCodexInvocationBody(rendered)
		rendered = normalizeCodexHelperPaths(rendered)

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "prompts", promptFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}
