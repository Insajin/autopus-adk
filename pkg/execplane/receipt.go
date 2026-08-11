package execplane

import "time"

// IntegrityReceiptSchema versions the tier-integrity receipt, matching the
// schema-tagged receipt convention the pipeline already uses.
const IntegrityReceiptSchema = "execplane_tier_integrity_receipt.v1"

// Verification statuses. There is no third, optimistic state: anything the gate
// could not establish is unverified with a reason.
const (
	StatusVerified   = "verified"
	StatusUnverified = "unverified"
)

// CatalogSource names the account a catalog was probed under together with the
// entitlement it was served at. Recording the account alone is not enough — the
// entitlement is what makes the catalog evidence for another account.
type CatalogSource struct {
	Account     string `json:"account"`
	Entitlement string `json:"entitlement"`
}

// EvidenceKind names what entitlement parity actually licenses for a provider.
// Recording it keeps two very different verifications from reading alike: a
// Codex catalog was fetched from the provider, while a Claude judgement only
// says the executor runs under the same plan as the reference session. Folding
// both into a bare "verified" would recreate the silent conflation this gate
// exists to prevent.
type EvidenceKind string

const (
	// EvidenceNone means nothing backs the tier claim.
	EvidenceNone EvidenceKind = "none"
	// EvidenceProbedCatalog means a provider-served model catalog is held, and
	// entitlement parity licenses reading it as evidence for the execution
	// account.
	EvidenceProbedCatalog EvidenceKind = "probed_catalog"
	// EvidenceEntitlementParity means no catalog exists for this provider, so
	// parity only carries the reference session's working assumption across.
	EvidenceEntitlementParity EvidenceKind = "entitlement_parity"
)

// IntegrityReceipt reconstructs why a workload ran at the tier it ran at.
type IntegrityReceipt struct {
	Schema             string        `json:"schema"`
	Provider           string        `json:"provider"`
	RequestedTier      string        `json:"requested_tier"`
	ResolvedModel      string        `json:"resolved_model"`
	ExecutionAccount   string        `json:"execution_account"`
	CatalogSource      CatalogSource `json:"catalog_source"`
	ResolutionReason   string        `json:"resolution_reason"`
	VerificationStatus string        `json:"verification_status"`
	EvidenceKind       EvidenceKind  `json:"evidence_kind"`
	CheckedAt          time.Time     `json:"checked_at"`
}

// TierRequest is the policy plane's ask, carried into the gate unchanged.
type TierRequest struct {
	Provider      string
	RequestedTier string
	ResolvedModel string
	// Evidence declares what the pipeline holds for this provider. A provider
	// with no evidence can never reach StatusVerified no matter how cleanly the
	// entitlements match, because parity transfers evidence rather than
	// creating it.
	Evidence EvidenceKind
}

// Evaluate produces the integrity receipt for one provider. It performs no I/O:
// callers supply the account resolution and both entitlements, which keeps the
// decision reproducible and keeps the gate free of execution side effects.
//
// StatusVerified needs three things together: a determined execution account, a
// trusted entitlement comparison, and evidence to transfer. Parity moves
// evidence across accounts; it never manufactures it, so a provider that holds
// nothing stays unverified however cleanly its grades match.
func Evaluate(
	request TierRequest,
	resolution AccountResolution,
	execEntitlement, probeEntitlement Entitlement,
	now time.Time,
) IntegrityReceipt {
	receipt := IntegrityReceipt{
		Schema:        IntegrityReceiptSchema,
		Provider:      request.Provider,
		RequestedTier: request.RequestedTier,
		ResolvedModel: request.ResolvedModel,
		CatalogSource: CatalogSource{
			Account:     probeEntitlement.Source,
			Entitlement: probeEntitlement.Grade,
		},
		EvidenceKind: request.Evidence,
		CheckedAt:    now.UTC(),
	}
	if !resolution.Determined() {
		receipt.VerificationStatus = StatusUnverified
		receipt.ResolutionReason = resolution.Reason
		if receipt.ResolutionReason == "" {
			receipt.ResolutionReason = "execution account is indeterminate"
		}
		return receipt
	}
	receipt.ExecutionAccount = resolution.Account.Email
	if receipt.ExecutionAccount == "" {
		receipt.ExecutionAccount = resolution.Account.ID
	}

	verdict, reason := CompareEntitlement(execEntitlement, probeEntitlement)
	receipt.ResolutionReason = reason
	switch {
	case verdict != VerdictTrusted:
		receipt.VerificationStatus = StatusUnverified
	case request.Evidence == "" || request.Evidence == EvidenceNone:
		receipt.VerificationStatus = StatusUnverified
		receipt.ResolutionReason = reason + "; no evidence is held for this provider"
	default:
		receipt.VerificationStatus = StatusVerified
	}
	return receipt
}

// Complete reports whether every field a reader needs is present. A receipt
// that omits one is not a weaker receipt, it is an unusable one.
func (r IntegrityReceipt) Complete() bool {
	if r.Schema == "" || r.RequestedTier == "" || r.ResolvedModel == "" ||
		r.ResolutionReason == "" || r.VerificationStatus == "" || r.EvidenceKind == "" {
		return false
	}
	if r.VerificationStatus == StatusVerified {
		return r.ExecutionAccount != "" && r.CatalogSource.Account != "" &&
			r.CatalogSource.Entitlement != "" && r.EvidenceKind != EvidenceNone
	}
	return true
}
