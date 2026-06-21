package claude

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

// workflowRoute pairs an embedded generated workflow template with its on-disk
// target. Both Route A and the team route share the same render contract.
type workflowRoute struct {
	templatePath string
	targetPath   string
}

// workflowRoutes lists every generated workflow surface the claude adapter
// installs. Route A is the always-on opt-in route; route_team is the deterministic
// team route (T17). The team workflow surface is claude-only by construction —
// non-claude adapters never reference these templates.
var workflowRoutes = []workflowRoute{
	{
		templatePath: "claude/workflows/route_a.workflow.js.tmpl",
		targetPath:   ".claude/workflows/route_a.workflow.js",
	},
	{
		templatePath: "claude/workflows/route_team.workflow.js.tmpl",
		targetPath:   ".claude/workflows/route_team.workflow.js",
	},
}

// workflowFiles renders every generated workflow JS (REQ-004 / S1 adapter half,
// plus the team route T17) and returns them as FileMappings. Each JS is a
// generated surface, so it uses OverwriteAlways and its first line preserves the
// edit-forbidden warning.
func (a *Adapter) workflowFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	mappings := make([]adapter.FileMapping, 0, len(workflowRoutes))
	for _, route := range workflowRoutes {
		tmplContent, err := templates.FS.ReadFile(route.templatePath)
		if err != nil {
			return nil, fmt.Errorf("워크플로우 템플릿 읽기 실패 %s: %w", route.templatePath, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("워크플로우 템플릿 렌더링 실패 %s: %w", route.templatePath, err)
		}

		// The first line MUST remain the generated / edit-forbidden warning. If
		// the template engine mangled it (e.g. via brace handling), fall back to
		// the raw embedded bytes so the generated-surface contract holds.
		if !isGeneratedWarning(rendered) {
			rendered = string(tmplContent)
		}

		mappings = append(mappings, adapter.FileMapping{
			TargetPath:      route.targetPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}
	return mappings, nil
}

// isGeneratedWarning reports whether the first line carries the generated and
// edit-forbidden markers.
func isGeneratedWarning(content string) bool {
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}
	return strings.Contains(firstLine, "GENERATED") && strings.Contains(firstLine, "DO NOT EDIT")
}
