package desktopobserve

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeReceiptV1_ExactAllowlistAndNullSuccessReason(t *testing.T) {
	t.Parallel()

	receipt := successfulReceipt(RuntimeProviderLocal)
	require.NoError(t, receipt.Validate())
	body, err := json.Marshal(receipt)
	require.NoError(t, err)

	var object map[string]any
	require.NoError(t, json.Unmarshal(body, &object))
	assert.ElementsMatch(t, []string{
		"schema_version",
		"provider",
		"scope",
		"capability_summary",
		"reason_code",
		"next_step",
		"redaction",
		"quarantine",
	}, mapKeys(object))
	assert.Contains(t, object, "reason_code")
	assert.Nil(t, object["reason_code"])
	assert.Contains(t, object, "next_step")
	assert.Nil(t, object["next_step"])

	provider := object["provider"].(map[string]any)
	assert.ElementsMatch(t, []string{"name", "version", "protocol_version"}, mapKeys(provider))
	scope := object["scope"].(map[string]any)
	assert.ElementsMatch(t, []string{"kind", "public_ref"}, mapKeys(scope))
	redaction := object["redaction"].(map[string]any)
	assert.ElementsMatch(t, []string{"status"}, mapKeys(redaction))
	quarantine := object["quarantine"].(map[string]any)
	assert.ElementsMatch(t, []string{"status"}, mapKeys(quarantine))
}

func TestRuntimeReceiptV1_RejectsUnknownOrSensitiveFields(t *testing.T) {
	t.Parallel()

	unknown := []byte(`{
		"schema_version":"qamesh.runtime_receipt.v1",
		"provider":{"name":"autopus-desktop-local","version":"1.0.0","protocol_version":1},
		"scope":{"kind":"window","public_ref":"main-window"},
		"capability_summary":[],
		"reason_code":null,
		"next_step":null,
		"redaction":{"status":"not_required"},
		"quarantine":{"status":"empty"},
		"extensions":{"pid":42}
	}`)
	_, err := DecodeRuntimeReceipt(unknown)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownField)

	receipt := successfulReceipt(RuntimeProviderLocal)
	receipt.Provider.Name = "/Users/alice/bin/provider --socket /tmp/private.sock"
	err = receipt.Validate()
	require.Error(t, err)
}

func TestRuntimeReceiptV1_FailureContainsOnlyNormalizedReason(t *testing.T) {
	t.Parallel()

	reason := ReasonRedactionFailed
	receipt := successfulReceipt(RuntimeProviderOrca)
	receipt.ReasonCode = &reason
	nextStep := NextStep(reason)
	receipt.NextStep = &nextStep
	receipt.Redaction.Status = RedactionFailed
	receipt.Quarantine.Status = QuarantineBlocked

	require.NoError(t, receipt.Validate())
	body, err := json.Marshal(receipt)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "provider error")
	assert.NotContains(t, string(body), "raw_tree")
	assert.NotContains(t, string(body), "screenshot")
	assert.NotContains(t, string(body), "/Users/")
}

func successfulReceipt(provider RuntimeProvider) RuntimeReceipt {
	return successfulReceiptForScope(provider, ReceiptScope{Kind: ScopeWindow, PublicRef: "main-window"})
}

func successfulStateReceipt(provider RuntimeProvider) RuntimeReceipt {
	receipt := successfulReceipt(provider)
	receipt.Redaction.Status = RedactionApplied
	receipt.Quarantine.Status = QuarantineCleared
	return receipt
}

func successfulReceiptForScope(provider RuntimeProvider, scope ReceiptScope) RuntimeReceipt {
	identity := providerIdentity(provider)
	return RuntimeReceipt{
		SchemaVersion: RuntimeReceiptSchemaVersion,
		Provider:      identity,
		Scope:         scope,
		CapabilitySummary: []CapabilityStatus{
			{Name: OperationCapabilities, Status: CapabilitySupported},
			{Name: OperationGetState, Status: CapabilitySupported},
			{Name: OperationListApps, Status: CapabilitySupported},
			{Name: OperationListWindows, Status: CapabilitySupported},
			{Name: OperationPermissions, Status: CapabilitySupported},
		},
		ReasonCode: nil,
		NextStep:   nil,
		Redaction:  RedactionReceipt{Status: RedactionNotRequired},
		Quarantine: QuarantineReceipt{Status: QuarantineEmpty},
	}
}

func providerIdentity(provider RuntimeProvider) ProviderIdentity {
	name := "autopus-desktop-local"
	if provider == RuntimeProviderOrca {
		name = "orca-computer-use-macos"
	}
	return ProviderIdentity{
		Name: name, Version: "1.0.0", ProtocolVersion: ProtocolVersion,
	}
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
