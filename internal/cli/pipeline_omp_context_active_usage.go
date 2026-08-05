package cli

import (
	"context"
	"encoding/json"
	"errors"
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
