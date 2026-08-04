package omp

func selectOMPRoutingCandidate(
	evaluated []evaluatedOMPRoutingCandidate,
	request OMPModelRouteRequest,
) *evaluatedOMPRoutingCandidate {
	if request.Role == "advisor" && request.ExecutorFamily != "" {
		for index := range evaluated {
			candidate := &evaluated[index]
			if candidate.reason == "compatible" && candidate.model.Family != request.ExecutorFamily {
				return candidate
			}
		}
		for index := range evaluated {
			if evaluated[index].reason == "compatible" {
				return &evaluated[index]
			}
		}
		return nil
	}
	for index := range evaluated {
		if evaluated[index].reason == "compatible" {
			return &evaluated[index]
		}
	}
	return nil
}

func routingAttemptsThroughSelection(
	evaluated []evaluatedOMPRoutingCandidate,
	selected *evaluatedOMPRoutingCandidate,
	request OMPModelRouteRequest,
) []OMPRoutingAttempt {
	limit := len(evaluated)
	if selected != nil {
		limit = selected.index + 1
	}
	attempts := make([]OMPRoutingAttempt, 0, limit)
	for index := 0; index < limit; index++ {
		candidate := evaluated[index]
		attempt := OMPRoutingAttempt{
			Index: candidate.index, Selector: formatOMPRoutingSelector(candidate.candidate), Status: "skipped",
		}
		switch {
		case selected != nil && candidate.index == selected.index:
			attempt.Status, attempt.Reason = "selected", "selected"
		case candidate.reason == "compatible" && request.Role == "advisor" && request.ExecutorFamily != "":
			attempt.Reason = "same_family_deprioritized"
		default:
			attempt.Reason = candidate.reason
		}
		attempts = append(attempts, attempt)
	}
	return attempts
}

func resolveOMPFamilyDiversity(request OMPModelRouteRequest, selectedFamily string) OMPFamilyDiversity {
	if request.Role != "advisor" || request.ExecutorFamily == "" {
		return OMPFamilyDiversity{}
	}
	diversity := OMPFamilyDiversity{
		Status: "satisfied", Executor: request.ExecutorFamily, Reviewer: selectedFamily,
	}
	if selectedFamily == "" {
		diversity.Status, diversity.Reason = "degraded", "family_unknown"
	} else if selectedFamily == request.ExecutorFamily {
		diversity.Status, diversity.Reason = "degraded", "same_family_only"
	}
	return diversity
}
