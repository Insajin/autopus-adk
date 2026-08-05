package promptlayer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

// BuildOMPContextPromotionReportV1 derives the canonical body-free fields and
// re-enters the same strict verifier used by signed active admission.
func BuildOMPContextPromotionReportV1(
	report OMPContextPromotionReportV1,
) (OMPContextPromotionReportV1, []byte, error) {
	if report.SchemaVersion != "" && report.SchemaVersion != OMPContextPromotionReportSchemaV1 {
		return report, nil, fmt.Errorf("OMP context promotion report schema is invalid")
	}
	if report.TrustLane != "" && report.TrustLane != OMPContextPromotionTrustLaneV2 {
		return report, nil, fmt.Errorf("OMP context promotion report trust lane is invalid")
	}
	report.SchemaVersion = OMPContextPromotionReportSchemaV1
	report.TrustLane = OMPContextPromotionTrustLaneV2
	if len(report.Tasks) != 20 || len(report.Observations) != 40 {
		return report, nil, fmt.Errorf("OMP context promotion cohort is incomplete")
	}
	taskBytes, err := json.Marshal(report.Tasks)
	if err != nil {
		return report, nil, fmt.Errorf("marshal OMP context promotion tasks: %w", err)
	}
	report.CohortManifestDigest = promotionSHA256(taskBytes)
	report.OrderSeed = report.CohortManifestDigest
	rows := ompContextPromotionCanaryRowsV1(report)
	aggregate, err := ReduceOMPContextCanaryPairsV1(rows)
	if err != nil {
		return report, nil, fmt.Errorf("reduce OMP context promotion cohort: %w", err)
	}
	report.Gates = expectedOMPContextPromotionGatesV1(aggregate.MedianReductionBasisPoints)
	report.EvidenceID, err = computeOMPContextPromotionEvidenceIDV1(report)
	if err != nil {
		return report, nil, fmt.Errorf("compute OMP context promotion evidence ID: %w", err)
	}
	body, err := json.Marshal(report)
	if err != nil {
		return report, nil, fmt.Errorf("marshal OMP context promotion report: %w", err)
	}
	verified, err := decodeOMPContextPromotionReportV1(body)
	if err != nil {
		return report, nil, err
	}
	return verified, body, nil
}

func OMPContextPromotionReportCanaryRowsV1(
	report OMPContextPromotionReportV1,
) ([]OMPContextCanaryRowV1, error) {
	if err := validateOMPContextPromotionReportMetadataV1(report); err != nil {
		return nil, err
	}
	if err := validateOMPContextPromotionCohortV1(report); err != nil {
		return nil, err
	}
	return append([]OMPContextCanaryRowV1(nil), ompContextPromotionCanaryRowsV1(report)...), nil
}

func OMPContextPromotionProviderAuthorityDigestV1(report OMPContextPromotionReportV1) string {
	return ompContextPromotionProviderAuthorityDigestV1(report)
}

func OMPContextPromotionSessionAuthorityDigestV1(report OMPContextPromotionReportV1) string {
	return ompContextPromotionSessionAuthorityDigestV1(report)
}

func OMPContextPromotionReportPathV1(root string) string {
	return filepath.Join(root, filepath.FromSlash(ompContextPromotionReportRelativePathV2))
}

func WriteOMPContextPromotionReportV1(root string, body []byte) error {
	if _, err := decodeOMPContextPromotionReportV1(body); err != nil {
		return err
	}
	evidencePath, err := prepareOMPContextEvidenceStorePathV1(root, true)
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(evidencePath), ompContextPromotionReportFileNameV2)
	if err := companionmanifest.WriteAtomic(path, body); err != nil {
		return fmt.Errorf("write OMP context promotion report: %w", err)
	}
	return verifyOMPContextEvidenceFileV1(path)
}
