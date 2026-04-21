package cli

import (
	"github.com/insajin/autopus-adk/pkg/cost"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

type telemetrySummaryPayload struct {
	Run telemetry.PipelineRun `json:"run"`
}

type telemetryCostAgentPayload struct {
	Phase    string  `json:"phase"`
	Agent    string  `json:"agent"`
	Model    string  `json:"model"`
	Status   string  `json:"status"`
	Tokens   int     `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
	Duration int64   `json:"duration_ns"`
}

type telemetryCostPayload struct {
	Run          telemetry.PipelineRun       `json:"run"`
	Agents       []telemetryCostAgentPayload `json:"agents"`
	TotalCostUSD float64                     `json:"total_cost_usd"`
}

type telemetryComparePayload struct {
	Runs []telemetry.PipelineRun `json:"runs"`
}

func buildTelemetrySummaryPayload(run telemetry.PipelineRun) telemetrySummaryPayload {
	return telemetrySummaryPayload{Run: run}
}

func buildTelemetryCostPayload(run telemetry.PipelineRun) telemetryCostPayload {
	estimator := cost.NewEstimator(run.QualityMode)
	agents := make([]telemetryCostAgentPayload, 0)
	for _, phase := range run.Phases {
		for _, agent := range phase.Agents {
			agents = append(agents, telemetryCostAgentPayload{
				Phase:    phase.Name,
				Agent:    agent.AgentName,
				Model:    cost.ModelForAgent(run.QualityMode, agent.AgentName),
				Status:   agent.Status,
				Tokens:   agent.EstimatedTokens,
				CostUSD:  estimator.EstimateCost(agent),
				Duration: int64(agent.Duration),
			})
		}
	}

	return telemetryCostPayload{
		Run:          run,
		Agents:       agents,
		TotalCostUSD: estimator.EstimatePipelineCost(run),
	}
}

func buildTelemetryComparePayload(runs []telemetry.PipelineRun) telemetryComparePayload {
	return telemetryComparePayload{Runs: runs}
}

func buildTelemetryRunWarnings(run telemetry.PipelineRun) []jsonMessage {
	if run.FinalStatus == telemetry.StatusPass {
		return nil
	}
	return []jsonMessage{{
		Code:    "pipeline_not_pass",
		Message: "The selected pipeline run completed without PASS status.",
	}}
}

func buildTelemetryComparisonWarnings(runs []telemetry.PipelineRun) []jsonMessage {
	warnings := make([]jsonMessage, 0)
	for _, run := range runs {
		if run.FinalStatus != telemetry.StatusPass {
			warnings = append(warnings, jsonMessage{
				Code:    "pipeline_not_pass",
				Message: "Compared runs include at least one non-PASS result.",
			})
			break
		}
	}
	return warnings
}
