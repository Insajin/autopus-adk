package opencode

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func (a *Adapter) prepareCommandMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files := make([]adapter.FileMapping, 0, len(workflowSpecs))
	for _, spec := range workflowSpecs {
		rendered, err := a.renderWorkflowCommand(spec, cfg)
		if err != nil {
			return nil, err
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".opencode", "commands", spec.Name+".md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(rendered),
			Content:         []byte(rendered),
		})
	}
	return files, nil
}

func (a *Adapter) renderWorkflowCommand(spec workflowSpec, cfg *config.HarnessConfig) (string, error) {
	if spec.Name == "auto" {
		return a.renderRouterCommand(cfg)
	}
	if rendered, ok := renderCustomWorkflowCommand(spec); ok {
		return rendered, nil
	}

	raw, err := a.renderWorkflowPrompt(spec.PromptPath, cfg)
	if err != nil {
		return "", err
	}
	frontmatter, body := splitFrontmatter(raw)
	body = normalizeOpenCodeMarkdown(body)
	body = commandArgumentNote(spec.Name) + "\n" + body
	return buildMarkdown(augmentCommandFrontmatter(frontmatter), body), nil
}

func (a *Adapter) renderRouterCommand(cfg *config.HarnessConfig) (string, error) {
	raw, err := a.renderWorkflowPrompt("codex/prompts/auto.md.tmpl", cfg)
	if err != nil {
		return "", err
	}
	_, body := splitFrontmatter(raw)
	if strings.TrimSpace(body) == "" {
		body = raw
	}
	body = commandArgumentNote("auto") + "\n" + rewriteOpenCodeRouterBody(body)
	frontmatter := fmt.Sprintf("description: %q\nagent: build", routerDescription())
	return buildMarkdown(frontmatter, body), nil
}
