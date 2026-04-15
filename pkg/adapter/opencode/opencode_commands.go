package opencode

import (
	"fmt"
	"path/filepath"

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

func (a *Adapter) renderWorkflowCommand(spec workflowSpec, _ *config.HarnessConfig) (string, error) {
	if spec.Name == "auto" {
		return a.renderRouterCommand()
	}
	frontmatter := fmt.Sprintf("description: %q\nagent: build", spec.Description)
	return buildMarkdown(frontmatter, thinWorkflowCommandBody(spec.Name)), nil
}

func (a *Adapter) renderRouterCommand() (string, error) {
	frontmatter := fmt.Sprintf("description: %q\nagent: build", routerDescription())
	return buildMarkdown(frontmatter, thinRouterCommandBody()), nil
}
