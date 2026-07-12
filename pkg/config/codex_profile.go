package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
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
		return CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra}
	}
	return CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}
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
	if q.codexQualityMode() == "ultra" {
		effort := CodexEffortXHigh
		switch agentName {
		case "planner", "architect", "security-auditor":
			effort = CodexEffortMax
		}
		return CodexProfile{Model: CodexSolModel, Effort: effort}
	}

	switch q.codexAgentTier(agentName, fallbackTier) {
	case "opus":
		return CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}
	case "haiku":
		return CodexProfile{Model: CodexLunaModel, Effort: normalizeManagedCodexEffort(declaredEffort)}
	default:
		return CodexProfile{Model: CodexTerraModel, Effort: normalizeManagedCodexEffort(declaredEffort)}
	}
}

func (q QualityConf) CodexAgentModel(agentName, fallbackTier string) string {
	return q.CodexAgentProfile(agentName, fallbackTier, CodexEffortMedium).Model
}

func (q QualityConf) CodexAgentEffort(agentName, fallbackTier, declaredEffort string) string {
	return q.CodexAgentProfile(agentName, fallbackTier, declaredEffort).Effort
}

func (q QualityConf) codexQualityMode() string {
	if q.Default == "ultra" {
		return "ultra"
	}
	return "balanced"
}

func (q QualityConf) codexAgentTier(agentName, fallbackTier string) string {
	preset, ok := q.Presets[q.Default]
	if !ok {
		preset, ok = q.Presets["balanced"]
	}
	if ok {
		if tier, ok := normalizeCodexTier(preset.Agents[agentName]); ok {
			return tier
		}
	}
	if tier, ok := normalizeCodexTier(fallbackTier); ok {
		return tier
	}
	return "sonnet"
}

func normalizeCodexTier(tier string) (string, bool) {
	tier = strings.ToLower(strings.TrimSpace(tier))
	switch tier {
	case "opus", "sonnet", "haiku":
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

// ParseCodexModelCatalog parses and bounds `codex debug models` JSON.
func ParseCodexModelCatalog(data []byte) (CodexModelCatalog, error) {
	var catalog CodexModelCatalog
	if len(data) > MaxCodexModelCatalogBytes {
		return catalog, fmt.Errorf("codex model catalog exceeds %d bytes", MaxCodexModelCatalogBytes)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return catalog, fmt.Errorf("codex model catalog is empty")
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return CodexModelCatalog{}, fmt.Errorf("parse codex model catalog: %w", err)
	}
	if err := validateCodexModelCatalog(catalog); err != nil {
		return CodexModelCatalog{}, err
	}
	return catalog, nil
}

// Supports reports whether a catalog model advertises the requested effort.
func (c CodexModelCatalog) Supports(model, effort string) bool {
	entry, ok := c.findModel(model)
	return ok && entry.supportsEffort(effort)
}

// ResolveCodexProfile resolves a requested profile against a runtime catalog.
func ResolveCodexProfile(requested CodexProfile, catalogJSON []byte) CodexProfileResolution {
	catalog, err := ParseCodexModelCatalog(catalogJSON)
	if err != nil {
		return CodexProfileResolution{
			Requested:    requested,
			Effective:    legacyCodexProfile(requested.Effort),
			Fallback:     true,
			Reason:       CodexResolutionCatalogUnknown,
			CatalogError: err,
		}
	}
	if requested.Model == CodexLegacyModel {
		return resolveRequestedLegacyCodexProfile(catalog, requested)
	}

	model, ok := catalog.findModel(requested.Model)
	if !ok {
		legacy, legacyOK := catalog.findModel(CodexLegacyModel)
		if !legacyOK {
			return CodexProfileResolution{
				Requested: requested,
				Fallback:  true,
				Reason:    CodexResolutionRuntimeDefault,
			}
		}
		legacyEffort, effortOK := legacy.highestCompatibleEffort(capLegacyCodexEffort(requested.Effort))
		if !effortOK {
			return CodexProfileResolution{
				Requested: requested,
				Fallback:  true,
				Reason:    CodexResolutionRuntimeDefault,
			}
		}
		return CodexProfileResolution{
			Requested: requested,
			Effective: CodexProfile{Model: CodexLegacyModel, Effort: legacyEffort},
			Fallback:  true,
			Reason:    CodexResolutionModelUnavailable,
		}
	}
	if model.supportsEffort(requested.Effort) {
		return CodexProfileResolution{
			Requested: requested,
			Effective: requested,
			Reason:    CodexResolutionSupported,
		}
	}

	effectiveEffort, ok := model.highestCompatibleEffort(requested.Effort)
	if !ok {
		return CodexProfileResolution{
			Requested: requested,
			Effective: CodexProfile{Model: requested.Model},
			Fallback:  true,
			Reason:    CodexResolutionRuntimeDefault,
		}
	}
	return CodexProfileResolution{
		Requested: requested,
		Effective: CodexProfile{Model: requested.Model, Effort: effectiveEffort},
		Fallback:  true,
		Reason:    CodexResolutionEffortUnavailable,
	}
}

func (c CodexModelCatalog) findModel(slug string) (CodexCatalogModel, bool) {
	for _, model := range c.Models {
		if model.Slug == slug {
			return model, true
		}
	}
	return CodexCatalogModel{}, false
}

func (m CodexCatalogModel) supportsEffort(effort string) bool {
	for _, level := range m.SupportedReasoningLevels {
		if level.Effort == effort {
			return true
		}
	}
	return false
}

func (m CodexCatalogModel) highestCompatibleEffort(requested string) (string, bool) {
	requestedRank := codexEffortRank(requested)
	if requestedRank < 0 {
		requestedRank = codexEffortRank(m.DefaultReasoningLevel)
	}
	if requestedRank < 0 {
		requestedRank = len(codexEffortOrder) - 1
	}

	bestEffort := ""
	bestRank := -1
	for _, level := range m.SupportedReasoningLevels {
		rank := codexEffortRank(level.Effort)
		if rank >= 0 && rank <= requestedRank && rank > bestRank {
			bestEffort = level.Effort
			bestRank = rank
		}
	}
	return bestEffort, bestRank >= 0
}

func codexEffortRank(effort string) int {
	for rank, candidate := range codexEffortOrder {
		if candidate == effort {
			return rank
		}
	}
	return -1
}
