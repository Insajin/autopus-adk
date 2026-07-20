package cli

import "github.com/insajin/autopus-adk/pkg/orchestra"

// OrchestraFlags holds optional flags for runOrchestraCommand. A struct avoids
// silent breakage when call sites add or reorder command options.
type OrchestraFlags struct {
	NoDetach          bool
	KeepRelay         bool
	NoJudge           bool
	YieldRounds       bool
	ContextAware      bool
	SubprocessMode    bool
	TimeoutChanged    bool
	RiskTier          reviewRiskTier
	RiskInputs        []string
	FallbackMode      orchestra.ReliabilityFallbackMode
	ProvidersExplicit bool
	OutputFormat      string
}
