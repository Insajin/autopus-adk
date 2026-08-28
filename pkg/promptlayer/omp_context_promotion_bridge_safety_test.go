package promptlayer

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

func TestOMPContextPromotionBridge_K3TrustAloneDoesNotGrantWithoutPolicyOrEvidence(t *testing.T) {
	keys := committedOMPContextPromotionPublicKeysV2()
	if len(keys[OMPContextPromotionKeyID2026Q3K3]) != ed25519.PublicKeySize {
		t.Fatal("committed K3 trust pin is unavailable")
	}

	lineageCalled := false
	grant, err := verifyOMPContextPromotionRuntimeV3WithLineageAt(
		OMPContextPromotionRuntimeBundleV3{},
		OMPContextPromotionStaticPolicyV3{},
		OMPContextPromotionCurrentRuntimeV3{},
		time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC),
		func(
			[]byte,
			[]byte,
			companionmanifest.OMPContextReleaseLineagePolicy,
		) (time.Time, error) {
			lineageCalled = true
			return time.Time{}, nil
		},
	)
	if err == nil {
		t.Fatal("K3 trust pin created an active grant without static policy and evidence")
	}
	if grant.Valid() || grant.ReportDigest() != "" || grant.EvidenceID() != "" {
		t.Fatalf("failed bridge verification returned active authority: %#v", grant)
	}
	if lineageCalled {
		t.Fatal("missing bridge policy and evidence reached release-lineage verification")
	}
}
