package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func testStaticPolicyB64(t *testing.T) string {
	t.Helper()
	policy := promptlayer.OMPContextPromotionStaticPolicyV3{
		SchemaVersion:       promptlayer.OMPContextPromotionRuntimeSchemaV3,
		CandidateRepository: "Insajin/autopus-adk", SourceCommit: strings.Repeat("c", 40),
		SourceTree: strings.Repeat("d", 40), Target: "darwin-arm64",
		ReleaseLineageKeyID: "release-key", ReleaseLineageHandoff: "v1", MinimumRollbackFloor: 5093,
	}
	body, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(body)
}

func TestRun_RejectsUnpinnedEvidenceBeforeSignatureVerification(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	attestation := filepath.Join(dir, "attestation.json")
	if err := os.WriteFile(report, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attestation, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{
		"--mode", "historical", "--report", report, "--attestation", attestation,
		"--report-sha256", strings.Repeat("a", 64),
		"--attestation-sha256", strings.Repeat("b", 64),
		"--candidate-repository", "Insajin/autopus-adk",
		"--candidate-revision", strings.Repeat("c", 40),
		"--candidate-tree", strings.Repeat("d", 40),
		"--candidate-artifact-sha256", strings.Repeat("e", 64),
		"--static-policy-b64", testStaticPolicyB64(t),
	})
	if err == nil || !strings.Contains(err.Error(), "artifact digest differs") {
		t.Fatalf("unpinned evidence result = %v", err)
	}
}

func TestParseOptions_RejectsMalformedCoordinates(t *testing.T) {
	_, err := parseOptions([]string{
		"--mode", "active", "--report", "report", "--attestation", "attestation",
		"--report-sha256", strings.Repeat("a", 64),
		"--attestation-sha256", strings.Repeat("b", 64),
		"--candidate-repository", "Insajin/autopus-adk",
		"--candidate-revision", strings.Repeat("C", 40),
		"--candidate-tree", strings.Repeat("d", 40),
		"--candidate-artifact-sha256", strings.Repeat("e", 64),
		"--static-policy-b64", testStaticPolicyB64(t),
	})
	if err == nil || !strings.Contains(err.Error(), "candidate revision") {
		t.Fatalf("malformed coordinate result = %v", err)
	}
}

func TestDecodeStaticPolicy_RejectsUnknownAndNonCanonicalInput(t *testing.T) {
	valid := testStaticPolicyB64(t)
	policy, err := decodeStaticPolicy(valid)
	if err != nil || policy.Target != "darwin-arm64" {
		t.Fatalf("valid policy result = %#v, %v", policy, err)
	}
	body, err := base64.RawURLEncoding.DecodeString(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, encoded := range []string{
		base64.RawURLEncoding.EncodeToString(append([]byte(" "), body...)),
		base64.RawURLEncoding.EncodeToString(append(body[:len(body)-1], []byte(`,"unknown":true}`)...)),
		valid + "=",
	} {
		if _, err := decodeStaticPolicy(encoded); err == nil {
			t.Fatal("static policy wire drift was accepted")
		}
	}
}

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
			AutoVersion: "v0.50.97", AutoBinarySHA256: candidateArtifactSHA,
			OMPVersion: "omp/17.2.7", OMPExecutableSHA256: promotionTestHash("omp"),
			ExecutionClass: "external-live", ProductionPathEquivalent: true,
			RuntimeKind:                  "omp-pipeline-managed-rpc",
			PipelineImplementationDigest: promotionTestHash("pipeline-implementation"),
		},
		SessionFacts: promptlayer.OMPContextPromotionSessionFactsV1{
			FullProcessStarts: 1, OptimizedProcessStarts: 1, FullSessionCount: 1,
			OptimizedSessionCount: 1, MaxConcurrency: 1,
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
			inputTokens, compactionRequests := int64(10000), 0
			if variant == "B" {
				inputTokens = 7500
				if sessionSequences[variant] > 1 {
					compactionRequests = 1
				}
			}
			began := startedAt.Add(time.Duration(sequence*2) * time.Second)
			report.Observations = append(report.Observations, promptlayer.OMPContextPromotionObservationV1{
				Sequence: sequence, TaskIDDigest: taskDigest, Variant: variant,
				SessionReceiptDigest: promotionTestHash("session-" + variant),
				SessionSequence:      sessionSequences[variant], ProcessReused: sessionSequences[variant] > 1,
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

func TestPromotionVerifierWrappers_RejectMalformedTamperedAndStaleEvidence(t *testing.T) {
	fixture := newHistoricalVerifierFixture(t)
	reportBytes, expected := promotionVerifierInputs(t, fixture)

	for name, verifier := range map[string]promotionArtifactVerifier{
		"active":     verifyActivePromotionArtifact,
		"historical": verifyHistoricalPromotionArtifact,
	} {
		t.Run(name+" malformed attestation", func(t *testing.T) {
			if valid, err := verifier(reportBytes, []byte("{}"), expected); err == nil || valid {
				t.Fatalf("%s wrapper accepted malformed attestation: valid=%v error=%v", name, valid, err)
			}
		})
		t.Run(name+" intrinsically stale attestation", func(t *testing.T) {
			valid, err := verifier(reportBytes, stalePromotionAttestation(t, reportBytes), expected)
			if valid || !errors.Is(err, promptlayer.ErrOMPContextPromotionStale) {
				t.Fatalf("%s wrapper stale result: valid=%v error=%v", name, valid, err)
			}
		})
	}

	attestation := stalePromotionAttestation(t, reportBytes)
	tamperedReport := append(append([]byte(nil), reportBytes...), ' ')
	if valid, err := verifyHistoricalPromotionArtifact(tamperedReport, attestation, expected); err == nil || valid {
		t.Fatalf("historical wrapper accepted modified report: valid=%v error=%v", valid, err)
	}
}

func TestRun_HistoricalAcceptsRawCandidateDigestAgainstCanonicalReport(t *testing.T) {
	fixture := newHistoricalVerifierFixture(t)
	canonicalDigest := "sha256:" + strings.Repeat("e", 64)
	historicalVerifier := func(
		reportBytes, _ []byte,
		expected promptlayer.OMPContextPromotionExpectationV2,
	) (bool, error) {
		var report promptlayer.OMPContextPromotionReportV1
		if err := json.Unmarshal(reportBytes, &report); err != nil {
			t.Fatalf("decode report in verifier: %v", err)
		}
		if report.Candidate.ArtifactSHA256 != canonicalDigest ||
			expected.Candidate.ArtifactSHA256 != canonicalDigest ||
			expected.AutoBinarySHA256 != canonicalDigest {
			t.Fatalf("candidate digest boundary = report %q, candidate %q, runtime %q",
				report.Candidate.ArtifactSHA256, expected.Candidate.ArtifactSHA256,
				expected.AutoBinarySHA256)
		}
		return true, nil
	}
	if err := runWithVerifiers(fixture.arguments(strings.Repeat("e", 64)),
		verifyActivePromotionArtifact, historicalVerifier); err != nil {
		t.Fatalf("historical verification failed: %v", err)
	}
}

func TestRun_HistoricalRejectsMismatchedCandidateDigest(t *testing.T) {
	fixture := newHistoricalVerifierFixture(t)
	verifierCalled := false
	err := runWithVerifiers(
		fixture.arguments(strings.Repeat("f", 64)),
		verifyActivePromotionArtifact,
		func([]byte, []byte, promptlayer.OMPContextPromotionExpectationV2) (bool, error) {
			verifierCalled = true
			return true, nil
		},
	)
	if verifierCalled {
		t.Fatal("historical verifier called for mismatched candidate digest")
	}
	if err == nil || err.Error() != "candidate coordinates differ from exact release source" {
		t.Fatalf("mismatched candidate digest result = %v", err)
	}
}

func TestParseOptions_RejectsNonCanonicalCandidateArtifactDigest(t *testing.T) {
	for _, test := range []struct {
		name, value string
	}{
		{"prefixed", "sha256:" + strings.Repeat("e", 64)},
		{"uppercase", strings.Repeat("E", 64)},
		{"wrong length", strings.Repeat("e", 63)},
		{"non-hex", strings.Repeat("e", 63) + "g"},
		{"whitespace", " " + strings.Repeat("e", 63)},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseOptions([]string{
				"--mode", "historical", "--report", "report", "--attestation", "attestation",
				"--report-sha256", strings.Repeat("a", 64),
				"--attestation-sha256", strings.Repeat("b", 64),
				"--candidate-repository", "Insajin/autopus-adk",
				"--candidate-revision", strings.Repeat("c", 40),
				"--candidate-tree", strings.Repeat("d", 40),
				"--candidate-artifact-sha256", test.value,
				"--static-policy-b64", testStaticPolicyB64(t),
			})
			if err == nil || err.Error() != "candidate artifact digest is malformed" {
				t.Fatalf("non-canonical candidate digest result = %v", err)
			}
		})
	}
}
