package spec

import (
	"fmt"
	"strings"
)

// BuildReviewPrompt constructs a review prompt from a SPEC document and code context.
// opts.Mode controls whether a discover (open-ended) or verify (checklist) prompt is generated.
// The full spec.md content is included to prevent false positives from parser truncation
// (e.g., multi-line requirements with SQL schemas or itemized lists after the EARS header).
func BuildReviewPrompt(doc *SpecDocument, codeContext string, opts ReviewPromptOptions) string {
	var sb strings.Builder

	sb.WriteString("You are reviewing a SPEC document for correctness, completeness, and feasibility.\n\n")
	fmt.Fprintf(&sb, "## SPEC: %s — %s\n\n", doc.ID, doc.Title)

	// Include full spec.md content to avoid parser-induced truncation.
	// The EARS parser only captures single-line descriptions, missing multi-line
	// requirement bodies (SQL schemas, itemized lists, etc.).
	if doc.RawContent != "" {
		sb.WriteString("### Full SPEC Document\n\n")
		sb.WriteString(doc.RawContent)
		sb.WriteString("\n\n")
	} else {
		// Fallback: use parsed requirements when raw content is unavailable.
		if len(doc.Requirements) > 0 {
			sb.WriteString("### Requirements\n\n")
			for _, req := range doc.Requirements {
				fmt.Fprintf(&sb, "- **%s** [%s]: %s\n", req.ID, req.Type, req.Description)
			}
			sb.WriteString("\n")
		}
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

// Context collection functions are in context_collect.go.
