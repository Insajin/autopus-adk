package spec

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var completeReviewDocumentNames = []string{
	"plan.md",
	"research.md",
	"acceptance.md",
}

// TestBuildReviewPromptChecked_RequireCompleteDocuments_IncludesFullDocumentsAndMetadata
// is the package-level integration oracle. The internal CLI review options are
// unexported, so this test locks the exported pkg/spec production seam instead.
func TestBuildReviewPromptChecked_RequireCompleteDocuments_IncludesFullDocumentsAndMetadata(t *testing.T) {
	t.Parallel()

	// Given a complete review document set whose files all exceed the legacy
	// 200-line excerpt limit.
	doc := &SpecDocument{ID: "SPEC-COMPLETE-DOCS-001", Title: "Complete Review Documents"}
	specDir := newCompleteReviewSpecDir(t, doc.ID)
	specContent := completeReviewSpecFixture(doc.ID, doc.Title, 260)
	writeCompleteReviewDocument(t, specDir, "spec.md", specContent)
	contents := make(map[string]string, len(completeReviewDocumentNames))
	for _, name := range completeReviewDocumentNames {
		content := completeReviewDocumentFixture(name, 260)
		contents[name] = content
		writeCompleteReviewDocument(t, specDir, name, content)
	}

	opts := ReviewPromptOptions{
		SpecDir:                  specDir,
		DocContextMaxLines:       25,
		RequireCompleteDocuments: true,
	}

	// When the checked prompt builder runs in required-complete mode.
	prompt, err := BuildReviewPromptChecked(doc, "", opts)

	// Then every document is present byte-for-byte, including content beyond
	// line 200 and its unique tail marker, with bound source metadata.
	require.NoError(t, err)
	assert.True(t, strings.Contains(prompt, specContent), "spec.md must be included byte-for-byte")
	assert.Contains(t, prompt, completeReviewDocumentMetadataMarker("spec.md", specContent, specContent, "passed", "none"))
	for _, name := range completeReviewDocumentNames {
		content := contents[name]
		assert.True(t, strings.Contains(prompt, content), "%s must be included byte-for-byte", name)
		assert.Contains(t, prompt, completeReviewDocumentBeyond200Marker(name))
		assert.Contains(t, prompt, completeReviewDocumentTailMarker(name))
		assert.Contains(t, prompt, completeReviewDocumentMetadataMarker(name, content, content, "passed", "none"))
	}
	assert.NotContains(t, prompt, "[Review-context notice:", "required-complete documents must not be trimmed")
}

func TestBuildReviewPromptChecked_RequireCompleteDocumentsMissing_ReturnsErrorBeforeProvider(t *testing.T) {
	t.Parallel()

	for _, missingName := range completeReviewDocumentNames {
		missingName := missingName
		t.Run(missingName, func(t *testing.T) {
			t.Parallel()

			// Given one required review document is absent.
			specDir := newCompleteReviewSpecDir(t, "SPEC-MISSING-DOC-001")
			writeCompleteReviewDocument(t, specDir, "spec.md", completeReviewSpecFixture("SPEC-MISSING-DOC-001", "Missing Review Document", 5))
			for _, name := range completeReviewDocumentNames {
				if name == missingName {
					continue
				}
				writeCompleteReviewDocument(t, specDir, name, completeReviewDocumentFixture(name, 5))
			}

			providerCalls := 0
			doc := &SpecDocument{ID: "SPEC-MISSING-DOC-001", Title: "Missing Review Document"}
			opts := ReviewPromptOptions{SpecDir: specDir, RequireCompleteDocuments: true}

			// When the checked builder validates the document set before dispatch.
			prompt, err := BuildReviewPromptChecked(doc, "", opts)
			if err == nil {
				providerCalls++
			}

			// Then no prompt is dispatchable and no provider invocation occurs.
			require.Error(t, err)
			assert.ErrorContains(t, err, missingName)
			assert.Empty(t, prompt)
			assert.Zero(t, providerCalls)
		})
	}
}

func TestBuildReviewPromptChecked_RequireCompleteDocumentsEmpty_ReturnsErrorBeforeProvider(t *testing.T) {
	t.Parallel()

	for _, emptyName := range completeReviewDocumentNames {
		emptyName := emptyName
		t.Run(emptyName, func(t *testing.T) {
			t.Parallel()

			// Given one required review document contains only whitespace.
			specDir := newCompleteReviewSpecDir(t, "SPEC-EMPTY-DOC-001")
			writeCompleteReviewDocument(t, specDir, "spec.md", completeReviewSpecFixture("SPEC-EMPTY-DOC-001", "Empty Review Document", 5))
			for _, name := range completeReviewDocumentNames {
				content := completeReviewDocumentFixture(name, 5)
				if name == emptyName {
					content = " \n\t\n"
				}
				writeCompleteReviewDocument(t, specDir, name, content)
			}

			providerCalls := 0
			doc := &SpecDocument{ID: "SPEC-EMPTY-DOC-001", Title: "Empty Review Document"}
			opts := ReviewPromptOptions{SpecDir: specDir, RequireCompleteDocuments: true}

			// When the checked builder validates the document set before dispatch.
			prompt, err := BuildReviewPromptChecked(doc, "", opts)
			if err == nil {
				providerCalls++
			}

			// Then no prompt is dispatchable and no provider invocation occurs.
			require.Error(t, err)
			assert.ErrorContains(t, err, emptyName)
			assert.Empty(t, prompt)
			assert.Zero(t, providerCalls)
		})
	}
}

func TestBuildReviewPromptChecked_RequireCompleteDocumentsSymlinkEscape_ReturnsErrorBeforeProvider(t *testing.T) {
	t.Parallel()

	// Given plan.md escapes the SPEC directory through a symlink while the
	// other required documents are regular files.
	specDir := newCompleteReviewSpecDir(t, "SPEC-SYMLINK-DOC-001")
	outsideDir := t.TempDir()
	writeCompleteReviewDocument(t, specDir, "spec.md", completeReviewSpecFixture("SPEC-SYMLINK-DOC-001", "Symlink Review Document", 5))
	outsidePlan := filepath.Join(outsideDir, "outside-plan.md")
	require.NoError(t, os.WriteFile(outsidePlan, []byte(completeReviewDocumentFixture("plan.md", 5)), 0o644))
	if err := os.Symlink(outsidePlan, filepath.Join(specDir, "plan.md")); err != nil {
		t.Skipf("symlink setup is unavailable: %v", err)
	}
	writeCompleteReviewDocument(t, specDir, "research.md", completeReviewDocumentFixture("research.md", 5))
	writeCompleteReviewDocument(t, specDir, "acceptance.md", completeReviewDocumentFixture("acceptance.md", 5))

	providerCalls := 0
	doc := &SpecDocument{ID: "SPEC-SYMLINK-DOC-001", Title: "Symlink Review Document"}
	opts := ReviewPromptOptions{SpecDir: specDir, RequireCompleteDocuments: true}

	// When the checked builder validates paths before dispatch.
	prompt, err := BuildReviewPromptChecked(doc, "", opts)
	if err == nil {
		providerCalls++
	}

	// Then the escaping symlink fails closed before provider invocation.
	require.Error(t, err)
	assert.ErrorContains(t, err, "plan.md")
	assert.Empty(t, prompt)
	assert.Zero(t, providerCalls)
}

func TestBuildReviewPromptChecked_OversizedCompletePromptBlocksInsteadOfTruncating(t *testing.T) {
	t.Parallel()

	specDir := newCompleteReviewSpecDir(t, "SPEC-OVERSIZED-CONTEXT-001")
	writeCompleteReviewDocument(t, specDir, "spec.md", completeReviewSpecFixture("SPEC-OVERSIZED-CONTEXT-001", "Oversized Context", 5))
	for _, name := range completeReviewDocumentNames {
		writeCompleteReviewDocument(t, specDir, name, completeReviewDocumentFixture(name, 5))
	}
	doc := &SpecDocument{ID: "SPEC-OVERSIZED-CONTEXT-001", Title: "Oversized Context"}
	codeContext := strings.Repeat("context-must-not-be-truncated\n", maxCompleteReviewPromptTokens*4/16)

	prompt, err := BuildReviewPromptChecked(doc, codeContext, ReviewPromptOptions{
		SpecDir: specDir, RequireCompleteDocuments: true,
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "split the review instead of truncating")
	assert.Empty(t, prompt)
}

func completeReviewDocumentFixture(name string, lineCount int) string {
	lines := make([]string, lineCount)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s line %03d", name, i+1)
	}
	if lineCount > 200 {
		lines[200] = completeReviewDocumentBeyond200Marker(name)
	}
	lines[lineCount-1] = completeReviewDocumentTailMarker(name)
	return strings.Join(lines, "\n")
}

func completeReviewSpecFixture(id, title string, lineCount int) string {
	body := completeReviewDocumentFixture("spec.md", lineCount)
	return fmt.Sprintf("# %s: %s\n\n---\nid: %s\ntitle: %s\nversion: 0.1.0\nstatus: draft\n---\n\n%s", id, title, id, title, body)
}

func completeReviewDocumentBeyond200Marker(name string) string {
	return fmt.Sprintf("%s BEYOND-200-MARKER", name)
}

func completeReviewDocumentTailMarker(name string) string {
	return fmt.Sprintf("%s COMPLETE-TAIL-MARKER", name)
}

func completeReviewDocumentMetadataMarker(name, source, delivered, status, reason string) string {
	sourceDigest := sha256.Sum256([]byte(source))
	promptDigest := sha256.Sum256([]byte(delivered))
	return fmt.Sprintf(
		"[Review document metadata: source_ref=%s source_sha256=%x prompt_sha256=%x redaction_status=%s invalidation_reason=%s complete=true]",
		name, sourceDigest, promptDigest, status, reason,
	)
}

func writeCompleteReviewDocument(t *testing.T, specDir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(specDir, name), []byte(content), 0o644))
}

func newCompleteReviewSpecDir(t *testing.T, id string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), id)
	require.NoError(t, os.Mkdir(dir, 0o755))
	return dir
}
