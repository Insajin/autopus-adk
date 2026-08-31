package report

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/evidence"
)

// Preview budgets. A shared report must stay openable in a browser, so both the
// per-artifact slice and the whole-report total are bounded.
const (
	maxArtifactPreviewBytes = 64 << 10
	maxReportPreviewBytes   = 2 << 20
	maxPreviewArtifacts     = 256
)

// textArtifactExts mirrors the publishable text formats accepted by the evidence
// writer. Anything else is metadata-only here, exactly as it is at publication.
var textArtifactExts = map[string]bool{
	".json":   true,
	".log":    true,
	".txt":    true,
	".md":     true,
	".yml":    true,
	".yaml":   true,
	".stdout": true,
	".stderr": true,
}

// ansiRe strips terminal escape sequences so stdout/stderr artifacts render as
// readable text instead of raw control bytes.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type previewBudget struct {
	bytesLeft int
	slotsLeft int
}

func newPreviewBudget() *previewBudget {
	return &previewBudget{bytesLeft: maxReportPreviewBytes, slotsLeft: maxPreviewArtifacts}
}

// refund returns a preview's budget after a later projection superseded it, so
// suppressing one artifact never shrinks what the rest of the report can inline.
func (b *previewBudget) refund(text string) {
	if text == "" {
		return
	}
	b.slotsLeft++
	b.bytesLeft += len(text)
}

// artifactViews projects one manifest's artifacts, inlining previews only for
// publishable text files that survive re-redaction.
func artifactViews(root string, refs []evidence.ArtifactRef, budget *previewBudget) []ArtifactView {
	views := make([]ArtifactView, 0, len(refs))
	for _, ref := range refs {
		views = append(views, artifactView(root, ref, budget))
	}
	return views
}

func artifactView(root string, ref evidence.ArtifactRef, budget *previewBudget) ArtifactView {
	view := ArtifactView{
		Kind:        orUnknown(ref.Kind),
		Ref:         displayPath(root, ref.Path),
		Publishable: ref.Publishable,
		Redaction:   orUnknown(ref.Redaction),
	}
	info, err := os.Stat(ref.Path)
	if err != nil {
		view.Withheld = "missing_file"
		return view
	}
	if !info.Mode().IsRegular() {
		view.Withheld = "not_regular_file"
		return view
	}
	view.Bytes = info.Size()
	if !ref.Publishable {
		view.Withheld = "local_only_evidence"
		return view
	}
	if !textArtifactExts[strings.ToLower(filepath.Ext(ref.Path))] {
		view.Withheld = "unsupported_format"
		return view
	}
	if budget.slotsLeft <= 0 || budget.bytesLeft <= 0 {
		view.Withheld = "preview_budget_exhausted"
		return view
	}
	limit := maxArtifactPreviewBytes
	if budget.bytesLeft < limit {
		limit = budget.bytesLeft
	}
	body, truncated, err := readCapped(ref.Path, limit)
	if err != nil {
		view.Withheld = "unreadable_file"
		return view
	}
	text := evidence.RedactText(sanitizeText(body))
	if err := evidence.AssertSafeText(text, view.Ref); err != nil {
		view.Withheld = "redaction_failed"
		return view
	}
	if strings.TrimSpace(text) == "" {
		view.Withheld = "empty_artifact"
		return view
	}
	view.Preview = text
	view.Truncated = truncated || info.Size() > int64(limit)
	budget.slotsLeft--
	budget.bytesLeft -= len(text)
	return view
}

func readCapped(path string, limit int) (string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	// Read one extra byte so a file exactly at the cap is not reported truncated.
	body, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return "", false, err
	}
	if len(body) > limit {
		return string(body[:limit]), true, nil
	}
	return string(body), false, nil
}

// sanitizeText makes artifact bytes safe to place inside a <pre> block: valid
// UTF-8, no terminal escapes, and no control characters other than tab/newline.
func sanitizeText(value string) string {
	value = strings.ToValidUTF8(value, "\uFFFD")
	value = ansiRe.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
}

// displayPath renders an absolute evidence path as a project-relative slash path,
// falling back to redaction when the path lies outside the project root.
func displayPath(root, path string) string {
	if path == "" {
		return ""
	}
	if rel, err := filepath.Rel(root, path); err == nil && !escapesRoot(rel) {
		return filepath.ToSlash(rel)
	}
	return evidence.RedactText(filepath.ToSlash(path))
}

func orUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
