package worker

import (
	"fmt"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/compress"
)

func TestPipelineExecutor_DefaultCompressor_SoftPrunesAndEmitsEvent(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	var events []compress.CompactionEvent
	pe.SetCompactionRecorder(func(event compress.CompactionEvent) {
		events = append(events, event)
	})

	output := pipelineSuccessfulPairTrace(4)
	next, err := pe.nextPhaseInput(PhaseExecutor, output)
	if err != nil {
		t.Fatalf("nextPhaseInput returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("compaction events = %d, want 1", len(events))
	}
	if events[0].PrunedPairCount != 2 || !containsReason(events[0].ReasonCodes, compress.ReasonSoftPrune) {
		t.Fatalf("unexpected soft event: %#v", events[0])
	}
	if !strings.Contains(next, "artifact_ref=phase-output:sha256:") {
		t.Fatalf("soft-pruned next input missing artifact locator:\n%s", next)
	}
}

func TestPipelineExecutor_DefaultCompressor_IncompletePairBlocksNextPhase(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	output := pipelineSuccessfulPairTrace(4) + `
<tool_call>{"pair_id":"pending","ordinal":9,"command":"unfinished"}</tool_call>`

	_, err := pe.nextPhaseInput(PhaseExecutor, output)
	if err == nil || !strings.Contains(err.Error(), compress.ReasonIncompleteToolPair) {
		t.Fatalf("expected incomplete-pair blocker, got %v", err)
	}
}

func pipelineSuccessfulPairTrace(count int) string {
	var blocks []string
	for i := 1; i <= count; i++ {
		blocks = append(blocks,
			fmt.Sprintf(`<tool_call>{"pair_id":"pair-%d","ordinal":%d,"command":"call"}</tool_call>`, i, i),
			fmt.Sprintf(`<tool_result>{"pair_id":"pair-%d","ordinal":%d,"status":"success","body":"result"}</tool_result>`, i, i),
		)
	}
	return strings.Join(blocks, "\n")
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
