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
		Runtime: OMPContextPromotionRuntimeV1{
			AutoVersion: "v0.50.98", AutoBinarySHA256: hash("auto"), OMPVersion: "omp/17.2.7",
			OMPExecutableSHA256: hash("omp"), ExecutionClass: "external-live", ProductionPathEquivalent: true,
			RuntimeKind: "omp-pipeline-managed-rpc", PipelineImplementationDigest: hash("pipeline-implementation"),
		},
		SessionFacts: OMPContextPromotionSessionFactsV1{
			FullProcessStarts: 1, OptimizedProcessStarts: 1, FullSessionCount: 1,
			OptimizedSessionCount: 1, MaxConcurrency: 1, CrossSessionContamination: 0,
		},
		Provider: "openai", ModelScopeDigest: hash("models"), CohortManifestDigest: hash("cohort"), OrderSeed: hash("seed"), OraclePolicyDigest: hash("oracle"),
	}
	report.EvidenceID, _ = computeOMPContextPromotionEvidenceIDV1(report)
	start := time.Date(2026, 8, 4, 1, 0, 0, 0, time.UTC)
	sessionSequences := map[string]int{"A": 0, "B": 0}
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
			sessionSequences[variant]++
			input, compactionRequests := int64(10000), 0
			if variant == "B" && sessionSequences[variant] > 1 {
				input, compactionRequests = 7500, 1
			}
			began := start.Add(time.Duration(sequence*2) * time.Second)
			report.Observations = append(report.Observations, OMPContextPromotionObservationV1{
				Sequence: sequence, TaskIDDigest: taskDigest, Variant: variant,
				SessionReceiptDigest: hash("session-" + variant), SessionSequence: sessionSequences[variant],
				ProcessReused: sessionSequences[variant] > 1, Provider: report.Provider,
				ModelScopeDigest: report.ModelScopeDigest, EndpointClass: "external-provider", Transport: "provider-api",
				CredentialMode: "locator-only", ProviderAuthorityDigest: hash("provider-authority"),
				ExecutionMode: "external-live", StartedAt: began.Format(time.RFC3339Nano), CompletedAt: began.Add(time.Second).Format(time.RFC3339Nano),
				InputTokens: input, OutputTokens: 100, TotalTokens: input + 100, ObservationDigest: hash(fmt.Sprintf("observation-%02d", sequence)),
				SetupProviderRequests: 0, CompactionProviderRequests: compactionRequests,
				PrimaryProviderRequests: 1, TotalProviderRequests: 1 + compactionRequests,
				PreCompactionACKs: compactionRequests, PostCompactionACKs: compactionRequests,
				CanonicalReadmissions: compactionRequests, EphemeralReadmissions: compactionRequests,
				UsageDigest: hash(fmt.Sprintf("usage-%02d", sequence)), IntegrityPassed: true, SecurityPassed: true, QualityScore: 10000,
				FallbackVerified: true, RollbackVerified: true, CleanupVerified: true, RetryCount: 0, MaxConcurrency: 1,
			})
		}
	}
	taskBytes, _ := json.Marshal(report.Tasks)
	report.CohortManifestDigest = promotionSHA256(taskBytes)
	report.OrderSeed = report.CohortManifestDigest
	report.Gates = expectedOMPContextPromotionGatesV1(2500, 19)
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
		SigningKeyID: OMPContextPromotionKeyID2026Q3K1,
		Candidate:    report.Candidate, PolicyID: report.Policy.PolicyID, PolicyDigest: report.Policy.PolicyDigest,
		AutoVersion: report.Runtime.AutoVersion, AutoBinarySHA256: report.Runtime.AutoBinarySHA256,
		OMPVersion: report.Runtime.OMPVersion, OMPExecutableSHA256: report.Runtime.OMPExecutableSHA256,
		PipelineImplementationDigest: report.Runtime.PipelineImplementationDigest,
		Provider:                     report.Provider, ModelScopeDigest: report.ModelScopeDigest,
		CohortManifestDigest: report.CohortManifestDigest, OrderSeed: report.OrderSeed,
		OraclePolicyDigest: report.OraclePolicyDigest,
	}
}
