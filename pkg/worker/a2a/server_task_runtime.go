package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

func (s *Server) enqueueAndDispatchTask(ctx context.Context, reqID json.RawMessage, params SendMessageParams) error {
	if params.TaskID == "" {
		return fmt.Errorf("missing task ID")
	}
	if err := s.trackQueuedTask(params.TaskID); err != nil {
		return err
	}
	if err := s.prepareTaskDispatch(&params); err != nil {
		s.dropTask(params.TaskID)
		return err
	}

	_ = s.UpdateTaskStatus(params.TaskID, StatusWorking, nil)
	go s.dispatchTask(ctx, reqID, params)
	return nil
}

func (s *Server) trackQueuedTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; exists {
		return fmt.Errorf("duplicate task ID: %s", taskID)
	}
	s.tasks[taskID] = &Task{ID: taskID, Status: StatusWorking}
	return nil
}

func (s *Server) prepareTaskDispatch(params *SendMessageParams) error {
	if err := validateSecurityPolicySignature(params.TaskID, params.SecurityPolicy, params.PolicySignature); err != nil {
		return err
	}
	if err := validateControlPlaneSignature(
		params.TaskID,
		params.Model,
		params.PipelinePhases,
		params.PipelineInstructions,
		params.PipelinePromptTemplates,
		params.IterationBudget,
		params.ControlPlaneCapabilities,
		params.ControlPlaneSignature,
	); err != nil {
		return err
	}

	params.Model, params.PipelinePhases, params.PipelineInstructions, params.PipelinePromptTemplates, params.IterationBudget = applyControlPlaneCapabilities(
		params.Model,
		params.PipelinePhases,
		params.PipelineInstructions,
		params.PipelinePromptTemplates,
		params.IterationBudget,
		params.ControlPlaneCapabilities,
	)
	if err := cacheSecurityPolicy(params.TaskID, params.SecurityPolicy, params.PolicySignature); err != nil {
		log.Printf("[a2a] cache policy error: %v", err)
	}
	return nil
}

func (s *Server) dropTask(taskID string) {
	s.mu.Lock()
	delete(s.tasks, taskID)
	s.mu.Unlock()
}

// @AX:ANCHOR [AUTO] task execution contract — applies SecurityPolicy timeout and per-task context; callers rely on UpdateTaskStatus being sent for all terminal states — fan_in: 3 (handleSendMessage goroutine, cancel path, timeout path)
// dispatchTask runs the task handler and reports the result.
// Uses a per-task cancellable context so individual tasks can be canceled (REQ-A2A-H02).
// Applies SecurityPolicy.TimeoutSec as a hard deadline when configured.
func (s *Server) dispatchTask(ctx context.Context, reqID json.RawMessage, params SendMessageParams) {
	taskCtx, cancel := s.newTaskContext(ctx, params)
	s.storeTaskCancel(params.TaskID, cancel)
	defer s.releaseTaskContext(params.TaskID, cancel)

	payload, err := mergeTaskPayload(
		params.Payload,
		params.Model,
		params.PipelinePhases,
		params.PipelineInstructions,
		params.PipelinePromptTemplates,
		params.IterationBudget,
	)
	if err != nil {
		s.failTaskDispatch(params.TaskID, reqID, err)
		return
	}

	result, err := s.handler(taskCtx, params.TaskID, payload)
	if err != nil {
		s.failTaskDispatch(params.TaskID, reqID, err)
		return
	}
	result.Status = StatusCompleted
	_ = s.UpdateTaskStatus(params.TaskID, StatusCompleted, result)
	if len(reqID) > 0 {
		s.sendResult(reqID, result)
	}
}

func (s *Server) newTaskContext(ctx context.Context, params SendMessageParams) (context.Context, context.CancelFunc) {
	if params.SecurityPolicy.TimeoutSec <= 0 {
		return context.WithCancel(ctx)
	}

	timeout := time.Duration(params.SecurityPolicy.TimeoutSec) * time.Second
	log.Printf("[a2a] task %s: applying timeout %ds from SecurityPolicy", params.TaskID, params.SecurityPolicy.TimeoutSec)
	return context.WithTimeout(ctx, timeout)
}

func (s *Server) storeTaskCancel(taskID string, cancel context.CancelFunc) {
	s.mu.Lock()
	s.taskContexts[taskID] = cancel
	s.mu.Unlock()
}

func (s *Server) releaseTaskContext(taskID string, cancel context.CancelFunc) {
	cancel()
	s.mu.Lock()
	delete(s.taskContexts, taskID)
	s.mu.Unlock()
}

func (s *Server) failTaskDispatch(taskID string, reqID json.RawMessage, err error) {
	failResult := &TaskResult{Status: StatusFailed, Error: err.Error()}
	_ = s.UpdateTaskStatus(taskID, StatusFailed, failResult)
	if len(reqID) > 0 {
		s.sendResult(reqID, failResult)
	}
}

func mergeTaskPayload(payload json.RawMessage, model string, pipelinePhases []string, pipelineInstructions map[string]string, pipelinePromptTemplates map[string]string, iterationBudget *IterationBudget) (json.RawMessage, error) {
	if model == "" && len(pipelinePhases) == 0 && len(pipelineInstructions) == 0 && len(pipelinePromptTemplates) == 0 && !hasIterationBudget(iterationBudget) {
		return payload, nil
	}

	obj, err := taskPayloadObject(payload)
	if err != nil {
		return nil, err
	}
	applyTransportMetadata(obj, model, pipelinePhases, pipelineInstructions, pipelinePromptTemplates, iterationBudget)

	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal merged payload: %w", err)
	}
	return data, nil
}

func taskPayloadObject(payload json.RawMessage) (map[string]any, error) {
	if len(payload) == 0 {
		return map[string]any{}, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, fmt.Errorf("merge transport metadata into payload: %w", err)
	}
	if obj == nil {
		return map[string]any{}, nil
	}
	return obj, nil
}

func applyTransportMetadata(obj map[string]any, model string, pipelinePhases []string, pipelineInstructions map[string]string, pipelinePromptTemplates map[string]string, iterationBudget *IterationBudget) {
	if model != "" {
		obj["model"] = model
	}
	if len(pipelinePhases) > 0 {
		obj["pipeline_phases"] = pipelinePhases
	}
	if len(pipelineInstructions) > 0 {
		obj["pipeline_instructions"] = pipelineInstructions
	}
	if len(pipelinePromptTemplates) > 0 {
		obj["pipeline_prompt_templates"] = pipelinePromptTemplates
	}
	if hasIterationBudget(iterationBudget) {
		obj["iteration_budget"] = iterationBudget
	}
}
