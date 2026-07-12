package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func TestWriteOrchestraPrimaryOutput_PrefersPaneYieldMetadata(t *testing.T) {
	t.Parallel()

	want := orchestra.YieldOutput{
		Strategy:  "debate",
		Rounds:    1,
		Panes:     map[string]string{"claude": "surface:1", "codex": "surface:2"},
		SessionID: "orch-session",
	}
	result := &orchestra.OrchestraResult{
		Strategy: orchestra.StrategyDebate,
		Merged:   "must not be printed",
		Yield:    &want,
	}
	var output bytes.Buffer

	structured, err := writeOrchestraPrimaryOutput(&output, result, true, "")

	require.NoError(t, err)
	assert.True(t, structured)
	var got orchestra.YieldOutput
	require.NoError(t, json.Unmarshal(output.Bytes(), &got), "stdout must contain exactly one JSON document")
	assert.Equal(t, want, got)
	assert.NotContains(t, output.String(), "must not be printed")
}
