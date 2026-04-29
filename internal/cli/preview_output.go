package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	setupdocs "github.com/insajin/autopus-adk/pkg/setup"
)

type previewItem struct {
	Path     string
	Kind     string
	Category string
	Reason   string
	Scope    string
}

func normalizePreviewItems(items []previewItem) []previewItem {
	sorted := append([]previewItem(nil), items...)
	for i := range sorted {
		sorted[i].Path = filepath.ToSlash(sorted[i].Path)
		if sorted[i].Category == "" {
			sorted[i].Category = previewCategoryForPath(sorted[i].Path)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Category != sorted[j].Category {
			return sorted[i].Category < sorted[j].Category
		}
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Kind < sorted[j].Kind
	})
	return sorted
}

func printPreview(out io.Writer, title, hint string, items []previewItem) {
	fmt.Fprintf(out, "Preview: %s\n", title)
	if hint != "" {
		fmt.Fprintf(out, "%s\n", hint)
	}

	sorted := normalizePreviewItems(items)
	if len(sorted) == 0 {
		fmt.Fprintln(out, "No changes would be applied.")
		return
	}

	for _, item := range sorted {
		scope := ""
		if item.Scope != "" {
			scope = " (" + item.Scope + ")"
		}
		fmt.Fprintf(out, "- [%s] %s %s%s", item.Category, item.Kind, item.Path, scope)
		if item.Reason != "" {
			fmt.Fprintf(out, " — %s", item.Reason)
		}
		fmt.Fprintln(out)
	}
}

func previewCategoryForPath(path string) string {
	path = filepath.ToSlash(path)
	switch {
	case path == "autopus.yaml" || path == "config.toml" || path == ".codex/config.toml" || path == "opencode.json" || path == ".mcp.json":
		return "config"
	case strings.HasSuffix(path, "-manifest.json") ||
		strings.HasSuffix(path, ".meta.yaml") ||
		strings.HasPrefix(path, ".autopus/backup/") ||
		strings.HasPrefix(path, ".autopus/design/imports/"):
		return "runtime_state"
	case strings.HasPrefix(path, ".claude/") ||
		strings.HasPrefix(path, ".codex/") ||
		strings.HasPrefix(path, ".gemini/") ||
		strings.HasPrefix(path, ".opencode/") ||
		strings.HasPrefix(path, ".agents/") ||
		strings.HasPrefix(path, ".git/hooks/"):
		return "generated_surface"
	default:
		return "tracked_docs"
	}
}

func describeRepoAwareHint(dir string) string {
	info, err := setupdocs.Scan(dir)
	if err != nil {
		return ""
	}
	if info.MultiRepo != nil && info.MultiRepo.IsMultiRepo {
		role := "workspace root"
		for _, component := range info.MultiRepo.Components {
			if component.Path == "." && component.Role != "" {
				role = component.Role
				break
			}
		}
		return fmt.Sprintf("Repo-aware hint: multi-repo workspace detected (%s). Verify you are applying bootstrap changes in the owning repo/source-of-truth location.", role)
	}
	if len(info.Workspaces) > 0 {
		return fmt.Sprintf("Repo-aware hint: workspace root detected (%d workspace(s)). Preview targets the current repo root.", len(info.Workspaces))
	}
	return ""
}
