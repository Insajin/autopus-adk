// Package pipeline provides pipeline state management types and persistence.
package pipeline

import "encoding/json"

// Phase ID constants for the 5-phase pipeline.
const (
	// PhasePlan is the planning phase where the agent analyzes the SPEC.
	PhasePlan PhaseID = "plan"
	// PhaseTestScaffold is the phase where test scaffolding is generated.
	PhaseTestScaffold PhaseID = "test_scaffold"
	// PhaseImplement is the implementation phase.
	PhaseImplement PhaseID = "implement"
	// PhaseValidate is the validation phase where tests and lint are run.
	PhaseValidate PhaseID = "validate"
	// PhaseReview is the final review phase.
	PhaseReview PhaseID = "review"
)

// Phase describes a single pipeline phase and its dependencies.
type Phase struct {
	// ID is the unique identifier of this phase.
	ID PhaseID
	// DependsOn lists the phases that must complete before this phase runs.
	DependsOn []PhaseID
	// Gate is the quality gate applied after this phase completes.
	Gate GateType
	// MaxRetries is the maximum number of retry attempts when the gate fails.
	MaxRetries int
}

// @AX:ANCHOR: [AUTO] cross-cutting concern — canonical phase registry consumed by engine, runner, and tests (fan-in >= 3)
// DefaultPhases returns the canonical 5-phase pipeline in execution order.
func DefaultPhases() []Phase {
	return []Phase{
		{ID: PhasePlan, DependsOn: nil},
		{ID: PhaseTestScaffold, DependsOn: []PhaseID{PhasePlan}},
		{ID: PhaseImplement, DependsOn: []PhaseID{PhaseTestScaffold}},
		{ID: PhaseValidate, DependsOn: []PhaseID{PhaseImplement}},
		{ID: PhaseReview, DependsOn: []PhaseID{PhaseValidate}},
	}
}

// claudeOutput is the JSON structure returned by Claude CLI.
type claudeOutput struct {
	LastAssistantMessage string `json:"last_assistant_message"`
}

// codexOutput is the JSON structure returned by Codex CLI.
type codexOutput struct {
	Text string `json:"text"`
}

// geminiOutput is the JSON structure returned by Gemini CLI.
type geminiOutput struct {
	PromptResponse string `json:"prompt_response"`
}

// @AX:NOTE: [AUTO] @AX:REASON: downgraded from ANCHOR — fan-in 2 (phase.go + phase_test.go), below 3 threshold
// @AX:NOTE: [AUTO] magic constants — platform name strings "claude", "codex", "gemini" are implicit contract
// NormalizeOutput parses platform-specific JSON output and returns the
// extracted text. If the input is not valid JSON or the field is missing,
// the raw input is returned unchanged.
func NormalizeOutput(platform, output string) string {
	switch platform {
	case "claude":
		var v claudeOutput
		if err := json.Unmarshal([]byte(output), &v); err == nil && v.LastAssistantMessage != "" {
			return v.LastAssistantMessage
		}
	case "codex":
		var v codexOutput
		if err := json.Unmarshal([]byte(output), &v); err == nil && v.Text != "" {
			return v.Text
		}
	case "gemini":
		var v geminiOutput
		if err := json.Unmarshal([]byte(output), &v); err == nil && v.PromptResponse != "" {
			return v.PromptResponse
		}
	}
	return output
}
