package spec

// ApplyScopeLock filters findings based on mode and prior scope.
// In verify mode: new non-critical, non-security findings are tagged out_of_scope.
// Critical/security findings get EscapeHatch=true.
// In discover mode: all findings pass through unchanged.
// Findings already classified by ApplyRepeatDiscoveries pass through untouched:
// a repeat carries a status derived from the finding it restates (out_of_scope,
// or regressed for a critical repeat of a resolved finding) and re-running the
// generic new-finding rules here would erase that.
func ApplyScopeLock(incoming, prior []ReviewFinding, mode ReviewMode) []ReviewFinding {
	if mode != ReviewModeVerify {
		return incoming
	}

	// Build set of known IDs from prior findings.
	knownIDs := make(map[string]bool, len(prior))
	for _, f := range prior {
		if f.ID != "" {
			knownIDs[f.ID] = true
		}
	}

	result := make([]ReviewFinding, 0, len(incoming))
	for _, f := range incoming {
		if f.RepeatOf != "" {
			result = append(result, f)
			continue
		}
		if knownIDs[f.ID] {
			result = append(result, f)
			continue
		}
		// New finding in verify mode: apply scope lock
		if f.Severity == "critical" || f.Category == FindingCategorySecurity {
			f.EscapeHatch = true
			f.Status = FindingStatusOpen
		} else {
			f.Status = FindingStatusOutOfScope
		}
		result = append(result, f)
	}
	return result
}
