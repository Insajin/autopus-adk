package design

import (
	"os"
	"path/filepath"
	"strings"
)

// @AX:ANCHOR [AUTO]: Public design context loader used by design CLI, review prompt, and verify flow.
// @AX:REASON: It owns safe source resolution, skip semantics, and summary construction before prompt injection.
func LoadContext(root string, opts Options) (Context, error) {
	if !opts.Enabled {
		return Context{SkipReason: SkipDisabled}, nil
	}
	maxLines := opts.MaxContextLines
	if maxLines <= 0 {
		maxLines = DefaultMaxContextLines
	}

	var diagnostics []Diagnostic
	for _, configured := range opts.Paths {
		ctx, ok, diag, err := loadSingle(root, configured, "", maxLines)
		if err != nil {
			return Context{}, err
		}
		if diag != nil {
			diagnostics = append(diagnostics, *diag)
			continue
		}
		if ok {
			ctx.Diagnostics = diagnostics
			return ctx, nil
		}
	}

	designPath := "DESIGN.md"
	designAbs, diag := ResolveDesignPath(root, designPath)
	if diag == nil {
		data, diag, err := readLocalDesignFile(root, designAbs)
		if err != nil {
			return Context{}, err
		}
		if diag != nil {
			diagnostics = append(diagnostics, *diag)
			data = nil
		}
		if data != nil {
			for _, baseline := range parseSourceOfTruth(string(data)) {
				bctx, ok, bdiag, err := loadSingle(root, baseline, relPath(root, designAbs), maxLines)
				if err != nil {
					return Context{}, err
				}
				if bdiag != nil {
					diagnostics = append(diagnostics, *bdiag)
					continue
				}
				if ok {
					bctx.Diagnostics = diagnostics
					return bctx, nil
				}
			}
			return Context{
				Found:       true,
				SourcePath:  relPath(root, designAbs),
				Summary:     BuildSummary(string(data), maxLines),
				Diagnostics: diagnostics,
			}, nil
		}
	}
	if diag != nil && diag.Category != CategoryMissingPath {
		diagnostics = append(diagnostics, *diag)
	}

	// @AX:NOTE [AUTO]: Fallback path is the canonical project design knowledge location.
	for _, fallback := range []string{filepath.ToSlash(filepath.Join(".autopus", "project", "design.md"))} {
		ctx, ok, fdiag, err := loadSingle(root, fallback, "", maxLines)
		if err != nil {
			return Context{}, err
		}
		if fdiag != nil {
			if fdiag.Category != CategoryMissingPath {
				diagnostics = append(diagnostics, *fdiag)
			}
			continue
		}
		if ok {
			ctx.Diagnostics = diagnostics
			return ctx, nil
		}
	}

	return Context{Diagnostics: diagnostics, SkipReason: SkipMissing}, nil
}

func loadSingle(root, path, declaredBy string, maxLines int) (Context, bool, *Diagnostic, error) {
	abs, diag := ResolveDesignPath(root, path)
	if diag != nil {
		return Context{}, false, diag, nil
	}
	data, diag, err := readLocalDesignFile(root, abs)
	if err != nil {
		return Context{}, false, nil, err
	}
	if diag != nil {
		return Context{}, false, diag, nil
	}
	ctx := Context{
		Found:      true,
		SourcePath: relPath(root, abs),
		Summary:    BuildSummary(string(data), maxLines),
	}
	if declaredBy != "" {
		ctx.SourcePath = declaredBy
		ctx.BaselinePath = relPath(root, abs)
	}
	return ctx, true, nil, nil
}

func readLocalDesignFile(root, abs string) ([]byte, *Diagnostic, error) {
	info, err := os.Stat(abs)
	if err != nil {
		return nil, nil, err
	}
	if info.Size() > MaxLocalContextBytes {
		return nil, &Diagnostic{Path: relPath(root, abs), Category: CategoryBodyTooLarge, Message: "design context exceeds local byte limit"}, nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, nil, err
	}
	sanitized := sanitizeImportContent(string(data))
	if sanitized.Rejected {
		return nil, &Diagnostic{Path: relPath(root, abs), Category: CategoryUnsafeContent, Message: strings.Join(sanitized.Reasons, ",")}, nil
	}
	return []byte(sanitized.Content), nil, nil
}
