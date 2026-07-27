package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/internal/cli/tui"
)

// Context-weight soft caps drive the non-blocking doctor guard (REQ-CLD-007).
// They sit above the per-document compression targets in ContextLoadSet so the
// guard fires only on genuine re-bloat, not on a document resting slightly over
// its rotation target.
const (
	// contextWeightTotalWarnBytes is the soft cap for the combined byte size of
	// the present context catalog documents. Above it the doctor warns. The 20000B
	// headroom above the 100000B rotation target absorbs normal churn.
	contextWeightTotalWarnBytes = 120000

	// contextWeightPerDocWarnBytes is the soft cap for any single context catalog
	// document, evaluated independently of the combined total.
	contextWeightPerDocWarnBytes = 20000
)

// contextLoadDoc is one profile-eligible context catalog entry: its path
// relative to the workspace root and its per-document rotation cap
// (REQ-CLD-005). The historical type name is kept for compatibility.
type contextLoadDoc struct {
	Name    string
	RelPath string
	Cap     int
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONTEXT-ENGINEERING-001: preserve the exported context catalog names, order, and rotation caps.
// @AX:REASON [AUTO]: doctor measurement and contract tests consume this table directly as the context-weight identity.
// ContextLoadSet enumerates the profile-eligible context catalog in a
// deterministic order together with each document's per-document rotation cap
// (REQ-CLD-005). It does not mean every document loads in every session. The
// caps sum to 100000 bytes, the combined rotation target protected by the
// context-weight guard. A slice keeps doctor output reproducible.
var ContextLoadSet = []contextLoadDoc{
	{Name: "product.md", RelPath: filepath.Join(".autopus", "project", "product.md"), Cap: 18000},
	{Name: "ARCHITECTURE.md", RelPath: "ARCHITECTURE.md", Cap: 16000},
	{Name: "scenarios.md", RelPath: filepath.Join(".autopus", "project", "scenarios.md"), Cap: 20000},
	{Name: "workspace.md", RelPath: filepath.Join(".autopus", "project", "workspace.md"), Cap: 12000},
	{Name: "tech.md", RelPath: filepath.Join(".autopus", "project", "tech.md"), Cap: 10000},
	{Name: "structure.md", RelPath: filepath.Join(".autopus", "project", "structure.md"), Cap: 18000},
	{Name: "canary.md", RelPath: filepath.Join(".autopus", "project", "canary.md"), Cap: 6000},
}

// contextDocWeight is the measured byte size of one context catalog document.
type contextDocWeight struct {
	Name    string
	Bytes   int
	Present bool
	OverCap bool
}

// contextWeightReport aggregates the context catalog measurement and guard verdict.
type contextWeightReport struct {
	Docs         []contextDocWeight
	TotalBytes   int
	PresentCount int
	OverTotal    bool
}

// warned reports whether either soft cap is exceeded (REQ-CLD-007 OR branches).
func (r contextWeightReport) warned() bool {
	if r.OverTotal {
		return true
	}
	for _, d := range r.Docs {
		if d.OverCap {
			return true
		}
	}
	return false
}

// measureContextWeight sizes the present context catalog documents under dir.
// Absent documents contribute nothing and are marked not present, so the guard
// stays silent in repositories without the meta-workspace context catalog.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONTEXT-ENGINEERING-001: keep context catalog measurement centralized.
// @AX:REASON [AUTO]: text, JSON, and regression-test consumers share this deterministic measurement boundary.
func measureContextWeight(dir string) contextWeightReport {
	rep := contextWeightReport{Docs: make([]contextDocWeight, 0, len(ContextLoadSet))}
	for _, doc := range ContextLoadSet {
		d := contextDocWeight{Name: doc.Name}
		if info, err := os.Stat(filepath.Join(dir, doc.RelPath)); err == nil && !info.IsDir() {
			d.Present = true
			d.Bytes = int(info.Size())
			d.OverCap = d.Bytes > contextWeightPerDocWarnBytes
			rep.TotalBytes += d.Bytes
			rep.PresentCount++
		}
		rep.Docs = append(rep.Docs, d)
	}
	rep.OverTotal = rep.TotalBytes > contextWeightTotalWarnBytes
	return rep
}

// renderContextWeight prints the guard section. It is silent when no catalog
// document exists; otherwise it prints one status line for the combined total
// plus a WARN line for every over-cap document, in ContextLoadSet order.
func renderContextWeight(w io.Writer, rep contextWeightReport) {
	if rep.PresentCount == 0 {
		return
	}
	tui.SectionHeader(w, "Context Weight")
	if rep.OverTotal {
		tui.Warn(w, fmt.Sprintf("context catalog %dB exceeds %dB soft cap across %d docs; rotate history per doc-storage",
			rep.TotalBytes, contextWeightTotalWarnBytes, rep.PresentCount))
	} else {
		tui.OK(w, fmt.Sprintf("context catalog %dB across %d docs (soft cap %dB)",
			rep.TotalBytes, rep.PresentCount, contextWeightTotalWarnBytes))
	}
	for _, d := range rep.Docs {
		if d.OverCap {
			tui.Warn(w, fmt.Sprintf("context catalog: context doc %s %dB exceeds %dB soft cap; rotate history per doc-storage",
				d.Name, d.Bytes, contextWeightPerDocWarnBytes))
		}
	}
}

// checkContextWeight measures the context catalog documents under dir and emits
// non-blocking warnings when the catalog is over-weight. It is advisory: an
// over-weight catalog can degrade selected-profile quality but never fails
// harness health, so callers MUST NOT flip their allOK verdict on the result.
// The return reports whether a warning fired, for the JSON mirror and tests.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONTEXT-ENGINEERING-001: preserve the advisory text-check contract.
// @AX:REASON [AUTO]: doctor output and five regression call sites depend on its warning-only boolean semantics.
func checkContextWeight(w io.Writer, dir string) bool {
	rep := measureContextWeight(dir)
	renderContextWeight(w, rep)
	return rep.warned()
}
