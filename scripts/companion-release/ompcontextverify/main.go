package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

const (
	maxReportBytes       = 512 * 1024
	maxAttestationBytes  = 16 * 1024
	maxStaticPolicyBytes = 16 * 1024
)

var (
	lowerHex            = regexp.MustCompile(`^[0-9a-f]+$`)
	promotionSigningKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type options struct {
	mode, reportPath, attestationPath      string
	reportSHA, attestationSHA              string
	candidateRepository, candidateRevision string
	candidateTree, candidateArtifactSHA    string
	staticPolicyB64, expectedSigningKeyID  string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "OMP context evidence verifier: %v\n", err)
		os.Exit(1)
	}
}

type promotionArtifactVerifier func(
	reportBytes, attestationBytes []byte,
	expected promptlayer.OMPContextPromotionExpectationV2,
) (bool, error)

func run(arguments []string) error {
	return runWithVerifiers(arguments, verifyActivePromotionArtifact, verifyHistoricalPromotionArtifact)
}

func verifyActivePromotionArtifact(
	reportBytes, attestationBytes []byte,
	expected promptlayer.OMPContextPromotionExpectationV2,
) (bool, error) {
	verified, err := promptlayer.VerifyOMPContextPromotionArtifactV2(reportBytes, attestationBytes, expected)
	return verified.Valid(), err
}

func verifyHistoricalPromotionArtifact(
	reportBytes, attestationBytes []byte,
	expected promptlayer.OMPContextPromotionExpectationV2,
) (bool, error) {
	verified, err := promptlayer.VerifyOMPContextPromotionHistoricalArtifactV2(
		reportBytes, attestationBytes, expected)
	return verified.Valid(), err
}

func runWithVerifiers(
	arguments []string,
	activeVerifier, historicalVerifier promotionArtifactVerifier,
) error {
	opts, err := parseOptions(arguments)
	if err != nil {
		return err
	}
	candidateArtifactSHA := "sha256:" + opts.candidateArtifactSHA
	reportBytes, err := readArtifact(opts.reportPath, maxReportBytes)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	attestationBytes, err := readArtifact(opts.attestationPath, maxAttestationBytes)
	if err != nil {
		return fmt.Errorf("attestation: %w", err)
	}
	if digest(reportBytes) != opts.reportSHA || digest(attestationBytes) != opts.attestationSHA {
		return errors.New("artifact digest differs from exact release pin")
	}
	report, err := decodeReport(reportBytes)
	if err != nil {
		return err
	}
	policy, err := decodeStaticPolicy(opts.staticPolicyB64)
	if err != nil {
		return err
	}
	if policy.Target != "darwin-arm64" || policy.CandidateRepository != opts.candidateRepository ||
		policy.SourceCommit != opts.candidateRevision || policy.SourceTree != opts.candidateTree ||
		report.Candidate.Repository != opts.candidateRepository ||
		report.Candidate.Revision != opts.candidateRevision ||
		report.Candidate.TreeSHA != opts.candidateTree ||
		report.Candidate.ArtifactSHA256 != candidateArtifactSHA {
		return errors.New("candidate coordinates differ from exact release source")
	}
	if report.Candidate.ArtifactSHA256 != report.Runtime.AutoBinarySHA256 {
		return errors.New("candidate artifact digest differs from runtime binary digest")
	}
	expected := expectationFromStaticPolicy(policy, candidateArtifactSHA, opts.expectedSigningKeyID)
	switch opts.mode {
	case "active":
		valid, verifyErr := activeVerifier(reportBytes, attestationBytes, expected)
		if verifyErr != nil || !valid {
			return fmt.Errorf("active evidence verification failed: %w", verifyErr)
		}
	case "historical":
		valid, verifyErr := historicalVerifier(reportBytes, attestationBytes, expected)
		if verifyErr != nil || !valid {
			return fmt.Errorf("historical evidence verification failed: %w", verifyErr)
		}
	default:
		return errors.New("mode must be active or historical")
	}
	fmt.Printf("OMP context evidence: %s proof verified\n", opts.mode)
	return nil
}

func parseOptions(arguments []string) (options, error) {
	var result options
	set := flag.NewFlagSet("ompcontextverify", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	set.StringVar(&result.mode, "mode", "", "verification mode")
	set.StringVar(&result.reportPath, "report", "", "report path")
	set.StringVar(&result.attestationPath, "attestation", "", "attestation path")
	set.StringVar(&result.reportSHA, "report-sha256", "", "report digest")
	set.StringVar(&result.attestationSHA, "attestation-sha256", "", "attestation digest")
	set.StringVar(&result.candidateRepository, "candidate-repository", "", "candidate repository")
	set.StringVar(&result.candidateRevision, "candidate-revision", "", "candidate revision")
	set.StringVar(&result.candidateTree, "candidate-tree", "", "candidate tree")
	set.StringVar(&result.candidateArtifactSHA, "candidate-artifact-sha256", "", "candidate binary digest")
	set.StringVar(&result.staticPolicyB64, "static-policy-b64", "", "canonical raw-base64url static policy")
	set.StringVar(&result.expectedSigningKeyID, "expected-signing-key-id", "", "exact promotion signing key identity")
	if err := set.Parse(arguments); err != nil || set.NArg() != 0 {
		return result, errors.New("invalid arguments")
	}
	if result.mode == "" || result.reportPath == "" || result.attestationPath == "" ||
		result.candidateRepository == "" || result.staticPolicyB64 == "" ||
		result.expectedSigningKeyID == "" {
		return result, errors.New("required argument is missing")
	}
	for _, value := range []struct {
		name string
		body string
		size int
	}{
		{"report digest", result.reportSHA, 64}, {"attestation digest", result.attestationSHA, 64},
		{"candidate revision", result.candidateRevision, 40}, {"candidate tree", result.candidateTree, 40},
		{"candidate artifact digest", result.candidateArtifactSHA, 64},
	} {
		if len(value.body) != value.size || !lowerHex.MatchString(value.body) {
			return result, fmt.Errorf("%s is malformed", value.name)
		}
	}
	if !promotionSigningKey.MatchString(result.expectedSigningKeyID) {
		return result, errors.New("expected signing key identity is malformed")
	}
	return result, nil
}

func readArtifact(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm()&0o022 != 0 || info.Size() <= 0 || info.Size() > limit {
		return nil, errors.New("artifact file is unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, errors.New("artifact read failed")
	}
	return body, nil
}

func decodeReport(body []byte) (promptlayer.OMPContextPromotionReportV1, error) {
	var report promptlayer.OMPContextPromotionReportV1
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return report, fmt.Errorf("decode report: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return report, errors.New("report has trailing content")
	}
	return report, nil
}

func decodeStaticPolicy(encoded string) (promptlayer.OMPContextPromotionStaticPolicyV3, error) {
	var policy promptlayer.OMPContextPromotionStaticPolicyV3
	if len(encoded) == 0 || len(encoded) > 21846 {
		return policy, errors.New("static policy encoding is invalid")
	}
	body, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(body) == 0 || len(body) > maxStaticPolicyBytes {
		return policy, errors.New("static policy encoding is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return policy, errors.New("static policy JSON is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return policy, errors.New("static policy has trailing content")
	}
	canonical, err := json.Marshal(policy)
	if err != nil || !bytes.Equal(canonical, body) {
		return policy, errors.New("static policy is non-canonical")
	}
	if policy.SchemaVersion != promptlayer.OMPContextPromotionRuntimeSchemaV3 ||
		policy.ReleaseLineageKeyID == "" || policy.ReleaseLineageHandoff == "" ||
		policy.MinimumRollbackFloor == 0 {
		return policy, errors.New("static policy authority is invalid")
	}
	return policy, nil
}

func expectationFromStaticPolicy(
	policy promptlayer.OMPContextPromotionStaticPolicyV3,
	candidateArtifactSHA, expectedSigningKeyID string,
) promptlayer.OMPContextPromotionExpectationV2 {
	return promptlayer.OMPContextPromotionExpectationV2{
		ProducerRepository: policy.ProducerRepository, ProducerWorkflowRef: policy.ProducerWorkflowRef,
		SigningKeyID: expectedSigningKeyID,
		Candidate:    ompContextPromotionCandidateFromStaticPolicy(policy, candidateArtifactSHA),
		PolicyID:     policy.PolicyID, PolicyDigest: policy.PolicyDigest,
		AutoVersion: policy.AutoVersion, AutoBinarySHA256: candidateArtifactSHA,
		OMPVersion: policy.OMPVersion, OMPExecutableSHA256: policy.OMPExecutableSHA256,
		PipelineImplementationDigest: policy.PipelineImplementationDigest,
		Provider:                     policy.Provider, ModelScopeDigest: policy.ModelScopeDigest,
		CohortManifestDigest: policy.CohortManifestDigest, OrderSeed: policy.OrderSeed,
		OraclePolicyDigest: policy.OraclePolicyDigest,
	}
}

func ompContextPromotionCandidateFromStaticPolicy(
	policy promptlayer.OMPContextPromotionStaticPolicyV3,
	artifactSHA string,
) promptlayer.OMPContextPromotionCandidateV1 {
	return promptlayer.OMPContextPromotionCandidateV1{
		Repository: policy.CandidateRepository, Revision: policy.SourceCommit,
		TreeSHA: policy.SourceTree, ArtifactSHA256: artifactSHA,
	}
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
