package opencode

import (
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func (a *Adapter) prepareCommandMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files := make([]adapter.FileMapping, 0, len(workflowSpecs))
	for _, spec := range workflowSpecs {
		rendered, err := a.renderWorkflowPrompt(spec.PromptPath, cfg)
		if err != nil {
			return nil, err
		}
		frontmatter, body := splitFrontmatter(rendered)
		body = normalizeOpenCodeMarkdown(body)
		body = commandArgumentNote(spec.Name) + "\n" + body
		content := buildMarkdown(augmentCommandFrontmatter(frontmatter), body)
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".opencode", "commands", spec.Name+".md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}
