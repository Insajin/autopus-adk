package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type pipelineOMPVerifiedGrant interface {
	Valid() bool
	ReportDigest() string
	EvidenceID() string
	ExpiresAt() time.Time
	ProducerCoordinates() promptlayer.OMPContextPromotionProducerV1
	CandidateCoordinates() promptlayer.OMPContextPromotionCandidateV1
	PolicyDigest() string
	RuntimeCoordinates() promptlayer.OMPContextPromotionRuntimeV1
	ProviderScope() (string, string)
}

type pipelineOMPActivePolicyProvider func(
	pipelineOMPManagedActiveCandidate,
) (promptlayer.OMPContextPromotionStaticPolicyV3, error)

type pipelineOMPActiveCurrentRuntimeProvider func(
	context.Context,
	pipelineOMPManagedActiveCandidate,
) (promptlayer.OMPContextPromotionCurrentRuntimeV3, error)

type pipelineOMPActiveGrantLoader func(
	string,
	time.Time,
	promptlayer.OMPContextPromotionStaticPolicyV3,
	promptlayer.OMPContextPromotionCurrentRuntimeV3,
) (pipelineOMPVerifiedGrant, error)

// pipelineOMPActiveSessionSpawn consumes a prepared lease and must enter the
// managed child spawn without another caller-controlled authority lookup.
type pipelineOMPActiveSessionSpawn func(
	context.Context,
	pipelineOMPManagedActiveCandidate,
	pipelineOMPManagedActivePrepared,
) (string, error)

type pipelineOMPActivePersistentSession interface {
	Execute(context.Context, pipelineOMPManagedActiveCandidate, pipelineOMPManagedActivePrepared) (string, error)
	Close() error
}

type pipelineOMPActiveSessionStart func(
	context.Context,
	pipelineOMPManagedActiveCandidate,
	pipelineOMPManagedActivePrepared,
) (pipelineOMPActivePersistentSession, error)

type pipelineOMPManagedActiveCoordinator struct {
	mu        sync.Mutex
	now       func() time.Time
	policy    pipelineOMPActivePolicyProvider
	current   pipelineOMPActiveCurrentRuntimeProvider
	loadGrant pipelineOMPActiveGrantLoader
	spawn     pipelineOMPActiveSessionSpawn
	start     pipelineOMPActiveSessionStart
	session   pipelineOMPActivePersistentSession
}

func newPipelineOMPManagedActiveCoordinator() *pipelineOMPManagedActiveCoordinator {
	return &pipelineOMPManagedActiveCoordinator{
		now: time.Now, policy: compiledPipelineOMPActiveStaticPolicy,
		loadGrant: func(root string, now time.Time,
			expected promptlayer.OMPContextPromotionStaticPolicyV3,
			current promptlayer.OMPContextPromotionCurrentRuntimeV3,
		) (pipelineOMPVerifiedGrant, error) {
			_ = now
			return promptlayer.LoadVerifiedOMPContextPromotionRuntimeV3(root, expected, current)
		},
	}
}

func (runner *pipelineOMPManagedActiveCoordinator) Prepare(
	ctx context.Context,
	candidate pipelineOMPManagedActiveCandidate,
) (pipelineOMPManagedActivePrepared, error) {
	if runner == nil || runner.policy == nil || runner.current == nil || runner.loadGrant == nil ||
		(runner.spawn == nil && runner.start == nil) {
		return pipelineOMPManagedActivePrepared{}, errors.New("pipeline: managed active trust pins are unavailable")
	}
	if ctx == nil || ctx.Err() != nil {
		return pipelineOMPManagedActivePrepared{}, errors.New("pipeline: managed active context is unavailable")
	}
	if !validPipelineOMPActiveGitHash(candidate.AutoSourceCommit) ||
		!validPipelineOMPActiveGitHash(candidate.AutoSourceTree) {
		return pipelineOMPManagedActivePrepared{}, errors.New("pipeline: managed active build provenance is unavailable")
	}
	expected, err := runner.policy(candidate)
	if err != nil {
		return pipelineOMPManagedActivePrepared{}, err
	}
	current, err := runner.current(ctx, candidate)
	if err != nil {
		return pipelineOMPManagedActivePrepared{}, err
	}
	now := runner.now().UTC()
	grant, err := runner.loadGrant(candidate.Snapshot.ProjectDir, now, expected, current)
	if err != nil || grant == nil || !grant.Valid() {
		return pipelineOMPManagedActivePrepared{}, errors.New("pipeline: signed managed active grant is unavailable")
	}
	binding, err := buildPipelineOMPActiveLeaseBinding(candidate, grant, expected)
	if err != nil {
		return pipelineOMPManagedActivePrepared{}, err
	}
	validFor := grant.ExpiresAt().Sub(now)
	if validFor > pipelineOMPActiveLeaseMaxValidity {
		validFor = pipelineOMPActiveLeaseMaxValidity
	}
	lease, err := newPipelineOMPActiveLease(binding, now, validFor)
	if err != nil {
		return pipelineOMPManagedActivePrepared{}, err
	}
	return pipelineOMPManagedActivePrepared{Lease: lease, Binding: binding, grant: grant}, nil
}

// Activate starts and initializes the run-scoped process without admitting a
// provider prompt. The backend fixes active mode only after this preflight.
func (runner *pipelineOMPManagedActiveCoordinator) Activate(
	ctx context.Context,
	candidate pipelineOMPManagedActiveCandidate,
	prepared pipelineOMPManagedActivePrepared,
) error {
	if runner == nil || runner.start == nil {
		return nil
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.session != nil {
		return nil
	}
	session, err := runner.start(ctx, candidate, prepared)
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("pipeline: managed active session factory returned nil")
	}
	runner.session = session
	return nil
}

func (runner *pipelineOMPManagedActiveCoordinator) Execute(
	ctx context.Context,
	candidate pipelineOMPManagedActiveCandidate,
	prepared pipelineOMPManagedActivePrepared,
) (string, error) {
	if runner == nil || (runner.spawn == nil && runner.start == nil) || prepared.grant == nil || !prepared.grant.Valid() {
		return "", errors.New("pipeline: managed active execution authority is unavailable")
	}
	if err := prepared.Lease.Consume(prepared.Binding, runner.now().UTC()); err != nil {
		return "", err
	}
	if runner.start == nil {
		return runner.spawn(ctx, candidate, prepared)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.session == nil {
		return "", errors.New("pipeline: managed active session was not activated")
	}
	return runner.session.Execute(ctx, candidate, prepared)
}

func (runner *pipelineOMPManagedActiveCoordinator) Close() error {
	if runner == nil {
		return nil
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.session == nil {
		return nil
	}
	err := runner.session.Close()
	runner.session = nil
	return err
}

func buildPipelineOMPActiveLeaseBinding(
	candidate pipelineOMPManagedActiveCandidate,
	grant pipelineOMPVerifiedGrant,
	expected promptlayer.OMPContextPromotionStaticPolicyV3,
) (pipelineOMPActiveLeaseBinding, error) {
	provider, modelScope := grant.ProviderScope()
	runtime := grant.RuntimeCoordinates()
	coordinates := grant.CandidateCoordinates()
	if provider != candidate.ScopeProvider || modelScope != candidate.ModelScopeDigest ||
		modelScope != expected.ModelScopeDigest ||
		coordinates.Revision != candidate.AutoSourceCommit || coordinates.TreeSHA != candidate.AutoSourceTree ||
		coordinates.Repository != expected.CandidateRepository || coordinates.TreeSHA != expected.SourceTree ||
		grant.PolicyDigest() != expected.PolicyDigest ||
		runtime.OMPVersion != expected.OMPVersion || runtime.OMPExecutableSHA256 != expected.OMPExecutableSHA256 ||
		runtime.AutoVersion != expected.AutoVersion ||
		runtime.PipelineImplementationDigest != expected.PipelineImplementationDigest ||
		runtime.PipelineImplementationDigest != pipelineOMPActiveImplementationDigest() {
		return pipelineOMPActiveLeaseBinding{}, errors.New("pipeline: signed managed active coordinates mismatch")
	}
	cfg, err := loadHarnessConfigForDir(candidate.Snapshot.ProjectDir, globalFlags{})
	if err != nil {
		return pipelineOMPActiveLeaseBinding{}, err
	}
	policy, selected, err := workflowContextPolicyFromConfig(cfg)
	if err != nil || !selected || policy.HistoryMode != config.OMPContextHistoryActive ||
		policy.MemoryMode != config.OMPContextMemoryOff {
		return pipelineOMPActiveLeaseBinding{}, errors.New("pipeline: managed active policy is unavailable")
	}
	options := promptlayer.ContextDeliveryOptions{
		Root: candidate.Snapshot.ProjectDir, Command: candidate.DeliveryCommand, SpecDir: candidate.Snapshot.SpecDir,
	}
	delivery, err := promptlayer.BuildContextDelivery(options)
	if err != nil || promptlayer.VerifyContextDeliveryForOptions(options, delivery) != nil {
		return pipelineOMPActiveLeaseBinding{}, errors.New("pipeline: managed active canonical delivery is unavailable")
	}
	identity := pipelineOMPContextIdentity(candidate.Snapshot)
	history := pipelineOMPContextHistory(candidate.Snapshot.CompletedHistory)
	receipt, err := promptlayer.BuildOMPContextBinding(promptlayer.OMPContextBindingInput{
		WorkspaceID: cfg.ProjectName, SpecID: candidate.Snapshot.SpecID, TaskID: identity[0],
		Phase: string(candidate.Snapshot.PhaseID), SessionID: identity[1], DeliveryOptions: options,
		Delivery: delivery, Ephemeral: promptlayer.OMPContextEphemeral{
			OriginalTask: candidate.OriginalTask, DecisionDelta: candidate.Snapshot.ActivePrompt,
		}, History: history,
	})
	if err != nil {
		return pipelineOMPActiveLeaseBinding{}, err
	}
	return pipelineOMPActiveLeaseBinding{
		GrantDigest: grant.ReportDigest(), PolicyDigest: grant.PolicyDigest(), WorkspaceID: cfg.ProjectName,
		SpecID: candidate.Snapshot.SpecID, TaskID: identity[0], Phase: string(candidate.Snapshot.PhaseID),
		SessionID: identity[1], BindingHash: receipt.BindingHash, OptionsHash: receipt.OptionsHash,
		SnapshotHash: candidate.Snapshot.SnapshotHash, GitCommitHash: candidate.Snapshot.GitCommitHash,
		OriginalTaskHash:  workflowContextRuntimeHash(candidate.OriginalTask),
		DecisionDeltaHash: workflowContextRuntimeHash(candidate.Snapshot.ActivePrompt), RuntimeVersion: runtime.OMPVersion,
		ExecutableSHA256: runtime.OMPExecutableSHA256, AutoVersion: runtime.AutoVersion,
		AutoExecutableSHA256: runtime.AutoBinarySHA256, ModelScopeDigest: candidate.ModelScopeDigest,
		AutoSourceCommit: candidate.AutoSourceCommit, AutoSourceTree: candidate.AutoSourceTree,
		CohortDigest: expected.CohortManifestDigest, OracleDigest: expected.OraclePolicyDigest,
		EligibleHistoryHash: hashPipelineOMPActiveHistory(receipt.EligibleHistoryRefs),
	}, nil
}

func hashPipelineOMPActiveHistory(rows []promptlayer.OMPContextHistoryReference) string {
	parts := make([]string, 0, len(rows)*5)
	for _, row := range rows {
		parts = append(parts, row.ID, row.SourceRef, row.BodyHash, fmt.Sprintf("%d", row.TokenEstimate), row.Reason)
	}
	return pipelineOMPActiveHash([]byte(strings.Join(parts, "\x00")))
}
