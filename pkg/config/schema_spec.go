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
	// DocContextMaxLines limits how many lines of plan/research/acceptance docs to inject (default 200).
	DocContextMaxLines int `yaml:"doc_context_max_lines,omitempty"`
}
