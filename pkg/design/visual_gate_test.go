package design

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVisualGateReportClassifiesMissingScreenshotsAsFail(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReport(VisualGateInput{
		UIChanged:     []string{"src/components/Button.tsx"},
		Viewport:      "desktop",
		DesignContext: Context{Found: true, SourcePath: "DESIGN.md"},
	})
	assert.Equal(t, "FAIL", report.Verdict)
	assert.Equal(t, "screenshot_capture", report.Checks[1].ID)
	assert.Equal(t, "FAIL", report.Checks[1].Status)
}

func TestBuildVisualGateReportIncludesDeterministicDiffSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	expected := filepath.Join(dir, "expected.png")
	actual := filepath.Join(dir, "actual.png")
	writePNG(t, expected, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	writePNG(t, actual, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	report := BuildVisualGateReport(VisualGateInput{
		UIChanged: []string{"src/components/Button.tsx"},
		Screenshots: []string{
			"test-results/button-actual.png",
		},
		Artifacts: []VisualArtifact{
			{Name: "expected", Kind: "expected", Path: "test-results/expected.png", LocalPath: expected},
			{Name: "actual", Kind: "actual", Path: "test-results/actual.png", LocalPath: actual},
		},
		Viewport:      "all",
		DesignContext: Context{Found: true, SourcePath: "DESIGN.md"},
	})
	assert.Equal(t, 1, report.DiffSummary.PairsCompared)
	assert.Equal(t, int64(4), report.DiffSummary.ChangedPixels)
	assert.Equal(t, "WARN", report.Verdict)
	assert.Empty(t, report.Artifacts[0].LocalPath)
}

func visualCheckByID(t *testing.T, report VisualGateReport, id string) VisualCheck {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("visual check %q not found", id)
	return VisualCheck{}
}

func TestBuildVisualGateReportRedactsDiffComparisonErrors(t *testing.T) {
	t.Parallel()

	privateDir := filepath.Join(t.TempDir(), "private")
	report := BuildVisualGateReport(VisualGateInput{
		UIChanged:   []string{"src/components/Button.tsx"},
		Screenshots: []string{"test-results/actual.png"},
		Artifacts: []VisualArtifact{
			{Name: "expected", Kind: "expected", Path: "test-results/expected.png", LocalPath: filepath.Join(privateDir, "expected.png")},
			{Name: "actual", Kind: "actual", Path: "test-results/actual.png", LocalPath: filepath.Join(privateDir, "actual.png")},
		},
		Viewport:      "all",
		DesignContext: Context{Found: true, SourcePath: "DESIGN.md"},
	})
	require.Len(t, report.DiffSummary.ComparisonErrors, 1)
	assert.NotContains(t, report.DiffSummary.ComparisonErrors[0], privateDir)
	assert.Contains(t, report.DiffSummary.ComparisonErrors[0], "test-results/actual.png")
}

func TestRedactVisualPathRemovesExternalAbsolutePath(t *testing.T) {
	t.Parallel()

	redacted := RedactVisualPath(t.TempDir(), "/tmp/private/screenshot.png")
	assert.Contains(t, redacted, "external:")
	assert.NotContains(t, redacted, "/tmp/private")
	assert.Contains(t, redacted, "screenshot.png")
}

func writePNG(t *testing.T, path string, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, c)
		}
	}
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()
	require.NoError(t, png.Encode(file, img))
}

func TestBuildVisualGateReportWarnsForSingleViewportAndNoBaseline(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReport(VisualGateInput{
		UIChanged:     []string{"src/components/Button.tsx"},
		Screenshots:   []string{"test-results/button.png"},
		Viewport:      "mobile",
		DesignContext: Context{SkipReason: SkipMissing},
	})
	assert.Equal(t, "WARN", report.Verdict)
	assert.Equal(t, "WARN", report.Checks[0].Status)
	assert.Equal(t, "WARN", report.Checks[2].Status)
	assert.Equal(t, "WARN", report.Checks[3].Status)
}

func TestBuildVisualGateReportMergesVisualCriticFailure(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReport(VisualGateInput{
		UIChanged:     []string{"src/components/Button.tsx"},
		Screenshots:   []string{"test-results/button.png"},
		Viewport:      "all",
		VisualCritic:  VisualCriticReport{Status: "FAIL", Findings: []VisualCriticFinding{{Severity: "FAIL", Category: "overlap", Message: "overlap"}}},
		DesignContext: Context{Found: true, SourcePath: "DESIGN.md"},
	})
	assert.Equal(t, "FAIL", report.Verdict)
	assert.Equal(t, "FAIL", report.VisualCritic.Status)
}

func TestLoadVisualCriticReportDerivesStatus(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, ".autopus", "design", "critic.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`{"findings":[{"severity":"WARN","category":"contrast","message":"low contrast"}]}`), 0o644))

	report, err := LoadVisualCriticReport(root, ".autopus/design/critic.json")
	require.NoError(t, err)
	assert.Equal(t, "WARN", report.Status)
	assert.Equal(t, ".autopus/design/critic.json", report.Source)
}

func TestLoadVisualCriticReportRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "critic.json")
	require.NoError(t, os.WriteFile(outside, []byte(`{"status":"PASS"}`), 0o644))
	link := filepath.Join(root, "critic-link.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := LoadVisualCriticReport(root, "critic-link.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes project root")
}
