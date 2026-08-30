package omp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratedOMPTeamMultiContract(t *testing.T) {
	surfaces := generatedOMPTeamMultiSurfaces(t)
	required := []string{
		"owner `omp` native `task` batch with explicit Lead/Builder/Guardian responsibilities",
		"`--multi` selects provider-diverse planning and review, not team execution topology",
		"`--team --multi` composes team execution with provider-diverse planning and review",
		"without `--team` or `--solo`, owner `omp` uses the default native `task` batch pipeline",
		"`--team` conflicts with `--solo` and owner `orca`",
	}

	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			for _, contract := range required {
				assert.Contains(t, surface.body, contract)
			}
			assertOMPTeamCoordinationLine(t, surface.body)
		})
	}
}

func TestGeneratedOMPTeamMultiContractRejectsForeignCoordination(t *testing.T) {
	forbidden := []string{
		".omp/skills/agent-teams",
		"spawn_agent",
		"send_message",
		"followup_task",
		"wait_agent",
		"interrupt_agent",
		"list_agents",
		"get_goal",
		"create_goal",
		"update_goal",
		"Multi-Agent V2",
	}

	for _, surface := range generatedOMPTeamMultiSurfaces(t) {
		t.Run(surface.name, func(t *testing.T) {
			for _, token := range forbidden {
				assert.NotContains(t, surface.body, token)
			}
		})
	}
}

type ompTeamMultiSurface struct {
	name string
	body string
}

func generatedOMPTeamMultiSurfaces(t *testing.T) []ompTeamMultiSurface {
	t.Helper()
	root := generateOMPOnly(t)
	names := []string{"auto-go", "agent-pipeline"}
	surfaces := make([]ompTeamMultiSurface, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, ".omp", "skills", name, "SKILL.md"))
		require.NoError(t, err)
		_, body := splitEmittedFrontmatter(t, string(data))
		require.NotEmpty(t, strings.TrimSpace(body))
		surfaces = append(surfaces, ompTeamMultiSurface{name: name, body: body})
	}
	return surfaces
}

func assertOMPTeamCoordinationLine(t *testing.T, body string) {
	t.Helper()
	fragments := []string{"Lead", "Builder", "Guardian", "`task`", "`hub`", "`todo`"}
	for _, line := range strings.Split(body, "\n") {
		matched := true
		for _, fragment := range fragments {
			if !strings.Contains(line, fragment) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	assert.Fail(t, "missing native team coordination sentence", "one line must contain %q", fragments)
}
