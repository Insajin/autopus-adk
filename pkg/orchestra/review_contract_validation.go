package orchestra

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	reviewSnapshotDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reviewContractDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	reviewAdapterPattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
	reviewModelPattern          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,255}$`)
	reviewRolePattern           = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
)

var managedReviewAdapters = map[string]struct{}{
	"claude":   {},
	"codex":    {},
	"gemini":   {},
	"opencode": {},
}

func validateReviewPrepareContract(contract ReviewPrepareContract) error {
	if contract.SchemaVersion != ReviewPrepareSchemaV1 ||
		!reviewSnapshotDigestPattern.MatchString(contract.SnapshotDigest) ||
		!reviewContractDigestPattern.MatchString(contract.ContractDigest) ||
		!validReviewOpaqueRef(contract.RequestID) ||
		!validReviewOpaqueRef(contract.WorkspaceID) ||
		!validReviewOpaqueRef(contract.RepoScopeRef) ||
		!validReviewOpaqueRef(contract.WorkItemID) ||
		!validReviewOpaqueRef(contract.ReviewRunID) ||
		!reviewRolePattern.MatchString(contract.Role) ||
		contract.Bounds.MaxResultBytes < 1 ||
		contract.Bounds.MaxResultBytes > ReviewResultMaximumBytes ||
		contract.Bounds.MaxFindings < 1 ||
		contract.Bounds.MaxFindings > ReviewFindingsMaximum ||
		len(contract.Providers) < 2 || len(contract.Providers) > 4 {
		return ErrReviewPrepareInvalid
	}
	seen := make(map[string]struct{}, len(contract.Providers))
	for _, provider := range contract.Providers {
		if !reviewAdapterPattern.MatchString(provider.AdapterID) ||
			reviewProviderAllowed(provider.AdapterID) == false ||
			!reviewModelPattern.MatchString(provider.Model) ||
			!reviewRolePattern.MatchString(provider.Role) {
			return ErrReviewPrepareInvalid
		}
		if _, duplicate := seen[provider.AdapterID]; duplicate {
			return ErrReviewPrepareInvalid
		}
		seen[provider.AdapterID] = struct{}{}
	}
	return nil
}

func reviewProviderAllowed(adapterID string) bool {
	_, allowed := managedReviewAdapters[adapterID]
	return allowed
}

func validReviewOpaqueRef(value string) bool {
	if value == "" || len(value) > 256 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	if strings.ContainsAny(value, `/\\`) || strings.Contains(value, "://") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}
