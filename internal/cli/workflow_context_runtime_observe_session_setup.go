package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/version"
)

type workflowContextObserveSessionSetup struct {
	taskRoot            string
	backend             pipelineOMPBackendConfig
	candidate           pipelineOMPManagedActiveCandidate
	prepared            pipelineOMPManagedActivePrepared
	full                *pipelineOMPActiveEvaluatorSession
	optimized           *pipelineOMPActiveEvaluatorSession
	ompVersion          string
	ompExecutableSHA256 string
}

func validateWorkflowContextObserveSessionOptions(options workflowContextObserveSessionOptions) error {
	if !workflowContextObserveProviderPattern.MatchString(options.Provider) ||
		!workflowContextObserveProviderPattern.MatchString(options.Model) ||
		!workflowContextMetadataPattern.MatchString(options.SpecID) ||
		!pipelineOMPContextCohortLocatorPattern.MatchString(options.CredentialLocator) ||
		!validPipelineOMPActiveGitHash(options.TargetGitCommit) ||
		strings.TrimSpace(options.Endpoint) == "" || strings.TrimSpace(options.Executable) == "" {
		return errors.New("workflow context-runtime observe-session coordinates are invalid")
	}
	return nil
}

func prepareWorkflowContextObserveSession(
	ctx context.Context,
	options workflowContextObserveSessionOptions,
	challenge string,
) (workflowContextObserveSessionSetup, error) {
	projectDir, err := canonicalWorkflowContextObserveProject(options.ProjectDir)
	if err != nil {
		return workflowContextObserveSessionSetup{}, err
	}
	endpoint, err := validatePipelineOMPActiveEndpoint(options.Endpoint)
	if err != nil {
		return workflowContextObserveSessionSetup{}, errors.New("observe-session endpoint is invalid")
	}
	credential, found := os.LookupEnv(options.CredentialLocator)
	if !found || strings.TrimSpace(credential) == "" || strings.ContainsRune(credential, 0) {
		return workflowContextObserveSessionSetup{}, errors.New("observe-session credential is unavailable")
	}
	executable, err := exec.LookPath(options.Executable)
	if err != nil {
		return workflowContextObserveSessionSetup{}, errors.New("observe-session OMP executable is unavailable")
	}
	executable, executableID, err := canonicalPipelineOMPExecutable(executable)
	if err != nil {
		return workflowContextObserveSessionSetup{}, err
	}
	if !validPipelineOMPActiveGitHash(version.SourceCommit()) || !validPipelineOMPActiveGitHash(version.SourceTree()) {
		return workflowContextObserveSessionSetup{}, errors.New("observe-session candidate build provenance is unavailable")
	}
	taskRoot, err := os.MkdirTemp("", "autopus-omp-observe-session-")
	if err != nil {
		return workflowContextObserveSessionSetup{}, err
	}
	setup := workflowContextObserveSessionSetup{taskRoot: taskRoot}
	failed := true
	defer func() {
		if failed {
			_ = os.RemoveAll(taskRoot)
		}
	}()
	if err := os.Chmod(taskRoot, 0o700); err != nil {
		return setup, err
	}
	runtimeBase := filepath.Join(taskRoot, "runtime")
	if err := os.Mkdir(runtimeBase, 0o700); err != nil {
		return setup, err
	}
	phaseModels := map[pipeline.PhaseID]string{pipeline.PhasePlan: options.Provider + "/" + options.Model}
	backend, err := normalizePipelineOMPBackendConfig(pipelineOMPBackendConfig{
		Executable: executable, executableID: executableID, ProjectDir: projectDir,
		SpecID: options.SpecID, SpecDir: filepath.Join(projectDir, ".autopus", "specs", options.SpecID),
		SnapshotHash: challenge, GitCommitHash: options.TargetGitCommit, RuntimeBase: runtimeBase,
		Environment: []string{
			"PATH=" + os.Getenv("PATH"), pipelineOMPActiveEndpointKey + "=" + endpoint,
			pipelineOMPActiveCredentialKey + "=" + credential,
		},
		PhaseModels: phaseModels, MaxTime: 10 * time.Minute,
	})
	if err != nil {
		return setup, err
	}
	ompVersion, err := probeWorkflowContextObserveVersion(ctx, WorkflowContextManagedRPCOptions{
		Executable: executable, Workspace: projectDir, Environment: backend.Environment, AllowedEndpoint: endpoint,
	})
	if err != nil || verifyPipelineOMPExecutable(executable, executableID) != nil {
		return setup, errors.New("observe-session OMP identity probe failed")
	}
	snapshot := pipeline.OMPExecutionSnapshot{
		ProjectDir: projectDir, SpecID: options.SpecID, SpecDir: backend.SpecDir,
		SnapshotHash: challenge, GitCommitHash: options.TargetGitCommit, PhaseID: pipeline.PhasePlan,
		Attempt: 1, Prompt: "observe-session", ActivePrompt: "observe-session",
	}
	candidate, err := newPipelineOMPManagedActiveCandidate(snapshot, phaseModels[pipeline.PhasePlan], phaseModels)
	if err != nil {
		return setup, err
	}
	prepared := pipelineOMPManagedActivePrepared{Binding: pipelineOMPActiveLeaseBinding{
		GrantDigest: challenge, PolicyDigest: workflowContextRuntimeHash("observe-session-policy-v1"),
		WorkspaceID: filepath.Base(projectDir), SpecID: options.SpecID,
		GitCommitHash: options.TargetGitCommit, AutoSourceCommit: candidate.AutoSourceCommit,
		AutoSourceTree: candidate.AutoSourceTree, ModelScopeDigest: candidate.ModelScopeDigest,
	}}
	setup.backend, setup.candidate, setup.prepared = backend, candidate, prepared
	setup.ompVersion = ompVersion
	setup.ompExecutableSHA256 = fmt.Sprintf("sha256:%x", executableID.digest[:])
	failed = false
	return setup, nil
}

func (setup *workflowContextObserveSessionSetup) start(ctx context.Context) error {
	full, err := startPipelineOMPActiveEvaluatorSession(ctx, setup.backend, setup.candidate, setup.prepared, false)
	if err != nil {
		return err
	}
	optimized, err := startPipelineOMPActiveEvaluatorSession(ctx, setup.backend, setup.candidate, setup.prepared, true)
	if err != nil {
		_ = full.Close()
		return err
	}
	setup.full, setup.optimized = full, optimized
	return nil
}

func (setup *workflowContextObserveSessionSetup) close() error {
	if setup == nil || setup.taskRoot == "" {
		return nil
	}
	err := errors.Join(setup.full.Close(), setup.optimized.Close())
	err = errors.Join(err, os.RemoveAll(setup.taskRoot))
	setup.taskRoot = ""
	return err
}
