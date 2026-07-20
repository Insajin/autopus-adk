package orchestra

func preflightJudgeFamilySeparation(cfg OrchestraConfig) (*OrchestraResult, error) {
	if cfg.Strategy != StrategyDebate || cfg.JudgeProvider == "" || cfg.NoJudge || !cfg.RequireJudgeFamilySeparation {
		return nil, nil
	}
	evidence := evaluateJudgeFamilySeparation(cfg.Providers, findOrBuildJudgeConfig(cfg), true)
	if err := judgeSeparationError(evidence); err != nil {
		result := &OrchestraResult{
			Strategy: cfg.Strategy, RunID: cfg.RunID,
			Summary:  "orchestration blocked: required judge model-family separation failed",
			Degraded: true, DegradedReasons: []string{"judge_family_separation"},
			JudgeStatus: JudgeFailed, TerminalState: TerminalBlocked,
			GateStatus: "blocked", AnalysisVerdict: "incomplete",
			JudgeSeparation: evidence,
		}
		return finalizeOrchestraResultForConfig(result, cfg), err
	}
	return nil, nil
}

func subprocessPipelineContractConfig(cfg SubprocessPipelineConfig) OrchestraConfig {
	judge := cfg.Judge
	return OrchestraConfig{
		Providers: cfg.Providers, Strategy: StrategyDebate,
		JudgeProvider: judge.Name, JudgeConfig: &judge,
		RequireJudgeFamilySeparation: cfg.RequireJudgeFamilySeparation,
	}
}
