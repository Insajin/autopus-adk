package telemetry_test

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendAgentRun_AppendsOneClosedEventWithSafeEmptySpec(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	run := telemetry.AgentRun{AgentName: "worker", Status: telemetry.StatusPass}

	require.NoError(t, telemetry.AppendAgentRun(root, run))
	events, err := telemetry.LoadAllEvents(root)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, telemetry.EventTypeAgentRun, events[0].Type)
	var got telemetry.AgentRun
	require.NoError(t, json.Unmarshal(events[0].Data, &got))
	assert.Equal(t, telemetry.StatusPass, got.Status)
}
