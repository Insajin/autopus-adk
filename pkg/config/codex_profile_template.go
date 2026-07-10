package config

// CodexSupervisorModel exposes the desired managed supervisor model to templates.
func (c *HarnessConfig) CodexSupervisorModel() string {
	return c.Quality.CodexSupervisorModel()
}

// CodexSupervisorEffort exposes the desired managed supervisor effort to templates.
func (c *HarnessConfig) CodexSupervisorEffort() string {
	return c.Quality.CodexSupervisorEffort()
}

// CodexAgentModel exposes the desired managed agent model to templates.
func (c *HarnessConfig) CodexAgentModel(agentName, fallbackTier, declaredEffort string) string {
	return c.Quality.CodexAgentProfile(agentName, fallbackTier, declaredEffort).Model
}

// CodexAgentEffort exposes the desired managed agent effort to templates.
func (c *HarnessConfig) CodexAgentEffort(agentName, fallbackTier, declaredEffort string) string {
	return c.Quality.CodexAgentEffort(agentName, fallbackTier, declaredEffort)
}
