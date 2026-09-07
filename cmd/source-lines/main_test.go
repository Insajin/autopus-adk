package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceLimitExcludesCommentsButStillRejectsCodeOverLimit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.go")
	data := "package p\n" + strings.Repeat("var _ = 1\n", 299) + strings.Repeat("// docs\n", 400)
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
	var out, errOut bytes.Buffer
	assert.Equal(t, 0, run([]string{"--max", "300", root}, &out, &errOut), errOut.String())
	require.NoError(t, os.WriteFile(path, []byte(data+"var _ = 1\n"), 0o600))
	out.Reset()
	errOut.Reset()
	assert.Equal(t, 1, run([]string{"--max", "300", root}, &out, &errOut))
	assert.Contains(t, out.String(), "301")
	assert.Contains(t, out.String(), "file.go")
}

func TestSourceLimitExtensionFilterKeepsReleaseScope(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "small.go"), []byte("package p\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "large.sh"), []byte(strings.Repeat("echo code\n", 301)), 0o600))
	var out, errOut bytes.Buffer
	assert.Equal(t, 0, run([]string{"--ext", ".go", root}, &out, &errOut), errOut.String())
	assert.Equal(t, 1, run([]string{filepath.Join(root, "large.sh")}, &out, &errOut))
}

func TestSourceLimitCannotPassMissingInputs(t *testing.T) {
	var out, errOut bytes.Buffer
	assert.NotEqual(t, 0, run([]string{filepath.Join(t.TempDir(), "missing.go")}, &out, &errOut))
	assert.NotEmpty(t, errOut.String())
}
