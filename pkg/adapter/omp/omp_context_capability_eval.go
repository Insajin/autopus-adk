package omp

func ompContextCapabilityResult(id string, supported bool, observed, missing string) OMPContextCapability {
	reason := observed
	if supported {
		reason = "observed"
	} else if reason == "" {
		reason = missing
	}
	return OMPContextCapability{ID: id, Supported: supported, Reason: reason}
}

func ompContextEventCapability(id string, supported bool, observed string) OMPContextCapability {
	return ompContextCapabilityResult(id, supported, observed, "event_missing")
}

func firstOMPContextReason(reasons ...string) string {
	for _, reason := range reasons {
		if reason != "" {
			return reason
		}
	}
	return ""
}

func effectiveOMPContextHistoryMode(requested string, activeEligible bool) string {
	switch requested {
	case "active":
		if activeEligible {
			return "active"
		}
		return "shadow"
	case "shadow":
		return "shadow"
	default:
		return "off"
	}
}

func effectiveOMPContextMemoryMode(requested string, shadowEligible bool) string {
	if requested == "shadow" && shadowEligible {
		return "shadow"
	}
	return "off"
}
