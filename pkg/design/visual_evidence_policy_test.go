package design

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVisualGateReportV2_CustomRequiredProjects_RequiresPassPerProject(t *testing.T) {
	t.Parallel()

	input := decodeVisualGateInputV2(t, `{
		"strict": true,
		"required_projects": ["chromium", "Mobile Chrome"],
		"executed_projects": ["chromium", "Mobile Chrome"],
		"assertions": [{"name":"app.png","project":"chromium","status":"PASS"}],
		"snapshot_proof": {
			"playwright_version":"1.61.0",
			"update_snapshots":"none",
			"projects":[
				{"name":"chromium","comparison_status":"enabled"},
				{"name":"Mobile Chrome","comparison_status":"enabled"}
			]
		}
	}`)
	report := BuildVisualGateReportV2(input)

	assert.Equal(t, "FAIL", v2CheckStatus(t, report, "project_visual_coverage"))
}

func TestBuildVisualGateReportV2_UpdateSnapshots_NoneIsOnlyStrictPass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode string
		want string
	}{
		{mode: "none", want: "PASS"},
		{mode: "missing", want: "FAIL"},
		{mode: "changed", want: "FAIL"},
		{mode: "all", want: "FAIL"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.mode, func(t *testing.T) {
			raw := `{
					"strict":true,
					"required_projects":["webkit-custom"],
					"executed_projects":["webkit-custom"],
					"assertions":[{"name":"app.png","project":"webkit-custom","status":"PASS"}],
					"snapshot_proof":{"status":"enabled","playwright_version":"1.61.0","update_snapshots":` + quoteJSON(t, test.mode) + `,
						"projects":[{"name":"webkit-custom","comparison_status":"enabled"}]}
				}`
			report := BuildVisualGateReportV2(decodeVisualGateInputV2(t, raw))
			assert.Equal(t, test.want, v2CheckStatus(t, report, "snapshot_comparison_policy"))
		})
	}
}

func TestBuildVisualGateReportV2_StrictSnapshotProof_RequiresEnabledStatusAndVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  string
		version string
		want    string
	}{
		{name: "enabled", status: "enabled", version: "1.61.0", want: "PASS"},
		{name: "unproven", status: "unproven", version: "1.61.0", want: "FAIL"},
		{name: "disabled", status: "disabled", version: "1.61.0", want: "FAIL"},
		{name: "empty status", version: "1.61.0", want: "FAIL"},
		{name: "empty version", status: "enabled", want: "FAIL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := BuildVisualGateReportV2(visualProofPolicyInput(true, test.status, test.version))
			assert.Equal(t, test.want, v2CheckStatus(t, report, "snapshot_comparison_policy"))
		})
	}
}

func TestBuildVisualGateReportV2_AdvisoryInvalidSnapshotProof_RemainsWarn(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"", "unproven", "disabled"} {
		report := BuildVisualGateReportV2(visualProofPolicyInput(false, status, "1.61.0"))
		assert.Equal(t, "WARN", v2CheckStatus(t, report, "snapshot_comparison_policy"))
	}
}

func visualProofPolicyInput(strict bool, status, version string) VisualGateInputV2 {
	return VisualGateInputV2{
		Strict:           strict,
		RequiredProjects: []string{"desktop"},
		ExecutedProjects: []string{"desktop"},
		Assertions:       []VisualAssertion{{Name: "app.png", Project: "desktop", Status: "PASS"}},
		SnapshotProof: SnapshotComparisonProof{
			Status: status, PlaywrightVersion: version, UpdateSnapshots: "none",
			Projects: []SnapshotComparisonProject{{Name: "desktop", ComparisonStatus: "enabled"}},
		},
	}
}

func TestBuildVisualGateReportV2_PreservesSnapshotProofStatusAndDiagnostic(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		SnapshotProof: SnapshotComparisonProof{
			Status:            " unproven ",
			Diagnostic:        " public ignoreSnapshots unavailable ",
			PlaywrightVersion: "1.58.2",
			UpdateSnapshots:   "none",
		},
	})

	assert.Equal(t, "unproven", report.SnapshotProof.Status)
	assert.Equal(t, "public ignoreSnapshots unavailable", report.SnapshotProof.Diagnostic)
}

func decodeVisualGateInputV2(t *testing.T, raw string) VisualGateInputV2 {
	t.Helper()
	var input VisualGateInputV2
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&input))
	return input
}

func v2CheckStatus(t *testing.T, report VisualGateReportV2, id string) string {
	t.Helper()
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	var envelope struct {
		Checks []VisualCheck `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(raw, &envelope))
	for _, check := range envelope.Checks {
		if check.ID == id {
			return check.Status
		}
	}
	t.Fatalf("visual check %q not found", id)
	return ""
}

func quoteJSON(t *testing.T, value string) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}
