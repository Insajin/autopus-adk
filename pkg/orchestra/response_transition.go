package orchestra

func applyResponseTransitionEvidence(result *OrchestraResult) {
	if result == nil {
		return
	}
	allResponses := make([]ProviderResponse, 0, len(result.Responses))
	for _, round := range result.RoundHistory {
		allResponses = append(allResponses, round...)
	}
	allResponses = append(allResponses, result.Responses...)
	allSkipped := len(allResponses) > 0
	for _, response := range allResponses {
		if len(response.DegradedReasons) > 0 {
			result.Degraded = true
			for _, reason := range response.DegradedReasons {
				appendDegradedReason(result, reason)
			}
		}
		switch response.TerminalState {
		case TerminalBlocked:
			result.TerminalState = TerminalBlocked
		case TerminalSkipped:
			// Preserve allSkipped until the complete response set is inspected.
		default:
			allSkipped = false
		}
	}
	if result.TerminalState == "" && allSkipped {
		result.TerminalState = TerminalSkipped
		result.AnalysisVerdict = "skipped"
	}
}
