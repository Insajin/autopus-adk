package promptlayer

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	if !verified.Valid() || verified.ReportDigest() != fixture.reportDigest ||
		verified.EvidenceID() != fixture.report.EvidenceID || verified.ExpiresAt() != fixture.now.Add(time.Hour) {
		t.Fatalf("unexpected verified grant: %#v", verified)
	}
	if verified.ProducerCoordinates() != fixture.report.Producer ||
		verified.CandidateCoordinates() != fixture.report.Candidate ||
		verified.PolicyDigest() != fixture.report.Policy.PolicyDigest {
		t.Fatalf("verified coordinates do not match report")
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
		{name: "shadow", mutate: func(r *OMPContextPromotionReportV1) { r.Runtime.ExecutionClass = "shadow" }},
		{name: "integrity failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].IntegrityPassed = false }},
		{name: "security failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].SecurityPassed = false }},
		{name: "quality regression", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[1].QualityScore = 1 }},
		{name: "fallback failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].FallbackVerified = false }},
		{name: "rollback failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].RollbackVerified = false }},
		{name: "cleanup failure", mutate: func(r *OMPContextPromotionReportV1) { r.Observations[0].CleanupVerified = false }},
		{name: "gate tamper", mutate: func(r *OMPContextPromotionReportV1) { r.Gates[0].ObservedValue = "verdict-only" }},
		{name: "verdict only", mutate: func(r *OMPContextPromotionReportV1) { r.Observations = nil }},
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

type ompContextPromotionV2Fixture struct {
	now              time.Time
	privateKey       ed25519.PrivateKey
	publicKey        ed25519.PublicKey
	report           OMPContextPromotionReportV1
	reportBytes      []byte
	reportDigest     string
	attestation      OMPContextPromotionAttestationV2
	attestationBytes []byte
	expectation      OMPContextPromotionExpectationV2
}

func newOMPContextPromotionV2Fixture(t *testing.T) *ompContextPromotionV2Fixture {
	t.Helper()
	seed := sha256.Sum256([]byte("omp-context-promotion-verifier-test-key"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	fixture := &ompContextPromotionV2Fixture{
		now: time.Date(2026, 8, 4, 3, 0, 0, 0, time.UTC), privateKey: privateKey,
		publicKey: privateKey.Public().(ed25519.PublicKey), report: validOMPContextPromotionReportV1(),
	}
	fixture.expectation = expectationFromOMPContextPromotionReportV1(fixture.report)
	fixture.resign(t)
	return fixture
}

func (f *ompContextPromotionV2Fixture) resign(t *testing.T) {
	t.Helper()
	reportBytes, err := json.Marshal(f.report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	f.reportBytes = reportBytes
	f.reportDigest = promotionSHA256(reportBytes)
	f.attestation = OMPContextPromotionAttestationV2{
		SchemaVersion: OMPContextPromotionAttestationSchemaV2, KeyID: OMPContextPromotionKeyID2026Q3K1,
		Algorithm: "ed25519", ReportSHA256: f.reportDigest, IssuedAt: f.now.Add(-time.Minute).Format(time.RFC3339Nano),
		NotBefore: f.now.Add(-time.Minute).Format(time.RFC3339Nano), ExpiresAt: f.now.Add(time.Hour).Format(time.RFC3339Nano),
		TrustLane: OMPContextPromotionTrustLaneV2,
	}
	message, err := ompContextPromotionAttestationMessageV2(f.attestation)
	if err != nil {
		t.Fatalf("build statement: %v", err)
	}
	f.attestation.SignatureBase64 = base64.StdEncoding.EncodeToString(ed25519.Sign(f.privateKey, message))
	f.attestationBytes, err = json.Marshal(f.attestation)
	if err != nil {
		t.Fatalf("marshal attestation: %v", err)
	}
}

func (f *ompContextPromotionV2Fixture) attestationWithDomain(t *testing.T, domain string) []byte {
	t.Helper()
	statement, err := ompContextPromotionAttestationStatementBytesV2(f.attestation)
	if err != nil {
		t.Fatal(err)
	}
	mutated := f.attestation
	mutated.SignatureBase64 = base64.StdEncoding.EncodeToString(ed25519.Sign(f.privateKey, append([]byte(domain), statement...)))
	body, err := json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func validOMPContextPromotionReportV1() OMPContextPromotionReportV1 {
	hash := func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	report := OMPContextPromotionReportV1{
		SchemaVersion: OMPContextPromotionReportSchemaV1, ChallengeDigest: hash("challenge"),
		TrustLane: OMPContextPromotionTrustLaneV2,
		Producer:  OMPContextPromotionProducerV1{Repository: "Insajin/autopus-harness-bench", WorkflowRef: "refs/heads/main@0123456789abcdef0123456789abcdef01234567", RunID: "123456", RunAttempt: 1},
		Candidate: OMPContextPromotionCandidateV1{Repository: "Insajin/autopus-adk", Revision: "0123456789abcdef0123456789abcdef01234567", TreeSHA: "1123456789abcdef0123456789abcdef01234567", ArtifactSHA256: hash("artifact")},
		Policy:    OMPContextPromotionPolicyReportV1{PolicyID: "omp-context-active-v1", PolicyDigest: hash("policy"), HistoryMode: "active", MemoryMode: "off", MinPairCount: 20, MinReductionBasisPoints: 2000},
		Runtime:   OMPContextPromotionRuntimeV1{AutoVersion: "v0.50.93", AutoBinarySHA256: hash("auto"), OMPVersion: "omp/17.2.7", OMPExecutableSHA256: hash("omp"), ExecutionClass: "external-live", RuntimeKind: "omp-managed-rpc"},
		Provider:  "openai", ModelScopeDigest: hash("models"), CohortManifestDigest: hash("cohort"), OrderSeed: hash("seed"), OraclePolicyDigest: hash("oracle"),
	}
	report.EvidenceID, _ = computeOMPContextPromotionEvidenceIDV1(report)
	start := time.Date(2026, 8, 4, 1, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		taskDigest := hash(fmt.Sprintf("task-%02d", i))
		order := "AB"
		variants := []string{"A", "B"}
		if i%2 == 1 {
			order, variants = "BA", []string{"B", "A"}
		}
		report.Tasks = append(report.Tasks, OMPContextPromotionTaskV1{TaskIDDigest: taskDigest, Order: order})
		for _, variant := range variants {
			sequence := len(report.Observations) + 1
			input := int64(10000)
			if variant == "B" {
				input = 7500
			}
			began := start.Add(time.Duration(sequence*2) * time.Second)
			report.Observations = append(report.Observations, OMPContextPromotionObservationV1{
				Sequence: sequence, TaskIDDigest: taskDigest, Variant: variant, Provider: report.Provider,
				ModelScopeDigest: report.ModelScopeDigest, EndpointClass: "external-provider", Transport: "provider-api",
				CredentialMode: "locator-only", ExecutionMode: "external-live", StartedAt: began.Format(time.RFC3339Nano), CompletedAt: began.Add(time.Second).Format(time.RFC3339Nano),
				InputTokens: input, OutputTokens: 100, TotalTokens: input + 100, ObservationDigest: hash(fmt.Sprintf("observation-%02d", sequence)),
				UsageDigest: hash(fmt.Sprintf("usage-%02d", sequence)), IntegrityPassed: true, SecurityPassed: true, QualityScore: 100,
				FallbackVerified: true, RollbackVerified: true, CleanupVerified: true, RetryCount: 0, MaxConcurrency: 1,
			})
		}
	}
	report.Gates = expectedOMPContextPromotionGatesV1(2500)
	return report
}

func replaceJSONField(t *testing.T, body []byte, key string, value any) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	document[key] = value
	updated, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func expectationFromOMPContextPromotionReportV1(report OMPContextPromotionReportV1) OMPContextPromotionExpectationV2 {
	return OMPContextPromotionExpectationV2{
		ProducerRepository: report.Producer.Repository, ProducerWorkflowRef: report.Producer.WorkflowRef,
		Candidate: report.Candidate, PolicyID: report.Policy.PolicyID, PolicyDigest: report.Policy.PolicyDigest,
		AutoVersion: report.Runtime.AutoVersion, AutoBinarySHA256: report.Runtime.AutoBinarySHA256,
		OMPVersion: report.Runtime.OMPVersion, OMPExecutableSHA256: report.Runtime.OMPExecutableSHA256,
		Provider: report.Provider, ModelScopeDigest: report.ModelScopeDigest, CohortManifestDigest: report.CohortManifestDigest,
		OrderSeed: report.OrderSeed, OraclePolicyDigest: report.OraclePolicyDigest,
	}
}
