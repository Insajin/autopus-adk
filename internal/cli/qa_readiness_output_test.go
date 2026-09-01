package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// D10 regression: with no evidence yet, default resolution produced an empty
// path that surfaced as a path-less `open : no such file or directory`. The
// error must name the command that produces the missing index.
func TestQAReadinessCmd_NoEvidence_NamesTheProducingCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"qa", "readiness", "--workspace-root", dir, "--format", "json"})

	require.Error(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "error", payload["status"])

	errObj := payload["error"].(map[string]any)
	assert.Equal(t, "qa_readiness_no_evidence", errObj["code"])
	message := errObj["message"].(string)
	assert.Contains(t, message, "auto qa run")
	assert.Contains(t, message, filepath.Join(dir, ".autopus", "qa", "runs"))
	assert.NotContains(t, message, "open :")
}

// A workspace with runs but no release must point at the release command rather
// than blaming the run index.
func TestQAReadinessCmd_NoReleaseIndex_NamesTheReleaseCommand(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join("..", "..", "testdata", "qa", "readiness", "non_autopus_fixture")
	dir := t.TempDir()
	copyFixtureFile(t, filepath.Join(fixture, "qa", "run-index.json"),
		filepath.Join(dir, ".autopus", "qa", "runs", "run-001", "run-index.json"))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"qa", "readiness", "--workspace-root", dir, "--format", "json"})

	require.Error(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	errObj := payload["error"].(map[string]any)
	assert.Equal(t, "qa_readiness_no_evidence", errObj["code"])
	assert.Contains(t, errObj["message"], "auto qa release")
	assert.Contains(t, errObj["message"], "--release-index")
}

// D22 regression: the text output was "<verdict> <timestamp>" while the JSON
// payload carried lanes, setup gaps, evidence refs, and a trend summary.
func TestQAReadinessCmd_TextOutputSummarizesProjection(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join("..", "..", "testdata", "qa", "readiness", "non_autopus_fixture")
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"qa", "readiness",
		"--workspace-root", fixture,
		"--repo-root", filepath.Join(fixture, "repos", "portable-shop"),
		"--workspace-id", "fixture-workspace",
		"--repo-id", "portable-shop",
		"--run-index", filepath.Join(fixture, "qa", "run-index.json"),
		"--release-index", filepath.Join(fixture, "qa", "release-index.json"),
	})
	require.NoError(t, cmd.Execute())

	text := out.String()
	assert.Contains(t, text, "qa readiness blocked lanes=6 checks=3 passed=1 failed=1")
	assert.Contains(t, text, "lane: browser-staging failed")
	assert.Contains(t, text, "setup_gap: ")
	assert.Contains(t, text, "evidence: qa/evidence/manifests/login.json")
	assert.Contains(t, text, "trend: ")
	assert.Contains(t, text, "next: auto qa report")
}
