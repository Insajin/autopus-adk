package promptlayer

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestVerifyOMPContextPromotionArtifactV2_AcceptsSignedExternalCohort(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)

	verified, err := verifyOMPContextPromotionArtifactV2WithTrust(
		fixture.reportBytes, fixture.attestationBytes, fixture.now, fixture.expectation,
		map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil,
	)
	if err != nil {
		t.Fatalf("verify signed promotion: %v", err)
	}
	if !verified.validAt(fixture.now) || verified.ReportDigest() != fixture.reportDigest ||
		verified.EvidenceID() != fixture.report.EvidenceID || verified.ExpiresAt() != fixture.now.Add(time.Hour) {
		t.Fatalf("unexpected verified grant: %#v", verified)
	}
	if verified.ProducerCoordinates() != fixture.report.Producer ||
		verified.CandidateCoordinates() != fixture.report.Candidate ||
		verified.PolicyDigest() != fixture.report.Policy.PolicyDigest {
		t.Fatalf("verified coordinates do not match report")
	}
	if bytes.Contains(fixture.reportBytes, []byte("credential_locator")) ||
		bytes.Contains(fixture.reportBytes, []byte("credential_value")) ||
		bytes.Contains(fixture.reportBytes, []byte("assistant_text")) ||
		bytes.Contains(fixture.reportBytes, []byte("prompt_body")) {
		t.Fatal("promotion evidence contains credential or body-bearing fields")
	}
}

func TestVerifyOMPContextPromotionArtifactV2_RejectsUnsignedDowngradeAndTamper(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	tests := []struct {
		name        string
		report      []byte
		attestation []byte
		expectation OMPContextPromotionExpectationV2
	}{
		{name: "unsigned", report: fixture.reportBytes, attestation: nil, expectation: fixture.expectation},
		{name: "v1 downgrade", report: fixture.reportBytes, attestation: replaceJSONField(t, fixture.attestationBytes, "schema_version", OMPContextPromotionAttestationSchemaV1), expectation: fixture.expectation},
		{name: "report tamper", report: append(append([]byte(nil), fixture.reportBytes...), ' '), attestation: fixture.attestationBytes, expectation: fixture.expectation},
		{name: "wrong domain signature", report: fixture.reportBytes, attestation: fixture.attestationWithDomain(t, "wrong-domain\x00"), expectation: fixture.expectation},
		{name: "unknown key", report: fixture.reportBytes, attestation: replaceJSONField(t, fixture.attestationBytes, "key_id", "unknown-key"), expectation: fixture.expectation},
		{name: "cross lane", report: fixture.reportBytes, attestation: replaceJSONField(t, fixture.attestationBytes, "trust_lane", "other-lane"), expectation: fixture.expectation},
		{name: "wrong algorithm", report: fixture.reportBytes, attestation: replaceJSONField(t, fixture.attestationBytes, "algorithm", "rsa"), expectation: fixture.expectation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := verifyOMPContextPromotionArtifactV2WithTrust(test.report, test.attestation, fixture.now, test.expectation,
				map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil); err == nil {
				t.Fatal("invalid artifact was accepted")
			}
		})
	}
}

func TestVerifyOMPContextPromotionArtifactV2_RejectsInvalidCohortFacts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*OMPContextPromotionReportV1)
	}{
		{name: "evidence id tamper", mutate: func(r *OMPContextPromotionReportV1) { r.EvidenceID = promotionSHA256([]byte("other")) }},
		{name: "challenge tamper", mutate: func(r *OMPContextPromotionReportV1) { r.ChallengeDigest = promotionSHA256([]byte("other")) }},
		{name: "nineteen tasks", mutate: func(r *OMPContextPromotionReportV1) { r.Tasks = r.Tasks[:19]; r.Observations = r.Observations[:38] }},
		{name: "twenty one tasks", mutate: func(r *OMPContextPromotionReportV1) {
			r.Tasks = append(r.Tasks, r.Tasks[0])
			r.Observations = append(r.Observations, r.Observations[0], r.Observations[1])
		}},
		{name: "duplicate observation", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[1].ObservationDigest = r.Observations[0].ObservationDigest
		}},
		{name: "unpaired task", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[1].TaskIDDigest = r.Tasks[1].TaskIDDigest }},
		{name: "abba imbalance", mutate: func(r *OMPContextPromotionReportV1) { r.Tasks[1].Order = "AB" }},
		{name: "row swap", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[0], r.Observations[1] = r.Observations[1], r.Observations[0]
		}},
		{name: "overlapping calls", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[1].StartedAt = r.Observations[0].StartedAt }},
		{name: "zero token", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].InputTokens = 0 }},
		{name: "retry", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].RetryCount = 1 }},
		{name: "synthetic", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].ExecutionMode = "synthetic" }},
		{name: "loopback", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].EndpointClass = "loopback" }},
		{name: "provider authority drift", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[1].ProviderAuthorityDigest = promotionSHA256([]byte("other-provider-authority"))
		}},
		{name: "shadow", mutate: func(r *OMPContextPromotionReportV1) { r.Runtime.ExecutionClass = "shadow" }},
		{name: "non-production path", mutate: func(r *OMPContextPromotionReportV1) { r.Runtime.ProductionPathEquivalent = false }},
		{name: "wrong pipeline runtime", mutate: func(r *OMPContextPromotionReportV1) { r.Runtime.RuntimeKind = "omp-managed-rpc" }},
		{name: "pipeline digest missing", mutate: func(r *OMPContextPromotionReportV1) { r.Runtime.PipelineImplementationDigest = "" }},
		{name: "extra full process", mutate: func(r *OMPContextPromotionReportV1) { r.SessionFacts.FullProcessStarts = 2 }},
		{name: "session receipt drift", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[2].SessionReceiptDigest = promotionSHA256([]byte("other-session"))
		}},
		{name: "cross-variant session receipt", mutate: func(r *OMPContextPromotionReportV1) {
			fullReceipt := r.Observations[0].SessionReceiptDigest
			for index := range r.Observations {
				if r.Observations[index].Variant == "B" {
					r.Observations[index].SessionReceiptDigest = fullReceipt
				}
			}
		}},
		{name: "session sequence skip", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].SessionSequence = 2 }},
		{name: "first process reused", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].ProcessReused = true }},
		{name: "negative setup request", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[0].SetupProviderRequests = -1
			r.Observations[0].TotalProviderRequests = 0
		}},
		{name: "missing primary request", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[0].PrimaryProviderRequests = 0
			r.Observations[0].TotalProviderRequests = 0
		}},
		{name: "full compaction request", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[0].CompactionProviderRequests = 1
			r.Observations[0].TotalProviderRequests++
		}},
		{name: "optimized compaction request missing", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[1].CompactionProviderRequests = 0
			r.Observations[1].TotalProviderRequests--
		}},
		{name: "optimized pre ACK missing", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[1].PreCompactionACKs = 0
		}},
		{name: "optimized canonical readmission missing", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[1].CanonicalReadmissions = 0
		}},
		{name: "optimized ephemeral readmission missing", mutate: func(r *OMPContextPromotionReportV1) {
			r.Observations[1].EphemeralReadmissions = 0
		}},
		{name: "provider request total mismatch", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].TotalProviderRequests++ }},
		{name: "integrity failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].IntegrityPassed = false }},
		{name: "security failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].SecurityPassed = false }},
		{name: "quality regression", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[1].QualityScore = 1 }},
		{name: "fallback failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].FallbackVerified = false }},
		{name: "rollback failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].RollbackVerified = false }},
		{name: "cleanup failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].CleanupVerified = false }},
		{name: "gate tamper", mutate: func(r *OMPContextPromotionReportV1) { r.Gates[0].ObservedValue = "verdict-only" }},
		{name: "verdict only", mutate: func(r *OMPContextPromotionReportV1) { r.Observations = nil }},
		{name: "uniform low quality", mutate: func(r *OMPContextPromotionReportV1) {
			for index := range r.Observations {
				r.Observations[index].QualityScore = 1
			}
		}},
		{name: "task manifest substitution", mutate: func(r *OMPContextPromotionReportV1) {
			replacement := promotionSHA256([]byte("replacement-task"))
			r.Tasks[0].TaskIDDigest = replacement
			r.Observations[0].TaskIDDigest = replacement
			r.Observations[1].TaskIDDigest = replacement
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newOMPContextPromotionV2Fixture(t)
			test.mutate(&fixture.report)
			fixture.resign(t)
			if _, err := verifyOMPContextPromotionArtifactV2WithTrust(fixture.reportBytes, fixture.attestationBytes, fixture.now,
				fixture.expectation, map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil); err == nil {
				t.Fatal("invalid cohort was accepted")
			}
		})
	}
}

func TestVerifyOMPContextPromotionArtifactV2_AcceptsObservedSetupProviderRequestCounts(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	fixture.report.Observations[0].SetupProviderRequests = 1
	fixture.report.Observations[0].TotalProviderRequests = 2
	fixture.resign(t)

	if _, err := verifyOMPContextPromotionArtifactV2WithTrust(
		fixture.reportBytes, fixture.attestationBytes, fixture.now, fixture.expectation,
		map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil,
	); err != nil {
		t.Fatalf("observed setup request count rejected: %v", err)
	}
}
