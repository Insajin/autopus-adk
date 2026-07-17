package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLI_LearnQuery_SpecAndSkips(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(cwd)
	}()

	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))

	learningsDir := filepath.Join(tempDir, ".autopus", "learnings")
	require.NoError(t, os.MkdirAll(learningsDir, 0o755))
	jsonlPath := filepath.Join(learningsDir, "pipeline.jsonl")

	raw := `{"id":"L-001","timestamp":"2026-06-14T14:31:55Z","spec_id":"SPEC-A","pattern":"pattern A"}
{"id":"L-BAD","timestamp":
{"id":"L-002","timestamp":"2026-06-14T15:31:55Z","spec_id":"SPEC-B","pattern":"pattern B"}
{"id":"L-003","timestamp":"2026-06-14T16:31:55Z","spec_id":"SPEC-A","pattern":"pattern A2"}
{"id":"L-004","timestamp":"2026-06-14T17:31:55Z","pattern":"pattern no-spec"}
`
	require.NoError(t, os.WriteFile(jsonlPath, []byte(raw), 0o644))

	cmd := newLearnQueryCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--spec", "SPEC-A"})

	err = cmd.Execute()
	require.NoError(t, err)

	stderrStr := stderr.String()
	stdoutStr := stdout.String()

	assert.Contains(t, stderrStr, "WARNING: skipped 1 parsing error(s) at line(s): 2")
	assert.Contains(t, stdoutStr, "L-001")
	assert.Contains(t, stdoutStr, "L-003")
	// S5 exact-match filter: SPEC-B and the entry with no spec_id at all
	// must both be excluded, not just other SPEC IDs.
	assert.NotContains(t, stdoutStr, "L-002")
	assert.NotContains(t, stdoutStr, "L-004")
}
