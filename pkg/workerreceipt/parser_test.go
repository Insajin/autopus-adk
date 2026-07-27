package workerreceipt

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestParseMarkedOutput_OneShortValidReceiptIsAcceptedWithoutPadding(t *testing.T) {
	t.Parallel()

	want := validEnvelopeFixture()
	output := "worker summary\n" + markedEnvelope(t, want)

	got, err := ParseMarkedOutput(output)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want, *got)
	assert.LessOrEqual(t, promptlayer.EstimateTokens(markedEnvelope(t, want)), 2_000)
}

func TestParseMarkedOutput_MarkerlessOutputPreservesLegacyBehavior(t *testing.T) {
	t.Parallel()

	got, err := ParseMarkedOutput(`{"owned_paths":["pkg/pipeline"]}`)

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseMarkedOutput_RejectsAmbiguousMalformedOrOversizedMarkers(t *testing.T) {
	t.Parallel()

	valid := markedEnvelope(t, validEnvelopeFixture())
	oversized := validEnvelopeFixture()
	oversized.Receipt.Verification = []string{strings.Repeat("x", 9_000)}
	require.Greater(t, promptlayer.EstimateTokens(markedEnvelope(t, oversized)), 2_000)

	tests := []struct {
		name   string
		output string
	}{
		{name: "begin without end", output: BeginMarker + `{"schema_version":"autopus.worker_receipt.v1"}`},
		{name: "end without begin", output: EndMarker},
		{name: "duplicate marker", output: valid + "\n" + valid},
		{name: "malformed json", output: BeginMarker + "\n{\n" + EndMarker},
		{
			name: "unknown envelope field",
			output: BeginMarker + `
{"schema_version":"autopus.worker_receipt.v1","receipt":{"owned_paths":[],"changed_files":[],"verification":[],"blockers":[],"next_required_step":"review"},"unknown":true}
` + EndMarker,
		},
		{
			name: "unknown receipt field",
			output: BeginMarker + `
{"schema_version":"autopus.worker_receipt.v1","receipt":{"owned_paths":[],"changed_files":[],"verification":[],"blockers":[],"next_required_step":"review","extra":"no"}}
` + EndMarker,
		},
		{
			name: "trailing json",
			output: BeginMarker + `
{"schema_version":"autopus.worker_receipt.v1","receipt":{"owned_paths":[],"changed_files":[],"verification":[],"blockers":[],"next_required_step":"review"}} {}
` + EndMarker,
		},
		{name: "trailing output after terminal marker", output: valid + "\nuntrusted trailing prose"},
		{name: "oversized", output: markedEnvelope(t, oversized)},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMarkedOutput(tt.output)
			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestParseMarkedOutput_RejectsUnsafeReceiptAndEvidenceReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Envelope)
	}{
		{name: "absolute owned path", mutate: func(value *Envelope) {
			value.Receipt.OwnedPaths = []string{"/Users/example/project"}
		}},
		{name: "windows drive absolute owned path", mutate: func(value *Envelope) {
			value.Receipt.OwnedPaths = []string{"C:/Users/example/project"}
		}},
		{name: "traversing owned path", mutate: func(value *Envelope) {
			value.Receipt.OwnedPaths = []string{"../../etc/passwd"}
		}},
		{name: "non-clean owned path", mutate: func(value *Envelope) {
			value.Receipt.OwnedPaths = []string{"pkg/../internal"}
		}},
		{name: "nul changed file", mutate: func(value *Envelope) {
			value.Receipt.ChangedFiles = []string{"pkg/value.go\x00.md"}
		}},
		{name: "absolute evidence ref", mutate: func(value *Envelope) {
			value.Evidence[0].Ref = "/tmp/evidence.json"
		}},
		{name: "traversing evidence ref", mutate: func(value *Envelope) {
			value.Evidence[0].Ref = "../evidence.json"
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			value := validEnvelopeFixture()
			tt.mutate(&value)
			got, err := ParseMarkedOutput(markedEnvelope(t, value))
			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestParseMarkedOutput_RejectsUnsafeFreeTextBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Envelope)
	}{
		{name: "secret in verification", mutate: func(value *Envelope) {
			value.Receipt.Verification = []string{"OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz"}
		}},
		{name: "unix absolute path in blocker", mutate: func(value *Envelope) {
			value.Receipt.Blockers = []string{"inspect /Users/alice/private/output.log"}
		}},
		{name: "windows absolute path in next step", mutate: func(value *Envelope) {
			value.Receipt.NextRequiredStep = `inspect C:\Users\alice\private\output.log`
		}},
		{name: "raw body in next step", mutate: func(value *Envelope) {
			value.Receipt.NextRequiredStep = "persist raw prompt body"
		}},
		{name: "multiline verification", mutate: func(value *Envelope) {
			value.Receipt.Verification = []string{"go test ./pkg/workerreceipt\nraw output"}
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			value := validEnvelopeFixture()
			tt.mutate(&value)
			got, err := ParseMarkedOutput(markedEnvelope(t, value))
			require.Error(t, err)
			assert.Nil(t, got)
			assert.NotContains(t, err.Error(), "sk-proj-")
			assert.NotContains(t, err.Error(), "/Users/")
		})
	}
}

func validEnvelopeFixture() Envelope {
	return Envelope{
		SchemaVersion: SchemaVersion,
		Receipt: Receipt{
			OwnedPaths:       []string{"pkg/workerreceipt"},
			ChangedFiles:     []string{"pkg/workerreceipt/parser.go"},
			Verification:     []string{"go test ./pkg/workerreceipt"},
			Blockers:         []string{},
			NextRequiredStep: "pipeline consumer",
		},
		Evidence: []EvidenceReference{{
			Ref:  ".autopus/runtime/evidence/worker.json",
			Hash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}},
	}
}

func markedEnvelope(t *testing.T, value Envelope) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return fmt.Sprintf("%s\n%s\n%s", BeginMarker, data, EndMarker)
}
