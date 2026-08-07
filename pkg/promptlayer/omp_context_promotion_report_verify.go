package promptlayer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

const ompContextPromotionReportMaxBytesV1 = 512 * 1024

func decodeOMPContextPromotionReportV1(body []byte) (OMPContextPromotionReportV1, error) {
	if len(body) == 0 || len(body) > ompContextPromotionReportMaxBytesV1 {
		return OMPContextPromotionReportV1{}, fmt.Errorf("OMP context promotion report size is invalid")
	}
	var report OMPContextPromotionReportV1
	if err := decodeStrictOMPContextEvidenceV1(body, &report); err != nil {
		return report, fmt.Errorf("decode OMP context promotion report: %w", err)
	}
	canonical, err := json.Marshal(report)
	if err != nil || !bytes.Equal(canonical, body) {
		return report, fmt.Errorf("OMP context promotion report is not canonical JSON")
	}
	if err := validateOMPContextPromotionReportMetadataV1(report); err != nil {
		return report, err
	}
	if err := validateOMPContextPromotionCohortV1(report); err != nil {
		return report, err
	}
	return report, nil
}

func validateOMPContextPromotionReportMetadataV1(report OMPContextPromotionReportV1) error {
	if report.SchemaVersion != OMPContextPromotionReportSchemaV1 || report.TrustLane != OMPContextPromotionTrustLaneV2 ||
		!validOMPContextMemoryHashV1(report.EvidenceID) || !validOMPContextMemoryHashV1(report.ChallengeDigest) {
		return fmt.Errorf("OMP context promotion report identity is invalid")
	}
	expectedEvidenceID, err := computeOMPContextPromotionEvidenceIDV1(report)
	if err != nil || report.EvidenceID != expectedEvidenceID {
		return fmt.Errorf("OMP context promotion evidence ID is invalid")
	}
	if !validOMPContextPromotionProducerV1(report.Producer) || !validOMPContextPromotionCandidateV1(report.Candidate) {
		return fmt.Errorf("OMP context promotion report provenance is invalid")
	}
	policy := report.Policy
	if !safeOMPContextMemoryMetadataV1(policy.PolicyID) || !validOMPContextMemoryHashV1(policy.PolicyDigest) ||
		policy.HistoryMode != "active" || policy.MemoryMode != "off" || policy.MinPairCount != 20 ||
		policy.MinReductionBasisPoints != 2000 {
		return fmt.Errorf("OMP context promotion policy is invalid")
	}
	runtime := report.Runtime
	if !safeOMPContextMemoryMetadataV1(runtime.AutoVersion) || !validOMPContextMemoryHashV1(runtime.AutoBinarySHA256) ||
		!safeOMPContextMemoryMetadataV1(runtime.OMPVersion) || !validOMPContextMemoryHashV1(runtime.OMPExecutableSHA256) ||
		runtime.ExecutionClass != "external-live" || !runtime.ProductionPathEquivalent ||
		runtime.RuntimeKind != "omp-pipeline-managed-rpc" || !validOMPContextMemoryHashV1(runtime.PipelineImplementationDigest) {
		return fmt.Errorf("OMP context promotion runtime is invalid")
	}
	if report.SessionFacts != (OMPContextPromotionSessionFactsV1{
		FullProcessStarts: 1, OptimizedProcessStarts: 1,
		FullSessionCount: 1, OptimizedSessionCount: 1,
		MaxConcurrency: 1, CrossSessionContamination: 0,
	}) {
		return fmt.Errorf("OMP context promotion session facts are invalid")
	}
	if !safeOMPContextMemoryMetadataV1(report.Provider) || !validOMPContextMemoryHashV1(report.ModelScopeDigest) ||
		!validOMPContextMemoryHashV1(report.CohortManifestDigest) || !validOMPContextMemoryHashV1(report.OrderSeed) ||
		!validOMPContextMemoryHashV1(report.OraclePolicyDigest) {
		return fmt.Errorf("OMP context promotion scope is invalid")
	}
	taskBytes, err := json.Marshal(report.Tasks)
	if err != nil {
		return fmt.Errorf("OMP context promotion task manifest is invalid")
	}
	taskDigest := promotionSHA256(taskBytes)
	if report.CohortManifestDigest != taskDigest || report.OrderSeed != taskDigest {
		return fmt.Errorf("OMP context promotion task manifest digest is invalid")
	}
	return nil
}

func validOMPContextPromotionProducerV1(value OMPContextPromotionProducerV1) bool {
	if !safeOMPContextMemoryMetadataV1(value.Repository) || !safeOMPContextMemoryMetadataV1(value.WorkflowRef) || value.RunAttempt <= 0 {
		return false
	}
	runID, err := strconv.ParseUint(value.RunID, 10, 64)
	return err == nil && runID > 0 && strconv.FormatUint(runID, 10) == value.RunID
}

func validOMPContextPromotionCandidateV1(value OMPContextPromotionCandidateV1) bool {
	return safeOMPContextMemoryMetadataV1(value.Repository) && validOMPContextEvidenceGitHashV1(value.Revision) &&
		validOMPContextEvidenceGitHashV1(value.TreeSHA) && validOMPContextMemoryHashV1(value.ArtifactSHA256)
}

func validateOMPContextPromotionExpectationV2(value OMPContextPromotionExpectationV2) bool {
	return safeOMPContextMemoryMetadataV1(value.ProducerRepository) &&
		safeOMPContextMemoryMetadataV1(value.ProducerWorkflowRef) &&
		safeOMPContextMemoryMetadataV1(value.SigningKeyID) &&
		validOMPContextPromotionCandidateV1(value.Candidate) &&
		safeOMPContextMemoryMetadataV1(value.PolicyID) && validOMPContextMemoryHashV1(value.PolicyDigest) &&
		safeOMPContextMemoryMetadataV1(value.AutoVersion) && validOMPContextMemoryHashV1(value.AutoBinarySHA256) &&
		safeOMPContextMemoryMetadataV1(value.OMPVersion) && validOMPContextMemoryHashV1(value.OMPExecutableSHA256) &&
		validOMPContextMemoryHashV1(value.PipelineImplementationDigest) &&
		safeOMPContextMemoryMetadataV1(value.Provider) && validOMPContextMemoryHashV1(value.ModelScopeDigest) &&
		validOMPContextMemoryHashV1(value.CohortManifestDigest) && validOMPContextMemoryHashV1(value.OrderSeed) &&
		validOMPContextMemoryHashV1(value.OraclePolicyDigest)
}

func matchesOMPContextPromotionExpectationV2(report OMPContextPromotionReportV1, expected OMPContextPromotionExpectationV2) bool {
	return validateOMPContextPromotionExpectationV2(expected) && report.Producer.Repository == expected.ProducerRepository &&
		report.Producer.WorkflowRef == expected.ProducerWorkflowRef && reflect.DeepEqual(report.Candidate, expected.Candidate) &&
		report.Policy.PolicyID == expected.PolicyID &&
		report.Policy.PolicyDigest == expected.PolicyDigest && report.Runtime.AutoVersion == expected.AutoVersion &&
		report.Runtime.AutoBinarySHA256 == expected.AutoBinarySHA256 && report.Runtime.OMPVersion == expected.OMPVersion &&
		report.Runtime.OMPExecutableSHA256 == expected.OMPExecutableSHA256 &&
		report.Runtime.PipelineImplementationDigest == expected.PipelineImplementationDigest && report.Provider == expected.Provider &&
		report.ModelScopeDigest == expected.ModelScopeDigest && report.CohortManifestDigest == expected.CohortManifestDigest &&
		report.OrderSeed == expected.OrderSeed && report.OraclePolicyDigest == expected.OraclePolicyDigest
}
