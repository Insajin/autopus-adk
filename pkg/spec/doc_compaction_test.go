package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompactDocToBudget_PreservesTailCriticalSections is the AC-RINT-STRUCT-3
// oracle: a document whose head is filler and whose tail carries
// `## Self-Verify Summary` / `Q-COMP-05` must keep the tail-critical section
// and drop filler when compacted below its total line count.
func TestCompactDocToBudget_PreservesTailCriticalSections(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for i := 0; i < 240; i++ {
		b.WriteString("filler line ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("## Self-Verify Summary\n")
	b.WriteString("Q-COMP-05 | status: PASS")
	content := b.String() // 242 lines: 240 filler + 2 tail

	excerpt, injected := compactDocToBudget(content, 10)

	assert.Equal(t, 10, injected, "injected line count must equal the budget")
	assert.Contains(t, excerpt, "## Self-Verify Summary", "tail-critical heading must survive")
	assert.Contains(t, excerpt, "Q-COMP-05", "tail-critical evidence line must survive")
	assert.NotContains(t, excerpt, "filler line 100", "deep filler must be dropped")
	assert.NotContains(t, excerpt, "filler line 239", "filler just before the tail must be dropped")
	assert.Contains(t, excerpt, "additional lines were omitted", "an omission notice must mark the compaction")
}

// TestCompactDocToBudget_MultipleTailSectionsSurvive verifies the earliest
// tail-critical section onward is preserved, so Traceability and Open Issues
// after Self-Verify also survive.
func TestCompactDocToBudget_MultipleTailSectionsSurvive(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("filler\n")
	}
	b.WriteString("## Traceability Matrix\n")
	b.WriteString("REQ-1 maps to AC-1\n")
	b.WriteString("## Open Issues\n")
	b.WriteString("none")
	content := b.String()

	excerpt, _ := compactDocToBudget(content, 12)

	assert.Contains(t, excerpt, "## Traceability Matrix")
	assert.Contains(t, excerpt, "## Open Issues")
	assert.Contains(t, excerpt, "REQ-1 maps to AC-1")
}

// TestCompactDocToBudget_FullWhenWithinBudget verifies a document within budget
// is returned verbatim with no omission notice (REQ-RINT-FULL-02).
func TestCompactDocToBudget_FullWhenWithinBudget(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("line\n", 49) + "last"
	excerpt, injected := compactDocToBudget(content, 4000)

	assert.Equal(t, content, excerpt, "content must be returned verbatim within budget")
	assert.Equal(t, 50, injected)
	assert.NotContains(t, excerpt, "additional lines were omitted")
}

// TestCompactDocToBudget_NoCriticalSectionKeepsHead verifies a generic document
// with no recognized tail-critical section is compacted to the budget with an
// omission notice.
func TestCompactDocToBudget_NoCriticalSectionKeepsHead(t *testing.T) {
	t.Parallel()

	lines := make([]string, 250)
	for i := range lines {
		lines[i] = "line " + itoa(i)
	}
	content := strings.Join(lines, "\n")

	excerpt, injected := compactDocToBudget(content, 200)

	assert.Equal(t, 200, injected)
	assert.Contains(t, excerpt, "line 0", "head must be retained when no tail-critical section exists")
	assert.NotContains(t, excerpt, "line 249", "tail filler beyond budget is dropped")
	assert.Contains(t, excerpt, "additional lines were omitted")
}

// TestResolveAuxTotalBudget_FloorsToGenerousDefault verifies the historical
// per-doc cap can never shrink the total budget below the generous default,
// while an operator raising it above the default is honored.
func TestResolveAuxTotalBudget_FloorsToGenerousDefault(t *testing.T) {
	t.Parallel()

	assert.Equal(t, DefaultAuxTotalBudgetLines, ResolveAuxTotalBudget(0))
	assert.Equal(t, DefaultAuxTotalBudgetLines, ResolveAuxTotalBudget(200))
	assert.Equal(t, DefaultAuxTotalBudgetLines, ResolveAuxTotalBudget(-5))
	assert.Equal(t, DefaultAuxTotalBudgetLines+1000, ResolveAuxTotalBudget(DefaultAuxTotalBudgetLines+1000))
}

// TestInjectAuxDocs_FullByDefault verifies the prompt injection path injects all
// three documents in full under the default budget and adds no omission notice.
func TestInjectAuxDocs_FullByDefault(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	writeLinesDoc(t, specDir, "plan.md", 300, "plan line")
	writeLinesDoc(t, specDir, "research.md", 429, "research line")
	writeLinesDoc(t, specDir, "acceptance.md", 400, "acceptance line")

	var sb strings.Builder
	injectAuxDocs(&sb, specDir, DefaultAuxTotalBudgetLines)
	out := sb.String()

	assert.Contains(t, out, "### Plan Document")
	assert.Contains(t, out, "### Research Document")
	assert.Contains(t, out, "### Acceptance Criteria Document")
	assert.NotContains(t, out, "additional lines were omitted",
		"typical spec set must inject fully with no truncation notice")
}

// TestInjectAuxDocs_CompactsOnlyWhenOverBudget verifies compaction engages only
// when the combined document total exceeds the budget, preserving tail sections.
func TestInjectAuxDocs_CompactsOnlyWhenOverBudget(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	writeLinesDoc(t, specDir, "plan.md", 150, "plan line")

	var b strings.Builder
	for i := 0; i < 240; i++ {
		b.WriteString("filler\n")
	}
	b.WriteString("## Completion Debt\nnone remaining")
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "research.md"), []byte(b.String()), 0o644))

	var sb strings.Builder
	injectAuxDocs(&sb, specDir, 350)
	out := sb.String()

	assert.Contains(t, out, "## Completion Debt", "tail-critical section must survive compaction")
	assert.Contains(t, out, "additional lines were omitted", "over-budget set must carry an omission notice")
}

// TestCompactDocToBudget_CriticalTailExceedsBudgetKeepsTail verifies that when
// the tail-critical block alone is larger than the budget, the final lines are
// kept (dropping the head) so the most tail-critical content still survives.
func TestCompactDocToBudget_CriticalTailExceedsBudgetKeepsTail(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for i := 0; i < 3; i++ {
		b.WriteString("head filler\n")
	}
	b.WriteString("## Open Issues\n")
	for i := 0; i < 8; i++ {
		b.WriteString("issue ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("issue final")
	content := b.String() // 3 head + 1 heading + 9 issue lines = 13 lines

	excerpt, injected := compactDocToBudget(content, 5)

	assert.Equal(t, 5, injected)
	assert.Contains(t, excerpt, "issue final", "the document tail must survive")
	assert.NotContains(t, excerpt, "head filler", "the head is dropped when the critical tail is oversize")
	assert.Contains(t, excerpt, "additional lines were omitted")
}

// TestCompactDocToBudget_ZeroBudgetInjectsNothing verifies a non-positive budget
// injects no document lines and records injected 0.
func TestCompactDocToBudget_ZeroBudgetInjectsNothing(t *testing.T) {
	t.Parallel()

	excerpt, injected := compactDocToBudget("line a\nline b\nline c", 0)

	assert.Equal(t, 0, injected)
	assert.NotContains(t, excerpt, "line a")
	assert.Contains(t, excerpt, "additional lines were omitted")
}

// itoa is a tiny int-to-string helper to avoid importing strconv in tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
