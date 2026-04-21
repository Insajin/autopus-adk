package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
)

// taskPayloadMessage is the JSON structure received from the A2A backend.
type taskPayloadMessage struct {
	Description             string                  `json:"description"`
	Prompt                  string                  `json:"prompt,omitempty"`
	PMNotes                 string                  `json:"pm_notes,omitempty"`
	PolicySummary           string                  `json:"policy_summary,omitempty"`
	KnowledgeCtx            string                  `json:"knowledge_ctx,omitempty"`
	PipelinePhases          []string                `json:"pipeline_phases,omitempty"`
	PipelineInstructions    map[string]string       `json:"pipeline_instructions,omitempty"`
	PipelinePromptTemplates map[string]string       `json:"pipeline_prompt_templates,omitempty"`
	IterationBudget         *budget.IterationBudget `json:"iteration_budget,omitempty"`
	SpecID                  string                  `json:"spec_id,omitempty"`
	Model                   string                  `json:"model,omitempty"`
	CorrelationID           string                  `json:"correlation_id,omitempty"`
	SessionID               string                  `json:"session_id,omitempty"`
}

// handleTask is the A2A TaskHandler callback invoked when a task is received.
func (wl *WorkerLoop) handleTask(ctx context.Context, taskID string, payload json.RawMessage) (*a2a.TaskResult, error) {
	log.Printf("[worker] received task: %s", taskID)
	defer cleanupPolicy(taskID)

	taskMeta := taskRunMeta{TraceID: taskID}
	failTask := func(err error, result adapter.TaskResult, sessionID string) (*a2a.TaskResult, error) {
		pending, _ := wl.pendingApproval(taskID)
		if result.SessionID == "" {
			result.SessionID = strings.TrimSpace(sessionID)
		}
		result.Artifacts = ensureOutputArtifact(result.Output, result.Artifacts)
		traceID := resolveTaskTraceID(taskID, pending, taskMeta)
		failureResult := &a2a.TaskResult{
			Status:        a2a.StatusFailed,
			Artifacts:     convertArtifacts(result.Artifacts),
			Error:         err.Error(),
			SessionID:     result.SessionID,
			TraceID:       traceID,
			CorrelationID: taskMeta.CorrelationID,
		}
		wl.clearPendingApproval(taskID)
		wl.emitHostEvent(HostEvent{
			Type:          HostEventTaskFailed,
			TaskID:        taskID,
			TraceID:       traceID,
			CorrelationID: taskMeta.CorrelationID,
			Message:       err.Error(),
			Result:        buildHostResult("failed", "", err.Error(), result),
		})
		return failureResult, err
	}

	var msg taskPayloadMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return failTask(fmt.Errorf("parse task payload: %w", err), adapter.TaskResult{}, "")
	}
	taskMeta = newTaskRunMeta(taskID, msg.CorrelationID)
	wl.emitHostEvent(HostEvent{
		Type:          HostEventTaskReceived,
		TaskID:        taskID,
		TraceID:       taskMeta.TraceID,
		CorrelationID: taskMeta.CorrelationID,
		Message:       "dispatch admitted into the retained worker loop",
	})

	descriptionSeed := strings.TrimSpace(msg.Description)
	if descriptionSeed == "" {
		descriptionSeed = strings.TrimSpace(msg.Prompt)
	}
	memoryAgentID := resolveMemoryAgentID(wl.config)

	knowledgeCtx := msg.KnowledgeCtx
	if knowledgeCtx == "" && wl.knowledgeSearcher != nil && descriptionSeed != "" {
		knowledgeCtx = populateKnowledge(ctx, wl.knowledgeSearcher, descriptionSeed)
	}
	memoryCtx := populateMemory(ctx, wl.memorySearcher, memoryAgentID, descriptionSeed)

	prompt := strings.TrimSpace(msg.Prompt)
	if prompt == "" {
		prompt = wl.builder.Build(TaskPayload{
			TaskID:        taskID,
			Description:   msg.Description,
			PMNotes:       msg.PMNotes,
			PolicySummary: msg.PolicySummary,
			KnowledgeCtx:  knowledgeCtx,
			MemoryCtx:     memoryCtx,
			SpecID:        msg.SpecID,
		})
	}

	var model string
	if msg.Model != "" {
		model = msg.Model
	} else if wl.config.Router != nil && !a2a.SignedControlPlaneEnforced() {
		model = wl.config.Router.Route(wl.config.Provider.Name(), descriptionSeed)
	}

	taskCfg := adapter.TaskConfig{
		TaskID:    taskID,
		SessionID: msg.SessionID,
		Prompt:    prompt,
		MCPConfig: wl.config.MCPConfig,
		WorkDir:   wl.config.WorkDir,
		Model:     model,
	}
	budgetCfg := budgetConfigFromMessage(msg)

	phasePlan, err := ParsePhasePlan(msg.PipelinePhases)
	if err != nil {
		return failTask(fmt.Errorf("parse pipeline phases: %w", err), adapter.TaskResult{}, msg.SessionID)
	}
	phaseInstructions, err := ParsePhaseInstructions(msg.PipelineInstructions)
	if err != nil {
		return failTask(fmt.Errorf("parse pipeline instructions: %w", err), adapter.TaskResult{}, msg.SessionID)
	}
	phasePromptTemplates, err := ParsePhasePromptTemplates(msg.PipelinePromptTemplates)
	if err != nil {
		return failTask(fmt.Errorf("parse pipeline prompt templates: %w", err), adapter.TaskResult{}, msg.SessionID)
	}

	var result adapter.TaskResult
	if len(phasePlan) > 0 || len(phaseInstructions) > 0 || len(phasePromptTemplates) > 0 {
		result, err = wl.executePipelineWithParallel(
			ctx,
			taskID,
			prompt,
			model,
			phasePlan,
			phaseInstructions,
			phasePromptTemplates,
			budgetCfg,
			taskMeta,
		)
	} else {
		result, err = wl.executeWithParallel(ctx, taskCfg, budgetCfg, taskMeta)
	}
	if err != nil {
		log.Printf("[worker] task %s failed: %v", taskID, err)
		return failTask(err, result, taskCfg.SessionID)
	}

	log.Printf("[worker] task %s completed: cost=$%.4f duration=%dms", taskID, result.CostUSD, result.DurationMS)
	result.Artifacts = ensureOutputArtifact(result.Output, result.Artifacts)
	pending, _ := wl.pendingApproval(taskID)
	wl.clearPendingApproval(taskID)
	wl.emitHostEvent(HostEvent{
		Type:          HostEventTaskCompleted,
		TaskID:        taskID,
		TraceID:       resolveTaskTraceID(taskID, pending, taskMeta),
		CorrelationID: taskMeta.CorrelationID,
		CostUSD:       result.CostUSD,
		DurationMS:    result.DurationMS,
		Result:        buildHostResult("completed", "", "", result),
	})

	if wl.memorySearcher != nil && memoryAgentID != "" && result.Output != "" {
		go func() {
			writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := wl.memorySearcher.CreateMemory(writeCtx, knowledge.CreateMemoryRequest{
				AgentID: memoryAgentID,
				Title:   fmt.Sprintf("Task learning: %s", taskID),
				Content: truncateForMemory(descriptionSeed, result.Output),
				Source:  "agent_learning",
			})
			if err != nil {
				log.Printf("[worker] memory write-back failed: %v", err)
			}
		}()
	}

	return &a2a.TaskResult{
		Status:        a2a.StatusCompleted,
		Artifacts:     convertArtifacts(result.Artifacts),
		SessionID:     result.SessionID,
		TraceID:       resolveTaskTraceID(taskID, pending, taskMeta),
		CorrelationID: taskMeta.CorrelationID,
	}, nil
}
