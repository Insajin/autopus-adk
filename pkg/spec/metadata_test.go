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
