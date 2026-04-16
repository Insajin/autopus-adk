package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd_PathFlagPrintsCanonicalPath(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"version", "--path"})
	require.NoError(t, cmd.Execute())
	resolved := strings.TrimSpace(buf.String())
	assert.NotEmpty(t, resolved)
	assert.True(t, filepath.IsAbs(resolved))
	assert.Equal(t, filepath.Clean(resolved), resolved)
}
