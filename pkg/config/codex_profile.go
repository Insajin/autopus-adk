package config

import "strings"

const (
	CodexAstraModel  = "gpt-6-astra"
	CodexSolModel    = "gpt-5.6-sol"
	CodexTerraModel  = "gpt-5.6-terra"
	CodexLunaModel   = "gpt-5.6-luna"
	CodexLegacyModel = "gpt-5.5"

	CodexEffortLow    = "low"
	CodexEffortMedium = "medium"
	CodexEffortHigh   = "high"
	CodexEffortXHigh  = "xhigh"
	CodexEffortMax    = "max"
	CodexEffortUltra  = "ultra"
)

const (
	CodexResolutionSupported         CodexResolutionReason = "supported"
	CodexResolutionCatalogUnknown    CodexResolutionReason = "catalog_unknown"
	CodexResolutionModelUnavailable  CodexResolutionReason = "model_unavailable"
	CodexResolutionEffortUnavailable CodexResolutionReason = "effort_unavailable"
	CodexResolutionRuntimeDefault    CodexResolutionReason = "runtime_default"
)

var codexEffortOrder = []string{CodexEffortLow, CodexEffortMedium, CodexEffortHigh, CodexEffortXHigh, CodexEffortMax, CodexEffortUltra}

type CodexProfile struct {
	Model, Effort string
}

type CodexResolutionReason string

type CodexProfileResolution struct {
	Requested, Effective CodexProfile
	Fallback             bool
	Reason               CodexResolutionReason
	CatalogError         error
}

type CodexModelCatalog struct {
	Models []CodexCatalogModel `json:"models"`
}

// CodexCatalogModel describes one model and its supported reasoning levels.
type CodexCatalogModel struct {
	Slug                     string                       `json:"slug"`
	DefaultReasoningLevel    string                       `json:"default_reasoning_level"`
	SupportedReasoningLevels []CodexCatalogReasoningLevel `json:"supported_reasoning_levels"`
}

type CodexCatalogReasoningLevel struct {
	Effort      string `json:"effort"`
	Description string `json:"description,omitempty"`
}

// CodexSupervisorProfile returns the managed root Codex profile for this quality mode.
func (q QualityConf) CodexSupervisorProfile() CodexProfile {
	if q.codexQualityMode() == "ultra" {
		return CodexProfile{Model: CodexAstraModel, Effort: CodexEffortUltra}
	}
	return CodexProfile{Model: CodexAstraModel, Effort: CodexEffortXHigh}
}

func (q QualityConf) CodexSupervisorModel() string { return q.CodexSupervisorProfile().Model }

func (q QualityConf) CodexSupervisorEffort() string { return q.CodexSupervisorProfile().Effort }

// CodexOrchestraProfile returns the managed Codex subprocess profile.
func (q QualityConf) CodexOrchestraProfile() CodexProfile {
	profile := q.CodexSupervisorProfile()
	profile.Effort = normalizeManagedCodexEffort(profile.Effort)
	return profile
}

func (q QualityConf) CodexOrchestraModel() string { return q.CodexOrchestraProfile().Model }

func (q QualityConf) CodexOrchestraEffort() string { return q.CodexOrchestraProfile().Effort }

// CodexAgentProfile maps an agent's effective tier and declared effort to Codex.
func (q QualityConf) CodexAgentProfile(agentName, fallbackTier, declaredEffort string) CodexProfile {
	tier := q.codexAgentTier(agentName, fallbackTier)
	if q.codexQualityMode() == "ultra" && tier != "fable" && tier != "opus" {
		tier = "opus"
	}

	switch tier {
	case "fable":
		return CodexProfile{Model: CodexAstraModel, Effort: CodexEffortMax}
	case "opus":
		return CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}
	default:
		return CodexProfile{Model: CodexModelForTier(tier), Effort: normalizeManagedCodexEffort(declaredEffort)}
	}
}

func (q QualityConf) CodexAgentModel(agentName, fallbackTier string) string {
	return q.CodexAgentProfile(agentName, fallbackTier, CodexEffortMedium).Model
}

func (q QualityConf) CodexAgentEffort(agentName, fallbackTier, declaredEffort string) string {
	return q.CodexAgentProfile(agentName, fallbackTier, declaredEffort).Effort
}

func (q QualityConf) codexQualityMode() string {
	if q.ForProvider(QualityProviderCodex).Default == "ultra" {
		return "ultra"
	}
	return "balanced"
}

func (q QualityConf) codexAgentTier(agentName, fallbackTier string) string {
	return q.AgentTier(QualityProviderCodex, agentName, fallbackTier)
}

// CodexModelForTier maps a relative tier onto its managed Codex model.
func CodexModelForTier(tier string) string {
	switch tier {
	case "fable":
		return CodexAstraModel
	case "opus":
		return CodexSolModel
	case "haiku":
		return CodexLunaModel
	default:
		return CodexTerraModel
	}
}

func normalizeCodexTier(tier string) (string, bool) {
	tier = strings.ToLower(strings.TrimSpace(tier))
	switch tier {
	case "fable", "opus", "sonnet", "haiku":
		return tier, true
	default:
		return "", false
	}
}

func normalizeCodexEffort(effort string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	if codexEffortRank(effort) >= 0 {
		return effort
	}
	return CodexEffortMedium
}

func normalizeManagedCodexEffort(effort string) string {
	effort = normalizeCodexEffort(effort)
	if effort == CodexEffortUltra {
		return CodexEffortMax
	}
	return effort
}
