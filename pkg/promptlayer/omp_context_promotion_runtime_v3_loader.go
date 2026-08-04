package promptlayer

import (
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

const (
	ompContextPromotionReleaseLineageFileNameV3      = "release-lineage-v1.json"
	ompContextPromotionReleaseLineageSignatureV3     = "release-lineage-v1.sig"
	ompContextPromotionReleaseKeyBundleDirectoryV3   = "adk-companion-public-key-receipt.bundle"
	ompContextPromotionReleaseLineageMaxBytesV3      = 16 * 1024
	ompContextPromotionReleaseLineageSignatureSizeV3 = 64
)

// LoadVerifiedOMPContextPromotionRuntimeV3 reads one private local bundle and
// verifies it against static policy and current process coordinates.
func LoadVerifiedOMPContextPromotionRuntimeV3(root string,
	expected OMPContextPromotionStaticPolicyV3,
	current OMPContextPromotionCurrentRuntimeV3,
) (VerifiedOMPContextPromotion, error) {
	return loadVerifiedOMPContextPromotionRuntimeV3At(root, expected, current, time.Now().UTC())
}

func loadVerifiedOMPContextPromotionRuntimeV3At(root string,
	expected OMPContextPromotionStaticPolicyV3,
	current OMPContextPromotionCurrentRuntimeV3,
	now time.Time,
) (VerifiedOMPContextPromotion, error) {
	bound, err := bindOMPContextPromotionCurrentExecutableV3(current)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	current = bound
	report, attestation, lineage, lineageSignature, releaseKeyBundle, err :=
		readOMPContextPromotionRuntimeBundleV3(root)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	trusted, err := companionmanifest.VerifyConfiguredPublicKeyReceiptBundle(
		releaseKeyBundle,
		companionmanifest.PublicKeyReceiptPolicy{
			Now: now, ExpectedKeyID: expected.ReleaseLineageKeyID,
			ExpectedHandoff:      expected.ReleaseLineageHandoff,
			MinimumRollbackFloor: expected.MinimumRollbackFloor,
		},
	)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	return verifyOMPContextPromotionRuntimeV3At(OMPContextPromotionRuntimeBundleV3{
		ReportBytes: report, AttestationBytes: attestation,
		ReleaseLineageBytes: lineage, ReleaseLineageSignature: lineageSignature,
		ReleaseKey: trusted,
	}, expected, current, now)
}
