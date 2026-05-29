package design

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func BuildScreenshotDiffStats(artifacts []VisualArtifact) ScreenshotDiffStats {
	stats := ScreenshotDiffStats{DeterministicMode: "actual_expected_pixel_compare"}
	for _, artifact := range artifacts {
		if artifact.Kind == "diff" {
			stats.DiffArtifactRefs = append(stats.DiffArtifactRefs, artifact.Path)
		}
	}
	for _, pair := range pairActualExpected(artifacts) {
		diff, err := CompareImageFiles(pair.actual.LocalPath, pair.expected.LocalPath)
		if err != nil {
			stats.ComparisonErrors = append(stats.ComparisonErrors, fmt.Sprintf("%s: %s", pair.actual.Path, publicImageDiffError(err, pair)))
			continue
		}
		stats.PairsCompared++
		stats.ChangedPixels += diff.ChangedPixels
		stats.TotalPixels += diff.TotalPixels
		stats.MaxChangedRatio = math.Max(stats.MaxChangedRatio, diff.ChangedRatio)
	}
	sort.Strings(stats.DiffArtifactRefs)
	sort.Strings(stats.ComparisonErrors)
	return stats
}

func publicImageDiffError(err error, pair imagePair) string {
	message := err.Error()
	if pair.actual.LocalPath != "" {
		message = strings.ReplaceAll(message, pair.actual.LocalPath, pair.actual.Path)
	}
	if pair.expected.LocalPath != "" {
		message = strings.ReplaceAll(message, pair.expected.LocalPath, pair.expected.Path)
	}
	return message
}

type ImageDiff struct {
	ChangedPixels int64
	TotalPixels   int64
	ChangedRatio  float64
}

func CompareImageFiles(actualPath, expectedPath string) (ImageDiff, error) {
	if actualPath == "" || expectedPath == "" {
		return ImageDiff{}, fmt.Errorf("missing actual or expected path")
	}
	actual, err := decodeImage(actualPath)
	if err != nil {
		return ImageDiff{}, fmt.Errorf("decode actual: %w", err)
	}
	expected, err := decodeImage(expectedPath)
	if err != nil {
		return ImageDiff{}, fmt.Errorf("decode expected: %w", err)
	}
	if !actual.Bounds().Size().Eq(expected.Bounds().Size()) {
		return ImageDiff{}, fmt.Errorf("dimension mismatch actual=%v expected=%v", actual.Bounds().Size(), expected.Bounds().Size())
	}
	bounds := actual.Bounds()
	var changed int64
	var total int64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			total++
			ar, ag, ab, aa := actual.At(x, y).RGBA()
			er, eg, eb, ea := expected.At(x, y).RGBA()
			if ar != er || ag != eg || ab != eb || aa != ea {
				changed++
			}
		}
	}
	ratio := 0.0
	if total > 0 {
		ratio = float64(changed) / float64(total)
	}
	return ImageDiff{ChangedPixels: changed, TotalPixels: total, ChangedRatio: ratio}, nil
}

func ClassifyVisualArtifact(name, path string) string {
	lower := strings.ToLower(name + " " + filepath.ToSlash(path))
	switch {
	case strings.Contains(lower, "expected"):
		return "expected"
	case strings.Contains(lower, "actual"):
		return "actual"
	case strings.Contains(lower, "diff"):
		return "diff"
	case strings.Contains(lower, "screenshot") || strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg"):
		return "screenshot"
	default:
		return "other"
	}
}

type imagePair struct {
	actual   VisualArtifact
	expected VisualArtifact
}

func pairActualExpected(artifacts []VisualArtifact) []imagePair {
	byDir := map[string]map[string]VisualArtifact{}
	for _, artifact := range artifacts {
		if artifact.LocalPath == "" || (artifact.Kind != "actual" && artifact.Kind != "expected") {
			continue
		}
		dir := filepath.Dir(artifact.LocalPath)
		if byDir[dir] == nil {
			byDir[dir] = map[string]VisualArtifact{}
		}
		byDir[dir][artifact.Kind] = artifact
	}
	var pairs []imagePair
	for _, group := range byDir {
		actual, hasActual := group["actual"]
		expected, hasExpected := group["expected"]
		if hasActual && hasExpected {
			pairs = append(pairs, imagePair{actual: actual, expected: expected})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].actual.Path < pairs[j].actual.Path
	})
	return pairs
}

func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	return img, err
}
