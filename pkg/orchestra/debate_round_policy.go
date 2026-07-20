package orchestra

func mayStopDebateEarly(round, planned int, responses []ProviderResponse, cfg OrchestraConfig) bool {
	return round >= 2 && round < planned && len(responses) >= 2 && consensusReached(responses, cfg)
}
