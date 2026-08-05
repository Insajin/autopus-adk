package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/omp"
)

func healthyOMPDoctorReport() omp.OMPReadinessReport {
	ids := []string{
		"identity.version",
		"launch.rpc",
		"launch.no_session",
		"launch.cwd",
		"launch.model",
		"config.overlay_readback",
		"catalog.models_json",
		"rpc.command_discovery",
		"rpc.tool_events",
		"rpc.terminal",
	}
	capabilities := make([]omp.OMPCapabilityResult, 0, len(ids))
	for _, id := range ids {
		capabilities = append(capabilities, omp.OMPCapabilityResult{
			ID: id, Supported: true, Reason: "observed",
		})
	}
	return omp.OMPReadinessReport{
		Executable:    "omp",
		Version:       "omp/17.1.8",
		Capabilities:  capabilities,
		CatalogReason: "catalog_ready",
		SelectorResolutions: []omp.OMPSelectorResolution{{
			Selector: "s7dummy/s7-probe", ResolvedModel: "s7dummy/s7-probe",
			Status: "resolved", Reason: "available",
		}},
	}
}

func ompDoctorCheckByID(t *testing.T, checks []jsonCheck, id string) jsonCheck {
	t.Helper()
	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("missing OMP doctor check %q", id)
	return jsonCheck{}
}

func ompDoctorCheckContaining(t *testing.T, checks []jsonCheck, detail string) jsonCheck {
	t.Helper()
	for _, check := range checks {
		if strings.Contains(check.Detail, detail) {
			return check
		}
	}
	t.Fatalf("missing OMP doctor detail containing %q", detail)
	return jsonCheck{}
}

func validationDoctorIDs(checks []jsonCheck) map[string]string {
	ids := make(map[string]string)
	for _, check := range checks {
		if strings.HasPrefix(check.ID, "doctor.platform.omp.validation.") {
			ids[check.Detail] = check.ID
		}
	}
	return ids
}

func TestOMPDoctorProjection_ValueFindingsHaveStableUniqueIDsAndTextJSONParity(t *testing.T) {
	findings := []adapter.ValidationError{
		{
			File:    ".omp/config.yml",
			Message: "skills.customDirectories expected=[.agents/skills] got=[wrong/path]",
			Level:   "error",
		},
		{
			File:    ".agents/skills",
			Message: "workflow skills expected=20 got=19 missing=[auto-review/SKILL.md] extra=[]",
			Level:   "error",
		},
	}

	checks := projectOMPDoctorChecks(findings, healthyOMPDoctorReport())
	reversed := []adapter.ValidationError{findings[1], findings[0]}
	reorderedChecks := projectOMPDoctorChecks(reversed, healthyOMPDoctorReport())

	ids := validationDoctorIDs(checks)
	require.Len(t, ids, 2)
	assert.Equal(t, ids, validationDoctorIDs(reorderedChecks), "IDs must not depend on finding order")
	unique := make(map[string]struct{}, len(ids))
	for detail, id := range ids {
		assert.NotEmpty(t, id)
		assert.Equal(t, "fail", ompDoctorCheckByID(t, checks, id).Status)
		assert.Equal(t, "error", ompDoctorCheckByID(t, checks, id).Severity)
		assert.Equal(t, detail, ompDoctorCheckByID(t, checks, id).Detail)
		unique[id] = struct{}{}
	}
	assert.Len(t, unique, 2, "distinct value findings require distinct check IDs")

	jsonChecks := sanitizeJSONChecks(checks)
	var text bytes.Buffer
	renderOMPDoctorChecksText(&text, checks)
	for _, check := range jsonChecks {
		if strings.HasPrefix(check.ID, "doctor.platform.omp.validation.") {
			assert.Contains(t, text.String(), check.Detail, "text and JSON must expose the same detail")
		}
	}
}

func TestOMPDoctorProjection_CapabilityResultsUseStableIDsAndReasonPolicy(t *testing.T) {
	report := healthyOMPDoctorReport()
	report.Capabilities[1].Supported = false
	report.Capabilities[1].Reason = "flag_missing"

	checks := projectOMPDoctorChecks(nil, report)
	for _, capability := range report.Capabilities {
		check := ompDoctorCheckByID(t, checks,
			"doctor.platform.omp.capability."+capability.ID)
		assert.Contains(t, check.Detail, capability.Reason)
		if capability.Supported {
			assert.Equal(t, "pass", check.Status, capability.ID)
			assert.Equal(t, "info", check.Severity, capability.ID)
			continue
		}
		assert.Equal(t, "fail", check.Status, capability.ID)
		assert.Equal(t, "error", check.Severity, capability.ID)
	}
}

func TestOMPDoctorProjection_SelectorFailuresSeparateWarningsFromErrors(t *testing.T) {
	report := healthyOMPDoctorReport()
	report.SelectorResolutions = []omp.OMPSelectorResolution{
		{Selector: "s7dummy/s7-probe", ResolvedModel: "s7dummy/s7-probe", Status: "resolved", Reason: "credential_unavailable"},
		{Selector: "s7dummy/", Status: "invalid", Reason: "selector_malformed"},
		{Selector: "unknown-family", Status: "unresolved", Reason: "selector_unresolved"},
		{Selector: "catalog-family", Status: "unresolved", Reason: "catalog_timeout"},
	}

	checks := projectOMPDoctorChecks(nil, report)
	cases := []struct {
		reason, status, severity string
	}{
		{"credential_unavailable", "warn", "warning"},
		{"selector_malformed", "fail", "error"},
		{"selector_unresolved", "fail", "error"},
		{"catalog_timeout", "fail", "error"},
	}
	for _, tc := range cases {
		check := ompDoctorCheckContaining(t, checks, tc.reason)
		assert.True(t, strings.HasPrefix(check.ID, "doctor.platform.omp.selector."), check.ID)
		assert.Equal(t, tc.status, check.Status, tc.reason)
		assert.Equal(t, tc.severity, check.Severity, tc.reason)
	}
}

func TestOMPDoctorProjection_CatalogIsNotRequiredWithoutGeneratedSelectors(t *testing.T) {
	report := healthyOMPDoctorReport()
	report.CatalogReason = "catalog_empty"
	report.SelectorResolutions = nil

	check := ompDoctorCheckByID(t, projectOMPDoctorChecks(nil, report), "doctor.platform.omp.catalog")
	assert.Equal(t, "skip", check.Status)
	assert.Equal(t, "info", check.Severity)
	assert.Equal(t, "catalog reason=catalog_empty", check.Detail)
}

func TestOMPDoctorProjection_DropsRawCredentialAndAbsoluteRootFields(t *testing.T) {
	root := t.TempDir()
	credential := "sk-doctor-projection-secret"
	rawPayload := "RAW-PROVIDER-PAYLOAD"
	report := healthyOMPDoctorReport()
	report.Executable = filepath.Join(root, rawPayload+"-"+credential, "omp")
	findings := []adapter.ValidationError{{
		File:    filepath.Join(root, ".omp", "config.yml"),
		Message: "skills.customDirectories expected=[.agents/skills] got=[wrong/path]",
		Level:   "error",
	}}

	checks := projectOMPDoctorChecks(findings, report)
	encoded, err := json.Marshal(sanitizeJSONChecks(checks))
	require.NoError(t, err)
	var text bytes.Buffer
	renderOMPDoctorChecksText(&text, checks)
	combined := string(encoded) + text.String()
	for _, forbidden := range []string{root, credential, rawPayload} {
		assert.NotContains(t, combined, forbidden)
	}
}

func TestOMPDoctorProjection_HealthyReportHasNoValueOrReadinessFailures(t *testing.T) {
	checks := projectOMPDoctorChecks(nil, healthyOMPDoctorReport())
	require.NotEmpty(t, checks)

	var failed []string
	for _, check := range checks {
		if check.Status == "fail" || check.Status == "warn" {
			failed = append(failed, check.ID)
		}
	}
	sort.Strings(failed)
	assert.Empty(t, failed, "healthy OMP validation/readiness must have zero failures")
}
