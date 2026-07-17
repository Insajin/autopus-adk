package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTreeFile writes content at dir/rel, creating parent directories.
func writeTreeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// TestIsDriftSourceRepo verifies the source-repo gate: only a tree with all
// markers is a source repo; a bare install directory is not (S5 skip branch).
func TestIsDriftSourceRepo(t *testing.T) {
	assert.False(t, isDriftSourceRepo(t.TempDir()), "a bare install dir is not a source repo")

	dir := t.TempDir()
	for _, marker := range []string{"content", "templates", filepath.Join("cmd", "generate-templates")} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, marker), 0o755))
	}
	assert.True(t, isDriftSourceRepo(dir), "all markers present → source repo")
}

// TestDriftTemplateRegen_StaleTemplate is the S5 oracle core: a committed
// template that differs from its freshly regenerated counterpart is reported by
// its slash-form relative path, while byte-identical templates are not.
func TestDriftTemplateRegen_StaleTemplate(t *testing.T) {
	committed := t.TempDir()
	regen := t.TempDir()

	// agent-pipeline template is stale (committed differs from regenerated).
	writeTreeFile(t, committed, "codex/skills/agent-pipeline.md.tmpl", "OLD BODY")
	writeTreeFile(t, regen, "codex/skills/agent-pipeline.md.tmpl", "NEW BODY")
	// a second template is up to date.
	writeTreeFile(t, committed, "codex/skills/other.md.tmpl", "SAME")
	writeTreeFile(t, regen, "codex/skills/other.md.tmpl", "SAME")

	stale := diffRegeneratedTemplates(committed, regen)
	assert.Equal(t, []string{"codex/skills/agent-pipeline.md.tmpl"}, stale)

	check := templateRegenCheck(sourceDriftReport{RegenChecked: true, StaleTemplates: stale})
	assert.Equal(t, "doctor.drift.template_regen", check.ID)
	assert.Equal(t, "warn", check.Status)
	assert.Contains(t, check.Detail, "agent-pipeline.md.tmpl")
	assert.Contains(t, check.Detail, "generate-templates")
}

// TestDriftTemplateRegen_MissingCommitted treats a regenerated file with no
// committed counterpart as stale (a template that was never regenerated).
func TestDriftTemplateRegen_MissingCommitted(t *testing.T) {
	committed := t.TempDir()
	regen := t.TempDir()
	writeTreeFile(t, regen, "gemini/agents/new-agent.md.tmpl", "BODY")

	stale := diffRegeneratedTemplates(committed, regen)
	assert.Equal(t, []string{"gemini/agents/new-agent.md.tmpl"}, stale)
}

// TestCollectSourceDrift_NonSourceRepo_NoChecks verifies S5's skip branch: a
// non-source repo yields no template_regen or binary_stale checks.
func TestCollectSourceDrift_NonSourceRepo_NoChecks(t *testing.T) {
	rep := collectSourceDrift(t.TempDir())
	assert.False(t, rep.IsSourceRepo)
	assert.False(t, rep.RegenChecked, "no template_regen check off a source repo")
	assert.False(t, rep.BinaryChecked, "no binary_stale check off a source repo")
}

// TestCommitIsStale_PrefixComparison is the S6 oracle for INV-005: a 7-char build
// commit that is a prefix of the 40-char HEAD is NOT stale (length difference
// alone is not drift); a non-prefix commit is stale.
func TestCommitIsStale_PrefixComparison(t *testing.T) {
	const buildCommit = "a1b2c3d"
	assert.False(t, commitIsStale(buildCommit, "a1b2c3d9e8f70011223344556677889900aabbcc"),
		"7-char prefix of the full HEAD is not stale")
	assert.True(t, commitIsStale(buildCommit, "f0e1d2c3b4a5000000000000000000000000abcd"),
		"a non-prefix commit is stale")
}

// TestHeadPrefixForDisplay confirms the full HEAD is truncated to the build
// commit width for comparable reporting.
func TestHeadPrefixForDisplay(t *testing.T) {
	assert.Equal(t, "a1b2c3d", headPrefixForDisplay("a1b2c3d9e8f70011223344556677889900aabbcc", 7))
	assert.Equal(t, "f0e1d2c", headPrefixForDisplay("f0e1d2c3b4a5000000000000000000000000abcd", 7))
	assert.Equal(t, "short", headPrefixForDisplay("short", 7), "no panic when HEAD is shorter than width")
}

// TestBinaryStaleCheck_OracleValues is the S6 oracle for the JSON check: the warn
// branch names the build commit, the truncated HEAD prefix, and the rebuild hint;
// the pass branch reports a match.
func TestBinaryStaleCheck_OracleValues(t *testing.T) {
	warn := binaryStaleCheck(sourceDriftReport{
		BinaryChecked: true, BinaryStale: true, BuildCommit: "a1b2c3d", HeadPrefix: "f0e1d2c",
	})
	assert.Equal(t, "doctor.drift.binary_stale", warn.ID)
	assert.Equal(t, "warn", warn.Status)
	assert.Contains(t, warn.Detail, "a1b2c3d")
	assert.Contains(t, warn.Detail, "f0e1d2c")
	assert.Contains(t, warn.Detail, "go build")

	pass := binaryStaleCheck(sourceDriftReport{
		BinaryChecked: true, BinaryStale: false, BuildCommit: "a1b2c3d", HeadPrefix: "a1b2c3d",
	})
	assert.Equal(t, "pass", pass.Status)
}

// TestApplyBinaryStaleness_NonGitRepo_GracefulSkip verifies F-005: against a dir
// that is not a git repo, the binary check is skipped without a check or error.
func TestApplyBinaryStaleness_NonGitRepo_GracefulSkip(t *testing.T) {
	rep := sourceDriftReport{}
	applyBinaryStaleness(t.TempDir(), &rep)
	assert.False(t, rep.BinaryChecked, "git unavailable or non-repo → graceful skip")
}
