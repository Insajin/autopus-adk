package worker

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/compress"
)

func TestPipelineExecutor_AggregateResults(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")

	results := []PhaseResult{
		{Phase: PhasePlanner, Output: "plan output", CostUSD: 0.01, DurationMS: 100},
		{Phase: PhaseExecutor, Output: "exec output", CostUSD: 0.05, DurationMS: 500},
		{Phase: PhaseTester, Output: "test output", CostUSD: 0.02, DurationMS: 200},
		{Phase: PhaseReviewer, Output: "review output", CostUSD: 0.01, DurationMS: 100},
	}

	tr := pe.aggregateResults(results, 0.09, 900)
	if tr.CostUSD != 0.09 {
		t.Errorf("CostUSD = %f, want 0.09", tr.CostUSD)
	}
	if tr.DurationMS != 900 {
		t.Errorf("DurationMS = %d, want 900", tr.DurationMS)
	}
	for _, phase := range []string{"planner", "executor", "tester", "reviewer"} {
		if !strings.Contains(tr.Output, phase) {
			t.Errorf("output missing phase %q", phase)
		}
	}
}

func TestPipelineExecutor_PhasePrompts(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")

	tests := []struct {
		name   string
		fn     func(string) string
		expect string
	}{
		{"planner", pe.plannerPrompt, "PLANNER"},
		{"executor", pe.executorPrompt, "EXECUTOR"},
		{"tester", pe.testerPrompt, "TESTER"},
		{"reviewer", pe.reviewerPrompt, "REVIEWER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn("test input")
			if !strings.Contains(result, tt.expect) {
				t.Errorf("prompt missing %q role", tt.expect)
			}
			if !strings.Contains(result, "test input") {
				t.Error("prompt missing input content")
			}
		})
	}
}

func TestPipelineExecutor_PhasePromptUsesServerInstruction(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	pe.SetPhaseInstructions(map[Phase]string{
		PhasePlanner: "Use the server-selected planning instruction.",
	})

	result, err := pe.phasePrompt(PhasePlanner, "test input")
	if err != nil {
		t.Fatalf("phasePrompt returned error: %v", err)
	}
	if !strings.Contains(result, "server-selected planning instruction") {
		t.Fatal("phase prompt should use server-selected instruction")
	}
	if !strings.Contains(result, "test input") {
		t.Fatal("phase prompt should include phase input")
	}
}

func TestPipelineExecutor_PhasePromptUsesServerTemplate(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	pe.SetPhasePromptTemplates(map[Phase]string{
		PhasePlanner: "SERVER TEMPLATE\n\n{{input}}",
	})

	result, err := pe.phasePrompt(PhasePlanner, "test input")
	if err != nil {
		t.Fatalf("phasePrompt returned error: %v", err)
	}
	if !strings.Contains(result, "SERVER TEMPLATE") {
		t.Fatal("phase prompt should use server-selected template")
	}
	if !strings.Contains(result, "test input") {
		t.Fatal("phase prompt template should include phase input")
	}
}

func TestPipelineExecutor_PhasePromptFallsBackToRawInputInSignedMode(t *testing.T) {
	t.Setenv("AUTOPUS_A2A_POLICY_SIGNING_SECRET", "test-secret")

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	result, err := pe.phasePrompt(PhasePlanner, "test input")
	if err != nil {
		t.Fatalf("phasePrompt returned error: %v", err)
	}
	if result != "test input" {
		t.Fatalf("phase prompt = %q, want raw input passthrough", result)
	}
}

func TestNewPipelineExecutor(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "/tmp/mcp.json", "/work")
	if pe == nil {
		t.Fatal("expected non-nil PipelineExecutor")
	}
	if pe.mcpConfig != "/tmp/mcp.json" {
		t.Errorf("mcpConfig = %q, want %q", pe.mcpConfig, "/tmp/mcp.json")
	}
	if pe.workDir != "/work" {
		t.Errorf("workDir = %q, want %q", pe.workDir, "/work")
	}
}

func TestPipelineExecutor_SetCompressor(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	if pe.compressor != nil {
		t.Error("compressor should be nil by default")
	}

	c := compress.NewDefaultCompressor(2)
	pe.SetCompressor(c)
	if pe.compressor == nil {
		t.Error("compressor should be set after SetCompressor")
	}
}

// mockCompressor records calls for testing integration.
type mockCompressor struct {
	calls   []string
	replace bool
}

func (m *mockCompressor) Compress(phaseName, output, provider string) string {
	m.calls = append(m.calls, phaseName)
	if m.replace {
		return "[compressed:" + phaseName + "]"
	}
	return output
}

func TestPipelineExecutor_CompressorInPhaseLoop(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	mc := &mockCompressor{replace: true}
	pe.SetCompressor(mc)

	prevOutput := "raw planner output"
	compressed := pe.compressor.Compress("planner", prevOutput, pe.provider.Name())
	nextPrompt := pe.executorPrompt(compressed)

	if !strings.Contains(nextPrompt, "[compressed:planner]") {
		t.Error("executor prompt should receive compressed planner output")
	}
	if len(mc.calls) != 1 || mc.calls[0] != "planner" {
		t.Errorf("expected 1 call to compressor for 'planner', got %v", mc.calls)
	}
}

func TestPipelineExecutor_NilCompressorPassthrough(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	results := []PhaseResult{{Phase: PhasePlanner, Output: "plan output"}}

	tr := pe.aggregateResults(results, 0, 0)
	if !strings.Contains(tr.Output, "plan output") {
		t.Error("output should contain raw phase output when no compressor set")
	}
}

func TestPipelineExecutor_NopCompressorPassthrough(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	pe.SetCompressor(compress.NopCompressor{})

	output := "some phase output"
	result := pe.compressor.Compress("executor", output, "claude")
	if result != output {
		t.Error("NopCompressor should pass through unchanged")
	}
}
