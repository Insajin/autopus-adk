package spec

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/content"
)

func TestLoadChecklist_EmbeddedContentFS(t *testing.T) {
	t.Parallel()

	body, err := LoadChecklist(content.FS)
	require.NoError(t, err)
	assert.Contains(t, body, "# SPEC Quality Checklist")
}

func TestBuildReviewPrompt_InjectsChecklistBeforeInstructions(t *testing.T) {
	t.Parallel()

	doc := &SpecDocument{
		ID:         "SPEC-CHECKLIST-001",
		Title:      "Checklist Prompt",
		RawContent: "# SPEC-CHECKLIST-001",
	}

	prompt := BuildReviewPrompt(doc, "func Example() {}", ReviewPromptOptions{Mode: ReviewModeDiscover})

	codeIdx := strings.Index(prompt, "### Existing Code Context")
	checklistIdx := strings.Index(prompt, "## Quality Checklist")
	instructionsIdx := strings.Index(prompt, "### Instructions")

	require.NotEqual(t, -1, codeIdx)
	require.NotEqual(t, -1, checklistIdx)
	require.NotEqual(t, -1, instructionsIdx)
	assert.Greater(t, checklistIdx, codeIdx)
	assert.Greater(t, instructionsIdx, checklistIdx)
	assert.Contains(t, prompt, "### Checklist Response Format")
	assert.Contains(t, prompt, "CHECKLIST: <항목 ID> | PASS")
	assert.Contains(t, prompt, "CHECKLIST: <항목 ID> | FAIL | <reason>")
}

// The checklist is a stable instruction document: the reviewer must answer every
// Q-* item, so DocContextMaxLines (a compression-fallback threshold) must not
// head-trim it. Only a checklist beyond the generous budget is trimmed, and then
// only with a visible notice.
func TestBuildReviewPrompt_ChecklistBudgetIgnoresDocContextMaxLines(t *testing.T) {
	t.Parallel()

	prompt := buildPromptWithChecklistLines(t, 300, 200)

	assert.NotContains(t, prompt, "additional lines were omitted",
		"a 300-line checklist fits the generous budget and must inject in full")
	assert.Contains(t, prompt, strings.Repeat("checklist line\n", 299))
}

func TestBuildReviewPrompt_TrimsChecklistBeyondGenerousBudget(t *testing.T) {
	t.Parallel()

	prompt := buildPromptWithChecklistLines(t, DefaultAuxTotalBudgetLines+100, 200)

	assert.Contains(t, prompt, "[Review-context notice: 100 additional lines were omitted")
	assert.Contains(t, prompt, "not a source document defect")
}

// The canonical checklist must inject whole: a trimmed tail silently removes
// checklist items from the contract the reviewer is asked to report on.
func TestBuildReviewPrompt_CanonicalChecklistInjectsInFull(t *testing.T) {
	t.Parallel()

	body, err := LoadChecklist(content.FS)
	require.NoError(t, err)

	doc := &SpecDocument{ID: "SPEC-CHECKLIST-004", Title: "Canonical", RawContent: "# SPEC-CHECKLIST-004"}
	prompt := BuildReviewPrompt(doc, "", ReviewPromptOptions{DocContextMaxLines: 200})

	assert.NotContains(t, prompt, "additional lines were omitted")
	assert.Contains(t, prompt, strings.TrimSpace(body))
}

func buildPromptWithChecklistLines(t *testing.T, count, docContextMaxLines int) string {
	t.Helper()

	lines := make([]string, count)
	for i := range lines {
		lines[i] = "checklist line"
	}

	doc := &SpecDocument{
		ID:         "SPEC-CHECKLIST-002",
		Title:      "Checklist Budget",
		RawContent: "# SPEC-CHECKLIST-002",
	}
	opts := ReviewPromptOptions{
		DocContextMaxLines: docContextMaxLines,
		checklistFS: fstest.MapFS{
			checklistEmbedPath: &fstest.MapFile{Data: []byte(strings.Join(lines, "\n"))},
		},
	}

	return BuildReviewPrompt(doc, "", opts)
}

func TestBuildReviewPrompt_SoftFallbackWhenChecklistMissing(t *testing.T) {
	doc := &SpecDocument{
		ID:         "SPEC-CHECKLIST-003",
		Title:      "Checklist Missing",
		RawContent: "# SPEC-CHECKLIST-003",
	}
	missingPath := filepath.Join(t.TempDir(), "missing-checklist.md")
	opts := ReviewPromptOptions{
		Mode:               ReviewModeDiscover,
		checklistFS:        fstest.MapFS{},
		checklistDiskPaths: []string{missingPath},
	}

	stderr := captureStderr(t, func() {
		prompt := BuildReviewPrompt(doc, "", opts)
		assert.NotContains(t, prompt, "## Quality Checklist")
		assert.NotContains(t, prompt, "### Checklist Response Format")
		assert.Contains(t, prompt, "### Verdict Decision Rules")
		assert.Contains(t, prompt, "### Finding Format Examples")
	})

	assert.Contains(t, stderr, "경고: 체크리스트 로드 실패 ("+missingPath+")")
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stderr
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = writer
	defer func() { os.Stderr = orig }()

	fn()

	require.NoError(t, writer.Close())
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(data)
}
