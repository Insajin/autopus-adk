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
		Input  int64 `json:"input"`
		Output int64 `json:"output"`
		Total  int64 `json:"total"`
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
	if json.Unmarshal(data, &stats) != nil || stats.SessionID != expectedSession || stats.Tokens.Input < 0 ||
		stats.Tokens.Output < 0 || stats.Tokens.Total < 0 || stats.Tokens.Total < stats.Tokens.Input+stats.Tokens.Output {
		return pipelineOMPActiveUsage{}, errors.New("managed active OMP session stats are invalid")
	}
	return pipelineOMPActiveUsage{Input: stats.Tokens.Input, Output: stats.Tokens.Output, Total: stats.Tokens.Total}, nil
}
