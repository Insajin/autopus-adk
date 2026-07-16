package design

import (
	"image/color"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildScreenshotDiffStatsV2_MissingComparisonID_DoesNotCrossPairNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	buttonExpected := filepath.Join(dir, "button-expected.png")
	buttonActual := filepath.Join(dir, "button-actual.png")
	cardExpected := filepath.Join(dir, "card-expected.png")
	cardActual := filepath.Join(dir, "card-actual.png")
	writePNG(t, buttonExpected, color.RGBA{A: 255})
	writePNG(t, buttonActual, color.RGBA{A: 255})
	writePNG(t, cardExpected, color.RGBA{R: 255, A: 255})
	writePNG(t, cardActual, color.RGBA{R: 255, A: 255})

	stats := BuildScreenshotDiffStatsV2([]VisualArtifactV2{
		{Kind: "expected", Path: "shots/button-expected.png", LocalPath: buttonExpected},
		{Kind: "actual", Path: "shots/card-actual.png", LocalPath: cardActual},
		{Kind: "expected", Path: "shots/card-expected.png", LocalPath: cardExpected},
		{Kind: "actual", Path: "shots/button-actual.png", LocalPath: buttonActual},
	})

	assert.Equal(t, 2, stats.PairsCompared)
	assert.Equal(t, int64(0), stats.ChangedPixels)
}

func TestSortImagePairsV2_EqualActualPaths_UsesTotalDeterministicOrder(t *testing.T) {
	t.Parallel()

	pairs := []imagePairV2{
		{actual: VisualArtifactV2{Path: "same.png", ComparisonID: "b"}, expected: VisualArtifactV2{Path: "b.png"}},
		{actual: VisualArtifactV2{Path: "same.png", ComparisonID: "a"}, expected: VisualArtifactV2{Path: "a.png"}},
	}
	sortImagePairsV2(pairs)

	require.Len(t, pairs, 2)
	assert.Equal(t, "a", pairs[0].actual.ComparisonID)
	assert.Equal(t, "b", pairs[1].actual.ComparisonID)
}
