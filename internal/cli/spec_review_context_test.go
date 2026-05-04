package cli

// Phase 1.5 test scaffold for SPEC-SPECREV-001 integration scenarios
// (AC-CTX-1 / AC-CTX-2 / AC-CTX-3 / AC-CTX-OVR-INVALID / AC-CTX-OVR-OVER /
// AC-CTX-CEIL).
//
// These tests reference a helper `resolveSpecReviewContextLimit` that Phase 2
// will extract from `runSpecReview` (per plan.md T5). Compile failure here is
// the expected RED state.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specReviewCtxFixture creates a temporary SPEC directory with the given
// research.md citations and an optional frontmatter override.
func specReviewCtxFixture(t *testing.T, specID string, citedFiles []string, frontmatterOverride string) (projectRoot, specDir string) {
	t.Helper()

	root := t.TempDir()
	specDir = filepath.Join(root, ".autopus", "specs", specID)
	require.NoError(t, os.MkdirAll(specDir, 0o755))

	// Create cited files (200 lines each) at predictable paths.
	for _, rel := range citedFiles {
		full := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		var b bytes.Buffer
		b.WriteString("package fake\n\n")
		for i := 0; i < 198; i++ {
			b.WriteString("// filler line\n")
		}
		require.NoError(t, os.WriteFile(full, b.Bytes(), 0o644))
	}

	// Compose spec.md with optional frontmatter.
	var sb strings.Builder
	if frontmatterOverride != "" {
		sb.WriteString("---\n")
		sb.WriteString("id: ")
		sb.WriteString(specID)
		sb.WriteString("\n")
		sb.WriteString("review_context_lines: ")
		sb.WriteString(frontmatterOverride)
		sb.WriteString("\n---\n\n")
	}
	sb.WriteString("# ")
	sb.WriteString(specID)
	sb.WriteString(": fixture\n\nbody\n")
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(sb.String()), 0o644))

	// research.md cites the files.
	var rb strings.Builder
	rb.WriteString("# research\n\n")
	for _, rel := range citedFiles {
		rb.WriteString("- `")
		rb.WriteString(rel)
		rb.WriteString("`\n")
	}
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "research.md"), []byte(rb.String()), 0o644))

	return root, specDir
}

// TestResolveSpecReviewContextLimit_AC_CTX_1 covers AC-CTX-1: 4 cited files
// with no override and no ceiling -> applied=1500, log line "cited=4 applied=1500".
func TestResolveSpecReviewContextLimit_AC_CTX_1(t *testing.T) {
	t.Parallel()

	projectRoot, specDir := specReviewCtxFixture(t, "SPEC-FAKE-CTX1",
		[]string{"pkg/a.go", "pkg/b.go", "pkg/c.go", "pkg/d.go"}, "")

	var stderr bytes.Buffer
	cited, applied, override, err := resolveSpecReviewContextLimit(projectRoot, specDir, 0, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 4, cited)
	assert.Equal(t, 1500, applied)
	assert.Equal(t, "", override, "no frontmatter override expected")
	assert.Contains(t, stderr.String(), "SPEC review context: cited=4 applied=1500")
}

// TestResolveSpecReviewContextLimit_AC_CTX_2 covers AC-CTX-2: 7 cited files
// with frontmatter override 800 -> applied=800 with override=frontmatter log.
func TestResolveSpecReviewContextLimit_AC_CTX_2(t *testing.T) {
	t.Parallel()

	cited7 := []string{
		"pkg/a.go", "pkg/b.go", "pkg/c.go", "pkg/d.go",
		"pkg/e.go", "pkg/f.go", "pkg/g.go",
	}
	projectRoot, specDir := specReviewCtxFixture(t, "SPEC-FAKE-CTX2", cited7, "800")

	var stderr bytes.Buffer
	cited, applied, override, err := resolveSpecReviewContextLimit(projectRoot, specDir, 0, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 7, cited)
	assert.Equal(t, 800, applied, "frontmatter override 800 must beat auto-mapping 3000")
	assert.Equal(t, "frontmatter", override)
	assert.Contains(t, stderr.String(), "SPEC review context: cited=7 applied=800 override=frontmatter")
}

// TestResolveSpecReviewContextLimit_AC_CTX_3 covers AC-CTX-3: 0 cited files,
// no override -> applied=500 (default).
func TestResolveSpecReviewContextLimit_AC_CTX_3(t *testing.T) {
	t.Parallel()

	projectRoot, specDir := specReviewCtxFixture(t, "SPEC-FAKE-CTX3", nil, "")

	var stderr bytes.Buffer
	cited, applied, override, err := resolveSpecReviewContextLimit(projectRoot, specDir, 0, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cited)
	assert.Equal(t, 500, applied)
	assert.Equal(t, "", override)
	assert.Contains(t, stderr.String(), "SPEC review context: cited=0 applied=500")
}

// TestResolveSpecReviewContextLimit_AC_CTX_OVR_INVALID covers AC-CTX-OVR-INVALID:
// frontmatter -1 is rejected, falls back to auto-mapping, logs warning line.
func TestResolveSpecReviewContextLimit_AC_CTX_OVR_INVALID(t *testing.T) {
	t.Parallel()

	cited4 := []string{"pkg/a.go", "pkg/b.go", "pkg/c.go", "pkg/d.go"}
	projectRoot, specDir := specReviewCtxFixture(t, "SPEC-FAKE-CTX4", cited4, "-1")

	var stderr bytes.Buffer
	cited, applied, override, err := resolveSpecReviewContextLimit(projectRoot, specDir, 0, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 4, cited)
	assert.Equal(t, 1500, applied, "rejected override falls back to auto-mapping")
	assert.Equal(t, "", override)
	assert.Contains(t, stderr.String(),
		"경고: review_context_lines 무시 (값=-1, 사유=must be >0 and <=10000)")
}

// TestResolveSpecReviewContextLimit_AC_CTX_OVR_OVER covers AC-CTX-OVR-OVER:
// frontmatter 10001 is rejected, falls back to auto-mapping (3000 here).
func TestResolveSpecReviewContextLimit_AC_CTX_OVR_OVER(t *testing.T) {
	t.Parallel()

	cited6 := []string{"pkg/a.go", "pkg/b.go", "pkg/c.go", "pkg/d.go", "pkg/e.go", "pkg/f.go"}
	projectRoot, specDir := specReviewCtxFixture(t, "SPEC-FAKE-CTX5", cited6, "10001")

	var stderr bytes.Buffer
	cited, applied, override, err := resolveSpecReviewContextLimit(projectRoot, specDir, 0, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 6, cited)
	assert.Equal(t, 3000, applied)
	assert.Equal(t, "", override)
	assert.Contains(t, stderr.String(),
		"경고: review_context_lines 무시 (값=10001, 사유=must be >0 and <=10000)")
}

// TestResolveSpecReviewContextLimit_AC_CTX_CEIL covers AC-CTX-CEIL: ceiling=1200
// caps both auto-mapping (1500) and frontmatter override (2500) to 1200.
func TestResolveSpecReviewContextLimit_AC_CTX_CEIL(t *testing.T) {
	t.Parallel()

	cited4 := []string{"pkg/a.go", "pkg/b.go", "pkg/c.go", "pkg/d.go"}
	projectRoot, specDir := specReviewCtxFixture(t, "SPEC-FAKE-CTX6", cited4, "2500")

	var stderr bytes.Buffer
	const ceiling = 1200
	cited, applied, override, err := resolveSpecReviewContextLimit(projectRoot, specDir, ceiling, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 4, cited)
	assert.Equal(t, 1200, applied,
		"ceiling 1200 must cap frontmatter override 2500 and auto-mapping 1500")
	assert.Equal(t, "frontmatter", override)
	assert.Contains(t, stderr.String(),
		"SPEC review context: cited=4 applied=1200 override=frontmatter ceiling=1200")
}
