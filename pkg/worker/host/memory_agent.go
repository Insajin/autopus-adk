package host

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

var fetchWorkspaceAgents = setup.FetchWorkspaceAgents

func resolveRuntimeMemoryAgentID(backendURL, token, workspaceID, configured string) (string, []string) {
	configured = strings.TrimSpace(configured)
	if strings.TrimSpace(backendURL) == "" || strings.TrimSpace(token) == "" {
		return configured, nil
	}
	if _, err := uuid.Parse(strings.TrimSpace(workspaceID)); err != nil {
		return configured, nil
	}

	agents, err := fetchWorkspaceAgents(backendURL, token, workspaceID)
	if err != nil {
		if configured == "" {
			return "", []string{fmt.Sprintf("memory agent lookup failed: %v", err)}
		}
		return configured, []string{fmt.Sprintf("memory agent validation failed: %v", err)}
	}

	if configured != "" && hasWorkspaceAgentID(agents, configured) {
		return configured, nil
	}

	selected := setup.SelectMemoryAgentID(agents)
	if selected == "" {
		if configured == "" {
			return "", nil
		}
		return "", []string{fmt.Sprintf("configured memory_agent_id %s is not in workspace; memory context disabled", configured)}
	}

	if configured == "" {
		return selected, []string{fmt.Sprintf("memory_agent_id auto-selected: %s", selected)}
	}
	return selected, []string{fmt.Sprintf("configured memory_agent_id %s is not in workspace; using %s", configured, selected)}
}

func hasWorkspaceAgentID(agents []setup.WorkspaceAgent, id string) bool {
	for _, agent := range agents {
		if agent.ID == id {
			return true
		}
	}
	return false
}
