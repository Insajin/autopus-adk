package orchestra

import (
	"encoding/json"
	"fmt"
	"io"
)

// YieldOutput is the JSON structure output by --yield-rounds mode.
type YieldOutput struct {
	Strategy     string            `json:"strategy"`
	Rounds       int               `json:"rounds"`
	RoundHistory []YieldRound      `json:"round_history"`
	Panes        map[string]string `json:"panes"`      // provider -> pane ID
	SessionID    string            `json:"session_id"`
}

// YieldRound holds per-round provider responses.
type YieldRound struct {
	Round     int             `json:"round"`
	Responses []YieldResponse `json:"responses"`
}

// YieldResponse holds a single provider's output for one round.
type YieldResponse struct {
	Provider   string `json:"provider"`
	Output     string `json:"output"`
	DurationMs int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out"`
}

// WriteYieldOutput serializes YieldOutput as JSON to the writer.
func WriteYieldOutput(w io.Writer, output YieldOutput) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal yield output: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// BuildYieldOutputFromResult creates a YieldOutput from an OrchestraResult.
// Used by --no-judge to output structured JSON without requiring pane references.
func BuildYieldOutputFromResult(result *OrchestraResult, sessionID string) YieldOutput {
	var yieldRounds []YieldRound
	for i, responses := range result.RoundHistory {
		yr := YieldRound{Round: i + 1}
		for _, r := range responses {
			yr.Responses = append(yr.Responses, YieldResponse{
				Provider:   r.Provider,
				Output:     r.Output,
				DurationMs: r.Duration.Milliseconds(),
				TimedOut:   r.TimedOut,
			})
		}
		yieldRounds = append(yieldRounds, yr)
	}

	return YieldOutput{
		Strategy:     string(result.Strategy),
		Rounds:       len(result.RoundHistory),
		RoundHistory: yieldRounds,
		SessionID:    sessionID,
	}
}

// BuildYieldOutput creates a YieldOutput from debate state.
func BuildYieldOutput(cfg OrchestraConfig, panes []paneInfo, roundHistory [][]ProviderResponse, sessionID string) YieldOutput {
	paneMap := make(map[string]string)
	for _, pi := range panes {
		paneMap[pi.provider.Name] = string(pi.paneID)
	}

	var yieldRounds []YieldRound
	for i, responses := range roundHistory {
		yr := YieldRound{Round: i + 1}
		for _, r := range responses {
			yr.Responses = append(yr.Responses, YieldResponse{
				Provider:   r.Provider,
				Output:     r.Output,
				DurationMs: r.Duration.Milliseconds(),
				TimedOut:   r.TimedOut,
			})
		}
		yieldRounds = append(yieldRounds, yr)
	}

	return YieldOutput{
		Strategy:     string(cfg.Strategy),
		Rounds:       len(roundHistory),
		RoundHistory: yieldRounds,
		Panes:        paneMap,
		SessionID:    sessionID,
	}
}
