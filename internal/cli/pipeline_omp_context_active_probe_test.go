package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The probe overlay differs from the production overlay in the compaction
// chain and in nothing else.
func TestWorkflowContextProbeOverlayWidensOnlyTheCompactionChain(t *testing.T) {
	t.Parallel()
	production := string(workflowContextProductOverlayBody(false, workflowContextProductOverlayMethodOrder))
	probe := string(workflowContextProductOverlayBody(false, workflowContextProbeOverlayMethodOrder))

	assert.Contains(t, production, "methodOrder: [snapcompact]")
	assert.NotContains(t, production, "remote")
	assert.Contains(t, probe, "methodOrder: [remote, snapcompact]")
	assert.Equal(t, strings.Replace(production, "[snapcompact]", "[remote, snapcompact]", 1), probe)
}

func TestWorkflowContextObserveSessionProbeIsExplicitOptIn(t *testing.T) {
	t.Parallel()
	assert.True(t, validWorkflowContextObserveSessionProbeDir(""))
	assert.True(t, validWorkflowContextObserveSessionProbeDir("/tmp/canary/probe"))
	assert.False(t, validWorkflowContextObserveSessionProbeDir("relative/probe"))
	assert.False(t, validWorkflowContextObserveSessionProbeDir("/tmp/canary/probe/"))
	assert.False(t, validWorkflowContextObserveSessionProbeDir("/tmp/canary/../probe"))

	probe, err := startWorkflowContextObserveSessionProbe(workflowContextObserveSessionOptions{})
	require.NoError(t, err)
	assert.Nil(t, probe, "no probe directory means no probe")
	assert.False(t, probe.enabled())
	assert.Equal(t, 2, workflowContextObserveSessionCompactionFloor(probe),
		"a signed cohort still needs two completed compactions")
}

// A native completion is admitted for snapcompact everywhere and for remote
// only while a probe is watching.
func TestPipelineOMPActiveProbeWidensCompactionMethodAcceptance(t *testing.T) {
	t.Parallel()
	remoteEnd := pipelineOMPRPCFrame{
		Type: "auto_compaction_end", Action: "remote", Result: json.RawMessage(`{"summary":"x"}`),
	}
	var production *pipelineOMPActiveProbe
	probe := &pipelineOMPActiveProbe{}

	assert.True(t, production.acceptsCompactionAction("snapcompact"))
	assert.False(t, production.acceptsCompactionAction("remote"))
	assert.False(t, production.validNativeEnd(remoteEnd))
	assert.True(t, probe.acceptsCompactionAction("remote"))
	assert.True(t, probe.validNativeEnd(remoteEnd))
	assert.False(t, probe.acceptsCompactionAction("handoff"))
	assert.False(t, probe.validNativeEnd(pipelineOMPRPCFrame{
		Type: "auto_compaction_end", Action: "remote", Aborted: true, Result: json.RawMessage(`{"summary":"x"}`),
	}), "widening the method never relaxes the rest of the completion shape")
	assert.True(t, probe.toleratesUIRequest(pipelineOMPRPCFrame{Type: "extension_ui_request", Method: "notify"}))
	assert.False(t, probe.toleratesUIRequest(pipelineOMPRPCFrame{Type: "extension_ui_request", Method: "confirm"}))
	assert.False(t, production.toleratesUIRequest(pipelineOMPRPCFrame{Type: "extension_ui_request", Method: "notify"}))
}

func TestPipelineOMPActiveProbeTurnUsageReadsOnlyReportedMembers(t *testing.T) {
	t.Parallel()
	usage, found := pipelineOMPActiveProbeFrameUsage(pipelineOMPRPCFrame{
		Type:    "turn_end",
		Message: json.RawMessage(`{"role":"assistant","usage":{"input":120,"cacheRead":30,"output":9,"totalTokens":159}}`),
	})
	require.True(t, found)
	assert.Equal(t, ompprobe.Usage{Input: 120, CacheRead: 30, Output: 9, Total: 159}, usage)

	usage, found = pipelineOMPActiveProbeFrameUsage(pipelineOMPRPCFrame{
		Type: "turn_end", Usage: json.RawMessage(`{"input":7,"totalTokens":9}`),
	})
	require.True(t, found)
	assert.Equal(t, ompprobe.Usage{Input: 7, Total: 9}, usage)

	_, found = pipelineOMPActiveProbeFrameUsage(pipelineOMPRPCFrame{Type: "turn_end"})
	assert.False(t, found, "a turn that reported no usage contributes no usage")
	_, found = pipelineOMPActiveProbeFrameUsage(pipelineOMPRPCFrame{
		Type: "turn_end", Usage: json.RawMessage(`{"input":-1}`),
	})
	assert.False(t, found, "a negative member voids the reading instead of being clamped")
}

func TestPipelineOMPActiveProbeUsageIdentityDeltaSubtractsObservedTurns(t *testing.T) {
	t.Parallel()
	probe := &pipelineOMPActiveProbe{
		statsBefore: &ompprobe.Usage{Input: 100, CacheRead: 10},
		statsAfter:  &ompprobe.Usage{Input: 300, CacheRead: 40, CacheWrite: 5},
		turnUsages:  []ompprobe.Usage{{Input: 200, CacheRead: 30, CacheWrite: 5}},
	}
	assert.Zero(t, probe.usageIdentityDelta(), "frame usage that accounts for the charge reports zero")

	probe.turnUsages = nil
	assert.Equal(t, int64(235), probe.usageIdentityDelta(),
		"with no turn usage observed the whole billed delta is unaccounted for")
	probe.statsAfter = nil
	assert.Zero(t, probe.usageIdentityDelta())
}

func TestPipelineOMPActiveProbeTallyRejectsOutOfRangeIdentifiers(t *testing.T) {
	t.Parallel()
	var tally pipelineOMPActiveProbeTally
	tally.observe("compactionSummary", 3)
	tally.observe("compactionSummary", 3)
	tally.observe("toolCall", 3)
	tally.observe("Opaque-Item!", 3)
	tally.observe("", 3)

	assert.Equal(t, map[string]int{"compactionSummary": 2, "toolCall": 1}, tally.histogram())
	assert.Equal(t, []string{"compactionSummary", "toolCall"}, tally.names())
	assert.Equal(t, 2, tally.rejected)
	assert.NotContains(t, tally.histogram(), "unprintable")

	tally.observe("openaiResponsesHistory", 3)
	tally.observe("beyondTheBound", 3)
	assert.Equal(t, 3, tally.rejected, "a token past the record bound is rejected, not merged")
	assert.Len(t, tally.names(), 3)
}

func TestPipelineOMPActiveProbeRefusalClassCoversKnownDeclines(t *testing.T) {
	t.Parallel()
	for message, want := range map[string]string{
		"Nothing to compact (session too small)":        "too_small",
		"Nothing to compact (no messages yet)":          "no_messages",
		"snapcompact would not reduce context locally.": "would_not_reduce",
		"Already compacted":                             "none",
	} {
		assert.Equal(t, want, pipelineOMPActiveProbeRefusalClass(message), "refusal %q", message)
	}
}
