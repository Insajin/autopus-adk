package promptlayer

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

func TestVerifyOMPContextPromotionRuntimeV3_NonK3PolicyRejectsBeforeDownstreamWork(t *testing.T) {
	fixture, bundle, policy, current := ompContextPromotionRuntimeV3Fixture(t)
	withOMPContextPromotionV3FixtureTrust(t, fixture.publicKey,
		OMPContextPromotionKeyID2026Q3K1, OMPContextPromotionKeyID2026Q3K2, OMPContextPromotionKeyID2026Q3K3)
	bundle.AttestationBytes = []byte("not-json")
	current.ProviderAuthorityDigest = ""

	tests := []struct {
		name, keyID string
	}{
		{name: "missing"},
		{name: "K1", keyID: OMPContextPromotionKeyID2026Q3K1},
		{name: "K2", keyID: OMPContextPromotionKeyID2026Q3K2},
		{name: "unknown", keyID: "omp-context-promotion-unknown"},
		{name: "noncanonical", keyID: " " + OMPContextPromotionKeyID2026Q3K3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := policy
			candidate.PromotionSigningKeyID = test.keyID
			lineageCalls := 0
			_, err := verifyOMPContextPromotionRuntimeV3WithLineageAt(
				bundle, candidate, current, fixture.now,
				func([]byte, []byte, companionmanifest.OMPContextReleaseLineagePolicy) (time.Time, error) {
					lineageCalls++
					return fixture.now.Add(time.Hour), nil
				},
			)
			const wantError = "OMP context promotion active static policy is invalid"
			if err == nil || err.Error() != wantError || lineageCalls != 0 {
				t.Fatalf("non-K3 policy %q reached downstream work: error=%v lineage_calls=%d",
					test.keyID, err, lineageCalls)
			}
		})
	}
}

func TestVerifyOMPContextPromotionRuntimeV3_PolicyAttestationDriftRejectsBeforeSignatureProviderAndLineage(t *testing.T) {
	fixture, bundle, policy, current := ompContextPromotionRuntimeV3Fixture(t)
	withOMPContextPromotionV3FixtureTrust(t, fixture.publicKey,
		OMPContextPromotionKeyID2026Q3K1, OMPContextPromotionKeyID2026Q3K2, OMPContextPromotionKeyID2026Q3K3)
	attestation := fixture.attestation
	attestation.KeyID = OMPContextPromotionKeyID2026Q3K1
	attestation.SignatureBase64 = "AA=="
	var err error
	bundle.AttestationBytes, err = json.Marshal(attestation)
	if err != nil {
		t.Fatal(err)
	}
	current.ProviderAuthorityDigest = ""
	lineageCalls := 0
	_, err = verifyOMPContextPromotionRuntimeV3WithLineageAt(
		bundle, policy, current, fixture.now,
		func([]byte, []byte, companionmanifest.OMPContextReleaseLineagePolicy) (time.Time, error) {
			lineageCalls++
			return fixture.now.Add(time.Hour), nil
		},
	)
	if !errors.Is(err, ErrOMPContextPromotionMismatch) || lineageCalls != 0 {
		t.Fatalf("signer drift was not the first failure: error=%v lineage_calls=%d", err, lineageCalls)
	}
}

func TestLoadVerifiedOMPContextPromotionRuntimeV3_SignerDriftPrecedesReleaseTrustWork(t *testing.T) {
	requireSecureOMPContextPromotionArtifactPlatformV2(t)
	fixture, _, policy, current := ompContextPromotionRuntimeV3Fixture(t)
	current.ExecutableSHA256 = ""
	attestation := fixture.attestation
	attestation.KeyID = OMPContextPromotionKeyID2026Q3K1
	attestation.SignatureBase64 = "AA=="
	attestationBytes, err := json.Marshal(attestation)
	if err != nil {
		t.Fatal(err)
	}
	root := prepareOMPContextPromotionArtifactRootV2(t)
	writeOMPContextPromotionArtifactPairV2(t, root, fixture.reportBytes, attestationBytes)
	artifactRoot := filepath.Join(root, ".autopus", "runtime", "omp-context")
	writePrivateOMPContextPromotionArtifactV2(
		t, filepath.Join(artifactRoot, ompContextPromotionReleaseLineageFileNameV3), []byte("{}"),
	)
	writePrivateOMPContextPromotionArtifactV2(
		t, filepath.Join(artifactRoot, ompContextPromotionReleaseLineageSignatureV3),
		bytes.Repeat([]byte{0x5a}, ompContextPromotionReleaseLineageSignatureSizeV3),
	)
	if _, err := loadVerifiedOMPContextPromotionRuntimeV3At(root, policy, current, fixture.now); !errors.Is(err, ErrOMPContextPromotionMismatch) {
		t.Fatalf("signer drift did not precede release trust work: %v", err)
	}
}
