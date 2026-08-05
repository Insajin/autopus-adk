package promptlayer

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

func ompContextPromotionRuntimeV3Fixture(t *testing.T) (
	*ompContextPromotionV2Fixture,
	OMPContextPromotionRuntimeBundleV3,
	OMPContextPromotionStaticPolicyV3,
	OMPContextPromotionCurrentRuntimeV3,
) {
	t.Helper()
	fixture := newOMPContextPromotionV2Fixture(t)
	fixture.report.Runtime.AutoBinarySHA256 = fixture.report.Candidate.ArtifactSHA256
	var err error
	fixture.report.EvidenceID, err = computeOMPContextPromotionEvidenceIDV1(fixture.report)
	if err != nil {
		t.Fatal(err)
	}
	fixture.resign(t)
	policy := OMPContextPromotionStaticPolicyV3{
		SchemaVersion:       OMPContextPromotionRuntimeSchemaV3,
		ProducerRepository:  fixture.report.Producer.Repository,
		ProducerWorkflowRef: fixture.report.Producer.WorkflowRef,
		CandidateRepository: fixture.report.Candidate.Repository,
		SourceCommit:        fixture.report.Candidate.Revision, SourceTree: fixture.report.Candidate.TreeSHA,
		Target: "darwin-arm64", AutoVersion: fixture.report.Runtime.AutoVersion,
		PolicyID: fixture.report.Policy.PolicyID, PolicyDigest: fixture.report.Policy.PolicyDigest,
		OMPVersion:                   fixture.report.Runtime.OMPVersion,
		OMPExecutableSHA256:          fixture.report.Runtime.OMPExecutableSHA256,
		PipelineImplementationDigest: fixture.report.Runtime.PipelineImplementationDigest,
		Provider:                     fixture.report.Provider, ModelScopeDigest: fixture.report.ModelScopeDigest,
		CohortManifestDigest: fixture.report.CohortManifestDigest, OrderSeed: fixture.report.OrderSeed,
		OraclePolicyDigest:    fixture.report.OraclePolicyDigest,
		ReleaseLineageKeyID:   "release-lineage-2026-q3-k1",
		ReleaseLineageHandoff: "v1", MinimumRollbackFloor: 5093,
	}
	current := OMPContextPromotionCurrentRuntimeV3{
		ExecutableSHA256: promotionSHA256([]byte("distributed")),
		SourceCommit:     policy.SourceCommit, SourceTree: policy.SourceTree, Target: policy.Target,
		AutoVersion: policy.AutoVersion, OMPVersion: policy.OMPVersion,
		OMPExecutableSHA256:          policy.OMPExecutableSHA256,
		PipelineImplementationDigest: policy.PipelineImplementationDigest,
		ProviderAuthorityDigest:      ompContextPromotionProviderAuthorityDigestV1(fixture.report),
	}
	return fixture, OMPContextPromotionRuntimeBundleV3{
		ReportBytes: fixture.reportBytes, AttestationBytes: fixture.attestationBytes,
		ReleaseLineageBytes: []byte("opaque-lineage"), ReleaseLineageSignature: []byte("opaque-signature"),
	}, policy, current
}

func withOMPContextPromotionV3FixtureKey(t *testing.T, key ed25519.PublicKey) {
	t.Helper()
	original := ompContextPromotionPublicKeysV2
	originalRevoked := ompContextPromotionRevokedKeysV2
	ompContextPromotionPublicKeysV2 = map[string]ed25519.PublicKey{
		OMPContextPromotionKeyID2026Q3K1: append(ed25519.PublicKey(nil), key...),
	}
	ompContextPromotionRevokedKeysV2 = map[string]bool{}
	t.Cleanup(func() {
		ompContextPromotionPublicKeysV2 = original
		ompContextPromotionRevokedKeysV2 = originalRevoked
	})
}

func TestVerifyOMPContextPromotionRuntimeV3_BindsSignedUToCurrentD(t *testing.T) {
	fixture, bundle, policy, current := ompContextPromotionRuntimeV3Fixture(t)
	withOMPContextPromotionV3FixtureKey(t, fixture.publicKey)

	called := false
	verified, err := verifyOMPContextPromotionRuntimeV3WithLineageAt(
		bundle, policy, current, fixture.now,
		func(lineage, signature []byte, got companionmanifest.OMPContextReleaseLineagePolicy) (time.Time, error) {
			called = true
			if string(lineage) != "opaque-lineage" || string(signature) != "opaque-signature" {
				t.Fatal("lineage bytes changed before verification")
			}
			if got.ExpectedUpstreamSHA256 != fixture.report.Candidate.ArtifactSHA256 ||
				got.ExpectedExecutableSHA256 != current.ExecutableSHA256 ||
				got.ExpectedSourceCommit != current.SourceCommit || got.ExpectedSourceTree != current.SourceTree ||
				got.ExpectedHandoff != policy.ReleaseLineageHandoff ||
				got.MinimumRollbackFloor != policy.MinimumRollbackFloor {
				t.Fatalf("lineage policy = %#v", got)
			}
			return fixture.now.Add(time.Hour), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("release lineage verifier was not called")
	}
	if verified.runtime.RuntimeKind != fixture.report.Runtime.RuntimeKind {
		t.Fatal("verified runtime capability is empty")
	}
	if !verified.ExpiresAt().Equal(fixture.now.Add(time.Hour)) {
		t.Fatalf("verified expiry = %s", verified.ExpiresAt())
	}
}

func TestVerifyOMPContextPromotionRuntimeV3_StaticAndSignedCoordinatesFailClosed(t *testing.T) {
	fixture, bundle, policy, current := ompContextPromotionRuntimeV3Fixture(t)
	withOMPContextPromotionV3FixtureKey(t, fixture.publicKey)
	acceptLineage := func(_ []byte, _ []byte, lineage companionmanifest.OMPContextReleaseLineagePolicy) (time.Time, error) {
		if lineage.ExpectedExecutableSHA256 != current.ExecutableSHA256 {
			return time.Time{}, errors.New("distributed executable does not match signed lineage")
		}
		return fixture.now.Add(time.Hour), nil
	}

	for name, mutate := range map[string]func(*OMPContextPromotionStaticPolicyV3, *OMPContextPromotionCurrentRuntimeV3){
		"producer": func(policy *OMPContextPromotionStaticPolicyV3, _ *OMPContextPromotionCurrentRuntimeV3) {
			policy.ProducerRepository = "Insajin/other"
		},
		"source": func(policy *OMPContextPromotionStaticPolicyV3, _ *OMPContextPromotionCurrentRuntimeV3) {
			policy.SourceCommit = strings.Repeat("9", 40)
		},
		"distributed executable": func(_ *OMPContextPromotionStaticPolicyV3, current *OMPContextPromotionCurrentRuntimeV3) {
			current.ExecutableSHA256 = promotionSHA256([]byte("other-distributed"))
		},
		"implementation": func(policy *OMPContextPromotionStaticPolicyV3, current *OMPContextPromotionCurrentRuntimeV3) {
			policy.PipelineImplementationDigest = promotionSHA256([]byte("other-implementation"))
			current.PipelineImplementationDigest = policy.PipelineImplementationDigest
		},
		"provider authority": func(_ *OMPContextPromotionStaticPolicyV3, current *OMPContextPromotionCurrentRuntimeV3) {
			current.ProviderAuthorityDigest = promotionSHA256([]byte("other-provider-authority"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			mutatedPolicy, mutatedCurrent := policy, current
			mutate(&mutatedPolicy, &mutatedCurrent)
			if _, err := verifyOMPContextPromotionRuntimeV3WithLineageAt(
				bundle, mutatedPolicy, mutatedCurrent, fixture.now, acceptLineage,
			); err == nil {
				t.Fatal("coordinate drift was accepted")
			}
		})
	}

	t.Run("report U mismatch", func(t *testing.T) {
		mutated := *fixture
		mutated.report = fixture.report
		mutated.report.Runtime.AutoBinarySHA256 = promotionSHA256([]byte("different-upstream"))
		var err error
		mutated.report.EvidenceID, err = computeOMPContextPromotionEvidenceIDV1(mutated.report)
		if err != nil {
			t.Fatal(err)
		}
		mutated.resign(t)
		badBundle := bundle
		badBundle.ReportBytes, badBundle.AttestationBytes = mutated.reportBytes, mutated.attestationBytes
		if _, err := verifyOMPContextPromotionRuntimeV3WithLineageAt(
			badBundle, policy, current, fixture.now, acceptLineage,
		); err == nil {
			t.Fatal("report U mismatch was accepted")
		}
	})
}

func TestVerifyOMPContextPromotionRuntimeV3_UsesEarliestAuthorityExpiry(t *testing.T) {
	fixture, bundle, policy, current := ompContextPromotionRuntimeV3Fixture(t)
	withOMPContextPromotionV3FixtureKey(t, fixture.publicKey)
	lineageExpiry := fixture.now.Add(30 * time.Minute)

	verified, err := verifyOMPContextPromotionRuntimeV3WithLineageAt(
		bundle, policy, current, fixture.now,
		func([]byte, []byte, companionmanifest.OMPContextReleaseLineagePolicy) (time.Time, error) {
			return lineageExpiry, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !verified.ExpiresAt().Equal(lineageExpiry) {
		t.Fatalf("verified expiry = %s, want %s", verified.ExpiresAt(), lineageExpiry)
	}

	if _, err := verifyOMPContextPromotionRuntimeV3WithLineageAt(
		bundle, policy, current, fixture.now,
		func([]byte, []byte, companionmanifest.OMPContextReleaseLineagePolicy) (time.Time, error) {
			return fixture.now, nil
		},
	); err == nil {
		t.Fatal("expired release lineage authority was accepted")
	}
}
