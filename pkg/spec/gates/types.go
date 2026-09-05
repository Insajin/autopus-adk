// Package gates computes deterministic gate applicability for a SPEC change
// set and decides when previously recorded gate evidence may be reused.
package gates

// Applicability is the outcome of a gate decision.
type Applicability string

const (
	Required      Applicability = "required"
	Reusable      Applicability = "reusable"
	NotApplicable Applicability = "not_applicable"
	Blocked       Applicability = "blocked"
)

// ChangeClass is the deterministic classification of a change set.
type ChangeClass string

const (
	ClassDocOnly        ChangeClass = "doc_only"
	ClassUIOnly         ChangeClass = "ui_only"
	ClassSecurityOrData ChangeClass = "security_or_data"
	ClassMultiDomain    ChangeClass = "multi_domain"
	ClassGeneral        ChangeClass = "general"
)

// GateID identifies one gate of the closed catalog.
type GateID string

const (
	GateRiskFirstProbe      GateID = "risk_first_probe"
	GateBuild               GateID = "build"
	GateUnitTests           GateID = "unit_tests"
	GateIntegration         GateID = "integration"
	GateSecurity            GateID = "security"
	GateValidation          GateID = "validation"
	GateDataLoss            GateID = "data_loss"
	GateDeterministicOracle GateID = "deterministic_oracle"
	GateAccessibility       GateID = "accessibility"
	GateUXVerification      GateID = "ux_verification"
	GateAnnotation          GateID = "annotation"
	GateProviderReview      GateID = "provider_review"
	GateDocSync             GateID = "doc_sync"
)

// EvidenceStatus is the recorded outcome of a gate run.
type EvidenceStatus string

const (
	StatusPass    EvidenceStatus = "pass"
	StatusFail    EvidenceStatus = "fail"
	StatusPartial EvidenceStatus = "partial"
)

// Receipt schema identifiers.
const (
	ApplicabilitySchema = "autopus.gate-applicability.v1"
	EvidenceSchema      = "autopus.gate-evidence.v1"
)

// CatalogEntry describes one gate of the closed catalog.
type CatalogEntry struct {
	ID        GateID
	Mandatory bool // mandatory safety gates are never not_applicable
}

// Catalog lists every gate in receipt order.
var Catalog = []CatalogEntry{
	{ID: GateRiskFirstProbe},
	{ID: GateBuild},
	{ID: GateUnitTests},
	{ID: GateIntegration},
	{ID: GateSecurity, Mandatory: true},
	{ID: GateValidation, Mandatory: true},
	{ID: GateDataLoss, Mandatory: true},
	{ID: GateDeterministicOracle, Mandatory: true},
	{ID: GateAccessibility},
	{ID: GateUXVerification},
	{ID: GateAnnotation},
	{ID: GateProviderReview},
	{ID: GateDocSync},
}

// LookupGate returns the catalog entry for id.
func LookupGate(id GateID) (CatalogEntry, bool) {
	for _, entry := range Catalog {
		if entry.ID == id {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}

// ValidStatus reports whether s is a recognised evidence status.
func ValidStatus(s EvidenceStatus) bool {
	return s == StatusPass || s == StatusFail || s == StatusPartial
}

// ReusedEvidence points at the evidence receipt a reusable decision relies on.
type ReusedEvidence struct {
	Path       string         `json:"path"`
	ObservedAt string         `json:"observed_at"`
	Status     EvidenceStatus `json:"status"`
}

// GateDecision is one gate's applicability outcome.
type GateDecision struct {
	Gate               GateID          `json:"gate"`
	Applicability      Applicability   `json:"applicability"`
	Reason             string          `json:"reason"`
	Mandatory          bool            `json:"mandatory"`
	InputClosureSHA256 string          `json:"input_closure_sha256,omitempty"`
	ReusedEvidence     *ReusedEvidence `json:"reused_evidence,omitempty"`
}

// ApplicabilityReceipt is written to {SPEC_DIR}/gate-applicability.json.
type ApplicabilityReceipt struct {
	Schema       string         `json:"schema"`
	SpecID       string         `json:"spec_id"`
	ChangeClass  ChangeClass    `json:"change_class"`
	ChangedPaths []string       `json:"changed_paths"`
	GeneratedAt  string         `json:"generated_at"`
	Decisions    []GateDecision `json:"decisions"`
}

// Decision returns the decision for gate id, if present.
func (r ApplicabilityReceipt) Decision(id GateID) (GateDecision, bool) {
	for _, decision := range r.Decisions {
		if decision.Gate == id {
			return decision, true
		}
	}
	return GateDecision{}, false
}

// InputEntry is one hashed file of an evidence input closure.
type InputEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// EvidenceReceipt is written to {SPEC_DIR}/gates/evidence-<gate>.json.
// Receipts are body-free: they carry hashes and counts, never file contents.
type EvidenceReceipt struct {
	Schema             string         `json:"schema"`
	SpecID             string         `json:"spec_id"`
	Gate               GateID         `json:"gate"`
	Status             EvidenceStatus `json:"status"`
	Complete           bool           `json:"complete"`
	InputClosureSHA256 string         `json:"input_closure_sha256"`
	Inputs             []InputEntry   `json:"inputs"`
	DynamicDeps        []InputEntry   `json:"dynamic_deps"`
	InputGlobs         []string       `json:"input_globs"`
	Command            string         `json:"command,omitempty"`
	ObservedAt         string         `json:"observed_at"`
}
