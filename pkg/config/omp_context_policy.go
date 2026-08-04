package config

const (
	OMPContextHistoryOff    = "off"
	OMPContextHistoryShadow = "shadow"
	OMPContextHistoryActive = "active"

	OMPContextMemoryOff    = "off"
	OMPContextMemoryShadow = "shadow"

	OMPContextFallbackBlock         = "block"
	OMPContextFallbackCanonicalFull = "canonical_full"

	OMPContextCapabilityProbeRequired = "probe_required"

	OMPContextRuntimeNoSession         = "no_session"
	OMPContextRuntimeIsolatedTaskOwned = "isolated_task_owned"
	OMPContextMutationSessionOverlay   = "session_overlay"

	DefaultOMPContextHistoryTargetTokens = 1000
	MinOMPContextHistoryTargetTokens     = 128
	MaxOMPContextHistoryTargetTokens     = 32768
	DefaultOMPContextMemoryTTLSeconds    = 3600
	MinOMPContextMemoryTTLSeconds        = 60
	MaxOMPContextMemoryTTLSeconds        = 604800
	MaxOMPContextMemoryNamespaceLength   = 128
)

// ContextConf is the agent context enrichment configuration.
type ContextConf struct {
	SignatureMap bool `yaml:"signature_map"`
}

// OMPContextPolicyConf is an opt-in named profile catalog.
type OMPContextPolicyConf struct {
	Profile  string                           `yaml:"profile,omitempty"`
	Profiles map[string]OMPContextProfileConf `yaml:"profiles,omitempty"`
}

// OMPContextProfileConf controls only task-scoped OMP context optimization.
type OMPContextProfileConf struct {
	HistoryMode         string `yaml:"history_mode,omitempty"`
	MemoryMode          string `yaml:"memory_mode,omitempty"`
	HistoryTargetTokens int    `yaml:"history_target_tokens,omitempty"`
	MemoryTTLSeconds    int    `yaml:"memory_ttl_seconds,omitempty"`
	MemoryNamespace     string `yaml:"memory_namespace,omitempty"`
	Fallback            string `yaml:"fallback,omitempty"`
	CapabilityPolicy    string `yaml:"capability_policy,omitempty"`
	RuntimeRootPolicy   string `yaml:"runtime_root_policy,omitempty"`
	MutationScope       string `yaml:"mutation_scope,omitempty"`
}

// IsOptedIn reports only explicit profile selection, never profile presence alone.
func (c OMPContextPolicyConf) IsOptedIn() bool {
	return c.Profile != ""
}

// LookupSelectedOMPContextProfile returns the raw selected profile without defaults.
func (c OMPContextPolicyConf) LookupSelectedOMPContextProfile() (string, OMPContextProfileConf, bool) {
	if c.Profile == "" {
		return "", OMPContextProfileConf{}, false
	}
	profile, ok := c.Profiles[c.Profile]
	return c.Profile, profile, ok
}

// SelectedOMPContextProfile validates and returns a detached effective profile.
func (c OMPContextPolicyConf) SelectedOMPContextProfile() (string, OMPContextProfileConf, bool, error) {
	if !c.IsOptedIn() {
		return "", OMPContextProfileConf{}, false, nil
	}
	if err := c.Validate(); err != nil {
		return "", OMPContextProfileConf{}, false, err
	}
	name, profile, ok := c.LookupSelectedOMPContextProfile()
	if !ok {
		return name, OMPContextProfileConf{}, false, nil
	}
	return name, effectiveOMPContextProfile(profile), true, nil
}

// @AX:WARN [AUTO]: effective context-profile defaulting contains 8 if branches.
// @AX:REASON [AUTO]: history, memory, fallback, budget, TTL, and promotion defaults must remain internally consistent.
func effectiveOMPContextProfile(profile OMPContextProfileConf) OMPContextProfileConf {
	effective := profile
	if effective.HistoryMode == "" {
		effective.HistoryMode = OMPContextHistoryShadow
	}
	if effective.MemoryMode == "" {
		effective.MemoryMode = OMPContextMemoryOff
	}
	if effective.HistoryTargetTokens == 0 {
		effective.HistoryTargetTokens = DefaultOMPContextHistoryTargetTokens
	}
	if effective.MemoryTTLSeconds == 0 {
		effective.MemoryTTLSeconds = DefaultOMPContextMemoryTTLSeconds
	}
	if effective.Fallback == "" {
		effective.Fallback = OMPContextFallbackCanonicalFull
	}
	if effective.CapabilityPolicy == "" {
		effective.CapabilityPolicy = OMPContextCapabilityProbeRequired
	}
	if effective.RuntimeRootPolicy == "" {
		effective.RuntimeRootPolicy = OMPContextRuntimeIsolatedTaskOwned
	}
	if effective.MutationScope == "" {
		effective.MutationScope = OMPContextMutationSessionOverlay
	}
	return effective
}
