package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildReviewPrompt constructs a review prompt from a SPEC document and code context.
// opts.Mode controls whether a discover (open-ended) or verify (checklist) prompt is generated.
func BuildReviewPrompt(doc *SpecDocument, codeContext string, opts ReviewPromptOptions) string {
	var sb strings.Builder

	sb.WriteString("You are reviewing a SPEC document for correctness, completeness, and feasibility.\n\n")
	fmt.Fprintf(&sb, "## SPEC: %s — %s\n\n", doc.ID, doc.Title)

	if len(doc.Requirements) > 0 {
		sb.WriteString("### Requirements\n\n")
		for _, req := range doc.Requirements {
			fmt.Fprintf(&sb, "- **%s** [%s]: %s\n", req.ID, req.Type, req.Description)
		}
		sb.WriteString("\n")
	}

	if len(doc.AcceptanceCriteria) > 0 {
		sb.WriteString("### Acceptance Criteria\n\n")
		for _, ac := range doc.AcceptanceCriteria {
			fmt.Fprintf(&sb, "- %s: %s\n", ac.ID, ac.Description)
		}
		sb.WriteString("\n")
	}

	if codeContext != "" {
		sb.WriteString("### Existing Code Context\n\n")
		sb.WriteString("```\n")
		sb.WriteString(codeContext)
		sb.WriteString("\n```\n\n")
	}

	if opts.Mode == ReviewModeVerify || len(opts.PriorFindings) > 0 {
		buildVerifyInstructions(&sb, opts.PriorFindings)
	} else {
		buildDiscoverInstructions(&sb, opts.StaticFindings)
	}

	return sb.String()
}

// buildVerifyInstructions writes checklist-based instructions for verify mode.
func buildVerifyInstructions(sb *strings.Builder, priorFindings []ReviewFinding) {
	sb.WriteString("### Instructions (Verify Mode)\n\n")
	sb.WriteString("For each finding below, report its current status.\n\n")

	if len(priorFindings) > 0 {
		sb.WriteString("#### Prior Findings Checklist\n\n")
		for _, f := range priorFindings {
			fmt.Fprintf(sb, "- %s [%s] %s: %s\n", f.ID, f.Severity, f.ScopeRef, f.Description)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Respond with:\n")
	sb.WriteString("1. VERDICT: PASS, REVISE, or REJECT\n")
	sb.WriteString("2. For each prior finding, write: FINDING_STATUS: F-{id} | {open|resolved|regressed} | {reason}\n")
	sb.WriteString("3. Report any regression or newly broken behavior caused by fixes, even if not in the checklist.\n")
	sb.WriteString("   For new critical/security issues: FINDING: [severity] [category] [scope_ref] description\n")
}

// buildDiscoverInstructions writes open-ended instructions for discover mode.
func buildDiscoverInstructions(sb *strings.Builder, staticFindings []ReviewFinding) {
	sb.WriteString("### Instructions\n\n")

	if len(staticFindings) > 0 {
		sb.WriteString("#### Already Discovered Static Analysis Issues\n\n")
		for _, f := range staticFindings {
			fmt.Fprintf(sb, "- [%s] %s: %s\n", f.Severity, f.ScopeRef, f.Description)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Review the SPEC and respond with:\n")
	sb.WriteString("1. VERDICT: PASS, REVISE, or REJECT\n")
	sb.WriteString("2. For each issue found, write: FINDING: [severity] [category] [scope_ref] description\n")
	sb.WriteString("   Severity levels: critical, major, minor, suggestion\n")
	sb.WriteString("   Category: correctness, completeness, feasibility, style, security\n")
	sb.WriteString("3. Provide reasoning for your verdict.\n")
}

// skipDirs are directories excluded from context collection.
var skipDirs = map[string]bool{
	".git":         true,
	".cache":       true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	".autopus":     true,
	".claude":      true,
	"templates":    true,
	"__pycache__":  true,
	".next":        true,
	".nuxt":        true,
	"build":        true,
	"coverage":     true,
	".svelte-kit":  true,
}

// CollectContext recursively reads source files from a directory up to maxLines total.
func CollectContext(dir string, maxLines int) (string, error) {
	var sb strings.Builder
	lineCount := 0

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if lineCount >= maxLines {
			return filepath.SkipAll
		}
		if !isSourceFile(d.Name()) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)
		if relPath == "" {
			relPath = d.Name()
		}

		lines := strings.Split(string(content), "\n")
		remaining := maxLines - lineCount
		if remaining <= 0 {
			return filepath.SkipAll
		}

		fmt.Fprintf(&sb, "--- %s ---\n", relPath)
		lineCount++

		end := min(len(lines), remaining)
		for _, line := range lines[:end] {
			sb.WriteString(line)
			sb.WriteString("\n")
			lineCount++
		}
		sb.WriteString("\n")
		lineCount++
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}

	return sb.String(), nil
}

// isSourceFile returns true if the filename is a recognized source file.
func isSourceFile(name string) bool {
	exts := []string{".go", ".py", ".ts", ".js", ".rs", ".java", ".rb"}
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}
