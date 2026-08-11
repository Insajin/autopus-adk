package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/orcarun"
	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// pipelineOrcaPhaseRoles maps canonical model-receipt role names onto pipeline
// phases. It mirrors loadPipelineOMPPhaseModelsWithAuthority so both execution
// owners route the same role to the same phase.
var pipelineOrcaPhaseRoles = map[string]pipeline.PhaseID{
	"planner": pipeline.PhasePlan, "tester": pipeline.PhaseTestScaffold,
	"executor": pipeline.PhaseImplement, "validator": pipeline.PhaseValidate,
	"reviewer": pipeline.PhaseReview,
}

// pipelineOrcaProviderAgents maps receipt provider identifiers onto orca agent
// names. Unknown providers fail closed: running a phase under the wrong agent
// is worse than refusing to run it.
var pipelineOrcaProviderAgents = map[string]string{
	"openai-codex": "codex",
	"anthropic":    "claude",
	"google":       "gemini",
	"gemini":       "gemini",
}

// pipelineOrcaPhaseRole returns the receipt role a phase is routed from, used
// as the PhaseResponse role so orca receipts read like OMP receipts.
func pipelineOrcaPhaseRole(phase pipeline.PhaseID) string {
	for role, routed := range pipelineOrcaPhaseRoles {
		if routed == phase {
			return role
		}
	}
	return string(phase)
}

// loadPipelineOrcaPhaseLaunch derives one orca launch per canonical phase from
// the OMP model resolution receipt.
//
// The receipt keeps provider, model, selector, and thinking effort apart. Only
// the opaque provider model id and the effort level cross the process-plane
// boundary (REQ-107); the joined selector stays inside the policy plane.
func loadPipelineOrcaPhaseLaunch(projectDir string) (map[pipeline.PhaseID]orcarun.Launch, error) {
	receipt, err := ompadapter.LoadOMPModelResolutionReceipt(projectDir)
	if err != nil {
		return nil, err
	}
	launches := make(map[pipeline.PhaseID]orcarun.Launch, len(pipelineOrcaPhaseRoles))
	for _, role := range receipt.Roles {
		phase, wanted := pipelineOrcaPhaseRoles[role.Agent]
		if !wanted {
			continue
		}
		if _, duplicate := launches[phase]; duplicate {
			return nil, fmt.Errorf("duplicate orca model route for phase %s", phase)
		}
		agent, known := pipelineOrcaProviderAgents[role.Provider]
		if !known {
			return nil, fmt.Errorf("no orca agent is defined for provider %q (role %s)", role.Provider, role.Agent)
		}
		if strings.TrimSpace(role.Model) == "" {
			return nil, fmt.Errorf("orca model route for phase %s has no provider model id", phase)
		}
		launches[phase] = orcarun.Launch{Agent: agent, Model: role.Model, Effort: role.Thinking}
	}
	if len(launches) != len(pipelineOrcaPhaseRoles) {
		return nil, errors.New("OMP model receipt does not define all canonical pipeline phases")
	}
	return launches, nil
}

// newPipelineOrcaBackendForRun builds the run-scoped orca backend.
//
// Availability of the orca binary is checked before anything else: when the
// process plane is missing, no receipt is read, no subprocess is spawned, and
// no run state is written. The returned error wraps orcarun.ErrOrcaUnavailable
// so the caller can tell an absent process plane from a misconfigured one.
func newPipelineOrcaBackendForRun(
	projectDir string,
	specID string,
	resolvedSpec resolvedPipelineSpec,
	gitHash string,
) (*pipelineOrcaBackend, error) {
	if _, err := exec.LookPath(pipelineOrcaBinary); err != nil {
		return nil, fmt.Errorf("pipeline: %w: %v", orcarun.ErrOrcaUnavailable, err)
	}
	if strings.TrimSpace(resolvedSpec.SnapshotHash) == "" || strings.TrimSpace(gitHash) == "" {
		return nil, errors.New("pipeline: orca backend requires a resolved SPEC snapshot and git commit hash")
	}
	launches, err := loadPipelineOrcaPhaseLaunch(projectDir)
	if err != nil {
		return nil, fmt.Errorf("pipeline: load orca model routes: %w", err)
	}
	return newPipelineOrcaBackend(pipelineOrcaBackendConfig{
		SpecID:      specID,
		ProjectDir:  projectDir,
		PhaseLaunch: launches,
		Client:      orcarun.New(),
	})
}
