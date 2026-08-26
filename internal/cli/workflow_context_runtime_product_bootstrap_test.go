package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextProductEntrypoint_NoInjectedRuntimeFailsClosed(t *testing.T) {
	input, _, _, _ := newWorkflowContextProductFixture(t)
	intent := workflowContextProductIntentV1{
		SchemaVersion: workflowContextProductIntentSchemaVersion,
		OriginalTask:  input.OriginalTask, DecisionDelta: input.DecisionDelta,
	}

	receipt, err := workflowContextProductEntrypoint(context.Background())(
		context.Background(), intent,
	)

	require.Error(t, err)
	assert.Equal(t, WorkflowContextRuntimeReceipt{}, receipt)
	lower := strings.ToLower(err.Error())
	assert.Contains(t, lower, "authoritative runtime context is unavailable")
	assertProductOMPConfigsUnchanged(t, input.ProjectDir)
}

func TestWorkflowContextProductSession_CobraWithoutAuthorityFailsClosed(t *testing.T) {
	payload := `{"schema_version":"workflow-context-product-intent/v1",` +
		`"original_task":"/auto go SPEC-OMP-004 --auto","decision_delta":"private decision"}`
	root := NewRootCmd()
	root.SetIn(strings.NewReader(payload))
	var output strings.Builder
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"workflow", "context-runtime", "session", "--request-json", "-", "--format", "json"})

	err := root.Execute()

	require.ErrorContains(t, err, "authoritative runtime context is unavailable")
	assert.NotContains(t, output.String(), "/auto go SPEC-OMP-004 --auto")
	assert.NotContains(t, output.String(), "private decision")
}

func TestDecodeWorkflowContextProductIntent_FailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "scalar identity",
			payload: `{"schema_version":"workflow-context-product-intent/v1","original_task":"first","original_task":"second","decision_delta":"continue"}`,
		},
		{
			name:    "authority field",
			payload: `{"schema_version":"workflow-context-product-intent/v1","original_task":"/auto go SPEC-OMP-004 --auto","decision_delta":"continue","task_id":"forged"}`,
		},
		{
			name:    "prompt authority",
			payload: `{"schema_version":"workflow-context-product-intent/v1","original_task":"/auto go SPEC-OMP-004 --auto","decision_delta":"continue","prompts":[]}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeWorkflowContextProductIntent(strings.NewReader(test.payload))
			require.Error(t, err)
			assert.True(t, strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
				strings.Contains(strings.ToLower(err.Error()), "unknown field"))
		})
	}
}

func TestDecodeWorkflowContextProductIntent_RejectsAuthorityFields(t *testing.T) {
	for _, field := range []string{
		"project_dir", "command", "spec_dir", "spec_id", "task_id", "phase", "session_id",
		"prompts", "frozen_finding_ids", "ownership_paths", "forbidden_paths",
		"capabilities", "promotion", "canary_rows", "history_rows", "endpoint", "model", "validity_seconds",
	} {
		t.Run(field, func(t *testing.T) {
			payload := `{"schema_version":"workflow-context-product-intent/v1",` +
				`"original_task":"/auto go SPEC-OMP-004 --auto","decision_delta":"continue",` +
				`"` + field + `":null}`
			_, err := decodeWorkflowContextProductIntent(strings.NewReader(payload))
			require.ErrorContains(t, err, "unknown field")
		})
	}
}

func TestDecodeWorkflowContextProductIntent_ValidatesEnvelopeAndRequiredIntent(t *testing.T) {
	valid := `{"schema_version":"workflow-context-product-intent/v1",` +
		`"original_task":"/auto go SPEC-OMP-004 --auto","decision_delta":"continue"}`
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{name: "empty", payload: "  \n", want: "exactly one object"},
		{name: "array", payload: "[]", want: "exactly one object"},
		{name: "malformed", payload: `{"schema_version":`, want: "decode request JSON"},
		{name: "trailing object", payload: valid + `{}`, want: "exactly one object"},
		{name: "trailing malformed", payload: valid + `{`, want: "exactly one object"},
		{
			name: "wrong schema",
			payload: `{"schema_version":"workflow-context-product-intent/v2",` +
				`"original_task":"/auto go SPEC-OMP-004 --auto","decision_delta":"continue"}`,
			want: "unsupported product intent schema",
		},
		{
			name: "blank original task",
			payload: `{"schema_version":"workflow-context-product-intent/v1",` +
				`"original_task":" ","decision_delta":"continue"}`,
			want: "requires original_task and decision_delta",
		},
		{
			name: "blank decision delta",
			payload: `{"schema_version":"workflow-context-product-intent/v1",` +
				`"original_task":"/auto go SPEC-OMP-004 --auto","decision_delta":"\t"}`,
			want: "requires original_task and decision_delta",
		},
		{
			name: "nested duplicate before unknown field rejection",
			payload: `{"schema_version":"workflow-context-product-intent/v1",` +
				`"original_task":"task","decision_delta":"continue","nested":[{"key":1,"key":2}]}`,
			want: `duplicate object key "key"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeWorkflowContextProductIntent(strings.NewReader(test.payload))
			require.ErrorContains(t, err, test.want)
		})
	}

	intent, err := decodeWorkflowContextProductIntent(strings.NewReader(valid))
	require.NoError(t, err)
	assert.Equal(t, workflowContextProductIntentSchemaVersion, intent.SchemaVersion)
	assert.Equal(t, "/auto go SPEC-OMP-004 --auto", intent.OriginalTask)
	assert.Equal(t, "continue", intent.DecisionDelta)
}

func TestDecodeWorkflowContextProductIntent_BoundsAndPropagatesReads(t *testing.T) {
	_, err := decodeWorkflowContextProductIntent(workflowContextProductErrorReader{})
	require.ErrorContains(t, err, "read request JSON: forced read failure")

	oversized := strings.Repeat("x", workflowContextProductRequestMaxBytes+1)
	_, err = decodeWorkflowContextProductIntent(strings.NewReader(oversized))
	require.ErrorContains(t, err, "request JSON exceeds")
}

func TestWorkflowContextManagedProductCommandNames_RecognizesRouterForms(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   [2]string
		valid  bool
	}{
		{name: "exact root", prompt: "/auto", want: [2]string{"auto", "auto"}, valid: true},
		{name: "subcommand", prompt: "/auto go SPEC-OMP-004 --auto", want: [2]string{"auto", "auto-go"}, valid: true},
		{name: "flag first", prompt: "/auto --quality ultra go SPEC-OMP-004", want: [2]string{"auto", "auto-go"}, valid: true},
		{name: "equals flag first", prompt: "/auto --quality=ultra go SPEC-OMP-004", want: [2]string{"auto", "auto-go"}, valid: true},
		{name: "multiple flags first", prompt: "/auto --auto --model openai/gpt-5.6-sol --variant high go", want: [2]string{"auto", "auto-go"}, valid: true},
		{name: "direct command", prompt: "/auto-go SPEC-OMP-004 --auto", want: [2]string{"auto", "auto-go"}, valid: true},
		{name: "unknown route", prompt: "/auto deploy", valid: false},
		{name: "unknown direct route", prompt: "/auto-deploy", valid: false},
		{name: "unknown leading flag", prompt: "/auto --unknown go", valid: false},
		{name: "missing flag value", prompt: "/auto --quality", valid: false},
		{name: "flag followed by flag", prompt: "/auto --quality --auto go", valid: false},
		{name: "empty equals flag value", prompt: "/auto --quality= go", valid: false},
		{name: "flags without route", prompt: "/auto --auto", valid: false},
		{name: "malformed direct command", prompt: "/auto--go", valid: false},
		{name: "missing auto prefix", prompt: "go SPEC-OMP-004", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := workflowContextManagedProductCommandNames(test.prompt)
			validationErr := validateWorkflowContextManagedProductPrompts([]string{test.prompt, "continue"})
			if !test.valid {
				require.Error(t, err)
				require.Error(t, validationErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, validationErr)
			assert.Equal(t, test.want, got)
			assert.NotContains(t, got[1], "---")
		})
	}
}

type workflowContextProductErrorReader struct{}

func (workflowContextProductErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("forced read failure")
}
