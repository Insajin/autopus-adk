package telemetry

// UsageEnvelopeVersion is the current normalized receipt schema version.
const UsageEnvelopeVersion = 1

// Stable usage statuses.
const (
	UsageStatusActual      = "actual"
	UsageStatusCostOnly    = "cost_only"
	UsageStatusEstimated   = "estimated"
	UsageStatusUnavailable = "unavailable"
)

// Stable usage sources.
const (
	UsageSourceProvider = "provider"
	UsageSourceEstimate = "estimate"
)

// Stable component inclusion relations.
const (
	ComponentSubsetOfInput  = "subset_of_input"
	ComponentSubsetOfOutput = "subset_of_output"
	ComponentSeparate       = "separate"
)

// Stable reasons for unavailable usage.
const (
	UsageReasonProviderAbsent           = "provider_usage_absent"
	UsageReasonComponentRelationUnknown = "component_relation_unknown"
	UsageReasonInconsistentComponents   = "inconsistent_components"
	UsageReasonDuplicateCallConflict    = "duplicate_call_conflict"
	UsageReasonIdentityMissing          = "identity_missing"
)

// UsageInput contains provider usage without prompt or response bodies.
type UsageInput struct {
	RunID           string
	CallID          string
	TaskID          string
	Attempt         int
	Provider        string
	Model           string
	Effort          string
	ProviderVersion string
	ModelVersion    string
	RiskPolicy      string
	CacheStratum    string
	ConfigHash      string
	Phase           string
	Role            string
	Source          string
	SourceSchema    string

	InputTokensTotal         *int64
	UncachedInputTokens      *int64
	CachedInputTokens        *int64
	CacheCreationInputTokens *int64
	CacheReadInputTokens     *int64
	OutputTokensTotal        *int64
	ReasoningTokens          *int64
	ReasoningRelation        string
	ToolTokens               *int64
	ToolRelation             string
	ActualCostUSD            *float64
	EstimatedTotalTokens     *int64
	EstimatedCostUSD         *float64
	PricingVersion           string
}

// UsageEnvelope is the normalized, versioned usage receipt for one model call.
// Nullable numeric fields deliberately omit omitempty so missing actuals encode
// as JSON null rather than zero or absence.
type UsageEnvelope struct {
	Version         int    `json:"version"`
	RunID           string `json:"run_id"`
	CallID          string `json:"call_id"`
	TaskID          string `json:"task_id,omitempty"`
	Attempt         int    `json:"attempt,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	Effort          string `json:"effort,omitempty"`
	ProviderVersion string `json:"provider_version,omitempty"`
	ModelVersion    string `json:"model_version,omitempty"`
	RiskPolicy      string `json:"risk_policy,omitempty"`
	CacheStratum    string `json:"cache_stratum,omitempty"`
	ConfigHash      string `json:"config_hash,omitempty"`
	Phase           string `json:"phase,omitempty"`
	Role            string `json:"role,omitempty"`
	UsageStatus     string `json:"usage_status"`
	UsageSource     string `json:"usage_source,omitempty"`
	SourceSchema    string `json:"source_schema,omitempty"`

	InputTokensTotal         *int64   `json:"input_tokens_total"`
	UncachedInputTokens      *int64   `json:"uncached_input_tokens"`
	CachedInputTokens        *int64   `json:"cached_input_tokens"`
	CacheCreationInputTokens *int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *int64   `json:"cache_read_input_tokens"`
	OutputTokensTotal        *int64   `json:"output_tokens_total"`
	ReasoningTokens          *int64   `json:"reasoning_tokens"`
	ReasoningRelation        string   `json:"reasoning_relation,omitempty"`
	ToolTokens               *int64   `json:"tool_tokens"`
	ToolRelation             string   `json:"tool_relation,omitempty"`
	RawTotalTokens           *int64   `json:"raw_total_tokens"`
	ActualCostUSD            *float64 `json:"actual_cost_usd"`
	EstimatedTotalTokens     *int64   `json:"estimated_total_tokens"`
	EstimatedCostUSD         *float64 `json:"estimated_cost_usd"`
	PricingVersion           string   `json:"pricing_version,omitempty"`
	UnavailableReason        string   `json:"unavailable_reason,omitempty"`
}

// NormalizeUsage converts provider-specific inclusive or component usage into
// a single receipt without guessing missing component relations.
// @AX:ANCHOR: [AUTO] @AX:SPEC: SPEC-ADK-ULTRA-EFFICIENCY-001 — provider adapters and validation share this public normalization boundary.
// @AX:REASON: Five production consumers depend on its null, cache-subset, and separate-component semantics remaining consistent.
func NormalizeUsage(input UsageInput) UsageEnvelope {
	envelope := UsageEnvelope{
		Version: UsageEnvelopeVersion, RunID: input.RunID, CallID: input.CallID, TaskID: input.TaskID,
		Attempt: input.Attempt, Provider: input.Provider, Model: input.Model,
		Effort: input.Effort, Phase: input.Phase, Role: input.Role,
		ProviderVersion: input.ProviderVersion, ModelVersion: input.ModelVersion,
		RiskPolicy: input.RiskPolicy, CacheStratum: input.CacheStratum, ConfigHash: input.ConfigHash,
		UsageSource: input.Source, SourceSchema: input.SourceSchema,
		InputTokensTotal:         cloneInt64(input.InputTokensTotal),
		UncachedInputTokens:      cloneInt64(input.UncachedInputTokens),
		CachedInputTokens:        cloneInt64(input.CachedInputTokens),
		CacheCreationInputTokens: cloneInt64(input.CacheCreationInputTokens),
		CacheReadInputTokens:     cloneInt64(input.CacheReadInputTokens),
		OutputTokensTotal:        cloneInt64(input.OutputTokensTotal),
		ReasoningTokens:          cloneInt64(input.ReasoningTokens), ReasoningRelation: input.ReasoningRelation,
		ToolTokens: cloneInt64(input.ToolTokens), ToolRelation: input.ToolRelation,
		ActualCostUSD:        cloneFloat64(input.ActualCostUSD),
		EstimatedTotalTokens: cloneInt64(input.EstimatedTotalTokens),
		EstimatedCostUSD:     cloneFloat64(input.EstimatedCostUSD), PricingVersion: input.PricingVersion,
	}

	if envelope.RunID == "" || envelope.CallID == "" {
		return unavailable(envelope, UsageReasonIdentityMissing)
	}
	if reason := normalizeInputTokens(&envelope); reason != "" {
		return unavailable(envelope, reason)
	}
	if hasNegativeActual(envelope) {
		return unavailable(envelope, UsageReasonInconsistentComponents)
	}
	if input.Source == UsageSourceEstimate {
		envelope.UsageStatus = UsageStatusEstimated
		return envelope
	}
	if envelope.InputTokensTotal == nil || envelope.OutputTokensTotal == nil {
		if envelope.ActualCostUSD != nil && !hasAnyActualTokens(envelope) {
			envelope.UsageStatus = UsageStatusCostOnly
			return envelope
		}
		if hasAnyActualTokens(envelope) {
			return unavailable(envelope, UsageReasonInconsistentComponents)
		}
		return unavailable(envelope, UsageReasonProviderAbsent)
	}

	total := *envelope.InputTokensTotal + *envelope.OutputTokensTotal
	if reason := addSeparateComponent(&total, envelope.ReasoningTokens, envelope.ReasoningRelation); reason != "" {
		return unavailable(envelope, reason)
	}
	if reason := addSeparateComponent(&total, envelope.ToolTokens, envelope.ToolRelation); reason != "" {
		return unavailable(envelope, reason)
	}
	envelope.RawTotalTokens = int64Pointer(total)
	envelope.UsageStatus = UsageStatusActual
	return envelope
}

func normalizeInputTokens(envelope *UsageEnvelope) string {
	hasSplitCache := envelope.CacheCreationInputTokens != nil || envelope.CacheReadInputTokens != nil
	var cached *int64
	if hasSplitCache {
		value := int64(0)
		if envelope.CacheCreationInputTokens != nil {
			value += *envelope.CacheCreationInputTokens
		}
		if envelope.CacheReadInputTokens != nil {
			value += *envelope.CacheReadInputTokens
		}
		cached = int64Pointer(value)
		if envelope.CachedInputTokens != nil && *envelope.CachedInputTokens != value {
			return UsageReasonInconsistentComponents
		}
	} else {
		cached = envelope.CachedInputTokens
	}

	componentsPresent := envelope.UncachedInputTokens != nil || cached != nil
	if envelope.InputTokensTotal == nil && componentsPresent {
		if envelope.UncachedInputTokens == nil {
			return UsageReasonInconsistentComponents
		}
		total := *envelope.UncachedInputTokens
		if cached != nil {
			total += *cached
		}
		envelope.InputTokensTotal = int64Pointer(total)
	}
	if envelope.InputTokensTotal != nil && cached != nil && envelope.UncachedInputTokens == nil {
		if *cached > *envelope.InputTokensTotal {
			return UsageReasonInconsistentComponents
		}
		envelope.UncachedInputTokens = int64Pointer(*envelope.InputTokensTotal - *cached)
	}
	if envelope.InputTokensTotal != nil && envelope.UncachedInputTokens != nil {
		expected := *envelope.UncachedInputTokens
		if cached != nil {
			expected += *cached
		}
		if expected != *envelope.InputTokensTotal {
			return UsageReasonInconsistentComponents
		}
	}
	return ""
}

func addSeparateComponent(total *int64, value *int64, relation string) string {
	if value == nil {
		return ""
	}
	switch relation {
	case ComponentSubsetOfInput, ComponentSubsetOfOutput:
		return ""
	case ComponentSeparate:
		*total += *value
		return ""
	default:
		return UsageReasonComponentRelationUnknown
	}
}

func unavailable(envelope UsageEnvelope, reason string) UsageEnvelope {
	envelope.UsageStatus = UsageStatusUnavailable
	envelope.UnavailableReason = reason
	envelope.RawTotalTokens = nil
	return envelope
}

func hasNegativeActual(envelope UsageEnvelope) bool {
	values := []*int64{envelope.InputTokensTotal, envelope.UncachedInputTokens, envelope.CachedInputTokens,
		envelope.CacheCreationInputTokens, envelope.CacheReadInputTokens, envelope.OutputTokensTotal,
		envelope.ReasoningTokens, envelope.ToolTokens}
	for _, value := range values {
		if value != nil && *value < 0 {
			return true
		}
	}
	return false
}

func hasAnyActualTokens(envelope UsageEnvelope) bool {
	values := []*int64{envelope.InputTokensTotal, envelope.UncachedInputTokens, envelope.CachedInputTokens,
		envelope.CacheCreationInputTokens, envelope.CacheReadInputTokens, envelope.OutputTokensTotal,
		envelope.ReasoningTokens, envelope.ToolTokens}
	for _, value := range values {
		if value != nil {
			return true
		}
	}
	return false
}

func int64Pointer(value int64) *int64 { return &value }
func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	return int64Pointer(*value)
}
func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
