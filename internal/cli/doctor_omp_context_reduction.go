package cli

import "strings"

// Measured verdicts for the context-compaction benefit, kept beside the pin
// table in scripts/release-tools/advance-omp-pin.sh. The handshake probe cannot
// produce these: manual compaction needs a live provider session with history,
// so only a cohort run measures reduction.
//
// Measured 2026-09-03 with one plan generator, one workload, the binary swapped:
//
//	omp/17.2.7   8 compactions, median reduction cleared 2000 bp
//	omp/18.1.x   2 compactions, median reduction 0 bp
//
// The 18.1.x line emits "snapcompact would not reduce context locally." and
// declines; the string is absent from 17.2.7 and present in 18.1.2, 18.1.5 and
// 18.1.10.
var (
	ompContextReductionVerified = []string{"omp/17.2.7"}
	ompContextReductionZero     = []string{"omp/18.1."}
)

// ompContextReductionMeasuredGood reports whether this repository measured the
// installed version delivering the reduction the promotion evidence attests.
func ompContextReductionMeasuredGood(version string) bool {
	for _, good := range ompContextReductionVerified {
		if version == good {
			return true
		}
	}
	return false
}

// ompContextReductionReason separates "measured and does not reduce" from
// "never measured here". An unmeasured version is unknown, not broken, and
// saying otherwise would be a claim this repository cannot support.
func ompContextReductionReason(version string) string {
	if ompContextReductionMeasuredGood(version) {
		return "measured_reduction_verified"
	}
	for _, zero := range ompContextReductionZero {
		if strings.HasPrefix(version, zero) {
			return "measured_zero_reduction"
		}
	}
	if strings.TrimSpace(version) == "" {
		return "version_unavailable"
	}
	return "reduction_unmeasured"
}
