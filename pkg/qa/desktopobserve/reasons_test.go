package desktopobserve

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExactReasonTaxonomy_HasTenSafeNonEmptyNextSteps(t *testing.T) {
	t.Parallel()

	want := []ReasonCode{
		ReasonProviderUnavailable,
		ReasonCapabilityUnsupported,
		ReasonAccessibilityPermissionMissing,
		ReasonTargetAppNotFound,
		ReasonTargetWindowNotFound,
		ReasonStaleState,
		ReasonSemanticProjectionUnavailable,
		ReasonRedactionFailed,
		ReasonEvidenceQuarantined,
		ReasonProviderProtocolMismatch,
	}
	assert.Equal(t, want, ReasonCodes())
	assert.Len(t, ReasonCodes(), 10)

	unsafe := regexp.MustCompile(`(?i)(/users/|/tmp/|pid\s*[=:]|socket\s*[=:]|handle\s*[=:]|raw[_ -]?title|secret\s*[=:]|\b(?:sh|bash|zsh)\s+-c\b|\brm\s+-rf\b)`)
	for _, reason := range ReasonCodes() {
		next := NextStep(reason)
		assert.NotEmpty(t, next, reason)
		assert.False(t, unsafe.MatchString(next), "%s next_step is unsafe: %s", reason, next)
	}
}
