package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

func (a *Adapter) renderPromptTemplates(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	mappings, err := a.preparePromptFiles(cfg)
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		targetPath := filepath.Join(a.root, m.TargetPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf("codex prompts 디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(targetPath, m.Content, 0644); err != nil {
			return nil, fmt.Errorf("codex prompt 파일 쓰기 실패 %s: %w", targetPath, err)
		}
	}

	return mappings, nil
}

func (a *Adapter) preparePromptFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files := make([]adapter.FileMapping, 0, len(workflowSpecs))

	for _, spec := range workflowSpecs {
		rendered, err := a.renderWorkflowPrompt(spec, cfg)
		if err != nil {
			return nil, err
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "prompts", spec.Name+".md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

func (a *Adapter) renderWorkflowPrompt(spec workflowSpec, cfg *config.HarnessConfig) (string, error) {
	if rendered, ok := renderCustomWorkflowPrompt(spec); ok {
		return normalizeWorkflowPrompt(rendered), nil
	}
	if spec.PromptPath == "" {
		return "", fmt.Errorf("codex workflow prompt 경로 누락: %s", spec.Name)
	}

	tmplContent, err := templates.FS.ReadFile(spec.PromptPath)
	if err != nil {
		return "", fmt.Errorf("codex prompt 템플릿 읽기 실패 %s: %w", spec.PromptPath, err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return "", fmt.Errorf("codex prompt 템플릿 렌더링 실패 %s: %w", spec.Name, err)
	}
	if spec.Name == "auto" {
		rendered = rewriteCodexRouterBody(rendered)
	}

	return normalizeWorkflowPrompt(rendered), nil
}

func normalizeWorkflowPrompt(rendered string) string {
	rendered = normalizeCodexInvocationBody(rendered)
	rendered = normalizeCodexHelperPaths(rendered)
	rendered = normalizeCodexToolingBody(rendered)
	return rendered
}
