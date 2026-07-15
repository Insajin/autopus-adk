package cli

import (
	"github.com/insajin/autopus-adk/pkg/cost"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

type telemetrySummaryPayload struct {
	Run   telemetry.PipelineRun       `json:"run"`
	Usage telemetry.EfficiencySummary `json:"usage"`
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
	Usage        telemetry.EfficiencySummary `json:"usage"`
}

type telemetryComparePayload struct {
	Runs            []telemetry.PipelineRun       `json:"runs"`
	Usage           []telemetry.EfficiencySummary `json:"usage"`
	UsageComparison telemetry.SpendComparison     `json:"usage_comparison"`
}

func buildTelemetrySummaryPayload(run telemetry.PipelineRun) telemetrySummaryPayload {
	return telemetrySummaryPayload{Run: run, Usage: telemetry.SummarizeEfficiency(flattenAgentRuns(run))}
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

	usage := telemetry.SummarizeEfficiency(flattenAgentRuns(run))
	if usage.BillableEstimatedCostUSD == nil && usage.EstimatedTokens > 0 {
		estimate := estimator.EstimatePipelineCost(run)
		usage.EstimatedCostUSD = &estimate
		usage.BillableEstimatedCostUSD = &estimate
	}
	return telemetryCostPayload{
		Run:          run,
		Agents:       agents,
		TotalCostUSD: estimator.EstimatePipelineCost(run),
		Usage:        usage,
	}
}

func buildTelemetryComparePayload(runs []telemetry.PipelineRun) telemetryComparePayload {
	payload := telemetryComparePayload{Runs: runs, Usage: make([]telemetry.EfficiencySummary, len(runs))}
	usage := make([][]telemetry.UsageEnvelope, len(runs))
	for i, run := range runs {
		agents := flattenAgentRuns(run)
		payload.Usage[i] = telemetry.SummarizeEfficiency(agents)
		for _, agent := range agents {
			usage[i] = append(usage[i], agent.Usage...)
		}
	}
	if len(usage) >= 2 {
		payload.UsageComparison = telemetry.CompareUsageSpend(usage[0], usage[1])
	}
	return payload
}

func flattenAgentRuns(run telemetry.PipelineRun) []telemetry.AgentRun {
	agents := make([]telemetry.AgentRun, 0)
	for _, phase := range run.Phases {
		agents = append(agents, phase.Agents...)
	}
	return agents
}

func buildTelemetryRunWarnings(run telemetry.PipelineRun) []jsonMessage {
	warnings := usageRunWarnings(run)
	if run.FinalStatus != telemetry.StatusPass {
		warnings = append(warnings, jsonMessage{
			Code:    "pipeline_not_pass",
			Message: "The selected pipeline run completed without PASS status.",
		})
	}
	return warnings
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
		warnings = append(warnings, usageRunWarnings(run)...)
	}
	return warnings
}

func usageRunWarnings(run telemetry.PipelineRun) []jsonMessage {
	agents := flattenAgentRuns(run)
	observed := false
	for _, agent := range agents {
		if len(agent.Usage) > 0 {
			observed = true
			break
		}
	}
	if !observed {
		return nil
	}
	summary := telemetry.SummarizeEfficiency(agents)
	if summary.PromotionBlocked {
		return []jsonMessage{{Code: "usage_promotion_blocked", Message: summary.UnavailableReason}}
	}
	if summary.ActualCoverage < 1 {
		return []jsonMessage{{Code: "usage_actual_incomplete", Message: summary.UnavailableReason}}
	}
	if summary.RawTotalTokensPerAcceptedTask == nil {
		return []jsonMessage{{Code: "accepted_task_efficiency_unavailable", Message: summary.UnavailableReason}}
	}
	return nil
}
