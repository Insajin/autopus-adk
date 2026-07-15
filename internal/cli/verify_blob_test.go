package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectVisualEvidenceFromBlobRecordsPassedScreenshotAssertionWithoutAttachments(t *testing.T) {
	t.Parallel()

	blob := buildBlobReport(t, successfulScreenshotBlobEvents("/private/project", "../../.autopus/baselines/visual"), nil)
	evidence := collectVisualEvidence(blob)

	require.Len(t, evidence.Assertions, 1)
	assert.Equal(t, "home-default.png", evidence.Assertions[0].Name)
	assert.Equal(t, "test-home", evidence.Assertions[0].TestID)
	assert.Equal(t, "chromium", evidence.Assertions[0].Project)
	assert.Equal(t, "PASS", evidence.Assertions[0].Status)
	assert.NotEmpty(t, evidence.Assertions[0].ComparisonID)
	assert.Empty(t, evidence.Artifacts, "a successful Playwright comparison does not fabricate attachments")
	assert.Equal(t, []string{"chromium"}, evidence.Projects)
}

func TestCollectVisualEvidenceRejectsBlobZipSlipEntry(t *testing.T) {
	t.Parallel()

	events := successfulScreenshotBlobEvents("/private/project", "../../.autopus/baselines/visual")
	attachmentEvent := map[string]any{
		"method": "onAttach",
		"params": map[string]any{
			"testId":   "test-home",
			"resultId": "result-home",
			"attachments": []map[string]any{{
				"name":        "actual",
				"contentType": "image/png",
				"path":        "../private.png",
			}},
		},
	}
	events = append(events[:len(events)-2], append([]map[string]any{attachmentEvent}, events[len(events)-2:]...)...)
	blob := buildBlobReport(t, events, map[string][]byte{"../private.png": []byte("private")})

	evidence := collectVisualEvidence(blob)
	assert.Empty(t, evidence.Artifacts)
	encoded, err := json.Marshal(evidence)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "../private.png")
}

func TestCollectVisualEvidenceRejectsBlobWithAbnormalUncompressedSize(t *testing.T) {
	t.Parallel()

	blob := buildRawBlobEntry(t, "report.jsonl", uint64(1)<<40)
	evidence := collectVisualEvidence(blob)

	assert.Empty(t, evidence.Artifacts)
	assert.Empty(t, evidence.Assertions)
	assert.Empty(t, evidence.Projects)
}

func TestCollectVisualEvidenceRejectsBlobEventFlood(t *testing.T) {
	t.Parallel()

	var report strings.Builder
	for i := 0; i < 100_001; i++ {
		report.WriteString("{\"method\":\"onStdIO\",\"params\":{}}\n")
	}
	for _, event := range successfulScreenshotBlobEvents("/private/project", "../../.autopus/baselines/visual") {
		line, err := json.Marshal(event)
		require.NoError(t, err)
		report.Write(line)
		report.WriteByte('\n')
	}
	blob := buildBlobReportBytes(t, []byte(report.String()), nil)
	evidence := collectVisualEvidence(blob)

	assert.Empty(t, evidence.Artifacts)
	assert.Empty(t, evidence.Assertions)
	assert.Empty(t, evidence.Projects)
}

func TestCollectVisualEvidenceRedactsPrivateAbsolutePathsFromBlob(t *testing.T) {
	t.Parallel()

	blob := buildBlobReport(t, successfulScreenshotBlobEvents(
		"/Users/alice/private/project/tests/visual",
		"/Users/alice/private/project/.autopus/baselines/visual",
	), nil)
	evidence := collectVisualEvidence(blob)

	require.Len(t, evidence.Assertions, 1)
	encoded, err := json.Marshal(evidence)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "/Users/alice/private")
}

func successfulScreenshotBlobEvents(rootDir, snapshotDir string) []map[string]any {
	return []map[string]any{
		{
			"method": "onBlobReportMetadata",
			"params": map[string]any{"version": 2, "userAgent": "Playwright/1.59.1", "pathSeparator": "/"},
		},
		{
			"method": "onConfigure",
			"params": map[string]any{"config": map[string]any{"rootDir": rootDir, "version": "1.59.1"}},
		},
		{
			"method": "onProject",
			"params": map[string]any{"project": map[string]any{
				"name":        "chromium",
				"snapshotDir": snapshotDir,
				"suites": []map[string]any{{
					"title": "scenarios/home-default.spec.ts",
					"entries": []map[string]any{{
						"testId": "test-home",
						"title":  "home-default baseline",
						"location": map[string]any{
							"file": "scenarios/home-default.spec.ts",
							"line": 8,
						},
					}},
				}},
			}},
		},
		{"method": "onBegin"},
		{
			"method": "onTestBegin",
			"params": map[string]any{"testId": "test-home", "result": map[string]any{"id": "result-home"}},
		},
		{
			"method": "onStepBegin",
			"params": map[string]any{
				"testId":   "test-home",
				"resultId": "result-home",
				"step": map[string]any{
					"id":       "step-screenshot",
					"title":    "Expect \"toHaveScreenshot(home-default.png)\"",
					"category": "expect",
				},
			},
		},
		{
			"method": "onStepEnd",
			"params": map[string]any{
				"testId":   "test-home",
				"resultId": "result-home",
				"step":     map[string]any{"id": "step-screenshot", "duration": 42},
			},
		},
		{
			"method": "onTestEnd",
			"params": map[string]any{
				"test":   map[string]any{"testId": "test-home", "expectedStatus": "passed"},
				"result": map[string]any{"id": "result-home", "status": "passed", "errors": []any{}},
			},
		},
		{"method": "onEnd", "params": map[string]any{"result": map[string]any{"status": "passed"}}},
	}
}

func buildBlobReport(t *testing.T, events []map[string]any, entries map[string][]byte) []byte {
	t.Helper()
	blob := buildBlobReportWithoutProof(t, events, entries)
	ignored := false
	blob, err := appendSnapshotProofToBlob(blob, snapshotComparisonProof{
		Version: 2, Nonce: "fixture", PlaywrightVersion: "1.59.1", UpdateSnapshots: "none",
		Projects: []snapshotProjectProof{{
			Name: "chromium", IgnoreSnapshots: &ignored, State: "enabled", Source: "public",
		}},
	})
	require.NoError(t, err)
	return blob
}

func buildBlobReportWithoutProof(t *testing.T, events []map[string]any, entries map[string][]byte) []byte {
	t.Helper()
	var report bytes.Buffer
	for _, event := range events {
		line, err := json.Marshal(event)
		require.NoError(t, err)
		report.Write(line)
		report.WriteByte('\n')
	}
	return buildBlobReportBytes(t, report.Bytes(), entries)
}

func buildBlobReportBytes(t *testing.T, report []byte, entries map[string][]byte) []byte {
	t.Helper()
	var blob bytes.Buffer
	zw := zip.NewWriter(&blob)
	w, err := zw.Create("report.jsonl")
	require.NoError(t, err)
	_, err = w.Write(report)
	require.NoError(t, err)
	for name, content := range entries {
		entry, createErr := zw.Create(name)
		require.NoError(t, createErr)
		_, writeErr := entry.Write(content)
		require.NoError(t, writeErr)
	}
	require.NoError(t, zw.Close())
	return blob.Bytes()
}

func buildRawBlobEntry(t *testing.T, name string, uncompressedSize uint64) []byte {
	t.Helper()
	var blob bytes.Buffer
	zw := zip.NewWriter(&blob)
	header := &zip.FileHeader{
		Name:               name,
		Method:             zip.Store,
		CompressedSize64:   0,
		UncompressedSize64: uncompressedSize,
	}
	_, err := zw.CreateRaw(header)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return blob.Bytes()
}
