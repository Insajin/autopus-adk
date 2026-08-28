package promptlayer

import "time"

const (
	OMPContextPromotionAttestationSchemaV2 = "autopus.omp_context_promotion_attestation.v2"
	OMPContextPromotionKeyID2026Q3K1       = "omp-context-promotion-2026-q3-k1"
	OMPContextPromotionKeyID2026Q3K2       = "omp-context-promotion-2026-q3-k2"
	OMPContextPromotionKeyID2026Q3K3       = "omp-context-promotion-2026-q3-k3"
	OMPContextPromotionTrustLaneV2         = "autopus-main-omp-context-promotion"
	ompContextPromotionAttestationDomainV2 = "autopus.omp-context.promotion-attestation.v2\x00"
)

type OMPContextPromotionAttestationV2 struct {
	SchemaVersion   string `json:"schema_version"`
	KeyID           string `json:"key_id"`
	Algorithm       string `json:"algorithm"`
	ReportSHA256    string `json:"report_sha256"`
	IssuedAt        string `json:"issued_at"`
	NotBefore       string `json:"not_before"`
	ExpiresAt       string `json:"expires_at"`
	TrustLane       string `json:"trust_lane"`
	SignatureBase64 string `json:"signature_base64"`
}

type ompContextPromotionAttestationStatementV2 struct {
	SchemaVersion string `json:"schema_version"`
	KeyID         string `json:"key_id"`
	Algorithm     string `json:"algorithm"`
	ReportSHA256  string `json:"report_sha256"`
	IssuedAt      string `json:"issued_at"`
	NotBefore     string `json:"not_before"`
	ExpiresAt     string `json:"expires_at"`
	TrustLane     string `json:"trust_lane"`
}

type OMPContextPromotionExpectationV2 struct {
	ProducerRepository           string
	ProducerWorkflowRef          string
	SigningKeyID                 string
	Candidate                    OMPContextPromotionCandidateV1
	PolicyID                     string
	PolicyDigest                 string
	AutoVersion                  string
	AutoBinarySHA256             string
	OMPVersion                   string
	OMPExecutableSHA256          string
	PipelineImplementationDigest string
	Provider                     string
	ModelScopeDigest             string
	CohortManifestDigest         string
	OrderSeed                    string
	OraclePolicyDigest           string
}

// VerifiedOMPContextPromotion can only be populated after strict signature,
// provenance, freshness, and cohort verification succeeds.
type VerifiedOMPContextPromotion struct {
	reportDigest            string
	evidenceID              string
	expiresAt               time.Time
	producer                OMPContextPromotionProducerV1
	candidate               OMPContextPromotionCandidateV1
	policyDigest            string
	runtime                 OMPContextPromotionRuntimeV1
	provider                string
	modelScopeDigest        string
	providerAuthorityDigest string
	sessionAuthorityDigest  string
	canaryRows              []OMPContextCanaryRowV1
}

// VerifiedOMPContextPromotionHistoricalProof proves an immutable artifact was
// validly signed without granting current active-history authority.
type VerifiedOMPContextPromotionHistoricalProof struct {
	reportDigest string
	evidenceID   string
	issuedAt     time.Time
	expiresAt    time.Time
	producer     OMPContextPromotionProducerV1
	candidate    OMPContextPromotionCandidateV1
}

func (v VerifiedOMPContextPromotion) Valid() bool {
	return v.validAt(time.Now().UTC())
}

func (v VerifiedOMPContextPromotion) validAt(now time.Time) bool {
	return validOMPContextMemoryHashV1(v.reportDigest) && validOMPContextMemoryHashV1(v.evidenceID) &&
		validOMPContextMemoryHashV1(v.providerAuthorityDigest) &&
		validOMPContextMemoryHashV1(v.sessionAuthorityDigest) && len(v.canaryRows) == 40 &&
		!v.expiresAt.IsZero() && !now.IsZero() && now.Before(v.expiresAt)
}
func (v VerifiedOMPContextPromotion) ReportDigest() string { return v.reportDigest }
func (v VerifiedOMPContextPromotion) EvidenceID() string   { return v.evidenceID }
func (v VerifiedOMPContextPromotion) ExpiresAt() time.Time { return v.expiresAt }
func (v VerifiedOMPContextPromotion) ProducerCoordinates() OMPContextPromotionProducerV1 {
	return v.producer
}
func (v VerifiedOMPContextPromotion) CandidateCoordinates() OMPContextPromotionCandidateV1 {
	return v.candidate
}
func (v VerifiedOMPContextPromotion) PolicyDigest() string { return v.policyDigest }
func (v VerifiedOMPContextPromotion) RuntimeCoordinates() OMPContextPromotionRuntimeV1 {
	return v.runtime
}
func (v VerifiedOMPContextPromotion) ProviderScope() (string, string) {
	return v.provider, v.modelScopeDigest
}
func (v VerifiedOMPContextPromotion) ProviderAuthorityDigest() string {
	return v.providerAuthorityDigest
}
func (v VerifiedOMPContextPromotion) SessionAuthorityDigest() string {
	return v.sessionAuthorityDigest
}
func (v VerifiedOMPContextPromotion) CanaryRows() []OMPContextCanaryRowV1 {
	return append([]OMPContextCanaryRowV1(nil), v.canaryRows...)
}
func verifiedOMPContextPromotionFromReportV2(
	report OMPContextPromotionReportV1,
	reportDigest string,
	expiresAt time.Time,
) VerifiedOMPContextPromotion {
	return VerifiedOMPContextPromotion{
		reportDigest: reportDigest, evidenceID: report.EvidenceID, expiresAt: expiresAt,
		producer: report.Producer, candidate: report.Candidate, policyDigest: report.Policy.PolicyDigest,
		runtime: report.Runtime, provider: report.Provider, modelScopeDigest: report.ModelScopeDigest,
		providerAuthorityDigest: ompContextPromotionProviderAuthorityDigestV1(report),
		sessionAuthorityDigest:  ompContextPromotionSessionAuthorityDigestV1(report),
		canaryRows:              ompContextPromotionCanaryRowsV1(report),
	}
}

func (v VerifiedOMPContextPromotionHistoricalProof) Valid() bool {
	return v.reportDigest != "" && v.evidenceID != "" && !v.issuedAt.IsZero() && !v.expiresAt.IsZero()
}
func (v VerifiedOMPContextPromotionHistoricalProof) ReportDigest() string { return v.reportDigest }
func (v VerifiedOMPContextPromotionHistoricalProof) EvidenceID() string   { return v.evidenceID }
func (v VerifiedOMPContextPromotionHistoricalProof) IssuedAt() time.Time  { return v.issuedAt }
func (v VerifiedOMPContextPromotionHistoricalProof) ExpiresAt() time.Time { return v.expiresAt }
func (v VerifiedOMPContextPromotionHistoricalProof) ProducerCoordinates() OMPContextPromotionProducerV1 {
	return v.producer
}
func (v VerifiedOMPContextPromotionHistoricalProof) CandidateCoordinates() OMPContextPromotionCandidateV1 {
	return v.candidate
}
