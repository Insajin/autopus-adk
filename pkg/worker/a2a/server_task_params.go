package a2a

import (
	"context"
	"encoding/json"
	"fmt"
)

// HandlePolledTask routes a REST-polled task through the same dispatch path used
// by WebSocket-delivered tasks.
func (s *Server) HandlePolledTask(ctx context.Context, task PollResult) error {
	params, err := paramsFromPolledTask(task)
	if err != nil {
		return err
	}
	return s.enqueueAndDispatchTask(ctx, nil, params)
}

func paramsFromPolledTask(task PollResult) (SendMessageParams, error) {
	if task.ID == "" {
		return SendMessageParams{}, fmt.Errorf("missing polled task ID")
	}

	var params SendMessageParams
	if err := json.Unmarshal(task.Payload, &params); err == nil {
		return hydrateTransportTaskParams(task, params), nil
	}
	return paramsFromRawPolledTask(task), nil
}

func hydrateTransportTaskParams(task PollResult, params SendMessageParams) SendMessageParams {
	if params.TaskID == "" {
		params.TaskID = task.ID
	}
	if params.Model == "" {
		params.Model = task.Model
	}
	if len(params.PipelinePhases) == 0 {
		params.PipelinePhases = append([]string(nil), task.PipelinePhases...)
	}
	if len(params.PipelineInstructions) == 0 && len(task.PipelineInstructions) > 0 {
		params.PipelineInstructions = cloneStringMap(task.PipelineInstructions)
	}
	if len(params.PipelinePromptTemplates) == 0 && len(task.PipelinePromptTemplates) > 0 {
		params.PipelinePromptTemplates = cloneStringMap(task.PipelinePromptTemplates)
	}
	if params.IterationBudget == nil && task.IterationBudget != nil {
		params.IterationBudget = cloneIterationBudget(task.IterationBudget)
	}
	if len(params.ControlPlaneCapabilities) == 0 && len(task.ControlPlaneCapabilities) > 0 {
		params.ControlPlaneCapabilities = append([]string(nil), task.ControlPlaneCapabilities...)
	}
	if params.ControlPlaneSignature == "" {
		params.ControlPlaneSignature = task.ControlPlaneSignature
	}
	if params.PolicySignature == "" {
		params.PolicySignature = task.PolicySignature
	}
	if len(params.Payload) == 0 {
		params.Payload = task.Payload
	}
	return params
}

func paramsFromRawPolledTask(task PollResult) SendMessageParams {
	return SendMessageParams{
		TaskID:                   task.ID,
		Model:                    task.Model,
		PipelinePhases:           append([]string(nil), task.PipelinePhases...),
		PipelineInstructions:     cloneStringMap(task.PipelineInstructions),
		PipelinePromptTemplates:  cloneStringMap(task.PipelinePromptTemplates),
		IterationBudget:          cloneIterationBudget(task.IterationBudget),
		ControlPlaneCapabilities: append([]string(nil), task.ControlPlaneCapabilities...),
		ControlPlaneSignature:    task.ControlPlaneSignature,
		PolicySignature:          task.PolicySignature,
		Payload:                  task.Payload,
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
