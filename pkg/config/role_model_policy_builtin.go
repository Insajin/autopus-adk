package config

// Built-in role-model profiles project quality presets onto OMP routes. Every
// canonical agent receives its own preset tier as an agents.<name>.candidates
// override, so no agent is promoted by a sibling sharing its capability. The
// capability routes stay populated as defaults for hand-written profiles that
// copy the derived capabilities without the agent overrides. Selection stays
// opt-in, and an explicitly defined profile of the same name always wins.

const (
	builtinRoleModelFamilyAnthropic = "anthropic"
	builtinRoleModelFamilyOpenAI    = "openai"
)

type builtinModelFamily struct {
	provider     string
	family       string
	modelForTier func(string) string
}

var builtinModelFamilies = map[string]builtinModelFamily{
	builtinRoleModelFamilyAnthropic: {
		provider: "anthropic", family: "anthropic", modelForTier: ClaudeModelForTier,
	},
	builtinRoleModelFamilyOpenAI: {
		provider: "openai-codex", family: "openai", modelForTier: CodexModelForTier,
	},
}

var builtinRoleModelFallbackTier = map[string]string{
	"balanced": "sonnet",
	"ultra":    "opus",
}

var builtinThinkingByTier = map[string]string{
	"fable":  "max",
	"opus":   "xhigh",
	"sonnet": "medium",
	"haiku":  "low",
}

var builtinLowerTier = map[string]string{
	"fable": "opus", "opus": "sonnet", "sonnet": "haiku",
}

// builtinTierRank orders relative tiers for the capability default rule.
var builtinTierRank = map[string]int{"haiku": 0, "sonnet": 1, "opus": 2, "fable": 3}

// IsBuiltinRoleModelProfileName reports whether a profile name is served by a
// quality-derived profile when the config defines no profile under that name.
func IsBuiltinRoleModelProfileName(name string) bool {
	_, ok := builtinRoleModelFallbackTier[name]
	return ok
}

// BuiltinRoleModelProfile derives one closed, operator-attested profile from
// the quality presets and the selected anchor family.
func BuiltinRoleModelProfile(
	name string,
	quality QualityConf,
	family string,
	configMode string,
) (RoleModelProfileConf, bool) {
	fallbackTier, ok := builtinRoleModelFallbackTier[name]
	if !ok {
		return RoleModelProfileConf{}, false
	}
	anchorName, ok := effectiveBuiltinRoleModelFamily(family)
	if !ok {
		return RoleModelProfileConf{}, false
	}
	mode, ok := effectiveBuiltinRoleModelConfigMode(configMode)
	if !ok {
		return RoleModelProfileConf{}, false
	}
	anchor := builtinModelFamilies[anchorName]
	counterpart := builtinModelFamilies[counterpartBuiltinRoleModelFamily(anchorName)]
	agentTiers := builtinAgentTiers(quality, name, fallbackTier)
	capabilities := make(map[string]RoleCapabilityRouteConf, len(providerNeutralCapabilities))
	for capability, tier := range builtinCapabilityTiers(agentTiers) {
		capabilities[capability] = RoleCapabilityRouteConf{
			Candidates: builtinCandidatesForTier(builtinLadder(anchor, counterpart, capability), tier),
			Required:   true,
		}
	}
	return RoleModelProfileConf{
		ConfigMode:   mode,
		CatalogTrust: RoleModelCatalogTrustOperatorAttested,
		Capabilities: capabilities,
		Agents:       builtinAgentCandidates(agentTiers, anchor, counterpart),
		ManagedKeys:  builtinRoleModelManagedKeys(mode),
		// Dissent agents run on the counterpart family and additionally ask the
		// router for a family distinct from the executor's.
		FamilyDiversity: FamilyDiversityPolicyConf{
			Enabled: true,
			Roles:   []string{OMPAgentRoleName("reviewer"), OMPAgentRoleName("security-auditor")},
		},
	}, true
}

// builtinLadder picks the model family for one capability: independent
// dissent runs on the counterpart family so review stays independent from the
// execution-family anchor.
func builtinLadder(anchor, counterpart builtinModelFamily, capability string) builtinModelFamily {
	if capability == CapabilityIndependentDissent {
		return counterpart
	}
	return anchor
}

func effectiveBuiltinRoleModelFamily(family string) (string, bool) {
	if family == "" {
		return builtinRoleModelFamilyAnthropic, true
	}
	_, ok := builtinModelFamilies[family]
	return family, ok
}

func effectiveBuiltinRoleModelConfigMode(mode string) (string, bool) {
	if mode == "" {
		return RoleModelConfigModeOverlay, true
	}
	if mode != RoleModelConfigModeOverlay && mode != RoleModelConfigModeProjectManaged {
		return mode, false
	}
	return mode, true
}

func counterpartBuiltinRoleModelFamily(family string) string {
	if family == builtinRoleModelFamilyOpenAI {
		return builtinRoleModelFamilyAnthropic
	}
	return builtinRoleModelFamilyOpenAI
}

func builtinCandidatesForTier(family builtinModelFamily, tier string) []RoleModelCandidateConf {
	lower, hasLower := builtinLowerTier[tier]
	capacity := 1
	if hasLower {
		capacity++
	}
	candidates := make([]RoleModelCandidateConf, 0, capacity)
	candidates = append(candidates, builtinCandidateForTier(family, tier))
	if hasLower {
		candidates = append(candidates, builtinCandidateForTier(family, lower))
	}
	return candidates
}

func builtinCandidateForTier(family builtinModelFamily, tier string) RoleModelCandidateConf {
	return RoleModelCandidateConf{
		Selector: family.provider + "/" + family.modelForTier(tier),
		Thinking: builtinThinkingByTier[tier],
		Family:   family.family,
	}
}

func builtinRoleModelManagedKeys(mode string) map[string]RoleManagedKeyClaimConf {
	if mode != RoleModelConfigModeProjectManaged {
		return nil
	}
	missing := OMPMissingManagedValueFingerprint()
	return map[string]RoleManagedKeyClaimConf{
		"modelRoles": {
			PriorFingerprint: missing, Complete: true,
		},
		"retry.fallbackChains": {
			PriorFingerprint: missing, Complete: true, FullArrayOwnership: true,
		},
		"retry.modelFallback": {
			PriorFingerprint: missing, Complete: true,
		},
	}
}

// builtinAgentTiers resolves every matrix agent's relative tier under the
// profile's own mode.
func builtinAgentTiers(quality QualityConf, mode, fallbackTier string) map[string]string {
	view := quality.WithGlobalOverride(mode)
	if _, ok := view.Presets[mode]; !ok {
		// A missing mode preset resolves to that mode's own characteristic tier
		// instead of borrowing the balanced preset.
		view.Presets = nil
	}
	tiers := make(map[string]string, len(capabilityBySourceAgent))
	for agent := range capabilityBySourceAgent {
		tiers[agent] = view.AgentTier(QualityProviderClaude, agent, fallbackTier)
	}
	return tiers
}

// builtinAgentCandidates projects each agent's own tier onto its ladder. This
// is the route the bridge resolves; agents never fold onto a shared tier.
func builtinAgentCandidates(
	agentTiers map[string]string,
	anchor, counterpart builtinModelFamily,
) map[string]RoleAgentOverrideConf {
	agents := make(map[string]RoleAgentOverrideConf, len(agentTiers))
	for agent, tier := range agentTiers {
		ladder := builtinLadder(anchor, counterpart, capabilityBySourceAgent[agent])
		agents[agent] = RoleAgentOverrideConf{Candidates: builtinCandidatesForTier(ladder, tier)}
	}
	return agents
}

// builtinCapabilityTiers folds agent tiers onto capability defaults: each
// capability takes the highest tier among its agents, and the matrix covers
// every capability. Built-in profiles override every agent, so these defaults
// only route agents whose override a hand-written copy of the profile dropped.
func builtinCapabilityTiers(agentTiers map[string]string) map[string]string {
	tiers := make(map[string]string, len(providerNeutralCapabilities))
	for agent, tier := range agentTiers {
		capability := capabilityBySourceAgent[agent]
		if current, seen := tiers[capability]; !seen || builtinTierRank[tier] > builtinTierRank[current] {
			tiers[capability] = tier
		}
	}
	return tiers
}
