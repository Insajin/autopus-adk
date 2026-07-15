package design

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVisualGateReport_V1StrictDecoder_PreservesReleasedShape(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReport(VisualGateInput{
		UIChanged:   []string{"src/App.tsx"},
		Screenshots: []string{"snapshots/app.png"},
		Artifacts: []VisualArtifact{{
			Name: "app.png", Kind: "screenshot", ContentType: "image/png", Path: "snapshots/app.png",
		}},
		Viewport: "chromium",
	})
	raw, err := json.Marshal(report)
	require.NoError(t, err)

	var legacy legacyVisualGateReport
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&legacy))
	assert.Equal(t, 1, legacy.Version)
}

func TestWriteVisualGateReportBundle_WritesV1AndSeparateV2(t *testing.T) {
	root := t.TempDir()
	legacy := VisualGateReport{Version: 1, Verdict: "WARN"}
	evidence := BuildVisualGateReportV2(VisualGateInputV2{})

	require.NoError(t, invokeVisualBundleWriter(root, legacy, evidence))
	v1Path := filepath.Join(root, ".autopus", "design", "verify", "latest.json")
	v1 := readJSONMap(t, v1Path)
	v2 := readJSONMap(t, filepath.Join(root, ".autopus", "design", "verify", "latest.v2.json"))
	assert.Equal(t, float64(1), v1["version"])
	assert.Equal(t, float64(2), v2["version"])
	v1Raw, err := os.ReadFile(v1Path)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256(v1Raw)), v2["legacy_sha256"])

	readV1, readV2, err := ReadVisualGateReportBundle(root)
	require.NoError(t, err)
	assert.Equal(t, legacy.Verdict, readV1.Verdict)
	assert.Equal(t, evidence.GeneratedAt, readV2.GeneratedAt)
}

func TestBuildVisualGateReports_PreservesV1QAMESHAndMakesV2Advisory(t *testing.T) {
	t.Parallel()

	v1Report := BuildVisualGateReport(VisualGateInput{})
	check := visualCheckByID(t, v1Report, "qamesh_handoff")
	assert.Equal(t, "PASS", check.Status)
	assert.Equal(t, "visual gate report is metadata-only and can be consumed by QAMESH", check.Message)

	v2Report := BuildVisualGateReportV2(VisualGateInputV2{})
	v2Check := visualCheckV2ByID(t, v2Report, "qamesh_handoff")
	assert.Equal(t, "WARN", v2Check.Status)
	assert.Contains(t, v2Check.Message, "handoff candidate")
	assert.Contains(t, v2Check.Message, "ingestion unproven")
}

func TestReadVisualGateReportBundle_ExactLegacyBytesChanged_RejectsV2(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, WriteVisualGateReportBundle(
		root,
		VisualGateReport{Version: 1, Verdict: "WARN"},
		VisualGateReportV2{Version: 2, Verdict: "WARN"},
	))

	v1Path := filepath.Join(root, ".autopus", "design", "verify", "latest.json")
	raw, err := os.ReadFile(v1Path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(v1Path, append(raw, ' '), 0o644))

	_, untrustedV2, err := ReadVisualGateReportBundle(root)
	require.ErrorIs(t, err, ErrVisualGateBundleUncommitted)
	assert.Zero(t, untrustedV2)
}

type legacyVisualGateReport struct {
	Version        int                 `json:"version"`
	GeneratedAt    string              `json:"generated_at"`
	Verdict        string              `json:"verdict"`
	Viewport       string              `json:"viewport"`
	UIChanged      []string            `json:"ui_changed"`
	Screenshots    []string            `json:"screenshots"`
	Artifacts      []legacyArtifact    `json:"artifacts,omitempty"`
	DiffSummary    ScreenshotDiffStats `json:"screenshot_diff_summary"`
	VisualCritic   json.RawMessage     `json:"visual_critic,omitempty"`
	MaxFixAttempts int                 `json:"max_fix_attempts"`
	Checks         []VisualCheck       `json:"checks"`
	PlaywrightErr  string              `json:"playwright_error,omitempty"`
}

type legacyArtifact struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ContentType string `json:"content_type,omitempty"`
	Path        string `json:"path"`
}

func invokeVisualBundleWriter(root string, legacy VisualGateReport, evidence VisualGateReportV2) error {
	fn := reflect.ValueOf(WriteVisualGateReportBundle)
	if fn.Type().NumIn() != 3 {
		return fmt.Errorf("WriteVisualGateReportBundle must accept root, v1, and v2")
	}
	results := fn.Call([]reflect.Value{reflect.ValueOf(root), reflect.ValueOf(legacy), reflect.ValueOf(evidence)})
	if len(results) == 0 {
		return nil
	}
	last := results[len(results)-1]
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if !last.Type().Implements(errorType) {
		return nil
	}
	if last.IsNil() {
		return nil
	}
	return last.Interface().(error)
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	return decoded
}
