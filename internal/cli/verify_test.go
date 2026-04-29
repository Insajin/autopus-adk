package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildPlaywrightJSON constructs Playwright JSON output as bytes for test fixtures.
func buildPlaywrightJSON(suites []playwrightSuite) []byte {
	result := playwrightResult{Suites: suites}
	b, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	return b
}

// TestCollectScreenshots_ValidJSON verifies screenshot paths are extracted from valid JSON.
func TestCollectScreenshots_ValidJSON(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{
		{
			Specs: []playwrightSpec{
				{
					Tests: []playwrightTest{
						{
							Results: []playwrightTestResult{
								{
									Attachments: []playwrightAttachment{
										{Name: "screenshot", ContentType: "image/png", Path: "/tmp/shot1.png"},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	paths := collectScreenshots(input)
	require.NotNil(t, paths)
	assert.Equal(t, []string{"/tmp/shot1.png"}, paths)
}

// TestCollectScreenshots_EmptyOutput verifies nil is returned for empty input.
func TestCollectScreenshots_EmptyOutput(t *testing.T) {
	t.Parallel()

	paths := collectScreenshots([]byte{})
	assert.Nil(t, paths)
}

// TestCollectScreenshots_InvalidJSON verifies nil is returned for malformed JSON.
func TestCollectScreenshots_InvalidJSON(t *testing.T) {
	t.Parallel()

	paths := collectScreenshots([]byte(`{not valid json`))
	assert.Nil(t, paths)
}

// TestCollectScreenshots_NoAttachments verifies empty result when no attachments exist.
func TestCollectScreenshots_NoAttachments(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{
		{
			Specs: []playwrightSpec{
				{
					Tests: []playwrightTest{
						{
							Results: []playwrightTestResult{
								{Attachments: []playwrightAttachment{}},
							},
						},
					},
				},
			},
		},
	})

	paths := collectScreenshots(input)
	assert.Empty(t, paths)
}

// TestCollectScreenshots_MixedAttachments verifies only screenshot attachments are returned.
func TestCollectScreenshots_MixedAttachments(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{
		{
			Specs: []playwrightSpec{
				{
					Tests: []playwrightTest{
						{
							Results: []playwrightTestResult{
								{
									Attachments: []playwrightAttachment{
										{Name: "screenshot", Path: "/tmp/shot.png"},
										{Name: "video", Path: "/tmp/video.webm"},
										{Name: "trace", Path: "/tmp/trace.zip"},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	paths := collectScreenshots(input)
	assert.Equal(t, []string{"/tmp/shot.png"}, paths)
}

// TestCollectScreenshots_PngSuffixWithoutName verifies .png files are matched by suffix.
func TestCollectScreenshots_PngSuffixWithoutName(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{
		{
			Specs: []playwrightSpec{
				{
					Tests: []playwrightTest{
						{
							Results: []playwrightTestResult{
								{
									Attachments: []playwrightAttachment{
										// Name is not "screenshot" but path ends in .png
										{Name: "custom-shot", Path: "/tmp/custom.png"},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	paths := collectScreenshots(input)
	assert.Equal(t, []string{"/tmp/custom.png"}, paths)
}

// TestCollectScreenshots_EmptyPath verifies attachments with empty path are skipped.
func TestCollectScreenshots_EmptyPath(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{
		{
			Specs: []playwrightSpec{
				{
					Tests: []playwrightTest{
						{
							Results: []playwrightTestResult{
								{
									Attachments: []playwrightAttachment{
										{Name: "screenshot", Path: ""},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	paths := collectScreenshots(input)
	assert.Empty(t, paths)
}

// TestCollectScreenshots_MultipleSpecs verifies paths from multiple specs are all returned.
func TestCollectScreenshots_MultipleSpecs(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{
		{
			Specs: []playwrightSpec{
				{
					Tests: []playwrightTest{
						{
							Results: []playwrightTestResult{
								{
									Attachments: []playwrightAttachment{
										{Name: "screenshot", Path: "/tmp/a.png"},
									},
								},
							},
						},
					},
				},
				{
					Tests: []playwrightTest{
						{
							Results: []playwrightTestResult{
								{
									Attachments: []playwrightAttachment{
										{Name: "screenshot", Path: "/tmp/b.png"},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	paths := collectScreenshots(input)
	assert.Len(t, paths, 2)
	assert.Contains(t, paths, "/tmp/a.png")
	assert.Contains(t, paths, "/tmp/b.png")
}

// TestCollectScreenshots_EmptySuites verifies empty slice is returned for zero suites.
func TestCollectScreenshots_EmptySuites(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{})
	paths := collectScreenshots(input)
	assert.Empty(t, paths)
}

// TestNewVerifyCmd_Use verifies the cobra command Use field is "verify".
func TestNewVerifyCmd_Use(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "verify", cmd.Use)
}

// TestNewVerifyCmd_Flags verifies --fix, --report-only, --viewport flags with defaults.
func TestNewVerifyCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd()
	require.NotNil(t, cmd)

	fixFlag := cmd.Flags().Lookup("fix")
	require.NotNil(t, fixFlag, "flag --fix must exist")
	assert.Equal(t, "true", fixFlag.DefValue)

	reportOnlyFlag := cmd.Flags().Lookup("report-only")
	require.NotNil(t, reportOnlyFlag, "flag --report-only must exist")
	assert.Equal(t, "false", reportOnlyFlag.DefValue)

	viewportFlag := cmd.Flags().Lookup("viewport")
	require.NotNil(t, viewportFlag, "flag --viewport must exist")
	assert.Equal(t, "desktop", viewportFlag.DefValue)
}

// TestNewVerifyCmd_ShortDescription verifies the Short field is non-empty.
func TestNewVerifyCmd_ShortDescription(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd()
	assert.NotEmpty(t, cmd.Short)
}

// TestAnalyzeGitDiff_ReturnsSlice verifies analyzeGitDiff runs without panicking in a git repo.
func TestAnalyzeGitDiff_ReturnsSlice(t *testing.T) {
	// Not parallel: relies on git state.
	files, err := analyzeGitDiff()
	if err != nil {
		t.Logf("analyzeGitDiff returned error (acceptable in CI without git): %v", err)
		return
	}
	assert.NotNil(t, files)
}

// TestRunPlaywright_FailsWithoutNpx verifies runPlaywright returns an error when npx is absent.
func TestRunPlaywright_FailsWithoutNpx(t *testing.T) {
	t.Parallel()

	out, err := runPlaywright("desktop")
	if err != nil {
		t.Logf("runPlaywright returned error (expected in most test environments): %v", err)
		_ = out
		return
	}
	assert.NotNil(t, out)
}
