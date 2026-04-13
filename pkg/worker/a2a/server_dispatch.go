package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// @AX:NOTE [AUTO] magic constants — maxBackoff:30s, initial backoff:500ms; controls REST fallback activation latency
// @AX:ANCHOR [AUTO] connection resilience loop — fires OnConnectionExhausted and starts RESTPoller on exhaustion; do not remove without updating fallback path — fan_in: 3 (Start, transport error, ctx cancel)
// messageLoop reads incoming messages and dispatches them.
// Applies backoff on consecutive receive errors to avoid tight CPU loops (SEC-006).
// When backoff reaches maxBackoff, OnConnectionExhausted is fired once to activate REST polling fallback.
func (s *Server) messageLoop(ctx context.Context) {
	const maxBackoff = 30 * time.Second
	backoff := time.Duration(0)
	exhaustedFired := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		data, err := s.transport.Receive()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[a2a] receive error: %v", err)

			if reconnectErr := s.ReconnectTransport(ctx); reconnectErr == nil {
				backoff = 0
				if exhaustedFired && s.restPoller != nil {
					s.restPoller.Stop()
				}
				exhaustedFired = false
				log.Printf("[a2a] transport recovered after receive error")
				continue
			} else {
				log.Printf("[a2a] reconnect attempt after receive error failed: %v", reconnectErr)
			}

			// Exponential backoff on consecutive errors.
			if backoff == 0 {
				backoff = 500 * time.Millisecond
			} else if backoff < maxBackoff {
				backoff *= 2
			}

			// Fire exhausted callback once when backoff ceiling is reached.
			if backoff >= maxBackoff && !exhaustedFired {
				exhaustedFired = true
				if s.config.OnConnectionExhausted != nil {
					s.config.OnConnectionExhausted()
				}
				if s.restPoller != nil {
					s.restPoller.Start(ctx)
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		// Successful receive: reset backoff and stop REST poller if it was active.
		if backoff > 0 {
			backoff = 0
			if exhaustedFired && s.restPoller != nil {
				s.restPoller.Stop()
			}
			exhaustedFired = false
		}
		s.handleMessage(ctx, data)
	}
}

// @AX:WARN [AUTO] high cyclomatic complexity — switch + nested nil checks; adding new methods here risks missing the heartbeat ack path
// handleMessage parses a JSON-RPC message and routes it by method.
// Heartbeat ack responses (no method, result with status "ok") update the heartbeat lastAck.
func (s *Server) handleMessage(ctx context.Context, msg []byte) {
	var req JSONRPCRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		log.Printf("[a2a] invalid message: %v", err)
		return
	}

	// Handle JSON-RPC responses (no method field) — may be heartbeat ack or registration ack.
	if req.Method == "" {
		s.handleResponse(msg)
		return
	}

	switch req.Method {
	case MethodSendMessage:
		s.handleSendMessage(ctx, req)
	case MethodCancelTask:
		s.handleCancelTask(req)
	case MethodApproval:
		s.handleApproval(req)
	default:
		log.Printf("[a2a] unknown method: %s", req.Method)
	}
}

// handleResponse processes JSON-RPC response messages (no method field).
// Detects heartbeat ack responses and calls Ack() on the heartbeat instance.
func (s *Server) handleResponse(msg []byte) {
	var resp struct {
		Result map[string]string `json:"result"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return
	}
	if resp.Result["status"] == "ok" && s.heartbeat != nil {
		s.heartbeat.Ack()
	}
}

// @AX:WARN [AUTO] concurrent map mutation — tasks map guarded by mu; ensure lock is held for all reads/writes to s.tasks
// handleSendMessage extracts the task payload, caches the security policy, and dispatches.
func (s *Server) handleSendMessage(ctx context.Context, req JSONRPCRequest) {
	var params SendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("[a2a] invalid SendMessage params: %v", err)
		s.sendError(req.ID, -32602, "invalid params")
		return
	}

	if err := s.enqueueAndDispatchTask(ctx, req.ID, params); err != nil {
		log.Printf("[a2a] send message rejected: %v", err)
		s.sendError(req.ID, -32602, err.Error())
		return
	}
}

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
		return params, nil
	}

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
	}, nil
}

func (s *Server) enqueueAndDispatchTask(ctx context.Context, reqID json.RawMessage, params SendMessageParams) error {
	if params.TaskID == "" {
		return fmt.Errorf("missing task ID")
	}

	// SEC-007: Reject duplicate task IDs to prevent state overwrites.
	s.mu.Lock()
	if _, exists := s.tasks[params.TaskID]; exists {
		s.mu.Unlock()
		return fmt.Errorf("duplicate task ID: %s", params.TaskID)
	}
	s.tasks[params.TaskID] = &Task{ID: params.TaskID, Status: StatusWorking}
	s.mu.Unlock()

	if err := validateSecurityPolicySignature(params.TaskID, params.SecurityPolicy, params.PolicySignature); err != nil {
		s.mu.Lock()
		delete(s.tasks, params.TaskID)
		s.mu.Unlock()
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
		s.mu.Lock()
		delete(s.tasks, params.TaskID)
		s.mu.Unlock()
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

	// Notify backend of working status.
	_ = s.UpdateTaskStatus(params.TaskID, StatusWorking, nil)

	// Dispatch asynchronously.
	go s.dispatchTask(ctx, reqID, params)
	return nil
}

// @AX:ANCHOR [AUTO] task execution contract — applies SecurityPolicy timeout and per-task context; callers rely on UpdateTaskStatus being sent for all terminal states — fan_in: 3 (handleSendMessage goroutine, cancel path, timeout path)
// dispatchTask runs the task handler and reports the result.
// Uses a per-task cancellable context so individual tasks can be canceled (REQ-A2A-H02).
// Applies SecurityPolicy.TimeoutSec as a hard deadline when configured.
func (s *Server) dispatchTask(ctx context.Context, reqID json.RawMessage, params SendMessageParams) {
	var taskCtx context.Context
	var cancel context.CancelFunc
	if params.SecurityPolicy.TimeoutSec > 0 {
		timeout := time.Duration(params.SecurityPolicy.TimeoutSec) * time.Second
		taskCtx, cancel = context.WithTimeout(ctx, timeout)
		log.Printf("[a2a] task %s: applying timeout %ds from SecurityPolicy", params.TaskID, params.SecurityPolicy.TimeoutSec)
	} else {
		taskCtx, cancel = context.WithCancel(ctx)
	}
	s.mu.Lock()
	s.taskContexts[params.TaskID] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.taskContexts, params.TaskID)
		s.mu.Unlock()
	}()

	payload, err := mergeTaskPayload(params.Payload, params.Model, params.PipelinePhases, params.PipelineInstructions, params.PipelinePromptTemplates, params.IterationBudget)
	if err != nil {
		failResult := &TaskResult{Status: StatusFailed, Error: err.Error()}
		_ = s.UpdateTaskStatus(params.TaskID, StatusFailed, failResult)
		if len(reqID) > 0 {
			s.sendResult(reqID, failResult)
		}
		return
	}

	result, err := s.handler(taskCtx, params.TaskID, payload)
	if err != nil {
		failResult := &TaskResult{Status: StatusFailed, Error: err.Error()}
		_ = s.UpdateTaskStatus(params.TaskID, StatusFailed, failResult)
		if len(reqID) > 0 {
			s.sendResult(reqID, failResult)
		}
		return
	}
	result.Status = StatusCompleted
	_ = s.UpdateTaskStatus(params.TaskID, StatusCompleted, result)
	if len(reqID) > 0 {
		s.sendResult(reqID, result)
	}
}

func mergeTaskPayload(payload json.RawMessage, model string, pipelinePhases []string, pipelineInstructions map[string]string, pipelinePromptTemplates map[string]string, iterationBudget *IterationBudget) (json.RawMessage, error) {
	if model == "" && len(pipelinePhases) == 0 && len(pipelineInstructions) == 0 && len(pipelinePromptTemplates) == 0 && !hasIterationBudget(iterationBudget) {
		return payload, nil
	}

	if len(payload) == 0 {
		obj := map[string]any{}
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
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("marshal transport metadata payload: %w", err)
		}
		return data, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, fmt.Errorf("merge transport metadata into payload: %w", err)
	}
	if obj == nil {
		obj = make(map[string]any)
	}
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

	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal merged payload: %w", err)
	}
	return data, nil
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

// handleCancelTask marks a task as canceled.
func (s *Server) handleCancelTask(req JSONRPCRequest) {
	var p struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.sendError(req.ID, -32602, "invalid params")
		return
	}
	// Cancel the per-task context if it exists (REQ-A2A-H02).
	s.mu.Lock()
	if cancelFn, ok := s.taskContexts[p.TaskID]; ok {
		cancelFn()
	}
	s.mu.Unlock()

	_ = s.UpdateTaskStatus(p.TaskID, StatusCanceled, nil)
	s.sendResult(req.ID, map[string]string{"status": "canceled"})
}
