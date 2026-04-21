package cli

import (
	"path/filepath"
	"strings"

	setupdocs "github.com/insajin/autopus-adk/pkg/setup"
)

func setupPreviewItems(plan *setupdocs.ChangePlan) []previewItem {
	items := make([]previewItem, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		items = append(items, previewItem{
			Path:     change.Path,
			Kind:     string(change.Action),
			Category: string(change.Class),
			Reason:   change.Reason,
		})
	}
	return items
}

func setupPreviewHint(plan *setupdocs.ChangePlan) string {
	if plan == nil || len(plan.WorkspaceHints) == 0 {
		return ""
	}

	messages := make([]string, 0, len(plan.WorkspaceHints))
	for _, hint := range plan.WorkspaceHints {
		message := formatWorkspaceHint(hint)
		if strings.TrimSpace(message) == "" {
			continue
		}
		messages = append(messages, message)
	}
	return strings.Join(messages, " | ")
}

func formatWorkspaceHint(hint setupdocs.WorkspaceHint) string {
	parts := make([]string, 0, 3)
	if msg := strings.TrimSpace(hint.Message); msg != "" {
		parts = append(parts, msg)
	}
	if repo := strings.TrimSpace(hint.Repo); repo != "" {
		parts = append(parts, "repo: "+repo)
	}
	if source := strings.TrimSpace(hint.SourceOfTruth); source != "" {
		parts = append(parts, "source-of-truth: "+filepath.ToSlash(source))
	}
	return strings.Join(parts, "; ")
}
