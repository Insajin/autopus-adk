package spec_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestResolveSpecDir_TopLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-AUTH-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# SPEC-AUTH-001: Auth"), 0o644))

	result, err := spec.ResolveSpecDir(dir, "SPEC-AUTH-001")
	require.NoError(t, err)
	assert.Equal(t, specDir, result.SpecDir)
	assert.Equal(t, ".", result.TargetModule)
}

func TestResolveSpecDir_Submodule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, "autopus-adk", ".autopus", "specs", "SPEC-PIPE-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# SPEC-PIPE-001: Pipeline"), 0o644))

	result, err := spec.ResolveSpecDir(dir, "SPEC-PIPE-001")
	require.NoError(t, err)
	assert.Equal(t, specDir, result.SpecDir)
	assert.Equal(t, "autopus-adk", result.TargetModule)
}

func TestResolveSpecDir_WorkspaceRootFindsAutopusSubmoduleSpec(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, "Autopus", ".autopus", "specs", "SPEC-OPCOCK-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# SPEC-OPCOCK-001: OpenCode Cockpit"), 0o644))

	result, err := spec.ResolveSpecDir(dir, "SPEC-OPCOCK-001")
	require.NoError(t, err)
	assert.Equal(t, specDir, result.SpecDir)
	assert.Equal(t, "Autopus", result.TargetModule)
}

func TestResolveSpecDir_NestedSubmoduleDepth2(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, "apps", "backend", ".autopus", "specs", "SPEC-GO-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# SPEC-GO-001: Backend"), 0o644))

	result, err := spec.ResolveSpecDir(dir, "SPEC-GO-001")
	require.NoError(t, err)
	assert.Equal(t, specDir, result.SpecDir)
	assert.Equal(t, filepath.Join("apps", "backend"), result.TargetModule)
}

func TestResolveSpecDir_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := spec.ResolveSpecDir(dir, "SPEC-MISSING-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveSpecDir_NotFoundWithAvailable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, "mymod", ".autopus", "specs", "SPEC-EXIST-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# SPEC-EXIST-001: Existing"), 0o644))

	_, err := spec.ResolveSpecDir(dir, "SPEC-OTHER-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SPEC-EXIST-001")
}

func TestResolveSpecDir_Duplicate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create same SPEC-ID in two locations
	topDir := filepath.Join(dir, ".autopus", "specs", "SPEC-DUP-001")
	require.NoError(t, os.MkdirAll(topDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(topDir, "spec.md"), []byte("# SPEC-DUP-001: Top"), 0o644))

	subDir := filepath.Join(dir, "submod", ".autopus", "specs", "SPEC-DUP-001")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "spec.md"), []byte("# SPEC-DUP-001: Sub"), 0o644))

	_, err := spec.ResolveSpecDir(dir, "SPEC-DUP-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestResolveSpecDir_SkipsHiddenDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Place SPEC inside a hidden directory — should not be found
	hiddenDir := filepath.Join(dir, ".hidden", ".autopus", "specs", "SPEC-HIDE-001")
	require.NoError(t, os.MkdirAll(hiddenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "spec.md"), []byte("# SPEC-HIDE-001"), 0o644))

	_, err := spec.ResolveSpecDir(dir, "SPEC-HIDE-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
