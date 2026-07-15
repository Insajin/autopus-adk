// Package telemetry defines types for recording agent and pipeline execution metrics.
// Events are serialized to JSONL files under .autopus/telemetry/.
package telemetry

import (
	"encoding/json"
	"time"
)

// Status constants for agent and pipeline runs.
const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
)

// Event type constants for JSONL serialization.
const (
	EventTypeAgentRun      = "agent_run"
	EventTypePhaseStart    = "phase_start"
	EventTypePhaseEnd      = "phase_end"
	EventTypePipelineStart = "pipeline_start"
	EventTypePipelineEnd   = "pipeline_end"
)

// AgentRun records a single agent execution within a pipeline phase.
type AgentRun struct {
	AgentName        string          `json:"agent_name"`
	SpecID           string          `json:"spec_id,omitempty"`
	TaskID           string          `json:"task_id,omitempty"`
	RunID            string          `json:"run_id,omitempty"`
	CallID           string          `json:"call_id,omitempty"`
	Attempt          int             `json:"attempt,omitempty"`
	Provider         string          `json:"provider,omitempty"`
	Model            string          `json:"model,omitempty"`
	Effort           string          `json:"effort,omitempty"`
	Phase            string          `json:"phase,omitempty"`
	Role             string          `json:"role,omitempty"`
	StartTime        time.Time       `json:"start_time"`
	EndTime          time.Time       `json:"end_time"`
	Duration         time.Duration   `json:"duration_ns"`
	Status           string          `json:"status"` // PASS or FAIL
	AcceptanceStatus string          `json:"acceptance_status,omitempty"`
	FilesModified    int             `json:"files_modified"`
	EstimatedTokens  int             `json:"estimated_tokens"`
	ToolCalls        int             `json:"tool_calls,omitempty"`
	Usage            []UsageEnvelope `json:"usage,omitempty"`
}

// PhaseRecord records a single phase within a pipeline execution.
type PhaseRecord struct {
	Name      string        `json:"name"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration_ns"`
	Status    string        `json:"status"` // PASS or FAIL
	Agents    []AgentRun    `json:"agents"`
}

// PipelineRun records the full execution of a SPEC pipeline.
type PipelineRun struct {
	SpecID        string        `json:"spec_id"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	TotalDuration time.Duration `json:"total_duration_ns"`
	Phases        []PhaseRecord `json:"phases"`
	RetryCount    int           `json:"retry_count"`
	FinalStatus   string        `json:"final_status"` // PASS or FAIL
	QualityMode   string        `json:"quality_mode"`
}

// Event is the top-level JSONL record written to the telemetry file.
// Data holds an AgentRun, PhaseRecord, or PipelineRun depending on Type.
type Event struct {
	Type      string          `json:"type"` // see EventType* constants
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// CostEstimator calculates an estimated monetary cost for an agent run.
// Implementations live in the cost package and are injected at runtime.
type CostEstimator interface {
	EstimateCost(run AgentRun) float64
}
