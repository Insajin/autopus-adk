package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/delivery"
)

func TestDeliveryCLIPlanOutputsCanonicalJSON(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"delivery", "plan", "--format", "json", "--repository", "autopus-adk"})

	require.NoError(t, cmd.Execute())

	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto delivery plan")
	data := payload["data"].(map[string]any)
	require.Equal(t, delivery.DryRunPlanSchemaV1, data["schema_version"])
	require.Equal(t, delivery.WorkflowMode, data["workflow_mode"])
	phases := data["phases"].([]any)
	require.Len(t, phases, len(delivery.CanonicalPhases()))
	for i, phase := range delivery.CanonicalPhases() {
		row := phases[i].(map[string]any)
		assert.Equal(t, string(phase), row["phase"])
	}
}

func TestDeliveryCLIValidateAcceptsValidEnvelope(t *testing.T) {
	t.Parallel()

	file := writeDeliveryEnvelope(t, delivery.PhaseResultEnvelope{
		SchemaVersion:      delivery.PhaseResultSchemaV1,
		RequestID:          "REQ-CLI-1",
		Phase:              delivery.PhaseVerify,
		Status:             delivery.StatusPassed,
		Summary:            "verified",
		ChangedFiles:       []string{"pkg/source.go"},
		TestStatus:         "passed",
		EvidenceRefs:       []string{"evidence://verify/1"},
		Blockers:           []string{},
		NextRequiredAction: "enter_qa",
		RedactionStatus:    delivery.RedactionClean,
	})
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"delivery", "validate", "--file", file, "--format", "json"})

	require.NoError(t, cmd.Execute())

	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto delivery validate")
	data := payload["data"].(map[string]any)
	assert.Equal(t, true, data["valid"])
	assert.Equal(t, string(delivery.PhaseVerify), data["phase"])
}

func TestDeliveryCLIValidateRejectsGeneratedRuntimeDrift(t *testing.T) {
	t.Parallel()

	file := writeDeliveryEnvelope(t, delivery.PhaseResultEnvelope{
		SchemaVersion:      delivery.PhaseResultSchemaV1,
		RequestID:          "REQ-CLI-2",
		Phase:              delivery.PhaseSync,
		Status:             delivery.StatusPassed,
		Summary:            "synced",
		ChangedFiles:       []string{".opencode/agents/executor.md"},
		TestStatus:         "passed",
		EvidenceRefs:       []string{"evidence://sync/1"},
		Blockers:           []string{},
		NextRequiredAction: "continue",
		RedactionStatus:    delivery.RedactionClean,
	})
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"delivery", "validate", "--file", file, "--format", "json"})

	err := cmd.Execute()

	require.Error(t, err)
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto delivery validate")
	assert.Equal(t, "error", payload["status"])
	errorPayload := payload["error"].(map[string]any)
	assert.Equal(t, "delivery_validate_failed", errorPayload["code"])
	assert.Contains(t, errorPayload["message"], ".opencode/agents/executor.md")
}

func writeDeliveryEnvelope(t *testing.T, envelope delivery.PhaseResultEnvelope) string {
	t.Helper()
	data, err := json.Marshal(envelope)
	require.NoError(t, err)
	file := filepath.Join(t.TempDir(), "phase-result.json")
	require.NoError(t, os.WriteFile(file, data, 0o644))
	return file
}
