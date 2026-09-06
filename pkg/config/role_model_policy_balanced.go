package config

// The balanced built-in profile is an explicit OMP role matrix instead of a
// projection of the quality presets. OMP routes each agent through exactly one
// model at exactly one thinking level, so an absent model or an unsupported
// thinking level must block activation rather than quietly landing the agent
// on a weaker rung. Every agent follows the selected anchor family, review
// included: multi-provider review is an orchestra setting and never a routing
// concern, which is why this profile leaves family diversity off.

// balancedRoleRung groups the canonical agents by the depth their work needs.
type balancedRoleRung int

const (
	balancedRungTop balancedRoleRung = iota
	balancedRungImplementation
	balancedRungRoutine
)

// balancedRungByAgent covers every canonical agent exactly once. Debugger and
// deep-worker share the top rung with the planning and review roles: both
// routinely have to hold a whole failing system in context at once.
var balancedRungByAgent = map[string]balancedRoleRung{
	"architect":        balancedRungTop,
	"debugger":         balancedRungTop,
	"deep-worker":      balancedRungTop,
	"planner":          balancedRungTop,
	"reviewer":         balancedRungTop,
	"security-auditor": balancedRungTop,
	"spec-writer":      balancedRungTop,

	"devops":              balancedRungImplementation,
	"executor":            balancedRungImplementation,
	"frontend-specialist": balancedRungImplementation,
	"perf-engineer":       balancedRungImplementation,
	"tester":              balancedRungImplementation,

	"annotator":    balancedRungRoutine,
	"explorer":     balancedRungRoutine,
	"ux-validator": balancedRungRoutine,
	"validator":    balancedRungRoutine,
}

// balancedRungCandidates pins one exact candidate per anchor family and rung.
// A rung never carries a second candidate: ordered availability fallback stays
// available to hand-written profiles, and this profile declines it so an
// unavailable model surfaces as a named blocker.
var balancedRungCandidates = map[string]map[balancedRoleRung]RoleModelCandidateConf{
	builtinRoleModelFamilyAnthropic: {
		balancedRungTop: {
			Selector: "anthropic/" + ClaudeFableModel, Thinking: "max", Family: "anthropic",
		},
		balancedRungImplementation: {
			Selector: "anthropic/" + ClaudeSonnetModel, Thinking: "max", Family: "anthropic",
		},
		balancedRungRoutine: {
			Selector: "anthropic/" + ClaudeSonnetModel, Thinking: "high", Family: "anthropic",
		},
	},
	builtinRoleModelFamilyOpenAI: {
		balancedRungTop: {
			Selector: "openai-codex/" + CodexAstraModel, Thinking: "max", Family: "openai",
		},
		balancedRungImplementation: {
			Selector: "openai-codex/" + CodexLunaModel, Thinking: "max", Family: "openai",
		},
		balancedRungRoutine: {
			Selector: "openai-codex/" + CodexLunaModel, Thinking: "max", Family: "openai",
		},
	},
}

// balancedRoleModelProfile builds the balanced profile for one anchor family.
func balancedRoleModelProfile(family, mode string) RoleModelProfileConf {
	rungs := balancedRungCandidates[family]
	agents := make(map[string]RoleAgentOverrideConf, len(balancedRungByAgent))
	for agent, rung := range balancedRungByAgent {
		agents[agent] = RoleAgentOverrideConf{
			Candidates: []RoleModelCandidateConf{rungs[rung]},
		}
	}
	capabilities := make(map[string]RoleCapabilityRouteConf, len(providerNeutralCapabilities))
	for _, capability := range providerNeutralCapabilities {
		capabilities[capability] = RoleCapabilityRouteConf{
			Candidates: []RoleModelCandidateConf{rungs[balancedCapabilityRung(capability)]},
			Required:   true,
		}
	}
	return RoleModelProfileConf{
		ConfigMode:   mode,
		CatalogTrust: RoleModelCatalogTrustOperatorAttested,
		Capabilities: capabilities,
		Agents:       agents,
		ManagedKeys:  builtinRoleModelManagedKeys(mode),
	}
}

// balancedCapabilityRung gives a capability the rung of its representative
// agent. Capability routes are defaults for a hand-written profile that copies
// them without the per-agent overrides, so coding_tool_use deliberately keeps
// the implementation rung: folding it onto the highest rung among its agents
// would put every execution role behind debugger's top-rung model.
func balancedCapabilityRung(capability string) balancedRoleRung {
	return balancedRungByAgent[canonicalAgentByCapability[capability]]
}
