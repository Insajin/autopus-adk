package delivery

import "strings"

type PlanOptions struct {
	Repository    string
	ProviderMode  ProviderMode
	OwnerAgentID  string
	CorrelationID string
	RetryBudget   int
}

func BuildDryRunPlan(opts PlanOptions) (DryRunPlan, error) {
	if opts.Repository == "" {
		opts.Repository = "."
	}
	if opts.ProviderMode == "" {
		opts.ProviderMode = ProviderCodexSubscriptionInteractive
	}
	if err := ValidateProviderMode(opts.ProviderMode); err != nil {
		return DryRunPlan{}, err
	}
	if opts.OwnerAgentID == "" {
		opts.OwnerAgentID = DefaultOwnerAgentID
	}
	if opts.CorrelationID == "" {
		opts.CorrelationID = DefaultCorrelationID
	}
	if opts.RetryBudget == 0 {
		opts.RetryBudget = DefaultRetryBudget
	}
	phases := make([]DeliveryPhasePlan, 0, len(CanonicalPhases()))
	for _, phase := range CanonicalPhases() {
		phases = append(phases, DeliveryPhasePlan{
			Phase:              phase,
			WorkflowMode:       WorkflowMode,
			OwnerAgentID:       opts.OwnerAgentID,
			Repository:         opts.Repository,
			ProviderMode:       opts.ProviderMode,
			ProviderRoute:      routeForProviderMode(opts.ProviderMode),
			Attempt:            1,
			RetryBudget:        opts.RetryBudget,
			ElapsedBudget:      DefaultElapsedBudget,
			BrokerState:        DefaultBrokerState,
			GateDecision:       buildGateDecision(phase),
			CorrelationID:      opts.CorrelationID,
			NextRequiredAction: nextRequiredAction(phase),
		})
	}
	return DryRunPlan{
		SchemaVersion: DryRunPlanSchemaV1,
		WorkflowMode:  WorkflowMode,
		ProviderMode:  opts.ProviderMode,
		Repository:    opts.Repository,
		Phases:        phases,
	}, nil
}

func buildGateDecision(phase Phase) GateDecision {
	return GateDecision{
		SchemaVersion: GateDecisionSchemaV1,
		Phase:         phase,
		Status:        "pending",
		RequiredRefs:  requiredRefsForPhase(phase),
		BlocksOnDrift: phase == PhaseVerify || phase == PhaseSync,
	}
}

func requiredRefsForPhase(phase Phase) []string {
	switch phase {
	case PhasePlan:
		return []string{"phase_result"}
	case PhaseImplement:
		return []string{"phase_result", "changed_files"}
	case PhaseVerify:
		return []string{"verification_evidence", "drift_check"}
	case PhaseQA:
		return []string{"qamesh_evidence"}
	case PhaseSync:
		return []string{"source_owned_sync_summary"}
	case PhaseDeploymentHandoff:
		return []string{"deployment_approval", "proof"}
	case PhaseCanary:
		return []string{"canary_evidence"}
	default:
		return []string{"phase_result"}
	}
}

func nextRequiredAction(phase Phase) string {
	if phase == PhaseCanary {
		return "complete_delivery_after_canary_gate"
	}
	return "evaluate_gate_before_next_phase"
}

func routeForProviderMode(mode ProviderMode) string {
	if strings.HasSuffix(string(mode), "_subscription_interactive") {
		return DefaultProviderRoute
	}
	return "headless_api"
}
