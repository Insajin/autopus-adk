package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	markerBegin = "<!-- AUTOPUS:BEGIN -->"
	markerEnd   = "<!-- AUTOPUS:END -->"
)

var markerRe = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(markerBegin) + `.*?` + regexp.QuoteMeta(markerEnd))

// injectMarkerSection creates or updates the AUTOPUS marker section in AGENTS.md.
func (a *Adapter) injectMarkerSection(cfg *config.HarnessConfig) (string, error) {
	agentsPath := filepath.Join(a.root, "AGENTS.md")

	var existing string
	if data, err := os.ReadFile(agentsPath); err == nil {
		existing = string(data)
	}

	sectionContent, err := a.engine.RenderString(agentsMDTemplate, cfg)
	if err != nil {
		return "", fmt.Errorf("AGENTS.md 템플릿 렌더링 실패: %w", err)
	}

	// Append inline agents section.
	agentsSection, err := renderAgentsSection()
	if err != nil {
		return "", fmt.Errorf("agents 섹션 렌더링 실패: %w", err)
	}
	sectionContent += agentsSection

	// Reference separate rule files instead of inlining.
	sectionContent += "\n## Rules\n\nSee .codex/rules/autopus/ for detailed guidelines.\n"

	newSection := markerBegin + "\n" + sectionContent + "\n" + markerEnd

	if strings.Contains(existing, markerBegin) && strings.Contains(existing, markerEnd) {
		return replaceMarkerSection(existing, newSection), nil
	}

	if existing == "" {
		return newSection + "\n", nil
	}
	return existing + "\n\n" + newSection + "\n", nil
}

func replaceMarkerSection(content, newSection string) string {
	return markerRe.ReplaceAllString(content, newSection)
}

func removeMarkerSection(content string) string {
	return strings.TrimSpace(markerRe.ReplaceAllString(content, "")) + "\n"
}
