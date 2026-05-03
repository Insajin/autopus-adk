package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveArtifactPaths_WithEmptyBaseDirReturnsManifest(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "package")

	resolved, err := ResolveArtifactPaths(manifest, "")

	require.NoError(t, err)
	assert.Equal(t, manifest.Artifacts[0].Path, resolved.Artifacts[0].Path)
}

func TestResolveArtifactPaths_ResolvesRelativeArtifactInsideBaseDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(base, "stdout.log"), []byte("ok\n"), 0o644))
	manifest := fixtureV2Manifest(t, "package")
	manifest.Artifacts[0].Path = "stdout.log"

	resolved, err := ResolveArtifactPaths(manifest, base)

	require.NoError(t, err)
	want, err := realAbsPath(filepath.Join(base, "stdout.log"))
	require.NoError(t, err)
	assert.Equal(t, want, resolved.Artifacts[0].Path)
}

func TestWriteFinalManifest_ReplacesExistingEmptyOutputDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	output := filepath.Join(dir, "final")
	require.NoError(t, os.Mkdir(output, 0o755))

	manifestPath, err := WriteFinalManifest(fixtureV2Manifest(t, "package"), output)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(output, "manifest.json"), manifestPath)
	assert.FileExists(t, filepath.Join(output, "manifest.json"))
}

func TestWriteFinalManifest_RejectsExistingNonEmptyOutputDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	output := filepath.Join(dir, "final")
	require.NoError(t, os.Mkdir(output, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(output, "existing.txt"), []byte("keep\n"), 0o644))

	_, err := WriteFinalManifest(fixtureV2Manifest(t, "package"), output)

	require.ErrorContains(t, err, "output directory must be empty")
	assert.FileExists(t, filepath.Join(output, "existing.txt"))
}

func TestValidateLocatorConvention_AcceptsStableTestIDFallback(t *testing.T) {
	t.Parallel()

	result := ValidateLocatorConvention([]LocatorContract{{
		Strategy:       "testid",
		Value:          "checkout-submit",
		FallbackReason: "third-party widget lacks semantic role",
		StableTestID:   "checkout-submit",
	}})

	assert.Equal(t, []string{"testid"}, result.Accepted)
	assert.Empty(t, result.Rejected)
	assert.False(t, result.FallbackReasonRequired)
	assert.False(t, result.StableTestIDRequired)
}
