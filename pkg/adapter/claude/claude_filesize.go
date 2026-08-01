package claude

import (
	"fmt"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

// fileSizeLimitData is the template data struct for the file-size-limit rule.
type fileSizeLimitData struct {
	Exclusions []content.FileSizeExclusion
	// Paths carries the source `globs` as a native claude-code `paths:` list,
	// which is what keeps this rule out of baseline context
	// (REQ-CONDRULE-COMPILE-01). The rule is rendered here rather than compiled
	// by pkg/rulecond because its exclusion list depends on the detected stack.
	Paths []string
}

// prepareFileSizeLimitRule renders the file-size-limit.md template with stack/framework exclusions.
func (a *Adapter) prepareFileSizeLimitRule(cfg *config.HarnessConfig) (adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile("claude/rules/file-size-limit.md.tmpl")
	if err != nil {
		return adapter.FileMapping{}, fmt.Errorf("file-size-limit 템플릿 읽기 실패: %w", err)
	}

	paths, err := claudeRulePaths(fileSizeLimitRuleFile)
	if err != nil {
		return adapter.FileMapping{}, err
	}

	exclusions := content.FileSizeExclusions(cfg.Stack, cfg.Framework)
	data := fileSizeLimitData{Exclusions: exclusions, Paths: paths}

	rendered, err := a.engine.RenderString(string(tmplContent), data)
	if err != nil {
		return adapter.FileMapping{}, fmt.Errorf("file-size-limit 템플릿 렌더링 실패: %w", err)
	}

	return adapter.FileMapping{
		TargetPath:      filepath.Join(".claude", "rules", "autopus", "file-size-limit.md"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(rendered),
		Content:         []byte(rendered),
	}, nil
}
