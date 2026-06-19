package content

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

// parityArtifacts bundles the three artifacts compared by the workflow parity
// gate: the authoritative schema, the derived JS template string, and the
// human markdown contract.
type parityArtifacts struct {
	schema     workflow.Schema
	derivedJS  string
	markdownMD string
}

// checkWorkflowParity compares phase-id, retry, budget, and result-type sets
// across the schema, the derived JS template, and the markdown contract.
//
// Divergence rules (any one fails closed, naming the diverging element):
//   - a schema phase-id is absent as a string token in the JS template;
//   - a JS phase-id is absent from the schema;
//   - a schema phase-id is absent as a string token in the markdown;
//   - retry/budget/result-type for a schema phase-id is absent from the JS
//     template (each value is embedded as a token by the generator).
//
// The JS template is the generated surface, so the schema is the authority:
// every schema phase-id and its retry/budget/result-type tokens must appear in
// the derived JS, and every schema phase-id must appear in the markdown.
func checkWorkflowParity(a parityArtifacts) error {
	ids := a.schema.PhaseIDs()
	if len(ids) == 0 {
		return fmt.Errorf("parity drift: schema declares no phases")
	}

	jsIDs := extractPhaseIDsFromJS(a.derivedJS)

	// Every schema phase-id must be present in the JS phase set.
	for _, id := range ids {
		if !jsIDs[id] {
			return fmt.Errorf("parity drift: phase %q present in schema but absent in derived JS", id)
		}
	}
	// Every JS phase-id must be present in the schema (no extra JS phases).
	schemaIDset := make(map[string]bool, len(ids))
	for _, id := range ids {
		schemaIDset[id] = true
	}
	for id := range jsIDs {
		if !schemaIDset[id] {
			return fmt.Errorf("parity drift: phase %q present in derived JS but absent in schema", id)
		}
	}

	// Every schema phase-id must appear as a string token in the markdown.
	for _, id := range ids {
		if !strings.Contains(a.markdownMD, id) {
			return fmt.Errorf("parity drift: phase %q present in schema but absent in markdown", id)
		}
	}
	// Reverse: every phase-id declared as a markdown heading must exist in the
	// schema, so removing a phase from the schema is detected and named.
	for _, id := range extractPhaseIDsFromMarkdownHeadings(a.markdownMD) {
		if !schemaIDset[id] {
			return fmt.Errorf("parity drift: phase %q present in markdown but absent in schema", id)
		}
	}

	// retry/budget/result-type tokens must be present in the JS for each phase.
	for _, p := range a.schema.Phases {
		retryTok := fmt.Sprintf("retry: %d", p.Retry)
		budgetTok := fmt.Sprintf("budget: %d", p.Budget)
		if !strings.Contains(a.derivedJS, retryTok) {
			return fmt.Errorf("parity drift: phase %q retry value %d absent in derived JS", p.ID, p.Retry)
		}
		if !strings.Contains(a.derivedJS, budgetTok) {
			return fmt.Errorf("parity drift: phase %q budget value %d absent in derived JS", p.ID, p.Budget)
		}
		if p.ResultType != "" && !strings.Contains(a.derivedJS, p.ResultType) {
			return fmt.Errorf("parity drift: phase %q result-type %q absent in derived JS", p.ID, p.ResultType)
		}
	}

	return nil
}

// extractPhaseIDsFromJS collects phase-ids declared in the derived JS template
// via the deterministic markers the generator emits: phase('<id>') calls and
// {title:'<id>'} meta entries.
func extractPhaseIDsFromJS(js string) map[string]bool {
	found := map[string]bool{}
	collect := func(marker, suffix string) {
		rest := js
		for {
			idx := strings.Index(rest, marker)
			if idx < 0 {
				break
			}
			rest = rest[idx+len(marker):]
			end := strings.Index(rest, suffix)
			if end < 0 {
				break
			}
			id := rest[:end]
			if id != "" {
				found[id] = true
			}
			rest = rest[end+len(suffix):]
		}
	}
	collect("phase('", "'")
	collect("{title:'", "'")
	return found
}

// extractPhaseIDsFromMarkdownHeadings collects phase-ids declared as level-3
// markdown headings ("### <id>") whose text is a single snake_case token. The
// route_a.md contract declares each phase under such a heading, giving a
// reliable phase set independent of the schema.
func extractPhaseIDsFromMarkdownHeadings(md string) []string {
	var ids []string
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "### ") {
			continue
		}
		token := strings.TrimSpace(strings.TrimPrefix(line, "### "))
		if isPhaseIDToken(token) {
			ids = append(ids, token)
		}
	}
	return ids
}

// isPhaseIDToken reports whether s is a single lowercase snake_case identifier.
func isPhaseIDToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '_' {
			return false
		}
	}
	return true
}
