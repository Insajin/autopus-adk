package promptlayer

import (
	"encoding/json"
	"errors"
	"reflect"
)

// BuildOMPContextPromotionStaticPolicyV3 binds a canonical production report
// to the release-only coordinates that cannot be derived from live evidence.
func BuildOMPContextPromotionStaticPolicyV3(
	report OMPContextPromotionReportV1,
	target, releaseLineageKeyID, releaseLineageHandoff string,
	minimumRollbackFloor uint64,
) (OMPContextPromotionStaticPolicyV3, []byte, error) {
	canonical, _, err := BuildOMPContextPromotionReportV1(report)
	if err != nil || !reflect.DeepEqual(canonical, report) {
		return OMPContextPromotionStaticPolicyV3{}, nil,
			errors.New("OMP context promotion report is not canonical")
	}
	policy := OMPContextPromotionStaticPolicyV3{
		SchemaVersion:                OMPContextPromotionRuntimeSchemaV3,
		ProducerRepository:           report.Producer.Repository,
		ProducerWorkflowRef:          report.Producer.WorkflowRef,
		CandidateRepository:          report.Candidate.Repository,
		SourceCommit:                 report.Candidate.Revision,
		SourceTree:                   report.Candidate.TreeSHA,
		Target:                       target,
		AutoVersion:                  report.Runtime.AutoVersion,
		PolicyID:                     report.Policy.PolicyID,
		PolicyDigest:                 report.Policy.PolicyDigest,
		OMPVersion:                   report.Runtime.OMPVersion,
		OMPExecutableSHA256:          report.Runtime.OMPExecutableSHA256,
		PipelineImplementationDigest: report.Runtime.PipelineImplementationDigest,
		Provider:                     report.Provider,
		ModelScopeDigest:             report.ModelScopeDigest,
		CohortManifestDigest:         report.CohortManifestDigest,
		OrderSeed:                    report.OrderSeed,
		OraclePolicyDigest:           report.OraclePolicyDigest,
		ReleaseLineageKeyID:          releaseLineageKeyID,
		ReleaseLineageHandoff:        releaseLineageHandoff,
		MinimumRollbackFloor:         minimumRollbackFloor,
	}
	body, err := MarshalOMPContextPromotionStaticPolicyV3(policy)
	if err != nil {
		return OMPContextPromotionStaticPolicyV3{}, nil, err
	}
	return policy, body, nil
}

// MarshalOMPContextPromotionStaticPolicyV3 validates and canonically encodes a
// policy assembled from deterministic pre-canary release coordinates.
func MarshalOMPContextPromotionStaticPolicyV3(policy OMPContextPromotionStaticPolicyV3) ([]byte, error) {
	if !validOMPContextPromotionStaticPolicyV3(policy) {
		return nil, errors.New("OMP context promotion static policy is invalid")
	}
	body, err := json.Marshal(policy)
	if err != nil {
		return nil, errors.New("marshal OMP context promotion static policy")
	}
	return body, nil
}
