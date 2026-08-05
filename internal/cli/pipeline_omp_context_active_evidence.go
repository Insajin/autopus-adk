package cli

import (
	"errors"
	"reflect"
	"sort"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type pipelineOMPVerifiedRunEvidence interface {
	ProviderAuthorityDigest() string
	SessionAuthorityDigest() string
	CanaryRows() []promptlayer.OMPContextCanaryRowV1
}

func verifyPipelineOMPActiveRunEvidence(
	candidate pipelineOMPManagedActiveCandidate,
	grant pipelineOMPVerifiedGrant,
	policy promptlayer.OMPContextPromotionPolicyV1,
	receipt promptlayer.OMPContextBindingReceipt,
	workspaceID string,
	taskID string,
	sessionID string,
	now time.Time,
) (time.Time, error) {
	evidence, ok := grant.(pipelineOMPVerifiedRunEvidence)
	policyDigest, err := promptlayer.OMPContextPromotionPolicyDigestV1(policy)
	if !ok || err != nil || policyDigest != grant.PolicyDigest() ||
		!validPipelineOMPActiveHash(evidence.ProviderAuthorityDigest()) ||
		!validPipelineOMPActiveHash(evidence.SessionAuthorityDigest()) || len(evidence.CanaryRows()) != 40 {
		return time.Time{}, errors.New("pipeline: signed observed run evidence is incomplete")
	}
	verified, err := promptlayer.LoadOMPContextEvidenceForExpectationV1(
		candidate.Snapshot.ProjectDir,
		promptlayer.OMPContextEvidenceExpectationV1{
			WorkspaceID: workspaceID, SpecID: candidate.Snapshot.SpecID,
			SnapshotHash: candidate.Snapshot.SnapshotHash, GitCommitHash: candidate.Snapshot.GitCommitHash,
			RuntimeVersion: grant.RuntimeCoordinates().OMPVersion,
		},
		promptlayer.OMPContextPromotionSubjectV1{
			WorkspaceID: workspaceID, SpecID: candidate.Snapshot.SpecID, TaskID: taskID,
			Phase: string(candidate.Snapshot.PhaseID), SessionID: sessionID, BindingHash: receipt.BindingHash,
		},
		policy,
		now,
	)
	if err != nil {
		return time.Time{}, err
	}
	if len(verified.HistoryRefs) != 0 ||
		!reflect.DeepEqual(canonicalPipelineOMPActiveCanaryRows(verified.Promotion.Rows),
			canonicalPipelineOMPActiveCanaryRows(evidence.CanaryRows())) {
		return time.Time{}, errors.New("pipeline: observed run evidence does not match the signed cohort")
	}
	expiresAt := verified.Promotion.Attestation.ExpiresAt()
	if expiresAt.IsZero() || !now.Before(expiresAt) {
		return time.Time{}, errors.New("pipeline: observed run evidence is stale")
	}
	return expiresAt, nil
}

func canonicalPipelineOMPActiveCanaryRows(
	rows []promptlayer.OMPContextCanaryRowV1,
) []promptlayer.OMPContextCanaryRowV1 {
	ordered := append([]promptlayer.OMPContextCanaryRowV1(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].TaskID != ordered[j].TaskID {
			return ordered[i].TaskID < ordered[j].TaskID
		}
		if ordered[i].Variant != ordered[j].Variant {
			return ordered[i].Variant < ordered[j].Variant
		}
		return ordered[i].Order < ordered[j].Order
	})
	return ordered
}
