package design

import (
	"encoding/json"
	"fmt"
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
	DocsProviders  []string
}

type Pack struct {
	Version        int              `json:"version"`
	DesignContext  PackContext      `json:"design_context"`
	TokenRefs      []SourceRef      `json:"token_refs,omitempty"`
	ComponentRefs  []SourceRef      `json:"component_refs,omitempty"`
	ScreenshotRefs []SourceRef      `json:"screenshot_refs,omitempty"`
	FigmaRefs      []FigmaRef       `json:"figma_refs,omitempty"`
	CodeConnect    CodeConnectAudit `json:"code_connect"`
	DesignDocs     DesignSystemDocs `json:"design_system_docs"`
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
	if err := collectDeclaredSourceRefs(root, maxRefs, &pack); err != nil {
		return Pack{}, err
	}
	if err := collectPackRefs(root, maxRefs, &pack); err != nil {
		return Pack{}, err
	}
	docs, err := BuildDesignSystemDocs(root, DesignSystemDocsOptions{
		Providers: opts.DocsProviders,
		MaxRefs:   maxRefs,
	})
	if err != nil {
		return Pack{}, err
	}
	pack.DesignDocs = docs
	pack.SetupGaps = appendMissingMany(pack.SetupGaps, docs.SetupGaps)
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
	if len(p.DesignDocs.Providers) == 0 {
		sb.WriteString("- Design-system docs: none detected\n")
	} else {
		sb.WriteString("- Design-system docs:\n")
		for _, provider := range p.DesignDocs.Providers {
			fmt.Fprintf(&sb, "  - %s (%s)\n", provider.Name, provider.Status)
			for _, cmd := range provider.Preflight {
				fmt.Fprintf(&sb, "    - `%s`\n", cmd)
			}
		}
	}
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

func appendMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendMissingMany(values []string, additions []string) []string {
	for _, addition := range additions {
		values = appendMissing(values, addition)
	}
	return values
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
