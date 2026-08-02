package omp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

func (a *Adapter) prepareCommandMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	if !ompOwnsCommandSurface(cfg) {
		return nil, nil
	}

	files := make([]adapter.FileMapping, 0, len(workflowSpecs))
	for _, spec := range workflowSpecs {
		rendered, err := a.renderWorkflowCommand(spec, cfg)
		if err != nil {
			return nil, err
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".agents", "commands", spec.Name+".md"),
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
	frontmatter := ompCommandFrontmatter(spec.Description)
	return buildMarkdown(frontmatter, thinWorkflowCommandBody(spec.Name)), nil
}

func (a *Adapter) renderRouterCommand() (string, error) {
	frontmatter := ompCommandFrontmatter("Autopus 명령 라우터 — oh-my-pi helper")
	return buildMarkdown(frontmatter, thinRouterCommandBody()), nil
}

func thinRouterCommandBody() string {
	return strings.TrimSpace(ompRouterBody(""))
}

func thinWorkflowCommandBody(name string) string {
	subcommand := strings.TrimPrefix(name, "auto-")
	return strings.TrimSpace(fmt.Sprintf("`$ARGUMENTS`\n\nTreat the text above as the full argument payload for `/%s`.\nLoad exact detail skill `%s`; this is the same target selected by `/auto %s ...`.\nPreserve `--model <provider/model>` and `--variant <value>` when present.\nDo not restate or expand the arguments unless needed for execution.", name, name, subcommand))
}

// ompCommandFrontmatter builds command frontmatter with the description escaped
// so it cannot close its field and add sibling keys.
func ompCommandFrontmatter(description string) string {
	return fmt.Sprintf("description: %s\nagent: build",
		pkgcontent.OMPYAMLScalar(ompDescriptionNormalizer.Replace(description)))
}

// ompDescriptionNormalizer rewrites product names that workflowSpecs inherited
// from the Codex and OpenCode wording. An omp user reading "Codex goal tool" has
// no such tool, so the emitted description names the capability instead of a
// foreign product. It deliberately does not claim an omp equivalent exists.
var ompDescriptionNormalizer = strings.NewReplacer(
	"Codex goal wrapper", "goal 래퍼",
	"Codex goal tool", "goal tool",
	"OpenCode helper", "oh-my-pi helper",
)
