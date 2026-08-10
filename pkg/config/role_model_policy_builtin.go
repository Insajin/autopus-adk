package config

// Built-in role-model profiles project the quality presets onto OMP capability
// routes, so selecting `role_model_policy.profile: balanced` (or `ultra`) is
// enough to carry the quality tiers into OMP without hand-writing a candidate
// list. Selection stays opt-in: a config that names no profile is untouched,
// and an explicitly defined profile of the same name always wins.

// builtinSelectorProvider is the OMP provider segment of a derived selector.
// Derived candidates only ever name Claude slugs, so the family is fixed too.
const builtinSelectorProvider = "anthropic"

// builtinRoleModelFallbackTier is the tier a built-in profile assumes for an
// agent its quality preset does not mention. It keeps `ultra` at the top tier
// even for a config that carries no presets of its own.
var builtinRoleModelFallbackTier = map[string]string{
	"balanced": "sonnet",
	"ultra":    "opus",
}

// builtinThinkingByTier maps a relative tier onto an OMP native thinking level.
var builtinThinkingByTier = map[string]string{
	"opus":   "xhigh",
	"sonnet": "medium",
	"haiku":  "low",
}

// builtinTierRank orders the relative tiers for the max-wins capability rule.
var builtinTierRank = map[string]int{"haiku": 0, "sonnet": 1, "opus": 2}

// IsBuiltinRoleModelProfileName reports whether a profile name is served by a
// quality-derived profile when the config defines no profile under that name.
func IsBuiltinRoleModelProfileName(name string) bool {
	_, ok := builtinRoleModelFallbackTier[name]
	return ok
}

// BuiltinRoleModelProfile derives one built-in profile from the quality presets.
func BuiltinRoleModelProfile(name string, quality QualityConf) (RoleModelProfileConf, bool) {
	fallbackTier, ok := builtinRoleModelFallbackTier[name]
	if !ok {
		return RoleModelProfileConf{}, false
	}
	tiers := builtinCapabilityTiers(quality, name, fallbackTier)
	capabilities := make(map[string]RoleCapabilityRouteConf, len(providerNeutralCapabilities))
	for _, capability := range providerNeutralCapabilities {
		tier := tiers[capability]
		capabilities[capability] = RoleCapabilityRouteConf{
			// A derived profile must never block an OMP apply the user did not
			// hand-write: when the local catalog has no such Claude model the
			// route degrades to OMP's own runtime default instead of failing.
			DegradedAction: "runtime_default",
			Candidates: []RoleModelCandidateConf{{
				Selector: builtinSelectorProvider + "/" + ClaudeModelForTier(tier),
				Thinking: builtinThinkingByTier[tier],
				Family:   builtinSelectorProvider,
			}},
		}
	}
	return RoleModelProfileConf{
		ConfigMode:   RoleModelConfigModeOverlay,
		Capabilities: capabilities,
		// Enabled with no roles: every derived candidate is Anthropic, so no
		// role can be routed to a distinct family. The OMP adapter still
		// requires the diversity policy to be explicitly present.
		FamilyDiversity: FamilyDiversityPolicyConf{Enabled: true},
	}, true
}

// builtinCapabilityTiers folds the per-agent preset tiers onto capabilities by
// composing the agent -> native role and native role -> capability matrices.
//
// A capability takes the highest tier of every agent routed to it (max wins).
// Over-serving an agent only costs money, while under-serving one silently
// demotes it below the tier its Claude and Codex profiles already use, which
// would quietly undo a measured tier promotion.
func builtinCapabilityTiers(quality QualityConf, mode, fallbackTier string) map[string]string {
	view := quality.WithGlobalOverride(mode)
	if _, ok := view.Presets[mode]; !ok {
		// Without a preset of its own the mode must resolve to its own
		// characteristic tier; dropping the presets stops AgentTier from
		// borrowing the balanced preset for an ultra profile.
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
