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
	CheckedAt          time.Time     `json:"checked_at"`
}

// TierRequest is the policy plane's ask, carried into the gate unchanged.
type TierRequest struct {
	Provider      string
	RequestedTier string
	ResolvedModel string
}

// Evaluate produces the integrity receipt for one provider. It performs no I/O:
// callers supply the account resolution and both entitlements, which keeps the
// decision reproducible and keeps the gate free of execution side effects.
//
// A trusted verdict is the only path to StatusVerified. Both reprobe and
// unverified land on StatusUnverified here, because this function is given a
// catalog that was already probed — re-probing is the caller's move, and until
// it happens the held evidence does not cover the execution account.
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
		CheckedAt: now.UTC(),
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
	if verdict == VerdictTrusted {
		receipt.VerificationStatus = StatusVerified
	} else {
		receipt.VerificationStatus = StatusUnverified
	}
	return receipt
}

// Complete reports whether every field a reader needs is present. A receipt
// that omits one is not a weaker receipt, it is an unusable one.
func (r IntegrityReceipt) Complete() bool {
	if r.Schema == "" || r.RequestedTier == "" || r.ResolvedModel == "" ||
		r.ResolutionReason == "" || r.VerificationStatus == "" {
		return false
	}
	if r.VerificationStatus == StatusVerified {
		return r.ExecutionAccount != "" && r.CatalogSource.Account != "" &&
			r.CatalogSource.Entitlement != ""
	}
	return true
}
