package design

import (
	"encoding/binary"
	"hash/crc32"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVisualGateReportV2_DifferentResultOrRetry_DoesNotCrossPair(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	expected := filepath.Join(dir, "expected.png")
	actual := filepath.Join(dir, "actual.png")
	writePNG(t, expected, color.RGBA{A: 255})
	writePNG(t, actual, color.RGBA{R: 255, A: 255})
	report := BuildVisualGateReportV2(VisualGateInputV2{Artifacts: []VisualArtifactV2{
		{Name: "app-expected.png", Kind: "expected", Path: "expected.png", LocalPath: expected, ComparisonID: "app", ResultID: "result-1", Retry: 0},
		{Name: "app-actual.png", Kind: "actual", Path: "actual.png", LocalPath: actual, ComparisonID: "app", ResultID: "result-2", Retry: 1},
	}})

	assert.Equal(t, 0, report.DiffSummary.PairsCompared)
}

func TestRedactVisualPath_RelativeTraversal_IsRedacted(t *testing.T) {
	t.Parallel()

	redacted := RedactVisualPath(t.TempDir(), "../private/secret.png")
	assert.True(t, strings.HasPrefix(redacted, "external:"))
	assert.NotContains(t, redacted, "..")
}

func TestBuildVisualGateReportV2_PublicReferences_DoNotExposeOutsidePaths(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		UIChanged:   []string{"/Users/alice/private/App.tsx"},
		Screenshots: []string{"../private/screenshot.png"},
		Artifacts: []VisualArtifactV2{{
			Name:         "actual.png",
			Kind:         "actual",
			Path:         "/Users/alice/private/actual.png",
			ComparisonID: "../private/comparison.png",
		}},
		Assertions: []VisualAssertion{{
			Name:         "actual.png",
			Status:       "PASS",
			BaselinePath: "/Users/alice/private/baseline.png",
			ComparisonID: "C:\\Users\\alice\\private\\comparison.png",
		}},
	})

	require.Len(t, report.Artifacts, 1)
	require.Len(t, report.Assertions, 1)
	for _, value := range []string{
		report.UIChanged[0],
		report.Screenshots[0],
		report.Artifacts[0].Path,
		report.Artifacts[0].ComparisonID,
		report.Assertions[0].BaselinePath,
		report.Assertions[0].ComparisonID,
	} {
		assert.True(t, strings.HasPrefix(value, "external:"), value)
		assert.NotContains(t, value, "..")
		assert.NotContains(t, value, "Users")
	}
}

func TestWriteVisualGateReportBundle_SymlinkTarget_IsRejected(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(root, ".autopus", "design")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(target, "verify")))

	err := invokeVisualBundleWriter(root, VisualGateReport{Version: 1}, VisualGateReportV2{Version: 2})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "symlink")
}

func TestWriteVisualGateReportBundle_V1SymlinkFailure_DoesNotPublishV2(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".autopus", "design", "verify")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	v2Path := filepath.Join(dir, "latest.v2.json")
	oldV2 := []byte("{\"version\":2,\"marker\":\"old\"}\n")
	require.NoError(t, os.WriteFile(v2Path, oldV2, 0o644))
	outside := filepath.Join(t.TempDir(), "legacy.json")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "latest.json")))

	err := WriteVisualGateReportBundle(root, VisualGateReport{Version: 1}, VisualGateReportV2{Version: 2})
	require.Error(t, err)
	gotV2, readErr := os.ReadFile(v2Path)
	require.NoError(t, readErr)
	assert.Equal(t, oldV2, gotV2)
	temps, globErr := filepath.Glob(filepath.Join(dir, ".visual-report-*"))
	require.NoError(t, globErr)
	assert.Empty(t, temps)
}

func TestWriteVisualGateReportBundle_IntermediateDirectorySwap_DoesNotEscapeRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	verifyDir := filepath.Join(root, ".autopus", "design", "verify")
	require.NoError(t, os.MkdirAll(verifyDir, 0o755))
	rootFS, err := os.OpenRoot(root)
	require.NoError(t, err)
	defer func() { _ = rootFS.Close() }()
	swapping := &swapVerifyOnOpenRoot{
		visualReportRoot: rootFS,
		root:             root,
		outside:          outside,
	}

	err = writeVisualGateReportBundleRoot(
		swapping,
		VisualGateReport{Version: 1},
		VisualGateReportV2{Version: 2},
	)
	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(outside, "latest.json"))
	assert.NoFileExists(t, filepath.Join(outside, "latest.v2.json"))
	entries, readErr := os.ReadDir(outside)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

type swapVerifyOnOpenRoot struct {
	visualReportRoot
	root    string
	outside string
	swapped bool
}

func (root *swapVerifyOnOpenRoot) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if !root.swapped {
		root.swapped = true
		verify := filepath.Join(root.root, ".autopus", "design", "verify")
		moved := filepath.Join(root.root, ".autopus", "design", "verify-original")
		if err := os.Rename(verify, moved); err != nil {
			return nil, err
		}
		if err := os.Symlink(root.outside, verify); err != nil {
			return nil, err
		}
	}
	return root.visualReportRoot.OpenFile(name, flag, perm)
}

func TestCompareImageFiles_FileSizeBudgetBoundary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	expected := filepath.Join(dir, "expected.png")
	actual := filepath.Join(dir, "actual.png")
	writePNG(t, expected, color.RGBA{A: 255})
	writePNG(t, actual, color.RGBA{A: 255})
	require.NoError(t, os.Truncate(actual, 16<<20))
	_, err := compareImageFilesV2(actual, expected)
	require.NoError(t, err)
	require.NoError(t, os.Truncate(actual, (16<<20)+1))
	_, err = compareImageFilesV2(actual, expected)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "file size budget")
}

func TestCompareImageFiles_PixelBudgetExceededBeforeDecode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oversized := filepath.Join(dir, "oversized.png")
	writePNGHeader(t, oversized, 4097, 4096)
	expected := filepath.Join(dir, "expected.png")
	writePNG(t, expected, color.RGBA{A: 255})

	_, err := compareImageFilesV2(oversized, expected)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "pixel budget")
}

func TestBuildVisualGateReportV2_ComparisonPairBudgetBoundary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	expected := filepath.Join(dir, "expected.png")
	actual := filepath.Join(dir, "actual.png")
	writePNG(t, expected, color.RGBA{A: 255})
	writePNG(t, actual, color.RGBA{A: 255})
	artifacts := make([]VisualArtifactV2, 0, 258)
	for i := 0; i < 129; i++ {
		id := "pair-" + strconv.Itoa(i)
		artifacts = append(artifacts,
			VisualArtifactV2{Kind: "expected", Path: id + "-expected.png", LocalPath: expected, ComparisonID: id, ResultID: id},
			VisualArtifactV2{Kind: "actual", Path: id + "-actual.png", LocalPath: actual, ComparisonID: id, ResultID: id},
		)
	}
	report := BuildVisualGateReportV2(VisualGateInputV2{Artifacts: artifacts})

	assert.LessOrEqual(t, report.DiffSummary.PairsCompared, 128)
	assert.Contains(t, strings.ToLower(strings.Join(report.DiffSummary.ComparisonErrors, " ")), "pair budget")
}

func writePNGHeader(t *testing.T, path string, width, height uint32) {
	t.Helper()
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], width)
	binary.BigEndian.PutUint32(data[4:8], height)
	data[8], data[9], data[10], data[11], data[12] = 8, 6, 0, 0, 0
	chunk := append([]byte("IHDR"), data...)
	png := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d"), chunk...)
	crc := make([]byte, 4)
	binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(chunk))
	png = append(png, crc...)
	require.NoError(t, os.WriteFile(path, png, 0o600))
}
