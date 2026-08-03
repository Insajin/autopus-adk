package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: model-route loading is the installed OMP-to-phase authority boundary.
// @AX:REASON [AUTO]: Executable version, persisted receipt binding, and all five canonical roles determine process model selection.
func loadPipelineOMPPhaseModels(
	projectDir, executable string,
) (map[pipeline.PhaseID]string, error) {
	_, identity, err := canonicalPipelineOMPExecutable(executable)
	if err != nil {
		return nil, err
	}
	environment, err := normalizePipelineOMPEnvironment(os.Environ())
	if err != nil {
		return nil, err
	}
	return loadPipelineOMPPhaseModelsWithAuthority(projectDir, executable, identity, environment)
}

// @AX:WARN [AUTO]: Authority-bound model-route loading has cyclomatic complexity 16 across identity, freshness, duplicate, and completeness checks.
// @AX:REASON [AUTO]: A stale or partial receipt must never select a phase model for the long-lived OMP process.
func loadPipelineOMPPhaseModelsWithAuthority(
	projectDir, executable string,
	identity pipelineOMPExecutableIdentity,
	environment []string,
) (map[pipeline.PhaseID]string, error) {
	if err := verifyPipelineOMPExecutable(executable, identity); err != nil {
		return nil, err
	}
	normalizedEnvironment, err := normalizePipelineOMPEnvironment(environment)
	if err != nil {
		return nil, err
	}
	observedVersion, err := observePipelineOMPVersion(projectDir, executable, identity, normalizedEnvironment)
	if err != nil {
		return nil, err
	}
	metadataDir := filepath.Join(projectDir, ".autopus")
	if info, err := os.Lstat(metadataDir); err != nil {
		if os.IsNotExist(err) {
			return map[pipeline.PhaseID]string{}, nil
		}
		return nil, err
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("OMP model receipt parent is unsafe")
	}
	receiptPath := filepath.Join(projectDir, filepath.FromSlash(ompadapter.OMPModelReceiptRelativePath))
	if _, err := os.Lstat(receiptPath); err != nil {
		if os.IsNotExist(err) {
			return map[pipeline.PhaseID]string{}, nil
		}
		return nil, err
	}
	receipt, err := ompadapter.LoadOMPModelResolutionReceipt(projectDir)
	if err != nil {
		return nil, err
	}
	if receipt.OMPVersion != observedVersion {
		return nil, fmt.Errorf("OMP model receipt version does not match installed executable")
	}
	agentPhases := map[string]pipeline.PhaseID{
		"planner": pipeline.PhasePlan, "tester": pipeline.PhaseTestScaffold,
		"executor": pipeline.PhaseImplement, "validator": pipeline.PhaseValidate,
		"reviewer": pipeline.PhaseReview,
	}
	models := make(map[pipeline.PhaseID]string, len(agentPhases))
	for _, role := range receipt.Roles {
		phase, wanted := agentPhases[role.Agent]
		if !wanted {
			continue
		}
		if _, duplicate := models[phase]; duplicate {
			return nil, fmt.Errorf("duplicate OMP model route for phase %s", phase)
		}
		models[phase] = role.Selector
	}
	if len(models) != len(agentPhases) {
		return nil, fmt.Errorf("OMP model receipt does not define all canonical pipeline phases")
	}
	return models, nil
}

func observePipelineOMPVersion(
	projectDir, executable string,
	identity pipelineOMPExecutableIdentity,
	environment []string,
) (string, error) {
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: executable identity probes are bounded to 5 seconds and 4 KiB.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "--version")
	cmd.Dir = projectDir
	cmd.Env = append([]string(nil), environment...)
	if err := verifyPipelineOMPExecutable(executable, identity); err != nil {
		return "", err
	}
	output, err := processprobe.OutputLimited(cmd, 4<<10)
	if err != nil {
		return "", fmt.Errorf("OMP pipeline executable identity probe failed")
	}
	if err := verifyPipelineOMPExecutable(executable, identity); err != nil {
		return "", err
	}
	observed := strings.TrimSpace(string(output))
	if !installedOMPVersionPattern.MatchString(observed) {
		return "", fmt.Errorf("OMP pipeline executable identity is invalid")
	}
	return observed, nil
}
