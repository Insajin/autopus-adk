package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPlaywright_InvalidOrMissingJSONReport_ReturnsHardErrorWithProofEvidence(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
	}{
		{name: "missing", stdout: ""},
		{name: "invalid", stdout: "not-json"},
		{name: "non-object", stdout: "[]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runPlaywrightWithRawStdout(t, tt.stdout)

			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), "json")
			evidence := collectVisualEvidence(output)
			assert.Equal(t, "enabled", evidence.SnapshotProofStatus)
			assert.Equal(t, []string{"chromium"}, evidence.RequiredProjects)
		})
	}
}

func TestRunPlaywright_InvalidBlobReport_ReturnsHardErrorWithProofEvidence(t *testing.T) {
	blob := buildBlobReportBytes(t, []byte("not-json\n"), nil)
	blobPath := filepath.Join(t.TempDir(), "invalid-report.zip")
	require.NoError(t, os.WriteFile(blobPath, blob, 0o600))

	run, err := runPlaywrightWithCapturedArgsMode(t, "desktop", blobPath, "enabled")

	require.Error(t, err)
	evidence := collectVisualEvidence(run.Output)
	assert.Equal(t, "enabled", evidence.SnapshotProofStatus)
	assert.Equal(t, []string{"chromium"}, evidence.RequiredProjects)
}

func TestAppendSnapshotProofToBlob_RejectsArchiveThatExceedsFinalLimit(t *testing.T) {
	blob := buildBlobReportBytes(t, []byte("{}\n"), nil)
	proof := snapshotComparisonProof{Version: 2, Nonce: "nonce", PlaywrightVersion: "1.59.1"}

	_, err := appendSnapshotProofToBlobWithLimit(blob, proof, len(blob))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "제한")
}

func runPlaywrightWithRawStdout(t *testing.T, stdout string) ([]byte, error) {
	t.Helper()
	dir := t.TempDir()
	npxPath := filepath.Join(dir, "npx")
	stdoutPath := filepath.Join(dir, "stdout.txt")
	require.NoError(t, os.WriteFile(stdoutPath, []byte(stdout), 0o600))
	script := `#!/bin/sh
printf '{"version":2,"nonce":"%s","playwright_version":"1.59.1","update_snapshots":"none","projects":[{"name":"chromium","ignore_snapshots":false,"state":"enabled","source":"public"}]}' "$AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_NONCE" > "$AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_FILE"
/bin/cat "$VERIFY_STDOUT_FIXTURE"
`
	require.NoError(t, os.WriteFile(npxPath, []byte(script), 0o755))
	t.Setenv("VERIFY_STDOUT_FIXTURE", stdoutPath)
	t.Setenv("PATH", dir)
	return runPlaywright("desktop")
}
