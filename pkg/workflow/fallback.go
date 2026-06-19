package workflow

// FallbackClass is the taxonomy bucket a workflow route failure is classified
// into. Every known failure maps to exactly one class; silent opt-out is
// forbidden (REQ-008).
type FallbackClass string

const (
	// FallbackFailFast: abort the workflow immediately and fall back to Route A.
	FallbackFailFast FallbackClass = "fail-fast"
	// FallbackFailClosed: refuse to proceed and block (e.g. parity drift).
	FallbackFailClosed FallbackClass = "fail-closed"
	// FallbackResumable: the run can be resumed from a recorded checkpoint.
	FallbackResumable FallbackClass = "resumable"
	// FallbackExplicit: surface to the operator for an explicit decision.
	FallbackExplicit FallbackClass = "explicit"
)

// FailureKind enumerates the workflow route failure causes the classifier
// recognizes.
type FailureKind string

const (
	FailureNonClaudePlatform FailureKind = "non_claude_platform"
	FailureDoctorFail        FailureKind = "doctor_fail"
	FailureParityDrift       FailureKind = "parity_drift"
	FailureExecutionAbort    FailureKind = "execution_abort"
	FailureAPIUnavailable    FailureKind = "api_unavailable"
)

// fallbackTaxonomy maps every known failure kind to exactly one class.
var fallbackTaxonomy = map[FailureKind]FallbackClass{
	FailureNonClaudePlatform: FallbackFailFast,
	FailureDoctorFail:        FallbackFailFast,
	FailureParityDrift:       FallbackFailClosed,
	FailureExecutionAbort:    FallbackResumable,
	FailureAPIUnavailable:    FallbackExplicit,
}

// Classify returns the fallback class for a failure kind. ok is false for an
// unknown kind so callers can detect (and never silently swallow) an
// unclassified failure.
func Classify(k FailureKind) (FallbackClass, bool) {
	class, ok := fallbackTaxonomy[k]
	return class, ok
}

// KnownFailureKinds returns the failure kinds the classifier recognizes.
func KnownFailureKinds() []FailureKind {
	return []FailureKind{
		FailureNonClaudePlatform,
		FailureDoctorFail,
		FailureParityDrift,
		FailureExecutionAbort,
		FailureAPIUnavailable,
	}
}
