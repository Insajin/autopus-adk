package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type historicalVerifierFixture struct {
	reportPath, attestationPath, reportSHA, attestationSHA, staticPolicyB64 string
}

func (f historicalVerifierFixture) arguments(candidateArtifactSHA string) []string {
	return []string{
		"--mode", "historical", "--report", f.reportPath, "--attestation", f.attestationPath,
		"--report-sha256", f.reportSHA, "--attestation-sha256", f.attestationSHA,
		"--candidate-repository", "Insajin/autopus-adk",
		"--candidate-revision", strings.Repeat("c", 40),
		"--candidate-tree", strings.Repeat("d", 40),
		"--candidate-artifact-sha256", candidateArtifactSHA,
		"--static-policy-b64", f.staticPolicyB64,
		"--expected-signing-key-id", promptlayer.OMPContextPromotionKeyID2026Q3K1,
	}
}

func promotionTestHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newHistoricalVerifierFixture(t *testing.T) historicalVerifierFixture {
	t.Helper()
	candidateArtifactSHA := "sha256:" + strings.Repeat("e", 64)
	report := promptlayer.OMPContextPromotionReportV1{
		ChallengeDigest: promotionTestHash("challenge"),
		Producer: promptlayer.OMPContextPromotionProducerV1{
			Repository:  "Insajin/autopus-harness-bench",
			WorkflowRef: "refs/heads/main@0123456789abcdef0123456789abcdef01234567",
			RunID:       "123456", RunAttempt: 1,
		},
		Candidate: promptlayer.OMPContextPromotionCandidateV1{
			Repository: "Insajin/autopus-adk", Revision: strings.Repeat("c", 40),
			TreeSHA: strings.Repeat("d", 40), ArtifactSHA256: candidateArtifactSHA,
		},
		Policy: promptlayer.OMPContextPromotionPolicyReportV1{
			PolicyID: "omp-context-active-v1", PolicyDigest: promotionTestHash("policy"),
			HistoryMode: "active", MemoryMode: "off", MinPairCount: 20, MinReductionBasisPoints: 2000,
		},
		Runtime: promptlayer.OMPContextPromotionRuntimeV1{
			AutoVersion: "v0.50.109", AutoBinarySHA256: candidateArtifactSHA,
			OMPVersion: "omp/17.2.7", OMPExecutableSHA256: promotionTestHash("omp"),
			ExecutionClass: "external-live", ProductionPathEquivalent: true,
			RuntimeKind:                  "omp-pipeline-managed-rpc",
			PipelineImplementationDigest: promotionTestHash("pipeline-implementation"),
		},
		SessionFacts: promptlayer.OMPContextPromotionSessionFactsV1{
			FullProcessStarts: 2, OptimizedProcessStarts: 2, FullSessionCount: 2,
			OptimizedSessionCount: 2, MaxConcurrency: 1,
		},
		Provider: "openai", ModelScopeDigest: promotionTestHash("models"),
		OraclePolicyDigest: promotionTestHash("oracle"),
	}
	startedAt := time.Date(2026, 8, 4, 1, 0, 0, 0, time.UTC)
	sessionSequences := map[string]int{"A": 0, "B": 0}
	for index := range 20 {
		taskDigest := promotionTestHash(fmt.Sprintf("task-%02d", index))
		order, variants := "AB", []string{"A", "B"}
		if index%2 == 1 {
			order, variants = "BA", []string{"B", "A"}
		}
		report.Tasks = append(report.Tasks, promptlayer.OMPContextPromotionTaskV1{
			TaskIDDigest: taskDigest, Order: order,
		})
		for _, variant := range variants {
			sequence := len(report.Observations) + 1
			sessionSequences[variant]++
			variantSequence := sessionSequences[variant]
			sessionSegment := (variantSequence - 1) / 10
			sessionSequence := (variantSequence-1)%10 + 1
			inputTokens, compactionRequests := int64(10000), 0
			if variant == "B" && sessionSequence > 1 {
				inputTokens, compactionRequests = 7500, 1
			}
			began := startedAt.Add(time.Duration(sequence*2) * time.Second)
			report.Observations = append(report.Observations, promptlayer.OMPContextPromotionObservationV1{
				Sequence: sequence, TaskIDDigest: taskDigest, Variant: variant,
				SessionReceiptDigest: promotionTestHash(fmt.Sprintf("session-%s-%d", variant, sessionSegment)),
				SessionSequence:      sessionSequence, ProcessReused: sessionSequence > 1,
				Provider: report.Provider, ModelScopeDigest: report.ModelScopeDigest,
				EndpointClass: "external-provider", Transport: "provider-api", CredentialMode: "locator-only",
				ProviderAuthorityDigest: promotionTestHash("provider-authority"), ExecutionMode: "external-live",
				StartedAt: began.Format(time.RFC3339Nano), CompletedAt: began.Add(time.Second).Format(time.RFC3339Nano),
				InputTokens: inputTokens, OutputTokens: 100, TotalTokens: inputTokens + 100,
				CompactionProviderRequests: compactionRequests, PrimaryProviderRequests: 1,
				PreCompactionACKs: compactionRequests, PostCompactionACKs: compactionRequests,
				CanonicalReadmissions: compactionRequests, EphemeralReadmissions: compactionRequests,
				TotalProviderRequests: 1 + compactionRequests,
				ObservationDigest:     promotionTestHash(fmt.Sprintf("observation-%02d", sequence)),
				UsageDigest:           promotionTestHash(fmt.Sprintf("usage-%02d", sequence)),
				IntegrityPassed:       true, SecurityPassed: true, QualityScore: 10000,
				FallbackVerified: true, RollbackVerified: true, CleanupVerified: true, MaxConcurrency: 1,
			})
		}
	}
	builtReport, reportBytes, err := promptlayer.BuildOMPContextPromotionReportV1(report)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	attestationBytes := []byte("{}")
	policy := promptlayer.OMPContextPromotionStaticPolicyV3{
		SchemaVersion:       promptlayer.OMPContextPromotionRuntimeSchemaV3,
		ProducerRepository:  builtReport.Producer.Repository,
		ProducerWorkflowRef: builtReport.Producer.WorkflowRef,
		CandidateRepository: builtReport.Candidate.Repository,
		SourceCommit:        builtReport.Candidate.Revision, SourceTree: builtReport.Candidate.TreeSHA,
		Target: "darwin-arm64", AutoVersion: builtReport.Runtime.AutoVersion,
		PolicyID: builtReport.Policy.PolicyID, PolicyDigest: builtReport.Policy.PolicyDigest,
		OMPVersion:                   builtReport.Runtime.OMPVersion,
		OMPExecutableSHA256:          builtReport.Runtime.OMPExecutableSHA256,
		PipelineImplementationDigest: builtReport.Runtime.PipelineImplementationDigest,
		Provider:                     builtReport.Provider, ModelScopeDigest: builtReport.ModelScopeDigest,
		CohortManifestDigest: builtReport.CohortManifestDigest, OrderSeed: builtReport.OrderSeed,
		OraclePolicyDigest:  builtReport.OraclePolicyDigest,
		ReleaseLineageKeyID: "release-key", ReleaseLineageHandoff: "v1", MinimumRollbackFloor: 5093,
	}
	policyBytes, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal static policy: %v", err)
	}
	dir := t.TempDir()
	reportPath, attestationPath := filepath.Join(dir, "report.json"), filepath.Join(dir, "attestation.json")
	if err := os.WriteFile(reportPath, reportBytes, 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := os.WriteFile(attestationPath, attestationBytes, 0o600); err != nil {
		t.Fatalf("write attestation: %v", err)
	}
	return historicalVerifierFixture{
		reportPath: reportPath, attestationPath: attestationPath,
		reportSHA: digest(reportBytes), attestationSHA: digest(attestationBytes),
		staticPolicyB64: base64.RawURLEncoding.EncodeToString(policyBytes),
	}
}

func promotionVerifierInputs(
	t *testing.T,
	fixture historicalVerifierFixture,
) ([]byte, promptlayer.OMPContextPromotionExpectationV2) {
	t.Helper()
	reportBytes, err := os.ReadFile(fixture.reportPath)
	if err != nil {
		t.Fatalf("read report fixture: %v", err)
	}
	policy, err := decodeStaticPolicy(fixture.staticPolicyB64)
	if err != nil {
		t.Fatalf("decode fixture policy: %v", err)
	}
	return reportBytes, expectationFromStaticPolicy(
		policy,
		"sha256:"+strings.Repeat("e", 64),
		promptlayer.OMPContextPromotionKeyID2026Q3K1,
	)
}

func stalePromotionAttestation(t *testing.T, reportBytes []byte) []byte {
	t.Helper()
	issuedAt := time.Date(2026, 8, 4, 3, 0, 0, 0, time.UTC)
	body, err := json.Marshal(promptlayer.OMPContextPromotionAttestationV2{
		SchemaVersion:   promptlayer.OMPContextPromotionAttestationSchemaV2,
		KeyID:           promptlayer.OMPContextPromotionKeyID2026Q3K1,
		Algorithm:       "ed25519",
		ReportSHA256:    "sha256:" + digest(reportBytes),
		IssuedAt:        issuedAt.Format(time.RFC3339Nano),
		NotBefore:       issuedAt.Format(time.RFC3339Nano),
		ExpiresAt:       issuedAt.Add(25 * time.Hour).Format(time.RFC3339Nano),
		TrustLane:       promptlayer.OMPContextPromotionTrustLaneV2,
		SignatureBase64: base64.StdEncoding.EncodeToString(make([]byte, 64)),
	})
	if err != nil {
		t.Fatalf("marshal stale attestation: %v", err)
	}
	return body
}
