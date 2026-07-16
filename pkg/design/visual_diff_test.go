package design

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildScreenshotDiffStatsPairsSameComparisonAcrossDifferentDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	expected := filepath.Join(dir, "baselines", "home-default.png")
	actual := filepath.Join(dir, "test-results", "run-42", "home-default-actual.png")
	writePNGInCreatedDir(t, expected, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	writePNGInCreatedDir(t, actual, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	stats := BuildScreenshotDiffStatsV2([]VisualArtifactV2{
		{
			Name:         "home-default-expected.png",
			Kind:         "expected",
			Path:         "baselines/home-default.png",
			LocalPath:    expected,
			ComparisonID: "desktop/test-home/home-default.png",
		},
		{
			Name:         "home-default-actual.png",
			Kind:         "actual",
			Path:         "test-results/run-42/home-default-actual.png",
			LocalPath:    actual,
			ComparisonID: "desktop/test-home/home-default.png",
		},
	})

	assert.Equal(t, 1, stats.PairsCompared)
	assert.Equal(t, int64(4), stats.ChangedPixels)
	assert.Equal(t, int64(4), stats.TotalPixels)
}

func writePNGInCreatedDir(t *testing.T, path string, c color.RGBA) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	writePNG(t, path, c)
}
