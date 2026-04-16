package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCmd_ReadsLegacySpecIDAndHeadingTitle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specsDir := filepath.Join(dir, ".autopus", "specs")

	writeSpecFile(t, specsDir, "SPEC-ADKWA-001", `# SPEC: ADK Worker Approval Flow

**SPEC-ID**: SPEC-ADKWA-001
**Status**: completed
**Created**: 2026-04-16
`)

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--dir", dir})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "SPEC-ADKWA-001")
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, "ADK Worker Approval Flow")
	assert.Contains(t, output, "1/1 완료")
}
