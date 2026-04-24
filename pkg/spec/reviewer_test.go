package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildReviewPrompt_WithSpec(t *testing.T) {
	t.Parallel()

	doc := &SpecDocument{
		ID:    "SPEC-AUTH-001",
		Title: "User Authentication",
		Requirements: []Requirement{
			{ID: "REQ-001", Type: EARSEventDriven, Description: "WHEN user submits credentials THEN system SHALL validate"},
		},
	}

	prompt := BuildReviewPrompt(doc, "// existing auth code\nfunc Login() {}", ReviewPromptOptions{})
	assert.Contains(t, prompt, "SPEC-AUTH-001")
	assert.Contains(t, prompt, "REQ-001")
	assert.Contains(t, prompt, "existing auth code")
}

func TestBuildReviewPrompt_EmptyContext(t *testing.T) {
	t.Parallel()

	doc := &SpecDocument{
		ID:    "SPEC-TEST-001",
		Title: "Test Feature",
	}

	prompt := BuildReviewPrompt(doc, "", ReviewPromptOptions{})
	assert.Contains(t, prompt, "SPEC-TEST-001")
	assert.NotContains(t, prompt, "Existing Code Context")
}

func TestParseVerdict_Pass(t *testing.T) {
	t.Parallel()

	output := `## Review Result
VERDICT: PASS
The SPEC is well-defined and complete.`

	result := ParseVerdict("SPEC-TEST-001", output, "claude", 0, nil)
	assert.Equal(t, VerdictPass, result.Verdict)
	assert.Equal(t, "SPEC-TEST-001", result.SpecID)
}

func TestParseVerdict_Revise(t *testing.T) {
	t.Parallel()

	output := `## Review
VERDICT: REVISE
FINDING: [major] Missing error handling for edge case
FINDING: [minor] Naming could be clearer`

	result := ParseVerdict("SPEC-AUTH-001", output, "claude", 1, nil)
	assert.Equal(t, VerdictRevise, result.Verdict)
	assert.Equal(t, 1, result.Revision)
	assert.Len(t, result.Findings, 2)
	assert.Equal(t, "major", result.Findings[0].Severity)
	assert.Equal(t, "minor", result.Findings[1].Severity)
}

func TestParseVerdict_Reject(t *testing.T) {
	t.Parallel()

	output := `VERDICT: REJECT
FINDING: [critical] Fundamental design flaw`

	result := ParseVerdict("SPEC-X-001", output, "gemini", 0, nil)
	assert.Equal(t, VerdictReject, result.Verdict)
	assert.Len(t, result.Findings, 1)
	assert.Equal(t, "critical", result.Findings[0].Severity)
}

func TestParseVerdict_NoExplicitVerdict(t *testing.T) {
	t.Parallel()

	output := "The spec looks good overall. No major issues found."
	result := ParseVerdict("SPEC-TEST-001", output, "claude", 0, nil)
	assert.Equal(t, VerdictRevise, result.Verdict)
}

func TestParseVerdict_FindingWithoutVerdict(t *testing.T) {
	t.Parallel()

	output := `Some analysis here.
FINDING: [suggestion] Consider adding more tests`

	result := ParseVerdict("SPEC-TEST-001", output, "claude", 0, nil)
	assert.Equal(t, VerdictRevise, result.Verdict)
	assert.Len(t, result.Findings, 1)
}

func TestPersistReview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := &ReviewResult{
		SpecID:  "SPEC-AUTH-001",
		Verdict: VerdictPass,
		Findings: []ReviewFinding{
			{Provider: "claude", Severity: "minor", Description: "Naming improvement"},
		},
		Responses: []string{"Claude says OK", "Gemini agrees"},
		Revision:  0,
	}

	err := PersistReview(dir, result)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "review.md"))
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "SPEC-AUTH-001")
	assert.Contains(t, s, "PASS")
	assert.Contains(t, s, "minor")
	assert.Contains(t, s, "Naming improvement")
}

func TestPersistReview_Overwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Write initial review
	r1 := &ReviewResult{SpecID: "SPEC-X-001", Verdict: VerdictRevise, Revision: 0}
	require.NoError(t, PersistReview(dir, r1))

	// Overwrite with revision
	r2 := &ReviewResult{SpecID: "SPEC-X-001", Verdict: VerdictPass, Revision: 1}
	require.NoError(t, PersistReview(dir, r2))

	content, err := os.ReadFile(filepath.Join(dir, "review.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "PASS")
	assert.Contains(t, string(content), "**Revision**: 1")
}

func TestCollectContext_WithinLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a small Go file
	goFile := filepath.Join(dir, "auth.go")
	if err := os.WriteFile(goFile, []byte("package auth\n\nfunc Login() error {\n\treturn nil\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, err := CollectContext(dir, 500)
	require.NoError(t, err)
	assert.Contains(t, ctx, "func Login()")
}

func TestCollectContext_RecursiveSubdirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create nested directory structure
	subDir := filepath.Join(dir, "pkg", "auth")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "handler.go"), []byte("package auth\n\nfunc Handle() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, err := CollectContext(dir, 500)
	require.NoError(t, err)
	assert.Contains(t, ctx, "func main()")
	assert.Contains(t, ctx, "func Handle()")
	assert.Contains(t, ctx, "pkg/auth/handler.go")
}

func TestCollectContext_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, err := CollectContext(dir, 500)
	require.NoError(t, err)
	assert.Empty(t, ctx)
}

func TestCollectContext_ExceedsLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a large file exceeding the limit
	bigContent := strings.Repeat("line of code\n", 100)
	if err := os.WriteFile(filepath.Join(dir, "big.go"), []byte(bigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, err := CollectContext(dir, 10)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(ctx), "\n")
	// Should be truncated near the limit
	assert.LessOrEqual(t, len(lines), 15) // some overhead for file headers
}

func TestCollectContextForSpec_TargetFilesOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-CTX-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.go"), []byte("package main\n\nfunc Keep() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.go"), []byte("package main\n\nfunc Ignore() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "research.md"), []byte("## Target Files\n| 파일 | 역할 | 변경 필요 |\n|------|------|-----------|\n| keep.go | handler | yes |\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "plan.md"), []byte("File Impact Analysis\n| 파일 | 작업 | 설명 |\n|------|------|------|\n| keep.go | 수정 | core |\n"), 0o644))

	ctx, err := CollectContextForSpec(dir, specDir, 200)
	require.NoError(t, err)
	assert.Contains(t, ctx, "func Keep()")
	assert.NotContains(t, ctx, "func Ignore()")
}

func TestCollectContextForSpec_ModuleRelativePaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	moduleRoot := filepath.Join(root, "apps", "backend")
	specDir := filepath.Join(moduleRoot, ".autopus", "specs", "SPEC-GO-001")
	targetFile := filepath.Join(moduleRoot, "internal", "service", "task_queue_service.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(targetFile), 0o755))
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(targetFile, []byte("package service\n\nfunc TaskQueue() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unrelated.go"), []byte("package main\n\nfunc Unrelated() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "research.md"), []byte("### Target Files\n| 파일 | 역할 | 변경 필요 |\n|------|------|-----------|\n| internal/service/task_queue_service.go | queue | yes |\n"), 0o644))

	ctx, err := CollectContextForSpec(root, specDir, 200)
	require.NoError(t, err)
	assert.Contains(t, ctx, "func TaskQueue()")
	assert.NotContains(t, ctx, "func Unrelated()")
	assert.Contains(t, ctx, "apps/backend/internal/service/task_queue_service.go")
}

func TestCollectContextForSpec_NoTargets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-EMPTY-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "research.md"), []byte("# Empty"), 0o644))

	ctx, err := CollectContextForSpec(dir, specDir, 200)
	require.NoError(t, err)
	assert.Empty(t, ctx)
}

func TestMergeVerdicts_AllPass(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{
		{Verdict: VerdictPass},
		{Verdict: VerdictPass},
	}
	assert.Equal(t, VerdictPass, MergeVerdicts(results, 1.0, 2))
}

func TestMergeVerdicts_AnyReject(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{
		{Verdict: VerdictPass},
		{Verdict: VerdictReject},
	}
	assert.Equal(t, VerdictReject, MergeVerdicts(results, 1.0, 2))
}

func TestMergeVerdicts_AnyRevise(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{
		{Verdict: VerdictPass},
		{Verdict: VerdictRevise},
	}
	assert.Equal(t, VerdictRevise, MergeVerdicts(results, 0.5, 2))
}

func TestMergeVerdicts_Empty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, VerdictPass, MergeVerdicts(nil, 0.67, 0))
}
