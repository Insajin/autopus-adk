package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanaryBuildTargets_ContainsExpectedIDs verifies six build targets with expected IDs.
func TestCanaryBuildTargets_ContainsExpectedIDs(t *testing.T) {
	t.Parallel()

	targets := canaryBuildTargets("/workspace")
	ids := make([]string, 0, len(targets))
	for _, tgt := range targets {
		ids = append(ids, tgt.ID)
	}
	assert.Contains(t, ids, "H1")
	assert.Contains(t, ids, "H2")
	assert.Contains(t, ids, "H4")
	assert.Contains(t, ids, "H5a")
	assert.Len(t, targets, 6)
}

// TestCanaryBuildTargets_RootDirInjected verifies projectDir is embedded in Dir paths.
func TestCanaryBuildTargets_RootDirInjected(t *testing.T) {
	t.Parallel()

	targets := canaryBuildTargets("/myroot")
	found := false
	for _, tgt := range targets {
		if strings.HasPrefix(tgt.Dir, "/myroot/") || tgt.Dir == filepath.Join("/myroot", "autopus-adk") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected at least one target with /myroot prefix")
}

// TestErrOrDefault_UseErrWhenPresent returns the existing error unchanged.
func TestErrOrDefault_UseErrWhenPresent(t *testing.T) {
	t.Parallel()

	original := errors.New("original")
	got := errOrDefault(original, "fallback message")
	assert.Equal(t, original, got)
}

// TestErrOrDefault_NilErrReturnsFallback wraps the message in a new error.
func TestErrOrDefault_NilErrReturnsFallback(t *testing.T) {
	t.Parallel()

	got := errOrDefault(nil, "fallback message")
	require.Error(t, got)
	assert.Equal(t, "fallback message", got.Error())
}

// TestPrintCanaryText_RendersVerdictAndSummary verifies text output format.
func TestPrintCanaryText_RendersVerdictAndSummary(t *testing.T) {
	t.Parallel()

	cmd := newAgentCmd() // any cobra.Command with a stdout
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	result := canaryResult{
		Verdict: "PASS",
		Summary: map[string]string{
			"build": "PASS",
			"e2e":   "SKIP",
		},
	}
	printCanaryText(cmd, result)

	out := buf.String()
	assert.Contains(t, out, "canary PASS")
	assert.Contains(t, out, "build")
	assert.Contains(t, out, "PASS")
}

// TestWriteCanaryLatest_PersistsJSON verifies the file is written as parseable JSON.
func TestWriteCanaryLatest_PersistsJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := canaryResult{
		Verdict: "PASS",
		Summary: map[string]string{"build": "PASS"},
	}
	require.NoError(t, writeCanaryLatest(dir, result))

	data, err := os.ReadFile(filepath.Join(dir, ".autopus", "canary", "latest.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"verdict"`)
	assert.Contains(t, string(data), "PASS")
}

// TestRunCanaryExternal_EmptyCommand returns FAIL with descriptive detail.
func TestRunCanaryExternal_EmptyCommand(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	result := runCanaryExternal(ctx, "H-test", "empty command test", t.TempDir())
	assert.Equal(t, "FAIL", result.Status)
	assert.Contains(t, result.Detail, "empty command")
}
