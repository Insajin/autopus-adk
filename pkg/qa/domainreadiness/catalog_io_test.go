package domainreadiness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveCatalogPathDefaultsAndAbsolute asserts path resolution rules.
func TestResolveCatalogPathDefaultsAndAbsolute(t *testing.T) {
	t.Parallel()

	// Empty catalog path falls back to DefaultCatalogPath under project dir.
	got := ResolveCatalogPath("proj", "")
	assert.Equal(t, filepath.Clean(filepath.Join("proj", filepath.FromSlash(DefaultCatalogPath))), got)

	// Absolute path is returned cleaned and unchanged in directory.
	abs := filepath.Join(t.TempDir(), "catalog.json")
	assert.Equal(t, filepath.Clean(abs), ResolveCatalogPath("ignored", abs))

	// Empty project dir defaults to current directory.
	rel := ResolveCatalogPath("", "sub/catalog.json")
	assert.Equal(t, filepath.Clean(filepath.FromSlash("sub/catalog.json")), rel)
}

// TestWriteStarterCatalogCreatesFileThenRejectsOverwrite asserts the writer creates
// a valid catalog file once and refuses to clobber it.
func TestWriteStarterCatalogCreatesFileThenRejectsOverwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "qa-catalog.json")

	path, err := WriteStarterCatalog(dir, target)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(target), path)

	body, err := os.ReadFile(path)
	require.NoError(t, err)
	var catalog Catalog
	require.NoError(t, json.Unmarshal(body, &catalog))
	assert.Equal(t, CatalogSchemaVersion, catalog.SchemaVersion)
	require.NotEmpty(t, catalog.Scenarios)

	// Second write must error because the file already exists.
	_, err = WriteStarterCatalog(dir, target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestWriteStarterCatalogRejectsGeneratedSurface asserts generated paths are denied.
func TestWriteStarterCatalogRejectsGeneratedSurface(t *testing.T) {
	t.Parallel()

	_, err := WriteStarterCatalog(t.TempDir(), filepath.Join(".autopus", "qa", "runs", "c.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generated surface")
}

// TestLoadCatalogFileRoundTrips asserts a written catalog reloads from disk.
func TestLoadCatalogFileRoundTrips(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "catalog.json")
	_, err := WriteStarterCatalog(dir, target)
	require.NoError(t, err)

	catalog, err := LoadCatalogFile(target)
	require.NoError(t, err)
	assert.Equal(t, CatalogSchemaVersion, catalog.SchemaVersion)
}

// TestLoadCatalogFileErrorsOnMissingFile asserts a non-existent path errors.
func TestLoadCatalogFileErrorsOnMissingFile(t *testing.T) {
	t.Parallel()

	_, err := LoadCatalogFile(filepath.Join(t.TempDir(), "nope.json"))
	require.Error(t, err)
}

// TestLoadCatalogFileErrorsOnInvalidJSON asserts malformed JSON is rejected.
func TestLoadCatalogFileErrorsOnInvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))

	_, err := LoadCatalogFile(path)
	require.Error(t, err)
}

// TestValidateCatalogSourceDeniesGeneratedManifests asserts manifest guard.
func TestValidateCatalogSourceDeniesGeneratedManifests(t *testing.T) {
	t.Parallel()

	require.Error(t, ValidateCatalogSource(".autopus-foo-manifest.json"))
	require.Error(t, ValidateCatalogSource(filepath.Join("repo", ".agents", "hooks.json")))
	require.NoError(t, ValidateCatalogSource(filepath.Join("repo", "catalog.json")))
}
