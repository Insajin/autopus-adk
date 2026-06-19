package claude

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

const (
	// workflowTemplatePath is the embedded generated Route A workflow template.
	workflowTemplatePath = "claude/workflows/route_a.workflow.js.tmpl"
	// workflowTargetPath is the on-disk target for the generated workflow JS.
	workflowTargetPath = ".claude/workflows/route_a.workflow.js"
)

// workflowFiles renders the generated Route A workflow JS (REQ-004 / S1 adapter
// half) and returns it as a FileMapping. The JS is a generated surface, so it
// uses OverwriteAlways and its first line preserves the edit-forbidden warning.
func (a *Adapter) workflowFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile(workflowTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("워크플로우 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return nil, fmt.Errorf("워크플로우 템플릿 렌더링 실패: %w", err)
	}

	// The first line MUST remain the generated / edit-forbidden warning. If the
	// template engine mangled it (e.g. via brace handling), fall back to the
	// raw embedded bytes so the generated-surface contract holds.
	if !isGeneratedWarning(rendered) {
		rendered = string(tmplContent)
	}

	return []adapter.FileMapping{{
		TargetPath:      workflowTargetPath,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(rendered),
		Content:         []byte(rendered),
	}}, nil
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
