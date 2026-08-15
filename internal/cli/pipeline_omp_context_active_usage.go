package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

func boolToPipelineOMPCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

type pipelineOMPActiveSessionStats struct {
	SessionID string `json:"sessionId"`
	Tokens    struct {
		Input      int64 `json:"input"`
		Output     int64 `json:"output"`
		CacheRead  int64 `json:"cacheRead"`
		CacheWrite int64 `json:"cacheWrite"`
		Total      int64 `json:"total"`
	} `json:"tokens"`
}

type pipelineOMPActiveUsage struct {
	Input  int64
	Output int64
	Total  int64
}

func (protocol *pipelineOMPRPCProtocol) sessionStats(
	ctx context.Context,
	expectedSession string,
) (pipelineOMPActiveUsage, error) {
	data, err := protocol.call(ctx, pipelineOMPRPCCommand{Type: "get_session_stats"}, false)
	if err != nil {
		return pipelineOMPActiveUsage{}, err
	}
	var stats pipelineOMPActiveSessionStats
	if json.Unmarshal(data, &stats) != nil || stats.SessionID != expectedSession {
		return pipelineOMPActiveUsage{}, errors.New("managed active OMP session stats are invalid")
	}
	tokens := stats.Tokens
	effectiveInput := tokens.Input + tokens.CacheRead
	if tokens.Input < 0 || tokens.Output < 0 || tokens.CacheRead < 0 || tokens.CacheWrite < 0 ||
		tokens.Total < 0 || effectiveInput < tokens.Input {
		return pipelineOMPActiveUsage{}, errors.New("managed active OMP session stats are invalid")
	}
	effectiveInputWithWrite := effectiveInput + tokens.CacheWrite
	minimumTotal := effectiveInputWithWrite + tokens.Output
	if effectiveInputWithWrite < effectiveInput || minimumTotal < effectiveInputWithWrite ||
		tokens.Total < minimumTotal {
		return pipelineOMPActiveUsage{}, errors.New("managed active OMP session stats are invalid")
	}
	return pipelineOMPActiveUsage{Input: effectiveInputWithWrite, Output: tokens.Output, Total: tokens.Total}, nil
}

// pipelineOMPActiveUsageVerdict decides whether one managed call moved usage,
// and names why it did not. A fail-closed gate that reports only "empty" forces
// the operator to bisect a 40-call cohort; both the deltas and the declared
// window are known here, so the diagnosis belongs at the point of rejection.
//
// declaredWindow of zero means the caller did not assert one, which disables
// the exhaustion branch rather than guessing.
func pipelineOMPActiveUsageVerdict(before, after pipelineOMPActiveUsage, declaredWindow int) error {
	inputDelta, outputDelta := after.Input-before.Input, after.Output-before.Output
	totalDelta := after.Total - before.Total
	if inputDelta > 0 && outputDelta > 0 && totalDelta >= inputDelta+outputDelta {
		return nil
	}
	// A declared window smaller than the model's real one lets accumulation
	// cross it; OMP then stops calling the provider and every axis goes flat.
	if window := int64(declaredWindow); window > 0 && after.Input >= window {
		return fmt.Errorf(
			"managed active OMP usage delta is empty: cumulative input %d reached the declared "+
				"model context window %d, so the declared window does not match the model's real "+
				"window (in/out/total delta=%d/%d/%d)",
			after.Input, window, inputDelta, outputDelta, totalDelta)
	}
	return fmt.Errorf(
		"managed active OMP usage delta is empty (in/out/total delta=%d/%d/%d, before=%d/%d/%d after=%d/%d/%d)",
		inputDelta, outputDelta, totalDelta,
		before.Input, before.Output, before.Total, after.Input, after.Output, after.Total)
}
