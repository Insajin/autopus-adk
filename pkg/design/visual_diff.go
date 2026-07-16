package design

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxVisualImageFileBytes  = int64(16 << 20)
	maxVisualImagePixels     = int64(16_777_216)
	maxVisualComparisonPairs = 128
)

type ImageDiff struct {
	ChangedPixels int64
	TotalPixels   int64
	ChangedRatio  float64
}

func BuildScreenshotDiffStats(artifacts []VisualArtifact) ScreenshotDiffStats {
	stats := ScreenshotDiffStats{DeterministicMode: "actual_expected_pixel_compare"}
	for _, artifact := range artifacts {
		if artifact.Kind == "diff" {
			stats.DiffArtifactRefs = append(stats.DiffArtifactRefs, artifact.Path)
		}
	}
	pairs := pairActualExpected(artifacts)
	for _, pair := range pairs {
		diff, err := CompareImageFiles(pair.actual.LocalPath, pair.expected.LocalPath)
		if err != nil {
			stats.ComparisonErrors = append(stats.ComparisonErrors, redactImageDiffError(err, pair.actual.Path, pair.actual.LocalPath, pair.expected.Path, pair.expected.LocalPath))
			continue
		}
		accumulateImageDiff(&stats, diff)
	}
	sort.Strings(stats.DiffArtifactRefs)
	sort.Strings(stats.ComparisonErrors)
	return stats
}

func CompareImageFiles(actualPath, expectedPath string) (ImageDiff, error) {
	return compareImageFilesWithDecoder(actualPath, expectedPath, decodeImageLegacy)
}

func compareImageFilesV2(actualPath, expectedPath string) (ImageDiff, error) {
	return compareImageFilesWithDecoder(actualPath, expectedPath, decodeImage)
}

func compareImageFilesWithDecoder(actualPath, expectedPath string, decoder func(string) (image.Image, error)) (ImageDiff, error) {
	if actualPath == "" || expectedPath == "" {
		return ImageDiff{}, fmt.Errorf("missing actual or expected path")
	}
	actual, err := decoder(actualPath)
	if err != nil {
		return ImageDiff{}, fmt.Errorf("decode actual: %w", err)
	}
	expected, err := decoder(expectedPath)
	if err != nil {
		return ImageDiff{}, fmt.Errorf("decode expected: %w", err)
	}
	if !actual.Bounds().Size().Eq(expected.Bounds().Size()) {
		return ImageDiff{}, fmt.Errorf("dimension mismatch actual=%v expected=%v", actual.Bounds().Size(), expected.Bounds().Size())
	}
	bounds := actual.Bounds()
	var changed, total int64
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

func decodeImageLegacy(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	return decoded, err
}

func decodeImage(path string) (image.Image, error) {
	raw, err := readVisualImageSnapshot(path)
	if err != nil {
		return nil, err
	}
	return decodeVisualImageSnapshot(raw)
}

func readVisualImageSnapshot(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxVisualImageFileBytes {
		return nil, fmt.Errorf("image file size budget exceeded")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxVisualImageFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxVisualImageFileBytes {
		return nil, fmt.Errorf("image file size budget exceeded")
	}
	return raw, nil
}

func decodeVisualImageSnapshot(raw []byte) (image.Image, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > maxVisualImagePixels/int64(config.Height) {
		return nil, fmt.Errorf("image pixel budget exceeded")
	}
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	return decoded, err
}

func ClassifyVisualArtifact(name, artifactPath string) string {
	lower := strings.ToLower(name + " " + filepath.ToSlash(artifactPath))
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
	pairs := make([]imagePair, 0, len(byDir))
	for _, group := range byDir {
		actual, hasActual := group["actual"]
		expected, hasExpected := group["expected"]
		if hasActual && hasExpected {
			pairs = append(pairs, imagePair{actual: actual, expected: expected})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].actual.Path < pairs[j].actual.Path })
	return pairs
}

func sanitizeArtifacts(artifacts []VisualArtifact) []VisualArtifact {
	out := make([]VisualArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		artifact.Path = filepath.ToSlash(strings.TrimSpace(artifact.Path))
		artifact.LocalPath = strings.TrimSpace(artifact.LocalPath)
		if artifact.Path == "" {
			continue
		}
		if artifact.Kind == "" {
			artifact.Kind = ClassifyVisualArtifact(artifact.Name, artifact.Path)
		}
		out = append(out, artifact)
	}
	return out
}

func publicArtifacts(artifacts []VisualArtifact) []VisualArtifact {
	out := make([]VisualArtifact, len(artifacts))
	copy(out, artifacts)
	for i := range out {
		out[i].LocalPath = ""
	}
	return out
}

func accumulateImageDiff(stats *ScreenshotDiffStats, diff ImageDiff) {
	stats.PairsCompared++
	stats.ChangedPixels += diff.ChangedPixels
	stats.TotalPixels += diff.TotalPixels
	stats.MaxChangedRatio = math.Max(stats.MaxChangedRatio, diff.ChangedRatio)
}

func redactImageDiffError(err error, actualPath, actualLocal, expectedPath, expectedLocal string) string {
	message := err.Error()
	if actualLocal != "" {
		message = strings.ReplaceAll(message, actualLocal, actualPath)
	}
	if expectedLocal != "" {
		message = strings.ReplaceAll(message, expectedLocal, expectedPath)
	}
	return actualPath + ": " + message
}
