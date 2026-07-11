package config

// CodexSupervisorModel exposes a managed model only when the project opts in.
func (c *HarnessConfig) CodexSupervisorModel() string {
	if c == nil || !c.Quality.ManagesSupervisorModel() {
		return ""
	}
	return c.Quality.CodexSupervisorModel()
}

// CodexSupervisorEffort exposes managed effort only when the project opts in.
func (c *HarnessConfig) CodexSupervisorEffort() string {
	if c == nil || !c.Quality.ManagesSupervisorModel() {
		return ""
	}
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
