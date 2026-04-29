package host

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
	"github.com/stretchr/testify/assert"
)

func TestResolveRuntimeMemoryAgentID_ReplacesStaleConfiguredID(t *testing.T) {
	origFetch := fetchWorkspaceAgents
	t.Cleanup(func() { fetchWorkspaceAgents = origFetch })
	fetchWorkspaceAgents = func(_, _, _ string) ([]setup.WorkspaceAgent, error) {
		return []setup.WorkspaceAgent{
			{ID: "11111111-2222-4333-8444-555555555555", Type: "dev_worker", Tier: "worker", Status: "active"},
		}, nil
	}

	got, warnings := resolveRuntimeMemoryAgentID(
		"https://api.autopus.co",
		"token",
		"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		"99999999-2222-4333-8444-555555555555",
	)

	assert.Equal(t, "11111111-2222-4333-8444-555555555555", got)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "not in workspace")
}

func TestResolveRuntimeMemoryAgentID_KeepsValidConfiguredID(t *testing.T) {
	origFetch := fetchWorkspaceAgents
	t.Cleanup(func() { fetchWorkspaceAgents = origFetch })
	fetchWorkspaceAgents = func(_, _, _ string) ([]setup.WorkspaceAgent, error) {
		return []setup.WorkspaceAgent{
			{ID: "11111111-2222-4333-8444-555555555555", Type: "dev_worker", Tier: "worker", Status: "active"},
		}, nil
	}

	got, warnings := resolveRuntimeMemoryAgentID(
		"https://api.autopus.co",
		"token",
		"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		"11111111-2222-4333-8444-555555555555",
	)

	assert.Equal(t, "11111111-2222-4333-8444-555555555555", got)
	assert.Empty(t, warnings)
}

func TestResolveRuntimeMemoryAgentID_SkipsLookupForNonUUIDWorkspace(t *testing.T) {
	origFetch := fetchWorkspaceAgents
	t.Cleanup(func() { fetchWorkspaceAgents = origFetch })
	fetchWorkspaceAgents = func(_, _, _ string) ([]setup.WorkspaceAgent, error) {
		t.Fatal("fetchWorkspaceAgents must not be called")
		return nil, nil
	}

	got, warnings := resolveRuntimeMemoryAgentID(
		"https://api.autopus.co",
		"token",
		"local-workspace",
		"99999999-2222-4333-8444-555555555555",
	)

	assert.Equal(t, "99999999-2222-4333-8444-555555555555", got)
	assert.Empty(t, warnings)
}
