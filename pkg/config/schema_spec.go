package config

// SpecConf는 SPEC 엔진 설정이다.
type SpecConf struct {
	IDFormat   string         `yaml:"id_format"`
	EARSTypes  []string       `yaml:"ears_types"`
	ReviewGate ReviewGateConf `yaml:"review_gate,omitempty"`
}

// ReviewGateConf는 멀티-프로바이더 SPEC 리뷰 게이트 설정이다.
type ReviewGateConf struct {
	Enabled            bool     `yaml:"enabled"`
	Strategy           string   `yaml:"strategy"`
	Providers          []string `yaml:"providers,flow"`
	Judge              string   `yaml:"judge"`
	MaxRevisions       int      `yaml:"max_revisions"`
	AutoCollectContext bool     `yaml:"auto_collect_context"`
	ContextMaxLines    int      `yaml:"context_max_lines"`
	// ExcludeFailedFromDenom drops infra-failed providers from the supermajority
	// denominator (SPEC-SPECREV-001 REQ-VERD-3). Default false preserves legacy
	// behavior of dividing by the configured provider count.
	ExcludeFailedFromDenom bool `yaml:"exclude_failed_from_denom,omitempty"`
	// VerdictThreshold is the supermajority fraction required for a verdict (default 0.67).
	VerdictThreshold float64 `yaml:"verdict_threshold,omitempty"`
	// PassCriteria overrides the default verdict decision rules in the prompt.
	PassCriteria string `yaml:"pass_criteria,omitempty"`
	// DocContextMaxLines is the authoring-time over-cap warning threshold and the
	// per-document compression fallback threshold (default 200). It is NOT the total
	// injection budget: SPEC-ADK-REVIEW-INTEGRITY-001 REQ-RINT-FULL-02 makes
	// spec.ResolveAuxTotalBudget the single budget-resolution authority, which floors
	// this value up to spec.DefaultAuxTotalBudgetLines (operators may raise the total
	// above the default but can never shrink it below).
	// @AX:NOTE: [AUTO] behavior boundary — this field's semantics changed under the same name/yaml key; existing configs that set doc_context_max_lines to control injection size now only affect the authoring-warning threshold, not the total budget
	DocContextMaxLines int `yaml:"doc_context_max_lines,omitempty"`
	// MinProviders is the minimum number of providers that must return a usable
	// review before the gate may auto-promote status to "approved"
	// (SPEC-ADK-REVIEW-INTEGRITY-001 REQ-RINT-QUORUM-05). Zero means unset and
	// derives the majority default (configured/2+1) via spec.DefaultMinProviders at
	// the consumption site; a positive value overrides it.
	MinProviders int `yaml:"min_providers,omitempty"`
}
