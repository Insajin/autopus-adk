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

func TestBuildReviewPrompt_TrimsChecklistToDocContextMaxLines(t *testing.T) {
	t.Parallel()

	lines := make([]string, 300)
	for i := range lines {
		lines[i] = "checklist line"
	}

	doc := &SpecDocument{
		ID:         "SPEC-CHECKLIST-002",
		Title:      "Checklist Trim",
		RawContent: "# SPEC-CHECKLIST-002",
	}
	opts := ReviewPromptOptions{
		DocContextMaxLines: 200,
		checklistFS: fstest.MapFS{
			checklistEmbedPath: &fstest.MapFile{Data: []byte(strings.Join(lines, "\n"))},
		},
	}

	prompt := BuildReviewPrompt(doc, "", opts)

	assert.Contains(t, prompt, "... (trimmed 100 more lines)")
	assert.NotContains(t, prompt, strings.Repeat("checklist line\n", 250))
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
