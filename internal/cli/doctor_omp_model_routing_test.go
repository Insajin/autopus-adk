package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectOMPModelRoutingDoctorChecks_NoOptInPreservesExistingChecks(t *testing.T) {
	t.Parallel()

	existing := []jsonCheck{{ID: "doctor.platform.omp.catalog", Status: "pass", Detail: "catalog reason=catalog_ready"}}
	got := appendOMPModelRoutingDoctorChecks(existing, omp.OMPModelDoctorReport{})
	assert.Equal(t, existing, got)
}

func TestProjectOMPModelRoutingDoctorChecks_ProjectsSafeAvailabilityRows(t *testing.T) {
	t.Parallel()

	report := omp.OMPModelDoctorReport{
		Enabled: true, Profile: "balanced", Status: "degraded", Reason: "role_degraded", ReceiptStatus: "valid",
		Roles: []omp.OMPModelDoctorRoleRow{
			{Agent: "executor", Role: "autopus_executor", Capability: "coding_tool_use", Status: "supported", Reason: "selected", FamilyDiversity: "not_applicable", EvidenceClass: "availability"},
			{Agent: "reviewer", Role: "autopus_reviewer", Capability: "independent_dissent", Status: "degraded", Reason: "explicit_degraded", FamilyDiversity: "degraded", FamilyReason: "same_family_only", EvidenceClass: "availability"},
		},
	}
	checks := projectOMPModelRoutingDoctorChecks(report)
	require.Len(t, checks, 3)
	overall := ompDoctorCheckByID(t, checks, "doctor.platform.omp.model-routing.receipt")
	assert.Equal(t, "warn", overall.Status)
	assert.Contains(t, overall.Detail, "reason=role_degraded")
	assert.Contains(t, overall.Detail, "receipt=valid")

	encoded, err := json.Marshal(checks)
	require.NoError(t, err)
	text := string(encoded)
	assert.Contains(t, text, "agent=executor role=autopus_executor capability=coding_tool_use")
	assert.Contains(t, text, "evidence=availability quorum=false")
	assert.Contains(t, text, "family_diversity=degraded family_reason=same_family_only")
	for _, forbidden := range []string{"p/code", "q/review", "/Users/", "api_key", "Bearer "} {
		assert.NotContains(t, text, forbidden)
	}
}

func TestProjectOMPModelRoutingDoctorChecks_BlockedReasonsStayExact(t *testing.T) {
	t.Parallel()

	for _, reason := range []string{
		"version_stale", "catalog_stale", "projection_mismatch",
		"receipt_missing", "receipt_invalid", "catalog_metadata_insufficient",
	} {
		report := omp.OMPModelDoctorReport{
			Enabled: true, Profile: "balanced", Status: "blocked", Reason: reason, ReceiptStatus: "invalid",
		}
		check := ompDoctorCheckByID(t, projectOMPModelRoutingDoctorChecks(report),
			"doctor.platform.omp.model-routing.receipt")
		assert.Equal(t, "fail", check.Status, reason)
		assert.Contains(t, check.Detail, "reason="+reason)
	}
}

func TestAppendOMPModelRoutingDoctorChecks_EnabledSortsCombinedChecks(t *testing.T) {
	t.Parallel()

	existing := []jsonCheck{{ID: "doctor.platform.omp.zzz", Status: "pass"}}
	report := omp.OMPModelDoctorReport{
		Enabled: true, Profile: "balanced", Status: "supported", Reason: "fresh", ReceiptStatus: "valid",
	}
	checks := appendOMPModelRoutingDoctorChecks(existing, report)
	require.Len(t, checks, 2)
	assert.Equal(t, "doctor.platform.omp.model-routing.receipt", checks[0].ID)
	assert.Equal(t, "doctor.platform.omp.zzz", checks[1].ID)
}

func TestProjectOMPModelRoutingDoctorChecks_RedactsUnsafeFields(t *testing.T) {
	t.Parallel()

	secret := "sk-secret-provider-payload"
	report := omp.OMPModelDoctorReport{
		Enabled: true, Profile: secret, Status: "blocked", Reason: secret, ReceiptStatus: secret,
		Roles: []omp.OMPModelDoctorRoleRow{{
			Agent: secret, Role: secret, Capability: secret, Status: "blocked",
			Reason: secret, FamilyDiversity: secret, EvidenceClass: secret,
		}},
	}
	checks := projectOMPModelRoutingDoctorChecks(report)
	encoded, err := json.Marshal(checks)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), secret)
	assert.GreaterOrEqual(t, strings.Count(string(encoded), "redacted"), 2)
}
