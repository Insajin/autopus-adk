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

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/insajin/autopus-adk/pkg/version"
)

type workflowContextObserveSessionSetup struct {
	taskRoot                string
	projectDir              string
	endpoint                string
	credential              string
	backend                 pipelineOMPBackendConfig
	candidate               pipelineOMPManagedActiveCandidate
	prepared                pipelineOMPManagedActivePrepared
	full                    *pipelineOMPActiveEvaluatorSession
	deliveryOptions         promptlayer.ContextDeliveryOptions
	delivery                promptlayer.ContextDeliveryResult
	canonicalPromptHash     string
	optimized               *pipelineOMPActiveEvaluatorSession
	ompVersion              string
	ompExecutableSHA256     string
	autoVersion             string
	autoExecutableSHA256    string
	candidateTree           string
	providerAuthorityDigest string
	sandboxMode             pipelineOMPActiveSandboxMode
}

func validateWorkflowContextObserveSessionOptions(options workflowContextObserveSessionOptions) error {
	_, policyErr := promptlayer.OMPContextPromotionPolicyDigestV1(options.PromotionPolicy)
	metadata := []string{
		options.WorkspaceID, options.ProducerRepository, options.ProducerWorkflowRef,
		options.CandidateRepository, options.PolicyID,
	}
	metadataValid := true
	for _, value := range metadata {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value ||
			len(value) > 256 || strings.ContainsRune(value, 0) {
			metadataValid = false
		}
	}
	if !workflowContextObserveProviderPattern.MatchString(options.Provider) ||
		!workflowContextObserveProviderPattern.MatchString(options.Model) ||
		!workflowContextMetadataPattern.MatchString(options.SpecID) ||
		!pipelineOMPContextCohortLocatorPattern.MatchString(options.CredentialLocator) ||
		!pipelineOMPContextObservationRunID.MatchString(options.ProducerRunID) ||
		options.ProducerRunAttempt <= 0 || !metadataValid || !validPipelineOMPActiveHash(options.OraclePolicyDigest) ||
		!validPipelineOMPActiveGitHash(options.TargetGitCommit) || policyErr != nil ||
		options.PromotionPolicy.HistoryMode != config.OMPContextHistoryActive ||
		options.PromotionPolicy.MemoryMode != config.OMPContextMemoryOff ||
		options.EvidenceValidFor <= 0 || options.EvidenceValidFor > 24*time.Hour ||
		(options.SandboxMode != pipelineOMPActiveSandboxManaged &&
			options.SandboxMode != pipelineOMPActiveSandboxInheritedParent) ||
		strings.TrimSpace(options.Endpoint) == "" || strings.TrimSpace(options.Executable) == "" {
		return errors.New("workflow context-runtime observe-session coordinates are invalid")
	}
	return nil
}

// @AX:WARN [AUTO]: observe-session setup contains 13 if branches.
// @AX:REASON [AUTO]: provider authority, executable provenance, isolated runtime, phase scope, version proof, lease binding, and cleanup converge here.
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
	if !found || strings.TrimSpace(credential) == "" || len(credential) > pipelineOMPActiveCredentialMaxBytes ||
		strings.ContainsRune(credential, 0) {
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
	if !validPipelineOMPActiveGitHash(version.SourceCommit()) || !validPipelineOMPActiveGitHash(version.SourceTree()) ||
		options.TargetGitCommit != version.SourceCommit() {
		return workflowContextObserveSessionSetup{}, errors.New("observe-session candidate build provenance is unavailable")
	}
	autoExecutableSHA256, err := workflowContextObserveCurrentExecutableSHA256()
	if err != nil {
		return workflowContextObserveSessionSetup{}, err
	}
	policyDigest, err := promptlayer.OMPContextPromotionPolicyDigestV1(options.PromotionPolicy)
	if err != nil {
		return workflowContextObserveSessionSetup{}, err
	}
	specDirRef := filepath.ToSlash(filepath.Join(".autopus", "specs", options.SpecID))
	specDir := filepath.Join(projectDir, filepath.FromSlash(specDirRef))
	snapshotHash, err := pipeline.SpecSnapshotHash(specDir)
	if err != nil {
		return workflowContextObserveSessionSetup{}, errors.New("observe-session SPEC snapshot is unavailable")
	}
	deliveryOptions := promptlayer.ContextDeliveryOptions{
		Root: projectDir, Command: "go", SpecDir: specDirRef,
	}
	delivery, err := promptlayer.BuildContextDelivery(deliveryOptions)
	if err != nil || promptlayer.VerifyContextDeliveryForOptions(deliveryOptions, delivery) != nil {
		return workflowContextObserveSessionSetup{}, errors.New("observe-session canonical context is unavailable")
	}
	taskRoot, err := os.MkdirTemp("", "autopus-omp-observe-session-")
	if err != nil {
		return workflowContextObserveSessionSetup{}, err
	}
	setup := workflowContextObserveSessionSetup{
		taskRoot: taskRoot, projectDir: projectDir, endpoint: endpoint, credential: credential,
		deliveryOptions: deliveryOptions, delivery: delivery,
		canonicalPromptHash: workflowContextRuntimeHash(delivery.Prompt),
	}
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
	phaseModels := workflowContextObserveSessionPhaseModels(options.Provider + "/" + options.Model)
	backend, err := normalizePipelineOMPBackendConfig(pipelineOMPBackendConfig{
		Executable: executable, executableID: executableID, ProjectDir: projectDir,
		SpecID: options.SpecID, SpecDir: specDir,
		SnapshotHash: snapshotHash, GitCommitHash: options.TargetGitCommit, RuntimeBase: runtimeBase,
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
	}, options.SandboxMode)
	if err != nil || verifyPipelineOMPExecutable(executable, executableID) != nil {
		return setup, errors.New("observe-session OMP identity probe failed")
	}
	snapshot := pipeline.OMPExecutionSnapshot{
		ProjectDir: projectDir, SpecID: options.SpecID, SpecDir: backend.SpecDir,
		SnapshotHash: snapshotHash, GitCommitHash: options.TargetGitCommit, PhaseID: pipeline.PhaseImplement,
		Attempt: 1, Prompt: "observe-session", ActivePrompt: "observe-session",
	}
	candidate, err := newPipelineOMPManagedActiveCandidate(snapshot, phaseModels[pipeline.PhaseImplement], phaseModels)
	if err != nil {
		return setup, err
	}
	prepared := pipelineOMPManagedActivePrepared{Binding: pipelineOMPActiveLeaseBinding{
		GrantDigest: challenge, PolicyDigest: policyDigest,
		WorkspaceID: options.WorkspaceID, SpecID: options.SpecID,
		GitCommitHash: options.TargetGitCommit, AutoSourceCommit: candidate.AutoSourceCommit,
		AutoSourceTree: candidate.AutoSourceTree, ModelScopeDigest: candidate.ModelScopeDigest,
	}}
	setup.backend, setup.candidate, setup.prepared = backend, candidate, prepared
	setup.ompVersion = ompVersion
	setup.ompExecutableSHA256 = fmt.Sprintf("sha256:%x", executableID.digest[:])
	setup.autoVersion, setup.autoExecutableSHA256 = version.Version(), autoExecutableSHA256
	setup.candidateTree = version.SourceTree()
	setup.sandboxMode = options.SandboxMode
	failed = false
	return setup, nil
}

func (setup *workflowContextObserveSessionSetup) start(ctx context.Context) error {
	full, err := startPipelineOMPActiveEvaluatorSession(
		ctx, setup.backend, setup.candidate, setup.prepared, false, setup.sandboxMode,
	)
	if err != nil {
		return err
	}
	optimized, err := startPipelineOMPActiveEvaluatorSession(
		ctx, setup.backend, setup.candidate, setup.prepared, true, setup.sandboxMode,
	)
	if err != nil {
		_ = full.Close()
		return err
	}
	if full.binding.OptionsHash != optimized.binding.OptionsHash {
		_ = errors.Join(full.Close(), optimized.Close())
		return errors.New("observe-session provider authority changed across variants")
	}
	setup.full, setup.optimized = full, optimized
	setup.providerAuthorityDigest = full.binding.OptionsHash
	return nil
}

func (setup *workflowContextObserveSessionSetup) close() error {
	if setup == nil || setup.taskRoot == "" {
		return nil
	}
	root := setup.taskRoot
	err := errors.Join(setup.full.Close(), setup.optimized.Close())
	err = errors.Join(err, os.RemoveAll(root))
	if setup.full.PID() != 0 || setup.optimized.PID() != 0 {
		err = errors.Join(err, errors.New("observe-session process cleanup is incomplete"))
	}
	if _, statErr := os.Lstat(root); !errors.Is(statErr, os.ErrNotExist) {
		err = errors.Join(err, errors.New("observe-session runtime cleanup is incomplete"))
	}
	setup.taskRoot = ""
	return err
}

func workflowContextObserveSessionPhaseModels(selector string) map[pipeline.PhaseID]string {
	return map[pipeline.PhaseID]string{
		pipeline.PhasePlan:         selector,
		pipeline.PhaseTestScaffold: selector,
		pipeline.PhaseImplement:    selector,
		pipeline.PhaseValidate:     selector,
		pipeline.PhaseReview:       selector,
	}
}
