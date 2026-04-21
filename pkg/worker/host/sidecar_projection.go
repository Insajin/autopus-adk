package host

import (
	"fmt"

	worker "github.com/insajin/autopus-adk/pkg/worker"
)

func eventFromHostEvent(cfg RuntimeConfig, hostEvent worker.HostEvent) Event {
	event := Event{
		RuntimeID:     cfg.WorkerName,
		WorkspaceID:   cfg.WorkspaceID,
		Provider:      cfg.ProviderName,
		TaskID:        hostEvent.TaskID,
		TraceID:       hostEvent.TraceID,
		CorrelationID: hostEvent.CorrelationID,
		Phase:         hostEvent.Phase,
		Message:       hostEvent.Message,
		Execution:     projectExecutionContext(hostEvent.Execution),
		Result:        projectResult(hostEvent.Result),
	}

	switch hostEvent.Type {
	case worker.HostEventRuntimeDegraded:
		event.Event = "runtime.degraded"
	case worker.HostEventTaskReceived:
		event.Event = "task.accepted"
	case worker.HostEventTaskProgress:
		event.Event = "task.progress"
	case worker.HostEventApprovalRequested:
		event.Event = "task.approval_requested"
		event.Approval = &ApprovalState{
			ApprovalID: hostEvent.ApprovalID,
			TraceID:    hostEvent.TraceID,
			Action:     hostEvent.Action,
			RiskLevel:  hostEvent.RiskLevel,
			Context:    hostEvent.Context,
		}
	case worker.HostEventApprovalResolved:
		event.Event = "task.approval_resolved"
		event.Approval = &ApprovalState{
			ApprovalID: hostEvent.ApprovalID,
			TraceID:    hostEvent.TraceID,
			Resolution: hostEvent.Message,
		}
	case worker.HostEventTaskCompleted:
		event.Event = "task.completed"
		event.Metrics = &EventMetrics{
			CostUSD:    hostEvent.CostUSD,
			DurationMS: hostEvent.DurationMS,
		}
	case worker.HostEventTaskFailed:
		event.Event = "task.failed"
		event.Error = &ErrorPayload{
			Code:    "task_failed",
			Message: coalesceMessage(hostEvent.Message, "task execution failed"),
		}
	default:
		return unknownHostEvent(cfg, hostEvent)
	}

	return event
}

func unknownHostEvent(cfg RuntimeConfig, hostEvent worker.HostEvent) Event {
	message := fmt.Sprintf("unknown host event type `%s` requires desktop/adk update", hostEvent.Type)

	return Event{
		Event:         "runtime.degraded",
		RuntimeID:     cfg.WorkerName,
		WorkspaceID:   cfg.WorkspaceID,
		Provider:      cfg.ProviderName,
		TaskID:        hostEvent.TaskID,
		TraceID:       hostEvent.TraceID,
		CorrelationID: hostEvent.CorrelationID,
		Phase:         hostEvent.Phase,
		Message:       coalesceMessage(hostEvent.Message, message),
		Execution:     projectExecutionContext(hostEvent.Execution),
		Result:        projectResult(hostEvent.Result),
		Error: &ErrorPayload{
			Code:    "unknown_host_event",
			Message: message,
		},
	}
}

func errorPayload(err error, fallbackCode, fallbackMessage string) *ErrorPayload {
	if hostErr := AsError(err); hostErr != nil {
		return &ErrorPayload{
			Code:    string(hostErr.Code),
			Message: hostErr.Message,
		}
	}
	return &ErrorPayload{Code: fallbackCode, Message: fallbackMessage}
}

func coalesceMessage(message, fallback string) string {
	if message != "" {
		return message
	}
	return fallback
}

func projectExecutionContext(context *worker.HostExecutionContext) *ExecutionContext {
	if context == nil {
		return nil
	}
	return &ExecutionContext{
		WorkspaceID:   context.WorkspaceID,
		RootWorkDir:   context.RootWorkDir,
		ActiveWorkDir: context.ActiveWorkDir,
		WorktreePath:  context.WorktreePath,
		Mode:          context.Mode,
		BoundaryHint:  context.BoundaryHint,
	}
}

func projectResult(result *worker.HostResult) *TaskResultSummary {
	if result == nil {
		return nil
	}
	return &TaskResultSummary{
		Status:       result.Status,
		Summary:      result.Summary,
		ErrorMessage: result.ErrorMessage,
		CostLabel:    result.CostLabel,
		DurationMS:   result.DurationMS,
		SessionID:    result.SessionID,
		Artifacts:    projectArtifacts(result.Artifacts),
	}
}

func projectArtifacts(src []worker.HostArtifact) []ArtifactRef {
	if len(src) == 0 {
		return nil
	}

	artifacts := make([]ArtifactRef, 0, len(src))
	for _, artifact := range src {
		artifacts = append(artifacts, ArtifactRef{
			Name:     artifact.Name,
			MimeType: artifact.MimeType,
			Preview:  artifact.Preview,
			Source:   artifact.Source,
		})
	}
	return artifacts
}
