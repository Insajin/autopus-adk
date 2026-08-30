package promptlayer

import (
	"errors"
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
	if err := ValidateOMPContextPromotionActiveStaticPolicyV3(expected); err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	report, attestation, lineage, lineageSignature, releaseKeyBundle, err :=
		readOMPContextPromotionRuntimeBundleV3(root)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	bound, err := bindOMPContextPromotionCurrentExecutableV3(current)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	current = bound
	return verifyOMPContextPromotionRuntimeV3WithLineageAt(
		OMPContextPromotionRuntimeBundleV3{
			ReportBytes: report, AttestationBytes: attestation,
			ReleaseLineageBytes: lineage, ReleaseLineageSignature: lineageSignature,
		},
		expected,
		current,
		now,
		func(lineageBytes, signature []byte,
			lineagePolicy companionmanifest.OMPContextReleaseLineagePolicy,
		) (time.Time, error) {
			trusted, err := companionmanifest.VerifyConfiguredPublicKeyReceiptBundle(
				releaseKeyBundle,
				companionmanifest.PublicKeyReceiptPolicy{
					Now: lineagePolicy.Now, ExpectedKeyID: lineagePolicy.ExpectedKeyID,
					ExpectedHandoff:      lineagePolicy.ExpectedHandoff,
					MinimumRollbackFloor: lineagePolicy.MinimumRollbackFloor,
				},
			)
			if err != nil {
				return time.Time{}, err
			}
			verified, err := companionmanifest.VerifyOMPContextReleaseLineage(
				lineageBytes, signature, lineagePolicy, trusted,
			)
			if err != nil {
				return time.Time{}, err
			}
			expiresAt, ok := verified.ExpiresAt()
			if !ok {
				return time.Time{}, errors.New("OMP context release lineage expiry is unavailable")
			}
			return expiresAt, nil
		},
	)
}
