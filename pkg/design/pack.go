package design

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SourceRef struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Reason string `json:"reason,omitempty"`
}

type PackOptions struct {
	ContextOptions Options
	MaxRefs        int
}

type Pack struct {
	Version        int              `json:"version"`
	DesignContext  PackContext      `json:"design_context"`
	TokenRefs      []SourceRef      `json:"token_refs,omitempty"`
	ComponentRefs  []SourceRef      `json:"component_refs,omitempty"`
	ScreenshotRefs []SourceRef      `json:"screenshot_refs,omitempty"`
	FigmaRefs      []FigmaRef       `json:"figma_refs,omitempty"`
	CodeConnect    CodeConnectAudit `json:"code_connect"`
	SetupGaps      []string         `json:"setup_gaps,omitempty"`
}

type PackContext struct {
	Found        bool     `json:"found"`
	SourcePath   string   `json:"source_path,omitempty"`
	BaselinePath string   `json:"baseline_path,omitempty"`
	SkipReason   string   `json:"skip_reason,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Diagnostics  []string `json:"diagnostics,omitempty"`
}

func BuildPack(root string, opts PackOptions) (Pack, error) {
	maxRefs := opts.MaxRefs
	if maxRefs <= 0 {
		maxRefs = 30
	}
	ctx, err := LoadContext(root, opts.ContextOptions)
	if err != nil {
		return Pack{}, err
	}
	audit, err := AuditFigma(root, maxRefs)
	if err != nil {
		return Pack{}, err
	}

	pack := Pack{
		Version:       1,
		DesignContext: contextToPackContext(ctx),
		FigmaRefs:     audit.FigmaRefs,
		CodeConnect:   audit.CodeConnect,
		SetupGaps:     append([]string{}, audit.SetupGaps...),
	}
	if !ctx.Found {
		pack.SetupGaps = appendMissing(pack.SetupGaps, "design_context_missing")
	}
	if err := collectPackRefs(root, maxRefs, &pack); err != nil {
		return Pack{}, err
	}
	if len(pack.TokenRefs) == 0 {
		pack.SetupGaps = appendMissing(pack.SetupGaps, "token_refs_missing")
	}
	if len(pack.ComponentRefs) == 0 {
		pack.SetupGaps = appendMissing(pack.SetupGaps, "component_refs_missing")
	}
	return pack, nil
}

func (p Pack) Markdown() string {
	var sb strings.Builder
	sb.WriteString("## Design Source Pack\n")
	if p.DesignContext.Found {
		fmt.Fprintf(&sb, "- Design context: %s\n", p.DesignContext.SourcePath)
		if p.DesignContext.BaselinePath != "" {
			fmt.Fprintf(&sb, "- Source of truth: %s\n", p.DesignContext.BaselinePath)
		}
	} else {
		fmt.Fprintf(&sb, "- Design context: skipped (%s)\n", p.DesignContext.SkipReason)
	}
	writeRefList(&sb, "Token/theme refs", p.TokenRefs)
	writeRefList(&sb, "Component refs", p.ComponentRefs)
	writeRefList(&sb, "Screenshot/golden refs", p.ScreenshotRefs)
	if len(p.FigmaRefs) == 0 {
		sb.WriteString("- Figma refs: none\n")
	} else {
		sb.WriteString("- Figma refs:\n")
		for _, ref := range p.FigmaRefs {
			fmt.Fprintf(&sb, "  - %s (%s, %s)\n", ref.SourcePath, ref.Kind, ref.URLHash)
		}
	}
	fmt.Fprintf(&sb, "- Code Connect: %s\n", p.CodeConnect.Status)
	if len(p.SetupGaps) > 0 {
		sb.WriteString("- Setup gaps:\n")
		for _, gap := range p.SetupGaps {
			fmt.Fprintf(&sb, "  - %s\n", gap)
		}
	}
	return sb.String()
}

func (p Pack) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func contextToPackContext(ctx Context) PackContext {
	out := PackContext{
		Found:        ctx.Found,
		SourcePath:   ctx.SourcePath,
		BaselinePath: ctx.BaselinePath,
		Summary:      ctx.Summary,
	}
	if !ctx.Found {
		out.SkipReason = string(ctx.SkipReason)
	}
	for _, diag := range ctx.Diagnostics {
		label := string(diag.Category)
		if diag.Path != "" {
			label = diag.Path + ":" + label
		}
		out.Diagnostics = append(out.Diagnostics, label)
	}
	return out
}

func collectPackRefs(root string, maxRefs int, pack *Pack) error {
	return walkDesignCandidateFiles(root, maxRefs*10, func(rel, _ string, _ os.FileInfo) error {
		switch {
		case isTokenRef(rel):
			pack.TokenRefs = appendLimited(pack.TokenRefs, SourceRef{Path: rel, Kind: "token_or_theme"}, maxRefs)
		case isComponentRef(rel):
			pack.ComponentRefs = appendLimited(pack.ComponentRefs, SourceRef{Path: rel, Kind: "component"}, maxRefs)
		case isScreenshotRef(rel):
			pack.ScreenshotRefs = appendLimited(pack.ScreenshotRefs, SourceRef{Path: rel, Kind: "screenshot_ref"}, maxRefs)
		}
		return nil
	})
}

func walkDesignCandidateFiles(root string, visitLimit int, fn func(rel, abs string, info os.FileInfo) error) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = evaluatedRoot
	}
	visited := 0
	return filepath.WalkDir(rootAbs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := relPath(rootAbs, path)
		if entry.IsDir() {
			if shouldSkipPackDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if visitLimit > 0 && visited >= visitLimit {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxLocalContextBytes {
			return nil
		}
		visited++
		return fn(rel, path, info)
	})
}

func shouldSkipPackDir(rel string) bool {
	clean := filepath.ToSlash(rel)
	if clean == "." {
		return false
	}
	base := strings.ToLower(filepath.Base(clean))
	switch base {
	case ".git", "node_modules", "vendor", "dist", "build", ".next", "coverage", "target":
		return true
	}
	return strings.HasPrefix(clean, ".autopus/runtime") ||
		strings.HasPrefix(clean, ".autopus/qa") ||
		strings.HasPrefix(clean, ".autopus/orchestra") ||
		strings.HasPrefix(clean, ".autopus/brainstorms") ||
		strings.HasPrefix(clean, ".autopus/design/imports")
}

func isTokenRef(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(lower)
	if strings.Contains(lower, "design-system") || strings.Contains(lower, "/tokens/") || strings.Contains(lower, "/theme/") {
		return true
	}
	return strings.Contains(base, "token") || strings.Contains(base, "theme") || strings.HasPrefix(base, "tailwind.config") || base == "globals.css"
}

func isComponentRef(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	ext := filepath.Ext(lower)
	if ext != ".tsx" && ext != ".jsx" {
		return false
	}
	return strings.Contains(lower, "/components/") || strings.Contains(lower, "/ui/") || strings.Contains(lower, "/primitives/")
}

func isScreenshotRef(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	ext := filepath.Ext(lower)
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return false
	}
	return strings.Contains(lower, "screenshot") || strings.Contains(lower, "snapshot") || strings.Contains(lower, "golden")
}

func appendLimited(refs []SourceRef, ref SourceRef, max int) []SourceRef {
	if max > 0 && len(refs) >= max {
		return refs
	}
	refs = append(refs, ref)
	sortRefs(refs)
	return refs
}

func appendMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func sortRefs(refs []SourceRef) {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Path < refs[j].Path
	})
}

func writeRefList(sb *strings.Builder, label string, refs []SourceRef) {
	if len(refs) == 0 {
		fmt.Fprintf(sb, "- %s: none\n", label)
		return
	}
	fmt.Fprintf(sb, "- %s:\n", label)
	for _, ref := range refs {
		fmt.Fprintf(sb, "  - %s\n", ref.Path)
	}
}
