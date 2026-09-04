package config

// Built-in role-model profiles project quality presets onto OMP capability
// routes. Selection stays opt-in, and an explicitly defined profile of the same
// name always wins over derivation.

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

// builtinTierRank orders relative tiers for the max-wins capability rule.
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
	tiers := builtinCapabilityTiers(quality, name, fallbackTier)
	capabilities := make(map[string]RoleCapabilityRouteConf, len(providerNeutralCapabilities))
	for _, capability := range providerNeutralCapabilities {
		ladder := anchor
		if capability == CapabilityIndependentDissent {
			ladder = counterpart
		}
		capabilities[capability] = RoleCapabilityRouteConf{
			Candidates: builtinCandidatesForTier(ladder, tiers[capability]),
			Required:   true,
		}
	}
	return RoleModelProfileConf{
		ConfigMode:   mode,
		CatalogTrust: RoleModelCatalogTrustOperatorAttested,
		Capabilities: capabilities,
		ManagedKeys:  builtinRoleModelManagedKeys(mode),
		// The advisor deliberately uses the counterpart family so dissent is
		// independent from the execution-family anchor.
		FamilyDiversity: FamilyDiversityPolicyConf{Enabled: true, Roles: []string{OMPRoleAdvisor}},
	}, true
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

// builtinCapabilityTiers folds per-agent preset tiers onto capabilities through
// the agent-to-role and role-to-capability matrices. Each capability takes the
// highest tier assigned to any of its agents.
func builtinCapabilityTiers(quality QualityConf, mode, fallbackTier string) map[string]string {
	view := quality.WithGlobalOverride(mode)
	if _, ok := view.Presets[mode]; !ok {
		// A missing mode preset resolves to that mode's own characteristic tier
		// instead of borrowing the balanced preset.
		view.Presets = nil
	}
	tiers := make(map[string]string, len(providerNeutralCapabilities))
	for _, agent := range canonicalAgentNames {
		role, err := OMPAgentRole(agent)
		if err != nil {
			continue
		}
		capability, err := OMPNativeRoleCapability(role)
		if err != nil {
			continue
		}
		tier := view.AgentTier(QualityProviderClaude, agent, fallbackTier)
		if current, seen := tiers[capability]; !seen || builtinTierRank[tier] > builtinTierRank[current] {
			tiers[capability] = tier
		}
	}
	for _, capability := range providerNeutralCapabilities {
		if _, ok := tiers[capability]; !ok {
			tiers[capability] = fallbackTier
		}
	}
	return tiers
}
