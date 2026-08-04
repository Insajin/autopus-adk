package promptlayer

import "time"

const (
	ompContextPromotionReportRelativePathV2      = ".autopus/runtime/omp-context/promotion-report-v1.json"
	ompContextPromotionAttestationRelativePathV2 = ".autopus/runtime/omp-context/promotion-attestation-v2.json"
	ompContextPromotionReportFileNameV2          = "promotion-report-v1.json"
	ompContextPromotionAttestationFileNameV2     = "promotion-attestation-v2.json"
)

type ompContextPromotionArtifactReadHookV2 func(string) error

func LoadVerifiedOMPContextPromotionV2(root string,
	expected OMPContextPromotionExpectationV2) (VerifiedOMPContextPromotion, error) {
	return loadVerifiedOMPContextPromotionV2At(root, time.Now().UTC(), expected)
}

func loadVerifiedOMPContextPromotionV2At(root string, now time.Time,
	expected OMPContextPromotionExpectationV2) (VerifiedOMPContextPromotion, error) {
	reportBytes, attestationBytes, err := readOMPContextPromotionArtifactPairV2(root)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	return verifyOMPContextPromotionArtifactV2At(reportBytes, attestationBytes, now, expected)
}

func readOMPContextPromotionArtifactPairV2(root string) ([]byte, []byte, error) {
	return readOMPContextPromotionArtifactPairV2WithHook(root, nil)
}

func runOMPContextPromotionArtifactReadHookV2(hook ompContextPromotionArtifactReadHookV2, stage string) error {
	if hook == nil {
		return nil
	}
	return hook(stage)
}
