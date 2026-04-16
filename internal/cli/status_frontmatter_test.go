package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCmd_ReadsYAMLFrontmatterStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specsDir := filepath.Join(dir, ".autopus", "specs")

	writeSpecFile(t, specsDir, "SPEC-FRONT-001", `# SPEC-FRONT-001: Frontmatter Status

---
id: SPEC-FRONT-001
title: Frontmatter Status
status: approved
---
`)

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--dir", dir})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "SPEC-FRONT-001")
	assert.Contains(t, output, "approved")
	assert.Contains(t, output, "0/1 완료")
}
