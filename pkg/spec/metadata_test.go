package spec_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestParseSpecMetadata_ReadsLegacySpecIDAndStatus(t *testing.T) {
	t.Parallel()

	doc := spec.ParseSpecMetadata(`# SPEC: Legacy Approval Flow

**SPEC-ID**: SPEC-LEGACY-001
**Status**: completed
**Created**: 2026-04-16
`)

	assert.Equal(t, "SPEC-LEGACY-001", doc.ID)
	assert.Equal(t, "Legacy Approval Flow", doc.Title)
	assert.Equal(t, "completed", doc.Status)
}

func TestUpdateStatus_RewritesLegacyStatusLine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, "spec")
	require.NoError(t, os.MkdirAll(specDir, 0o755))

	content := `# SPEC: Legacy Approval Flow

**SPEC-ID**: SPEC-LEGACY-002
**Status**: draft
**Created**: 2026-04-16
`
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o644))

	require.NoError(t, spec.UpdateStatus(specDir, "approved"))

	body, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "**Status**: approved")
	assert.NotContains(t, string(body), "status: approved")

	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	assert.Equal(t, "approved", doc.Status)
	assert.Equal(t, "SPEC-LEGACY-002", doc.ID)
}

// Phase 1.5 scaffold for SPEC-SPECREV-001 REQ-CTX-2 / REQ-CTX-3.
// References spec.ParseReviewContextOverride which does not yet exist —
// compile failure is the expected RED state.

func TestParseReviewContextOverride_ValidValue_ReturnsPresent(t *testing.T) {
	t.Parallel()

	content := `---
id: SPEC-FAKE-CTX2
review_context_lines: 800
---

# SPEC-FAKE-CTX2: example
`
	value, present, err := spec.ParseReviewContextOverride(content)
	require.NoError(t, err)
	assert.True(t, present, "review_context_lines key should be detected as present")
	assert.Equal(t, 800, value)
}

func TestParseReviewContextOverride_MissingKey_ReturnsAbsent(t *testing.T) {
	t.Parallel()

	content := `---
id: SPEC-FAKE-NOOVR
status: approved
---

# SPEC-FAKE-NOOVR
`
	value, present, err := spec.ParseReviewContextOverride(content)
	require.NoError(t, err)
	assert.False(t, present, "absent key must report present=false")
	assert.Equal(t, 0, value)
}

func TestParseReviewContextOverride_InvalidValue_ReturnsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantVal int // value the parser returned (for diagnostic forwarding)
	}{
		{"zero is rejected", "0", 0},
		{"negative is rejected", "-1", -1},
		{"over 10000 is rejected", "10001", 10001},
		{"non-integer string is rejected", "abc", 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			content := "---\nid: SPEC-FAKE-INV\nreview_context_lines: " + tt.raw + "\n---\n"
			value, present, err := spec.ParseReviewContextOverride(content)
			require.Error(t, err, "invalid override %q must produce an error", tt.raw)
			// REQ-CTX-3: present is true (key exists) so caller can log reject reason,
			// but err signals the value must be ignored for the limit calculation.
			assert.True(t, present, "key present even when value invalid")
			assert.Equal(t, tt.wantVal, value, "invalid value forwarded to caller for logging")
		})
	}
}

func TestUpdateStatus_DoesNotTreatBodySeparatorsAsFrontmatter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, "spec")
	require.NoError(t, os.MkdirAll(specDir, 0o755))

	content := `# SPEC-LEGACY-003: Interactive Debate

**Status**: draft
**Created**: 2026-04-16

---

## Purpose

Legacy body section.

---

## Requirements
`
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o644))

	require.NoError(t, spec.UpdateStatus(specDir, "approved"))

	body, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "**Status**: approved")
	assert.Contains(t, string(body), "---\n\n## Purpose")
	assert.NotContains(t, string(body), "\nstatus: approved\n")
}
