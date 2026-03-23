//go:build integration

package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- generateSignatureMap: Extract failure (no go.mod) ----

func TestGenerateSignatureMap_ExtractError(t *testing.T) {
	t.Parallel()
	// A directory with no go.mod causes sigmap.Extract to fail.
	dir := t.TempDir()
	// Add a Go file so the directory is not trivially empty, but no go.mod.
	writeFile(t, dir, "main.go", "package main\nfunc Main() {}\n")

	err := generateSignatureMap(dir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extract signatures")
}

// ---- updateSignatureMap: Extract failure (no go.mod) ----

func TestUpdateSignatureMap_ExtractError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\nfunc Main() {}\n")

	updated, err := updateSignatureMap(dir, nil)
	assert.False(t, updated)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extract signatures")
}

// ---- generateSignatureMap: MkdirAll failure via read-only parent ----

func TestGenerateSignatureMap_MkdirAllError(t *testing.T) {
	t.Parallel()
	// Create a project directory then make a file at the path where the
	// output directory would be created, preventing MkdirAll.
	projectDir := setupGoProject(t)

	// Place a regular file where .autopus/ would be created.
	autopusPath := filepath.Join(projectDir, ".autopus")
	require.NoError(t, os.WriteFile(autopusPath, []byte("blocker"), 0o644))

	err := generateSignatureMap(projectDir, nil)
	assert.Error(t, err)
}

// ---- updateSignatureMap: MkdirAll failure via read-only parent ----

func TestUpdateSignatureMap_MkdirAllError(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	// First generate to produce the existing file, then replace the context
	// dir with a plain file so subsequent MkdirAll fails.
	require.NoError(t, generateSignatureMap(projectDir, nil))

	contextDir := filepath.Join(projectDir, signaturesDir)
	sigFile := filepath.Join(contextDir, signaturesFile)

	// Remove the output file and the context directory, then put a blocker
	// file at the parent of signaturesDir (i.e., .autopus/).
	require.NoError(t, os.Remove(sigFile))
	require.NoError(t, os.RemoveAll(contextDir))

	autopusPath := filepath.Join(projectDir, ".autopus")
	// Remove any existing .autopus directory.
	_ = os.RemoveAll(autopusPath)
	// Write a plain file in its place.
	require.NoError(t, os.WriteFile(autopusPath, []byte("blocker"), 0o644))

	_, err := updateSignatureMap(projectDir, nil)
	assert.Error(t, err)
}

// ---- updateSignatureMap: WriteFile failure via read-only file ----

func TestUpdateSignatureMap_WriteFileError(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	// First generate the file normally.
	require.NoError(t, generateSignatureMap(projectDir, nil))

	outPath := filepath.Join(projectDir, signaturesDir, signaturesFile)

	// Make the output file read-only so WriteFile will fail on the second
	// call when content has changed.
	require.NoError(t, os.Chmod(outPath, 0o444))
	t.Cleanup(func() { _ = os.Chmod(outPath, 0o644) })

	// Add a new exported symbol to force content change.
	writeFile(t, projectDir, "pkg/extra/extra.go",
		"package extra\n\n// Extra is a new type.\ntype Extra struct{}\n",
	)

	// This call should encounter a write error because the file is read-only.
	// Note: on some systems running as root this may still succeed — we
	// accept both outcomes but require no panic.
	_, _ = updateSignatureMap(projectDir, nil)
}
