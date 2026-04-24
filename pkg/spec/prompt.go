package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultDocContextMaxLines = 200

// BuildReviewPrompt constructs a review prompt from a SPEC document and code context.
// specDir (optional variadic): path to the spec directory for loading plan.md/research.md/acceptance.md.
// opts.Mode controls whether a discover (open-ended) or verify (checklist) prompt is generated.
// The full spec.md content is included to prevent false positives from parser truncation
// (e.g., multi-line requirements with SQL schemas or itemized lists after the EARS header).
func BuildReviewPrompt(doc *SpecDocument, codeContext string, opts ReviewPromptOptions, specDir ...string) string {
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

	// Inject auxiliary documentation context (plan.md, research.md, acceptance.md).
	dir := opts.SpecDir
	if len(specDir) > 0 && specDir[0] != "" {
		dir = specDir[0]
	}
	if dir != "" {
		maxLines := opts.DocContextMaxLines
		if maxLines <= 0 {
			maxLines = defaultDocContextMaxLines
		}
		injectAuxDocs(&sb, dir, maxLines)
	}

	if codeContext != "" {
		sb.WriteString("### Existing Code Context\n\n")
		sb.WriteString("```\n")
		sb.WriteString(codeContext)
		sb.WriteString("\n```\n\n")
	}

	checklistIncluded := false
	maxLines := opts.DocContextMaxLines
	if maxLines <= 0 {
		maxLines = defaultDocContextMaxLines
	}
	if checklistBody, checklistPath, err := loadChecklistForPrompt(opts); err != nil {
		fmt.Fprintf(os.Stderr, "경고: 체크리스트 로드 실패 (%s): %v\n", checklistPath, err)
	} else {
		InjectChecklistSection(&sb, checklistBody, maxLines)
		checklistIncluded = true
	}

	if opts.Mode == ReviewModeVerify || len(opts.PriorFindings) > 0 {
		buildVerifyInstructions(&sb, opts.PriorFindings, opts.PassCriteria, checklistIncluded)
	} else {
		buildDiscoverInstructions(&sb, opts.StaticFindings, opts.PassCriteria, checklistIncluded)
	}

	return sb.String()
}

// injectAuxDocs injects plan.md, research.md, and acceptance.md into the prompt.
// Missing files are silently skipped. Each file is trimmed to maxLines.
func injectAuxDocs(sb *strings.Builder, specDir string, maxLines int) {
	docs := []struct {
		name string
	}{
		{"plan.md"},
		{"research.md"},
		{"acceptance.md"},
	}

	sectionNames := map[string]string{
		"plan.md":       "### Plan Document",
		"research.md":   "### Research Document",
		"acceptance.md": "### Acceptance Criteria Document",
	}
	for _, d := range docs {
		path := filepath.Join(specDir, d.name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // file does not exist — not an error
		}
		content := trimToLines(string(data), maxLines)
		header := sectionNames[d.name]
		sb.WriteString(header)
		sb.WriteString("\n\n")
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}
}

// trimToLines truncates content to maxLines lines.
// Appends a trim notice when truncation occurs.
func trimToLines(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	trimmed := strings.Join(lines[:maxLines], "\n")
	extra := len(lines) - maxLines
	return fmt.Sprintf("%s\n... (trimmed %d more lines)", trimmed, extra)
}

// buildVerifyInstructions writes checklist-based instructions for verify mode.
func buildVerifyInstructions(sb *strings.Builder, priorFindings []ReviewFinding, passCriteria string, checklistIncluded bool) {
	sb.WriteString("### Verdict Decision Rules\n\n")
	writeVerdictRules(sb, passCriteria)
	sb.WriteString("\n")

	if checklistIncluded {
		sb.WriteString("### Checklist Response Format\n\n")
		writeChecklistExamples(sb)
		sb.WriteString("\n")
	}

	sb.WriteString("### Finding Format Examples\n\n")
	writeFindingExamples(sb)
	sb.WriteString("\n")

	sb.WriteString("### Instructions (Verify Mode)\n\n")
	sb.WriteString("For each finding below, report its current status.\n\n")
	sb.WriteString("Do NOT stop early, narrow the scope on your own, or replace the requested output with progress notes.\n")
	sb.WriteString("Responses that omit the required VERDICT/FINDING_STATUS lines are treated as malformed review output.\n\n")

	if len(priorFindings) > 0 {
		sb.WriteString("#### Prior Findings Checklist\n\n")
		for _, f := range priorFindings {
			fmt.Fprintf(sb, "- %s [%s] %s: %s\n", f.ID, f.Severity, f.ScopeRef, f.Description)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Respond with:\n")
	sb.WriteString("1. VERDICT: PASS, REVISE, or REJECT\n")
	if checklistIncluded {
		sb.WriteString("2. For each quality checklist item, write: CHECKLIST: <항목 ID> | PASS or CHECKLIST: <항목 ID> | FAIL | <reason>\n")
		sb.WriteString("3. For each prior finding, write: FINDING_STATUS: F-{id} | {open|resolved|regressed} | {reason}\n")
		sb.WriteString("4. Report any regression or newly broken behavior caused by fixes, even if not in the checklist.\n")
	} else {
		sb.WriteString("2. For each prior finding, write: FINDING_STATUS: F-{id} | {open|resolved|regressed} | {reason}\n")
		sb.WriteString("3. Report any regression or newly broken behavior caused by fixes, even if not in the checklist.\n")
	}
	sb.WriteString("   For new critical/security issues: FINDING: [severity] [category] [scope_ref] description\n")
}

// buildDiscoverInstructions writes open-ended instructions for discover mode.
func buildDiscoverInstructions(sb *strings.Builder, staticFindings []ReviewFinding, passCriteria string, checklistIncluded bool) {
	sb.WriteString("### Verdict Decision Rules\n\n")
	writeVerdictRules(sb, passCriteria)
	sb.WriteString("\n")

	if checklistIncluded {
		sb.WriteString("### Checklist Response Format\n\n")
		writeChecklistExamples(sb)
		sb.WriteString("\n")
	}

	sb.WriteString("### Finding Format Examples\n\n")
	writeFindingExamples(sb)
	sb.WriteString("\n")

	sb.WriteString("### Instructions\n\n")
	sb.WriteString("Do NOT stop early, narrow the scope on your own, or replace the requested output with progress notes.\n")
	sb.WriteString("Responses that omit the required VERDICT/FINDING lines are treated as malformed review output.\n\n")

	if len(staticFindings) > 0 {
		sb.WriteString("#### Already Discovered Static Analysis Issues\n\n")
		for _, f := range staticFindings {
			fmt.Fprintf(sb, "- [%s] %s: %s\n", f.Severity, f.ScopeRef, f.Description)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Review the SPEC and respond with:\n")
	sb.WriteString("1. VERDICT: PASS, REVISE, or REJECT\n")
	if checklistIncluded {
		sb.WriteString("2. For each quality checklist item, write: CHECKLIST: <항목 ID> | PASS or CHECKLIST: <항목 ID> | FAIL | <reason>\n")
		sb.WriteString("3. For each issue found, write: FINDING: [severity] [category] [scope_ref] description\n")
		sb.WriteString("4. Provide reasoning for your verdict.\n")
	} else {
		sb.WriteString("2. For each issue found, write: FINDING: [severity] [category] [scope_ref] description\n")
		sb.WriteString("3. Provide reasoning for your verdict.\n")
	}
	sb.WriteString("   Severity levels: critical, major, minor, suggestion\n")
	sb.WriteString("   Category: correctness, completeness, feasibility, style, security\n")
}

// writeVerdictRules writes the default or custom verdict decision rules.
func writeVerdictRules(sb *strings.Builder, passCriteria string) {
	if passCriteria != "" {
		sb.WriteString(passCriteria)
		sb.WriteString("\n")
		return
	}
	sb.WriteString("Apply the following rules to determine your VERDICT:\n")
	sb.WriteString("- PASS: critical == 0 AND security == 0 AND major <= 2\n")
	sb.WriteString("- REJECT: critical > 0 OR security > 0\n")
	sb.WriteString("- REVISE: otherwise (major > 2, or unresolved issues requiring author attention)\n")
}

// writeFindingExamples writes positive and negative few-shot examples for the FINDING format.
func writeFindingExamples(sb *strings.Builder) {
	sb.WriteString("Use the structured FINDING format exactly as shown:\n\n")
	sb.WriteString("GOOD (structured format — use this):\n")
	sb.WriteString("  FINDING: [major] [correctness] pkg/foo/bar.go:42 The retry logic does not handle timeout errors.\n")
	sb.WriteString("  FINDING: [critical] [security] pkg/auth/handler.go:88 JWT secret is logged in plaintext.\n\n")
	sb.WriteString("AVOID (legacy format — do NOT use this):\n")
	sb.WriteString("  FINDING: [major] The retry logic is broken.\n")
}

// Context collection functions are in context_collect.go.
