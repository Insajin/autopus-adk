package design

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testVisualCriticMaxBytes = 4 << 20

var visualTOCTOUFormatSequence atomic.Uint64

func TestDecodeImage_InPlaceMutation_UsesOneSnapshot(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "mutable.img")
	magic := fmt.Sprintf("VT%06d", visualTOCTOUFormatSequence.Add(1))
	original := []byte(magic + "-original")
	replacement := []byte(magic + "-replaced")
	require.NoError(t, os.WriteFile(path, original, 0o600))
	var mutationErr error
	image.RegisterFormat(
		"visual-toctou-test-"+magic,
		magic,
		func(reader io.Reader) (image.Image, error) {
			raw, err := io.ReadAll(reader)
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(raw, original) {
				return nil, fmt.Errorf("decode observed changed bytes: %q", raw)
			}
			return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
		},
		func(reader io.Reader) (image.Config, error) {
			raw, err := io.ReadAll(reader)
			if err != nil {
				return image.Config{}, err
			}
			if !bytes.Equal(raw, original) {
				return image.Config{}, fmt.Errorf("config observed unexpected bytes: %q", raw)
			}
			mutationErr = os.WriteFile(path, replacement, 0o600)
			return image.Config{ColorModel: color.RGBAModel, Width: 1, Height: 1}, nil
		},
	)

	// When
	decoded, err := decodeImage(path)

	// Then
	require.NoError(t, mutationErr)
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 1, 1), decoded.Bounds())
	raw, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, replacement, raw)
}

func TestCompareImageFiles_LegacyOversizedFile_RemainsSupported(t *testing.T) {
	t.Parallel()

	// Given
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual.png")
	expected := filepath.Join(dir, "expected.png")
	writePNG(t, actual, color.RGBA{A: 255})
	writePNG(t, expected, color.RGBA{A: 255})
	require.NoError(t, os.Truncate(actual, maxVisualImageFileBytes+1))

	// When
	diff, err := CompareImageFiles(actual, expected)

	// Then
	require.NoError(t, err)
	assert.Zero(t, diff.ChangedPixels)
}

func TestCompareImageFilesV2_OversizedFile_IsRejected(t *testing.T) {
	t.Parallel()

	// Given
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual.png")
	expected := filepath.Join(dir, "expected.png")
	writePNG(t, actual, color.RGBA{A: 255})
	writePNG(t, expected, color.RGBA{A: 255})
	require.NoError(t, os.Truncate(actual, maxVisualImageFileBytes+1))

	// When
	_, err := compareImageFilesV2(actual, expected)

	// Then
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "file size budget")
}

func TestBuildScreenshotDiffStats_LegacyMoreThanPairBudget_ProcessesAll(t *testing.T) {
	t.Parallel()

	// Given
	root := t.TempDir()
	artifacts := make([]VisualArtifact, 0, 2*(maxVisualComparisonPairs+1))
	for index := 0; index <= maxVisualComparisonPairs; index++ {
		dir := filepath.Join(root, fmt.Sprintf("pair-%03d", index))
		actual := filepath.Join(dir, "actual.png")
		expected := filepath.Join(dir, "expected.png")
		writePNGInCreatedDir(t, actual, color.RGBA{A: 255})
		writePNGInCreatedDir(t, expected, color.RGBA{A: 255})
		artifacts = append(artifacts,
			VisualArtifact{Kind: "actual", Path: fmt.Sprintf("pair-%03d/actual.png", index), LocalPath: actual},
			VisualArtifact{Kind: "expected", Path: fmt.Sprintf("pair-%03d/expected.png", index), LocalPath: expected},
		)
	}

	// When
	stats := BuildScreenshotDiffStats(artifacts)

	// Then
	assert.Equal(t, maxVisualComparisonPairs+1, stats.PairsCompared)
	assert.Empty(t, stats.ComparisonErrors)
}

func TestLoadVisualCriticReport_OversizedValidJSON_IsRejected(t *testing.T) {
	t.Parallel()

	// Given
	root := t.TempDir()
	path := filepath.Join(root, "critic.json")
	padding := strings.Repeat("a", testVisualCriticMaxBytes)
	payload := []byte(`{"status":"PASS","padding":"` + padding + `"}`)
	require.Greater(t, len(payload), testVisualCriticMaxBytes)
	require.NoError(t, os.WriteFile(path, payload, 0o600))

	// When
	_, err := LoadVisualCriticReport(root, "critic.json")

	// Then
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "size limit")
}

func TestLoadVisualCriticReport_StaticIntermediateSymlink_IsRejected(t *testing.T) {
	// Given
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "critic.json"), []byte(`{"status":"PASS"}`), 0o600))
	if err := os.Symlink(outside, filepath.Join(root, "reports")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	// When
	_, err := LoadVisualCriticReport(root, "reports/critic.json")

	// Then
	require.Error(t, err)
}

func TestLoadVisualCriticReportRoot_SymlinkSwap_DoesNotEscape(t *testing.T) {
	tests := []struct {
		name string
		swap func(root, outside string) error
	}{
		{
			name: "intermediate directory",
			swap: func(root, outside string) error {
				reports := filepath.Join(root, "reports")
				if err := os.Rename(reports, filepath.Join(root, "reports-original")); err != nil {
					return err
				}
				return os.Symlink(outside, reports)
			},
		},
		{
			name: "final file",
			swap: func(root, outside string) error {
				critic := filepath.Join(root, "reports", "critic.json")
				if err := os.Rename(critic, filepath.Join(root, "reports", "critic-original.json")); err != nil {
					return err
				}
				return os.Symlink(filepath.Join(outside, "critic.json"), critic)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			root := t.TempDir()
			outside := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(root, "reports"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "reports", "critic.json"), []byte(`{"status":"PASS"}`), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(outside, "critic.json"), []byte(`{"status":"FAIL"}`), 0o600))
			rootFS, err := os.OpenRoot(root)
			require.NoError(t, err)
			defer func() { _ = rootFS.Close() }()
			swapping := &swapVisualCriticOnOpenRoot{
				visualCriticRoot: rootFS,
				swap: func() error {
					return test.swap(root, outside)
				},
			}

			// When
			report, err := loadVisualCriticReportRoot(swapping, "reports/critic.json")

			// Then
			if swapping.swapErr != nil {
				t.Skipf("symlink unavailable: %v", swapping.swapErr)
			}
			require.Error(t, err)
			assert.Empty(t, report.Status)
		})
	}
}

func TestLoadVisualCriticReportRoot_FinalFileReplacement_IsRejected(t *testing.T) {
	// Given
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "critic.json"), []byte(`{"status":"PASS"}`), 0o600))
	rootFS, err := os.OpenRoot(root)
	require.NoError(t, err)
	defer func() { _ = rootFS.Close() }()
	swapping := &swapVisualCriticOnOpenRoot{
		visualCriticRoot: rootFS,
		swap: func() error {
			path := filepath.Join(root, "critic.json")
			if err := os.Rename(path, filepath.Join(root, "critic-original.json")); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(`{"status":"FAIL"}`), 0o600)
		},
	}

	// When
	report, err := loadVisualCriticReportRoot(swapping, "critic.json")

	// Then
	require.NoError(t, swapping.swapErr)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "changed while opening")
	assert.Empty(t, report.Status)
}

type swapVisualCriticOnOpenRoot struct {
	visualCriticRoot
	swap    func() error
	swapErr error
	swapped bool
}

func (root *swapVisualCriticOnOpenRoot) Open(name string) (*os.File, error) {
	if !root.swapped {
		root.swapped = true
		root.swapErr = root.swap()
		if root.swapErr != nil {
			return nil, root.swapErr
		}
	}
	return root.visualCriticRoot.Open(name)
}
