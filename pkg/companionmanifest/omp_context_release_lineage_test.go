package companionmanifest

import (
	"crypto/ed25519"
	"strings"
	"testing"
	"time"
)

func ompContextReleaseLineageFixture() OMPContextReleaseLineageV1 {
	return OMPContextReleaseLineageV1{
		SchemaVersion: OMPContextReleaseLineageSchemaV1,
		KeyID:         "rfc8032-vector-1", Algorithm: "ed25519",
		UpstreamSHA256:   "sha256:" + strings.Repeat("1", 64),
		ExecutableSHA256: "sha256:" + strings.Repeat("2", 64),
		SourceRepository: "Insajin/autopus-adk",
		SourceCommit:     strings.Repeat("3", 40), SourceTree: strings.Repeat("4", 40),
		Target: "darwin-arm64", Version: "v0.50.93",
	}
}

func ompContextReleaseLineagePolicyFixture(lineage OMPContextReleaseLineageV1) OMPContextReleaseLineagePolicy {
	return OMPContextReleaseLineagePolicy{
		Now:           time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		ExpectedKeyID: lineage.KeyID, ExpectedHandoff: "v1", MinimumRollbackFloor: 5069,
		ExpectedUpstreamSHA256:   lineage.UpstreamSHA256,
		ExpectedExecutableSHA256: lineage.ExecutableSHA256,
		ExpectedSourceRepository: lineage.SourceRepository,
		ExpectedSourceCommit:     lineage.SourceCommit, ExpectedSourceTree: lineage.SourceTree,
		ExpectedTarget: lineage.Target, ExpectedVersion: lineage.Version,
	}
}

func signedOMPContextReleaseLineageFixture(t *testing.T) (
	OMPContextReleaseLineageV1, []byte, []byte, TrustedPublicKeyReceipt,
) {
	t.Helper()
	lineage := ompContextReleaseLineageFixture()
	body, err := CanonicalOMPContextReleaseLineageBytes(lineage)
	if err != nil {
		t.Fatal(err)
	}
	message, err := OMPContextReleaseLineageSigningMessage(body)
	if err != nil {
		t.Fatal(err)
	}
	_, privateKey := receiptVectorKeyPair(t)
	return lineage, body, ed25519.Sign(privateKey, message), trustedPublicKeyReceiptFixture(t)
}

func TestVerifyOMPContextReleaseLineage_ExactSignedMappingPasses(t *testing.T) {
	lineage, body, signature, trusted := signedOMPContextReleaseLineageFixture(t)
	verified, err := VerifyOMPContextReleaseLineage(
		body, signature, ompContextReleaseLineagePolicyFixture(lineage), trusted,
	)
	if err != nil {
		t.Fatal(err)
	}
	coordinates, ok := verified.Coordinates()
	if !ok || coordinates != lineage {
		t.Fatalf("coordinates = %#v, %v", coordinates, ok)
	}
	expiresAt, ok := verified.ExpiresAt()
	receipt, receiptOK := trusted.Receipt()
	if !ok || !receiptOK || expiresAt.Format(time.RFC3339) != receipt.ExpiresAt {
		t.Fatalf("expires at = %s, %v", expiresAt, ok)
	}
}

func TestVerifyOMPContextReleaseLineage_TamperAndAuthorityDriftFailClosed(t *testing.T) {
	lineage, body, signature, trusted := signedOMPContextReleaseLineageFixture(t)

	t.Run("wrong signature domain", func(t *testing.T) {
		_, privateKey := receiptVectorKeyPair(t)
		wrong := ed25519.Sign(privateKey, append([]byte("wrong-domain\x00"), body...))
		if _, err := VerifyOMPContextReleaseLineage(
			body, wrong, ompContextReleaseLineagePolicyFixture(lineage), trusted,
		); err == nil {
			t.Fatal("wrong signature domain was accepted")
		}
	})

	for name, mutate := range map[string]func(*OMPContextReleaseLineagePolicy){
		"upstream": func(policy *OMPContextReleaseLineagePolicy) {
			policy.ExpectedUpstreamSHA256 = "sha256:" + strings.Repeat("9", 64)
		},
		"executable": func(policy *OMPContextReleaseLineagePolicy) {
			policy.ExpectedExecutableSHA256 = "sha256:" + strings.Repeat("8", 64)
		},
		"source":         func(policy *OMPContextReleaseLineagePolicy) { policy.ExpectedSourceCommit = strings.Repeat("7", 40) },
		"handoff":        func(policy *OMPContextReleaseLineagePolicy) { policy.ExpectedHandoff = "v2" },
		"rollback floor": func(policy *OMPContextReleaseLineagePolicy) { policy.MinimumRollbackFloor = 5070 },
	} {
		t.Run(name, func(t *testing.T) {
			policy := ompContextReleaseLineagePolicyFixture(lineage)
			mutate(&policy)
			if _, err := VerifyOMPContextReleaseLineage(body, signature, policy, trusted); err == nil {
				t.Fatal("authority drift was accepted")
			}
		})
	}

	t.Run("non canonical", func(t *testing.T) {
		if _, err := VerifyOMPContextReleaseLineage(
			append([]byte(" "), body...), signature, ompContextReleaseLineagePolicyFixture(lineage), trusted,
		); err == nil {
			t.Fatal("non-canonical lineage was accepted")
		}
	})

	t.Run("raw hash", func(t *testing.T) {
		invalid := lineage
		invalid.UpstreamSHA256 = strings.Repeat("1", 64)
		if _, err := CanonicalOMPContextReleaseLineageBytes(invalid); err == nil {
			t.Fatal("raw lineage hash was accepted")
		}
	})
}
