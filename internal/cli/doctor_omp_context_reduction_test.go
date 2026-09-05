package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The handshake probe passes on every omp/18.1.x tested, and the compaction
// benefit is still absent there. Reporting only what the probe proves would
// tell an operator that context compaction is healthy while a cohort measured
// zero basis points of reduction, so doctor carries the measured verdict and
// keeps three states apart.
func TestOMPContextReductionVerdict_SeparatesMeasuredFromUnmeasured(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		version string
		good    bool
		reason  string
	}{
		{version: "omp/17.2.7", good: true, reason: "measured_reduction_verified"},
		{version: "omp/18.1.2", good: false, reason: "measured_zero_reduction"},
		{version: "omp/18.1.5", good: false, reason: "measured_zero_reduction"},
		{version: "omp/18.1.10", good: false, reason: "measured_zero_reduction"},
		{version: "omp/18.2.0", good: false, reason: "reduction_unmeasured"},
		{version: "omp/19.0.0", good: false, reason: "reduction_unmeasured"},
		{version: "omp/17.2.6", good: false, reason: "reduction_unmeasured"},
		{version: "", good: false, reason: "version_unavailable"},
	} {
		assert.Equal(t, test.good, ompContextReductionMeasuredGood(test.version),
			"%q measured-good verdict", test.version)
		assert.Equal(t, test.reason, ompContextReductionReason(test.version),
			"%q reason", test.version)
	}
}

// Every reason this file can produce has to survive the doctor allow-list.
// A reason that redacts itself carries no information, which is the failure
// mode this row exists to avoid.
func TestOMPContextReductionReasons_SurviveTheDoctorAllowList(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"omp/17.2.7", "omp/18.1.5", "omp/18.2.0", ""} {
		reason := ompContextReductionReason(version)
		assert.Equal(t, reason, ompContextDoctorSafeReason(reason),
			"reason %q for %q must not be redacted", reason, version)
	}
}
