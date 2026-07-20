package desktopobserve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeReceipt_ProviderIdentityAndScopeValuesAreStrict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*RuntimeReceipt)
	}{
		{name: "schema", mutate: func(value *RuntimeReceipt) { value.SchemaVersion = "v2" }},
		{name: "local public name", mutate: func(value *RuntimeReceipt) { value.Provider.Name = "autopus-local" }},
		{name: "unknown provider", mutate: func(value *RuntimeReceipt) { value.Provider.Name = "other-provider" }},
		{name: "empty provider version", mutate: func(value *RuntimeReceipt) { value.Provider.Version = "" }},
		{name: "provider path", mutate: func(value *RuntimeReceipt) { value.Provider.Version = "/Applications/helper" }},
		{name: "protocol", mutate: func(value *RuntimeReceipt) { value.Provider.ProtocolVersion++ }},
		{name: "scope kind", mutate: func(value *RuntimeReceipt) { value.Scope.Kind = "process" }},
		{name: "scope alias", mutate: func(value *RuntimeReceipt) { value.Scope.PublicRef = "window-42" }},
		{name: "scope path", mutate: func(value *RuntimeReceipt) { value.Scope.PublicRef = "/Users/alice/window" }},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			receipt := successfulReceipt(RuntimeProviderLocal)
			test.mutate(&receipt)
			require.Error(t, receipt.Validate())
		})
	}

	require.NoError(t, successfulReceipt(RuntimeProviderLocal).Validate())
	require.NoError(t, successfulReceipt(RuntimeProviderOrca).Validate())
}

func TestRuntimeReceipt_AllFourPublicScopeKindsAcceptOnlyPublicRefs(t *testing.T) {
	t.Parallel()

	tests := []ReceiptScope{
		{Kind: ScopeProvider, PublicRef: "autopus-desktop-local"},
		{Kind: ScopeApplication, PublicRef: "autopus-desktop"},
		{Kind: ScopeWindow, PublicRef: "main-window"},
		{Kind: ScopeState, PublicRef: "state-001"},
	}
	for _, scope := range tests {
		scope := scope
		t.Run(string(scope.Kind), func(t *testing.T) {
			t.Parallel()
			receipt := successfulReceipt(RuntimeProviderLocal)
			receipt.Scope = scope
			require.NoError(t, receipt.Validate())
		})
	}
}

func TestRuntimeReceipt_CapabilitySummaryIsExactSortedSafeAllowlist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*RuntimeReceipt)
	}{
		{name: "missing", mutate: func(value *RuntimeReceipt) { value.CapabilitySummary = value.CapabilitySummary[:4] }},
		{name: "duplicate", mutate: func(value *RuntimeReceipt) { value.CapabilitySummary[4] = value.CapabilitySummary[3] }},
		{name: "unsorted", mutate: func(value *RuntimeReceipt) {
			value.CapabilitySummary[0], value.CapabilitySummary[1] = value.CapabilitySummary[1], value.CapabilitySummary[0]
		}},
		{name: "unsafe operation", mutate: func(value *RuntimeReceipt) {
			value.CapabilitySummary[0].Name = Operation("screenshot")
		}},
		{name: "invalid status", mutate: func(value *RuntimeReceipt) {
			value.CapabilitySummary[0].Status = "unknown"
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			receipt := successfulReceipt(RuntimeProviderLocal)
			test.mutate(&receipt)
			require.Error(t, receipt.Validate())
		})
	}
}

func TestRuntimeReceipt_ReasonNextStepAndFailClosedStatusesAreConsistent(t *testing.T) {
	t.Parallel()

	next := "retry"
	reason := ReasonProviderUnavailable
	tests := []struct {
		name   string
		mutate func(*RuntimeReceipt)
	}{
		{name: "success next step", mutate: func(value *RuntimeReceipt) { value.NextStep = &next }},
		{name: "failure missing next step", mutate: func(value *RuntimeReceipt) { value.ReasonCode = &reason }},
		{name: "unknown reason", mutate: func(value *RuntimeReceipt) {
			unknown := ReasonCode("provider_internal_error")
			value.ReasonCode, value.NextStep = &unknown, &next
		}},
		{name: "wrong normalized next step", mutate: func(value *RuntimeReceipt) {
			value.ReasonCode, value.NextStep = &reason, &next
		}},
		{name: "redaction status", mutate: func(value *RuntimeReceipt) { value.Redaction.Status = "partial" }},
		{name: "quarantine status", mutate: func(value *RuntimeReceipt) { value.Quarantine.Status = "uploaded" }},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			receipt := successfulReceipt(RuntimeProviderLocal)
			test.mutate(&receipt)
			assert.Error(t, receipt.Validate())
		})
	}

	failure := successfulReceipt(RuntimeProviderLocal)
	failure.ReasonCode = &reason
	normalized := NextStep(reason)
	failure.NextStep = &normalized
	require.NoError(t, failure.Validate())
}

func TestRuntimeReceipt_ValidEnumsStillRequireMeaningfullyConsistentCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		reason     ReasonCode
		redaction  RedactionStatus
		quarantine QuarantineStatus
		valid      bool
	}{
		{name: "success not required empty", redaction: RedactionNotRequired, quarantine: QuarantineEmpty, valid: true},
		{name: "success applied cleared", redaction: RedactionApplied, quarantine: QuarantineCleared, valid: true},
		{name: "success redaction failed", redaction: RedactionFailed, quarantine: QuarantineBlocked},
		{name: "success local only", redaction: RedactionApplied, quarantine: QuarantineLocalOnly},
		{name: "success quarantine blocked", redaction: RedactionApplied, quarantine: QuarantineBlocked},
		{name: "ordinary failure clean", reason: ReasonProviderUnavailable, redaction: RedactionNotRequired, quarantine: QuarantineEmpty, valid: true},
		{name: "ordinary failure claims redaction", reason: ReasonProviderUnavailable, redaction: RedactionFailed, quarantine: QuarantineBlocked},
		{name: "ordinary failure claims quarantine", reason: ReasonProviderUnavailable, redaction: RedactionApplied, quarantine: QuarantineLocalOnly},
		{name: "redaction failure exact", reason: ReasonRedactionFailed, redaction: RedactionFailed, quarantine: QuarantineBlocked, valid: true},
		{name: "redaction failure not required", reason: ReasonRedactionFailed, redaction: RedactionNotRequired, quarantine: QuarantineBlocked},
		{name: "redaction failure applied", reason: ReasonRedactionFailed, redaction: RedactionApplied, quarantine: QuarantineBlocked},
		{name: "redaction failure local only", reason: ReasonRedactionFailed, redaction: RedactionFailed, quarantine: QuarantineLocalOnly},
		{name: "redaction failure cleared", reason: ReasonRedactionFailed, redaction: RedactionFailed, quarantine: QuarantineCleared},
		{name: "evidence quarantined local only", reason: ReasonEvidenceQuarantined, redaction: RedactionApplied, quarantine: QuarantineLocalOnly, valid: true},
		{name: "evidence quarantined blocked", reason: ReasonEvidenceQuarantined, redaction: RedactionApplied, quarantine: QuarantineBlocked, valid: true},
		{name: "evidence quarantined empty", reason: ReasonEvidenceQuarantined, redaction: RedactionApplied, quarantine: QuarantineEmpty},
		{name: "evidence quarantined cleared", reason: ReasonEvidenceQuarantined, redaction: RedactionApplied, quarantine: QuarantineCleared},
		{name: "evidence quarantined wrong reason status", reason: ReasonEvidenceQuarantined, redaction: RedactionFailed, quarantine: QuarantineBlocked},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			receipt := successfulReceipt(RuntimeProviderLocal)
			receipt.Redaction.Status = test.redaction
			receipt.Quarantine.Status = test.quarantine
			if test.reason != "" {
				reason := test.reason
				next := NextStep(reason)
				receipt.ReasonCode, receipt.NextStep = &reason, &next
			}
			err := receipt.Validate()
			if test.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}
