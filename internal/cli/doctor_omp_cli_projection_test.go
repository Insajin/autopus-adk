package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ompPlatformDoctorChecks(checks []jsonCheck) []jsonCheck {
	var projected []jsonCheck
	for _, check := range checks {
		if strings.HasPrefix(check.ID, "doctor.platform.omp.") {
			projected = append(projected, check)
		}
	}
	return projected
}

func ompValidationDoctorChecks(checks []jsonCheck) []jsonCheck {
	var projected []jsonCheck
	for _, check := range checks {
		if strings.HasPrefix(check.ID, "doctor.platform.omp.validation.") {
			projected = append(projected, check)
		}
	}
	return projected
}

func failingOMPDoctorChecks(checks []jsonCheck) []jsonCheck {
	var failed []jsonCheck
	for _, check := range checks {
		if check.Status == "fail" || check.Status == "warn" {
			failed = append(failed, check)
		}
	}
	return failed
}

func failingNonOMPDoctorChecks(checks []jsonCheck) []jsonCheck {
	var failed []jsonCheck
	for _, check := range checks {
		if !strings.HasPrefix(check.ID, "doctor.platform.omp.") &&
			(check.Status == "fail" || check.Status == "warn") {
			failed = append(failed, check)
		}
	}
	return failed
}

func lineContainingOMPDoctorCheck(t *testing.T, output, id string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, id) {
			return line
		}
	}
	t.Fatalf("text doctor did not render OMP check %q", id)
	return ""
}

func assertOMPDoctorProjectionIsRedacted(t *testing.T, root, text string, checks []jsonCheck) {
	t.Helper()
	encoded, err := json.Marshal(checks)
	require.NoError(t, err)
	projected := string(encoded)
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "doctor.platform.omp.") {
			projected += line
		}
	}
	for _, forbidden := range []string{root, ompDoctorSecretSentinel, ompDoctorRawPayload} {
		assert.NotContains(t, projected, forbidden)
	}
}
