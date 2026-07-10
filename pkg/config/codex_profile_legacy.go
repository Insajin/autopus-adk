package config

func resolveRequestedLegacyCodexProfile(catalog CodexModelCatalog, requested CodexProfile) CodexProfileResolution {
	model, ok := catalog.findModel(CodexLegacyModel)
	if !ok {
		return CodexProfileResolution{Requested: requested, Fallback: true, Reason: CodexResolutionRuntimeDefault}
	}
	effort, ok := model.highestCompatibleEffort(capLegacyCodexEffort(requested.Effort))
	if !ok {
		return CodexProfileResolution{Requested: requested, Fallback: true, Reason: CodexResolutionRuntimeDefault}
	}
	effective := CodexProfile{Model: CodexLegacyModel, Effort: effort}
	if effective == requested {
		return CodexProfileResolution{Requested: requested, Effective: effective, Reason: CodexResolutionSupported}
	}
	return CodexProfileResolution{
		Requested: requested,
		Effective: effective,
		Fallback:  true,
		Reason:    CodexResolutionEffortUnavailable,
	}
}

func legacyCodexProfile(requestedEffort string) CodexProfile {
	return CodexProfile{Model: CodexLegacyModel, Effort: capLegacyCodexEffort(requestedEffort)}
}

func capLegacyCodexEffort(effort string) string {
	effort = normalizeCodexEffort(effort)
	if codexEffortRank(effort) > codexEffortRank(CodexEffortXHigh) {
		return CodexEffortXHigh
	}
	return effort
}
