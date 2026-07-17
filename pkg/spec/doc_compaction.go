package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultAuxTotalBudgetLines is the generous default TOTAL line budget shared
// across the three auxiliary documents (plan.md, research.md, acceptance.md).
// It is sized so a typical SPEC document set injects in full at 100% coverage;
// the motivating SPEC-DESKTOP-DEVICE-SETUP-001 set is 358+429+404 = 1191 lines,
// comfortably below this budget. Structure-preserving compaction only engages
// when the combined document total exceeds this budget (REQ-RINT-FULL-02).
// @AX:NOTE: [AUTO] magic constant — 4000-line total budget is sized from the SPEC-DESKTOP-DEVICE-SETUP-001 motivating case (1191 lines); raising it changes when compaction engages (REQ-RINT-FULL-02)
const DefaultAuxTotalBudgetLines = 4000

// auxDocNames is the ordered set of auxiliary documents injected into the
// review prompt. Order is fixed so prompts and coverage records are reproducible.
var auxDocNames = []string{"plan.md", "research.md", "acceptance.md"}

// auxDocSections maps each auxiliary document to its prompt section header.
var auxDocSections = map[string]string{
	"plan.md":       "### Plan Document",
	"research.md":   "### Research Document",
	"acceptance.md": "### Acceptance Criteria Document",
}

// tailCriticalHeadings names section headings whose content must survive
// compaction because reviewers depend on them (self-verification evidence,
// traceability, completion debt). They typically live at the document tail —
// exactly where the old head-only trim silently discarded them.
var tailCriticalHeadings = []string{
	"Self-Verify Summary",
	"Traceability Matrix",
	"Reviewer Brief",
	"Completion Debt",
	"Evolution Ideas",
	"Open Issues",
	"Revision", // "Revision N closure" blocks
}

// auxDocPlan is the computed injection plan for a single auxiliary document.
type auxDocPlan struct {
	name     string
	header   string
	excerpt  string
	coverage DocCoverage
}

// ResolveAuxTotalBudget maps the historical per-document line cap
// (doc_context_max_lines) onto the total aux-doc budget. The old small cap can
// never shrink the budget below the generous default, so typical SPECs inject
// in full; an operator raising the cap above the default is honored.
func ResolveAuxTotalBudget(configured int) int {
	if configured > DefaultAuxTotalBudgetLines {
		return configured
	}
	return DefaultAuxTotalBudgetLines
}

// planAuxDocInjection reads the auxiliary documents present in specDir and
// computes, under totalBudget lines, the excerpt and coverage for each. Missing
// files are skipped. Every present document injects in full while the combined
// total fits the budget; otherwise budget is allocated smallest-document-first
// so as many documents as possible stay whole and only the largest is
// compacted (structure-preserving, tail-critical sections retained).
func planAuxDocInjection(specDir string, totalBudget int) []auxDocPlan {
	type loadedDoc struct {
		name    string
		content string
		total   int
	}
	var present []loadedDoc
	for _, name := range auxDocNames {
		data, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			continue // file does not exist — not an error
		}
		content := string(data)
		present = append(present, loadedDoc{name: name, content: content, total: countLines(content)})
	}
	if len(present) == 0 {
		return nil
	}

	combined := 0
	for _, d := range present {
		combined += d.total
	}

	alloc := make(map[string]int, len(present))
	if combined <= totalBudget {
		for _, d := range present {
			alloc[d.name] = d.total // full injection
		}
	} else {
		order := make([]int, len(present))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool {
			return present[order[a]].total < present[order[b]].total
		})
		remaining := totalBudget
		for _, i := range order {
			d := present[i]
			if d.total <= remaining {
				alloc[d.name] = d.total
				remaining -= d.total
				continue
			}
			alloc[d.name] = remaining // compact the largest doc to what remains
			remaining = 0
		}
	}

	plans := make([]auxDocPlan, 0, len(present))
	for _, d := range present {
		excerpt, injected := compactDocToBudget(d.content, alloc[d.name])
		plans = append(plans, auxDocPlan{
			name:     d.name,
			header:   auxDocSections[d.name],
			excerpt:  excerpt,
			coverage: ComputeCoverage(d.name, injected, d.total),
		})
	}
	return plans
}

// compactDocToBudget returns the injected excerpt and injected line count for
// content under a line budget. Within budget the content is returned verbatim.
// Over budget, tail-critical sections are preserved instead of discarding the
// document tail (REQ-RINT-STRUCT-03); a source-safe omission notice marks the
// gap so reviewers do not read it as a document defect.
func compactDocToBudget(content string, budget int) (string, int) {
	lines := strings.Split(content, "\n")
	total := len(lines)
	if total <= budget {
		return content, total // full injection (budget >= total, or empty doc)
	}
	if budget <= 0 {
		return omissionNotice(total), 0
	}

	criticalStart := firstTailCriticalIndex(lines)
	if criticalStart < 0 {
		// No recognized tail-critical section: keep the head within budget.
		head := strings.Join(lines[:budget], "\n")
		return head + "\n" + omissionNotice(total-budget), budget
	}

	tail := lines[criticalStart:]
	if len(tail) >= budget {
		// The critical tail alone exceeds the budget: keep the final `budget`
		// lines so the most tail-critical content survives, dropping the head.
		kept := strings.Join(lines[total-budget:], "\n")
		return omissionNotice(total-budget) + "\n" + kept, budget
	}

	headBudget := budget - len(tail)
	head := strings.Join(lines[:headBudget], "\n")
	tailText := strings.Join(tail, "\n")
	return head + "\n" + omissionNotice(total-budget) + "\n" + tailText, budget
}

// firstTailCriticalIndex returns the index of the earliest markdown heading
// line whose text names a tail-critical section, or -1 when none is present.
func firstTailCriticalIndex(lines []string) int {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		for _, critical := range tailCriticalHeadings {
			if strings.Contains(heading, critical) {
				return i
			}
		}
	}
	return -1
}

// omissionNotice formats the source-safe notice appended where lines were
// dropped during compaction. It reuses the shared trim-notice wording so the
// existing reviewer instructions about the notice still apply.
func omissionNotice(omitted int) string {
	return fmt.Sprintf(trimNoticeFormat, omitted)
}

// countLines counts lines the same way compaction splits them, so coverage
// totals and injected counts stay consistent.
func countLines(content string) int {
	return len(strings.Split(content, "\n"))
}
