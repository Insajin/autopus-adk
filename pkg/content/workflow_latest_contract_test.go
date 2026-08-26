package content

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

func TestDeriveRouteA_DispatchesPlannerAndExecutorWithResultHandoff(t *testing.T) {
	t.Parallel()
	schema, err := workflow.LoadSchema(filepath.Join(repoContentDir(t), "workflows", routeASchemaFile))
	require.NoError(t, err)
	js := deriveWorkflowJS(schema)

	planning := strings.Index(js, "agent(`Plan SPEC")
	implementation := strings.Index(js, "agent(`Implement SPEC")
	require.NotEqual(t, -1, planning)
	require.NotEqual(t, -1, implementation)
	assert.Less(t, planning, implementation)
	assert.Contains(t, js, "planningResult")
	assert.Contains(t, js, "Planning result: ${JSON.stringify(planningResult)}")
}

func TestDeriveRouteTeam_UsesSharedTreeDisjointOwnershipWithoutPerCallIsolation(t *testing.T) {
	t.Parallel()
	schema, err := workflow.LoadSchema(filepath.Join(repoContentDir(t), "workflows", routeTeamSchemaFile))
	require.NoError(t, err)
	js := deriveTeamWorkflowJS(schema)

	assert.NotContains(t, js, "isolation:")
	assert.NotContains(t, js, "isolated-worktree")
	assert.Contains(t, js, "DISJOINT set of files")
	assert.Contains(t, js, "shared working tree")
}
