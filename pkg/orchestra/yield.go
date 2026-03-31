package orchestra

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// YieldOutput is the JSON structure emitted when --yield-rounds triggers an early exit.
type YieldOutput struct {
	Status       string               `json:"status"`        // "yielded"
	RoundsTotal  int                  `json:"rounds_total"`  // total configured rounds
	RoundsRan    int                  `json:"rounds_ran"`    // rounds actually executed before yield
	RoundHistory [][]ProviderResponse `json:"round_history"` // per-round provider responses
	Duration     time.Duration        `json:"duration"`      // wall-clock time elapsed
	SessionID    string               `json:"session_id"`    // session ID for later collect/cleanup
}

// WriteYieldOutput serializes the yield output as JSON to the given writer.
func WriteYieldOutput(w io.Writer, out YieldOutput) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("yield output: %w", err)
	}
	return nil
}
