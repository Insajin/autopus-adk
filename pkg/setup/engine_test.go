package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_GoProject(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	ds, err := Generate(projectDir, nil)
	require.NoError(t, err)
	require.NotNil(t, ds)

	// Verify docs directory created
	docsDir := filepath.Join(projectDir, ".autopus/docs")
	assert.DirExists(t, docsDir)

	// Verify all 7 doc files + .meta.yaml
	for _, fileName := range DocFiles {
		assert.FileExists(t, filepath.Join(docsDir, fileName))
	}
	assert.FileExists(t, filepath.Join(docsDir, ".meta.yaml"))

	// Verify index.md content
	data, err := os.ReadFile(filepath.Join(docsDir, "index.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "Go")
}

func TestGenerate_AlreadyExists_NoForce(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	// First generate
	_, err := Generate(projectDir, nil)
	require.NoError(t, err)

	// Second generate without --force
	_, err = Generate(projectDir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestGenerate_AlreadyExists_WithForce(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	_, err := Generate(projectDir, nil)
	require.NoError(t, err)

	// Force overwrite
	_, err = Generate(projectDir, &GenerateOptions{Force: true})
	assert.NoError(t, err)
}

func TestGenerate_CustomOutputDir(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)
	outputDir := filepath.Join(t.TempDir(), "custom-docs")

	_, err := Generate(projectDir, &GenerateOptions{OutputDir: outputDir})
	require.NoError(t, err)

	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "index.md"))
}

func TestUpdate_RegeneratesChanged(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	// Initial generate
	_, err := Generate(projectDir, nil)
	require.NoError(t, err)

	// Modify a source file
	writeFile(t, projectDir, "Makefile", "build:\n\tgo build ./...\n\ntest:\n\tgo test -race ./...\n")

	updated, err := Update(projectDir, "")
	require.NoError(t, err)

	// At least commands.md should be updated
	assert.NotEmpty(t, updated)
}

func TestUpdate_NoChanges(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	_, err := Generate(projectDir, nil)
	require.NoError(t, err)

	updated, err := Update(projectDir, "")
	require.NoError(t, err)
	assert.Empty(t, updated)
}

func TestUpdate_CorruptedMeta(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	// Create docs with corrupted meta
	docsDir := filepath.Join(projectDir, ".autopus/docs")
	require.NoError(t, os.MkdirAll(docsDir, 0755))
	writeFile(t, projectDir, ".autopus/docs/.meta.yaml", ":::bad yaml:::")

	// Should trigger full regeneration
	updated, err := Update(projectDir, "")
	require.NoError(t, err)
	// Full regeneration returns a single entry indicating all were regenerated
	assert.NotEmpty(t, updated)
}

func TestGetStatus_NoDocs(t *testing.T) {
	t.Parallel()
	projectDir := t.TempDir()

	status, err := GetStatus(projectDir, "")
	require.NoError(t, err)
	assert.False(t, status.Exists)
	assert.False(t, status.ProjectContext.Exists)
}

func TestGetStatus_ProjectContextWithoutDocs(t *testing.T) {
	t.Parallel()
	projectDir := t.TempDir()
	for _, name := range requiredProjectContextFiles {
		writeFile(t, projectDir, filepath.Join(".autopus", "project", name), "# "+name+"\n")
	}

	status, err := GetStatus(projectDir, "")
	require.NoError(t, err)
	assert.False(t, status.Exists, ".autopus/docs bundle is still absent")
	assert.True(t, status.ProjectContext.Exists)
	assert.Empty(t, status.ProjectContext.MissingFiles)
	assert.Contains(t, status.ProjectContext.Files, "workspace.md")
}

func TestValidateProjectContext_MissingRequiredFile(t *testing.T) {
	t.Parallel()
	projectDir := t.TempDir()
	writeFile(t, projectDir, ".autopus/project/workspace.md", "# workspace\n")

	report := ValidateProjectContext(projectDir)
	assert.False(t, report.Valid)
	assert.NotEmpty(t, report.Warnings)
	assert.Contains(t, report.Warnings[0].Type, "missing_project_context")
}

func TestGetStatus_WithDocs(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	_, err := Generate(projectDir, nil)
	require.NoError(t, err)

	status, err := GetStatus(projectDir, "")
	require.NoError(t, err)

	assert.True(t, status.Exists)
	assert.False(t, status.GeneratedAt.IsZero())
	assert.Equal(t, 0.0, status.DriftScore)

	for _, fs := range status.FileStatuses {
		assert.True(t, fs.Exists)
		assert.True(t, fs.Fresh)
	}
}

func TestGetStatus_StaleDocs(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	_, err := Generate(projectDir, nil)
	require.NoError(t, err)

	// Add a new build file to create drift
	writeFile(t, projectDir, "Makefile", "build:\n\tgo build ./...\n")

	status, err := GetStatus(projectDir, "")
	require.NoError(t, err)

	assert.True(t, status.Exists)
	assert.Greater(t, status.DriftScore, 0.0)
}
