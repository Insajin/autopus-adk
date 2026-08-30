package omp

import "strings"

var ompRoutingRoleCapabilities = map[string]string{
	"default":  "coding_tool_use",
	"smol":     "fast_validation",
	"slow":     "deep_reasoning",
	"plan":     "deep_reasoning",
	"vision":   "vision_design",
	"designer": "vision_design",
	"commit":   "deterministic_transform",
	"tiny":     "deterministic_transform",
	"task":     "coding_tool_use",
	"advisor":  "independent_dissent",
}

var ompRoutingRoleOrder = []string{
	"default", "smol", "slow", "plan", "vision", "designer", "commit", "tiny", "task", "advisor",
}

type OMPRoutingCandidate struct {
	Selector string `json:"selector"`
	Thinking string `json:"thinking"`
	Family   string `json:"family,omitempty"`
}

type OMPModelRouteRequest struct {
	Agent                        string                `json:"agent,omitempty"`
	Role                         string                `json:"role"`
	Capability                   string                `json:"capability"`
	Candidates                   []OMPRoutingCandidate `json:"candidates"`
	Required                     bool                  `json:"required"`
	DegradedAction               string                `json:"degraded_action,omitempty"`
	ExecutorFamily               string                `json:"executor_family,omitempty"`
	PreferDistinctExecutorFamily bool                  `json:"prefer_distinct_executor_family,omitempty"`
}

type OMPRoutingAttempt struct {
	Index    int    `json:"index"`
	Selector string `json:"selector"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
}

type OMPFamilyDiversity struct {
	Status   string `json:"status,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Executor string `json:"executor,omitempty"`
	Reviewer string `json:"reviewer,omitempty"`
}

type OMPModelRouteResolution struct {
	RouteID                     string              `json:"route_id,omitempty"`
	Agent                       string              `json:"agent,omitempty"`
	RequestedRole               string              `json:"requested_role"`
	Capability                  string              `json:"capability"`
	Required                    bool                `json:"required"`
	Status                      string              `json:"status"`
	Reason                      string              `json:"reason"`
	EffectiveProvider           string              `json:"effective_provider,omitempty"`
	EffectiveModel              string              `json:"effective_model,omitempty"`
	EffectiveSelector           string              `json:"effective_selector,omitempty"`
	Thinking                    string              `json:"thinking,omitempty"`
	EffectiveFamily             string              `json:"effective_family,omitempty"`
	FallbackAttempts            []OMPRoutingAttempt `json:"fallback_attempts,omitempty"`
	DegradedReason              string              `json:"degraded_reason,omitempty"`
	EvidenceClass               string              `json:"evidence_class"`
	FamilyDiversity             OMPFamilyDiversity  `json:"family_diversity,omitempty"`
	IndependentProviderEvidence bool                `json:"independent_provider_evidence"`
	QuorumEvidence              bool                `json:"quorum_evidence"`
	ConsensusEvidence           bool                `json:"consensus_evidence"`
}

type evaluatedOMPRoutingCandidate struct {
	index     int
	candidate OMPRoutingCandidate
	model     OMPModelMetadata
	reason    string
}

// ResolveOMPModelRoute applies exact matching and declared-order fallback.
// @AX:WARN [AUTO]: Route resolution has 9 conditional branches.
// @AX:REASON [AUTO]: role validation, catalog state, fallback, degradation, and attested evidence converge here.
func ResolveOMPModelRoute(
	catalog OMPModelCatalog,
	catalogReason string,
	request OMPModelRouteRequest,
) OMPModelRouteResolution {
	result := newOMPModelRouteResolution(request)
	wantCapability, roleKnown := ompRoutingRoleCapabilities[request.Role]
	if !roleKnown {
		result.Reason = "role_unknown"
		return result
	}
	if request.Capability != wantCapability {
		result.Reason = "capability_mismatch"
		return result
	}
	if request.DegradedAction != "" && request.DegradedAction != "runtime_default" {
		result.Reason = "degraded_action_invalid"
		return result
	}
	if catalogReason != "catalog_ready" || len(catalog.Models) == 0 {
		if catalogReason == "" || catalogReason == "catalog_ready" {
			catalogReason = "catalog_empty"
		}
		result.Reason = normalizeOMPCatalogRoutingReason(catalogReason)
		return result
	}

	evaluated := evaluateOMPRoutingCandidates(catalog, request)
	selected := selectOMPRoutingCandidate(evaluated, request)
	result.FallbackAttempts = routingAttemptsThroughSelection(evaluated, selected, request)
	if selected == nil {
		if request.DegradedAction != "" || !request.Required {
			result.Status = "degraded"
			result.Reason = "explicit_degraded"
			if request.DegradedAction != "" {
				result.DegradedReason = "explicit_" + request.DegradedAction
			} else {
				result.DegradedReason = "optional_runtime_default"
			}
			return result
		}
		result.Reason = "no_compatible_candidate"
		return result
	}

	result.Status, result.Reason = "selected", "selected"
	result.EffectiveProvider = selected.model.Provider
	result.EffectiveModel = selected.model.Model
	result.Thinking = selected.candidate.Thinking
	result.EffectiveSelector = formatOMPRoutingSelector(selected.candidate)
	result.EffectiveFamily = selected.model.Family
	result.FamilyDiversity = resolveOMPFamilyDiversity(request, selected.model.Family)
	if selected.model.OperatorAttested {
		result.EvidenceClass = "operator_attested"
	}
	return result
}

func newOMPModelRouteResolution(request OMPModelRouteRequest) OMPModelRouteResolution {
	return OMPModelRouteResolution{
		Agent: request.Agent, RequestedRole: request.Role, Capability: request.Capability,
		Required: request.Required, Status: "blocked", Reason: "no_compatible_candidate", EvidenceClass: "availability",
	}
}

func normalizeOMPCatalogRoutingReason(reason string) string {
	switch reason {
	case "catalog_empty", "catalog_invalid", "catalog_metadata_insufficient", "catalog_oversized", "catalog_timeout":
		return reason
	default:
		return "catalog_invalid"
	}
}

func evaluateOMPRoutingCandidates(catalog OMPModelCatalog, request OMPModelRouteRequest) []evaluatedOMPRoutingCandidate {
	evaluated := make([]evaluatedOMPRoutingCandidate, 0, len(request.Candidates))
	for index, candidate := range request.Candidates {
		entry := evaluatedOMPRoutingCandidate{index: index, candidate: candidate}
		entry.model, entry.reason = matchOMPModelCandidate(catalog.Models, request.Capability, candidate)
		evaluated = append(evaluated, entry)
	}
	return evaluated
}

func matchOMPModelCandidate(
	models []OMPModelMetadata,
	capability string,
	candidate OMPRoutingCandidate,
) (OMPModelMetadata, string) {
	provider, modelID, ok := parseOMPRoutingSelector(candidate.Selector)
	if !ok {
		return OMPModelMetadata{}, "selector_invalid"
	}
	providerFound := false
	for _, model := range models {
		if model.Provider != provider {
			continue
		}
		providerFound = true
		if model.Model != modelID {
			continue
		}
		switch {
		case model.Disabled:
			return model, "disabled"
		case !model.OperatorAttested && !model.Keyless && !model.AuthEnabled:
			return model, "unauthorized"
		case !containsOMPModelValue(model.Capabilities, capability):
			return model, "capability_mismatch"
		case !containsOMPModelValue(model.Thinking, candidate.Thinking):
			return model, "thinking_unsupported"
		case candidate.Family != "" && candidate.Family != model.Family:
			return model, "family_mismatch"
		default:
			return model, "compatible"
		}
	}
	if !providerFound {
		return OMPModelMetadata{}, "provider_unknown"
	}
	return OMPModelMetadata{}, "model_unknown"
}

func parseOMPRoutingSelector(selector string) (string, string, bool) {
	parts := strings.Split(selector, "/")
	if len(parts) != 2 || !safeOMPModelToken(parts[0]) || !safeOMPModelToken(parts[1]) {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func containsOMPModelValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func formatOMPRoutingSelector(candidate OMPRoutingCandidate) string {
	return candidate.Selector + ":" + candidate.Thinking
}
