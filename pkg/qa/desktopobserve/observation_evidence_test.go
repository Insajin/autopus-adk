package desktopobserve

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeObservationEvidence_ValidSuccessAndFailureRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		evidence ObservationEvidence
	}{
		{name: "success", evidence: successfulObservationFixture(t)},
		{name: "failure", evidence: failedObservationFixture()},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body, err := json.Marshal(test.evidence)
			require.NoError(t, err)

			decoded, err := DecodeObservationEvidence(body)
			require.NoError(t, err)
			assert.Equal(t, test.evidence.RuntimeReceipt, decoded.RuntimeReceipt)
			assert.Equal(t, test.evidence.DeterministicChecks, decoded.DeterministicChecks)
			if test.evidence.SemanticProjection == nil {
				assert.Nil(t, decoded.SemanticProjection)
				return
			}
			require.NotNil(t, decoded.SemanticProjection)
			assert.Equal(t, test.evidence.SemanticProjection.Digest, decoded.SemanticProjection.Digest)
			assert.NotEmpty(t, decoded.SemanticProjection.CanonicalJSON)
		})
	}
}

func TestDecodeObservationEvidence_RejectsNestedUnknownAndDuplicateFields(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(successfulObservationFixture(t))
	require.NoError(t, err)
	tests := []struct {
		name        string
		old         string
		replacement string
		want        error
	}{
		{name: "top level unknown", old: `{"semantic_projection":`, replacement: `{"raw_tree":{},"semantic_projection":`, want: ErrUnknownField},
		{name: "projection unknown", old: `"provider_ref":"provider-local"`, replacement: `"provider_ref":"provider-local","raw_handle":"0x42"`, want: ErrUnknownField},
		{name: "node unknown", old: `"role":"AXApplication"`, replacement: `"role":"AXApplication","index":7`, want: ErrUnknownField},
		{name: "state unknown", old: `"enabled":true`, replacement: `"enabled":true,"raw_value":"secret"`, want: ErrUnknownField},
		{name: "check unknown", old: `"id":"desktop-semantic-landmarks"`, replacement: `"id":"desktop-semantic-landmarks","error_text":"secret"`, want: ErrUnknownField},
		{name: "receipt unknown", old: `"name":"autopus-desktop-local"`, replacement: `"name":"autopus-desktop-local","helper_path":"/tmp/helper"`, want: ErrUnknownField},
		{name: "nested duplicate", old: `"provider_ref":"provider-local"`, replacement: `"provider_ref":"provider-local","provider_ref":"provider-other"`, want: ErrDuplicateKey},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mutated := bytes.Replace(body, []byte(test.old), []byte(test.replacement), 1)
			require.NotEqual(t, body, mutated)
			_, err := DecodeObservationEvidence(mutated)
			require.Error(t, err)
			assert.ErrorIs(t, err, test.want)
		})
	}
}

func TestDecodeObservationEvidence_RejectsMissingRecursiveRequiredKeys(t *testing.T) {
	t.Parallel()

	evidence := successfulObservationFixture(t)
	evidence.SemanticProjection.Root.SemanticState = SemanticState{}
	normalized, err := NormalizeProjection(*evidence.SemanticProjection, identityRedactor)
	require.NoError(t, err)
	evidence.SemanticProjection = &normalized
	body, err := json.Marshal(evidence)
	require.NoError(t, err)
	tests := []struct {
		name        string
		old         string
		replacement string
	}{
		{name: "semantic state", old: `,"semantic_state":{}`, replacement: ``},
		{name: "frame width", old: `,"width":1440`, replacement: ``},
		{name: "deterministic checks", old: `,"deterministic_checks":`, replacement: `,"removed_checks":`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mutated := bytes.Replace(body, []byte(test.old), []byte(test.replacement), 1)
			require.NotEqual(t, body, mutated)
			_, err := DecodeObservationEvidence(mutated)
			require.Error(t, err)
		})
	}
}

func TestDecodeObservationEvidence_RejectsTamperedDigestAndRawSensitiveValue(t *testing.T) {
	t.Parallel()

	t.Run("digest", func(t *testing.T) {
		t.Parallel()
		evidence := successfulObservationFixture(t)
		evidence.SemanticProjection.Digest = strings.Repeat("0", 64)
		body, err := json.Marshal(evidence)
		require.NoError(t, err)
		_, err = DecodeObservationEvidence(body)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrMalformedEnvelope)
	})

	t.Run("secret name", func(t *testing.T) {
		t.Parallel()
		evidence := successfulObservationFixture(t)
		evidence.SemanticProjection.Root.Name = "sk-proj-qamesh-secret-1234567890"
		normalized, err := NormalizeProjection(*evidence.SemanticProjection, identityRedactor)
		require.NoError(t, err)
		evidence.SemanticProjection = &normalized
		body, err := json.Marshal(evidence)
		require.NoError(t, err)
		_, err = DecodeObservationEvidence(body)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRedactionFailed)
	})

	t.Run("escaped secret name", func(t *testing.T) {
		t.Parallel()
		evidence := successfulObservationFixture(t)
		evidence.SemanticProjection.Root.Name = "sk-proj-qamesh-secret-1234567890"
		normalized, err := NormalizeProjection(*evidence.SemanticProjection, identityRedactor)
		require.NoError(t, err)
		evidence.SemanticProjection = &normalized
		body, err := json.Marshal(evidence)
		require.NoError(t, err)
		body = bytes.Replace(body, []byte("sk-proj-"), []byte(`sk\u002dproj\u002d`), 1)
		require.NotContains(t, string(body), "sk-proj-")
		_, err = DecodeObservationEvidence(body)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRedactionFailed)
	})
}

func TestDecodeObservationEvidence_UnknownAdvertisedActionEmitsNoSemanticPayload(t *testing.T) {
	t.Parallel()

	evidence := successfulObservationFixture(t)
	body, err := json.Marshal(evidence)
	require.NoError(t, err)
	body = bytes.Replace(body, []byte(`"AXPress"`), []byte(`"AXUnsafe"`), 1)
	require.Contains(t, string(body), "AXUnsafe")

	decoded, err := DecodeObservationEvidence(body)
	require.Error(t, err)
	assert.Nil(t, decoded.SemanticProjection)
}

func TestDecodeObservationEvidence_RejectsContradictoryChecksReceiptAndPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*ObservationEvidence)
	}{
		{name: "unknown check", mutate: func(evidence *ObservationEvidence) { evidence.DeterministicChecks[0].ID = "other-check" }},
		{name: "duplicate check", mutate: func(evidence *ObservationEvidence) {
			evidence.DeterministicChecks = append(evidence.DeterministicChecks, evidence.DeterministicChecks[0])
		}},
		{name: "success blocked", mutate: func(evidence *ObservationEvidence) { evidence.DeterministicChecks[0].Status = CheckBlocked }},
		{name: "success reason", mutate: func(evidence *ObservationEvidence) {
			reason := ReasonStaleState
			evidence.DeterministicChecks[0].ReasonCode = &reason
		}},
		{name: "success projection absent", mutate: func(evidence *ObservationEvidence) { evidence.SemanticProjection = nil }},
		{name: "success redaction proof absent", mutate: func(evidence *ObservationEvidence) {
			evidence.RuntimeReceipt.Redaction.Status = RedactionNotRequired
			evidence.RuntimeReceipt.Quarantine.Status = QuarantineEmpty
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			evidence := successfulObservationFixture(t)
			test.mutate(&evidence)
			body, err := json.Marshal(evidence)
			require.NoError(t, err)
			_, err = DecodeObservationEvidence(body)
			require.Error(t, err)
		})
	}

	failureTests := []struct {
		name   string
		mutate func(*ObservationEvidence)
	}{
		{name: "failure carries projection", mutate: func(evidence *ObservationEvidence) {
			success := successfulObservationFixture(t)
			evidence.SemanticProjection = success.SemanticProjection
		}},
		{name: "failure check passed", mutate: func(evidence *ObservationEvidence) { evidence.DeterministicChecks[0].Status = CheckPassed }},
		{name: "failure check reason absent", mutate: func(evidence *ObservationEvidence) {
			evidence.DeterministicChecks[0].ReasonCode = nil
		}},
		{name: "failure contradictory reason", mutate: func(evidence *ObservationEvidence) {
			reason := ReasonProviderUnavailable
			evidence.DeterministicChecks[0].ReasonCode = &reason
		}},
	}
	for _, test := range failureTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			evidence := failedObservationFixture()
			test.mutate(&evidence)
			body, err := json.Marshal(evidence)
			require.NoError(t, err)
			_, err = DecodeObservationEvidence(body)
			require.Error(t, err)
		})
	}
}

func TestDecodeObservationEvidence_RejectsMalformedAndOversizedInput(t *testing.T) {
	t.Parallel()

	_, err := DecodeObservationEvidence([]byte(`{"semantic_projection":`))
	require.ErrorIs(t, err, ErrMalformedEnvelope)
	_, err = DecodeObservationEvidence(bytes.Repeat([]byte("x"), MaxEnvelopeBytes+1))
	require.ErrorIs(t, err, ErrEnvelopeTooLarge)
}

func successfulObservationFixture(t *testing.T) ObservationEvidence {
	t.Helper()
	projection, err := NormalizeProjection(semanticFixture(), identityRedactor)
	require.NoError(t, err)
	return ObservationEvidence{
		SemanticProjection: &projection,
		DeterministicChecks: []DeterministicCheck{{
			ID: "desktop-semantic-landmarks", Status: CheckPassed,
		}},
		RuntimeReceipt: successfulStateReceipt(RuntimeProviderLocal),
	}
}

func failedObservationFixture() ObservationEvidence {
	reason := ReasonEvidenceQuarantined
	nextStep := NextStep(reason)
	receipt := successfulReceipt(RuntimeProviderLocal)
	receipt.ReasonCode = &reason
	receipt.NextStep = &nextStep
	receipt.Redaction.Status = RedactionApplied
	receipt.Quarantine.Status = QuarantineLocalOnly
	return ObservationEvidence{
		DeterministicChecks: []DeterministicCheck{{
			ID: "desktop-semantic-landmarks", Status: CheckBlocked, ReasonCode: &reason,
		}},
		RuntimeReceipt: receipt,
	}
}
