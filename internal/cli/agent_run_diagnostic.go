package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const diagnosticVerdictSchemaBasename = "gpt-diagnostic-verdict-schema-v1.json"

var diagnosticScopeHash = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

var diagnosticFindingCodes = map[string]struct{}{
	"correctness":            {},
	"security":               {},
	"regression":             {},
	"test_gap":               {},
	"task_mismatch":          {},
	"deterministic_conflict": {},
	"scope_uncertain":        {},
}

type diagnosticVerdictReceipt struct {
	Verdict            string
	FindingCount       int
	FindingCodes       []string
	FindingScopeHashes []string
}

func parseDiagnosticVerdict(output string) (diagnosticVerdictReceipt, error) {
	var wire struct {
		Verdict            *string   `json:"verdict"`
		FindingCount       *int      `json:"finding_count"`
		FindingCodes       *[]string `json:"finding_codes"`
		FindingScopeHashes *[]string `json:"finding_scope_hashes"`
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return diagnosticVerdictReceipt{}, fmt.Errorf("diagnostic verdict parse failed")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return diagnosticVerdictReceipt{}, fmt.Errorf("diagnostic verdict has trailing JSON")
	}
	if wire.Verdict == nil || wire.FindingCount == nil || wire.FindingCodes == nil || wire.FindingScopeHashes == nil {
		return diagnosticVerdictReceipt{}, fmt.Errorf("diagnostic verdict is incomplete")
	}
	receipt := diagnosticVerdictReceipt{Verdict: *wire.Verdict, FindingCount: *wire.FindingCount,
		FindingCodes:       append([]string{}, (*wire.FindingCodes)...),
		FindingScopeHashes: append([]string{}, (*wire.FindingScopeHashes)...)}
	if err := validateDiagnosticVerdict(receipt); err != nil {
		return diagnosticVerdictReceipt{}, err
	}
	return receipt, nil
}

func validateDiagnosticVerdict(receipt diagnosticVerdictReceipt) error {
	if receipt.Verdict != "PASS" && receipt.Verdict != "FAIL" || receipt.FindingCount < 0 || receipt.FindingCount > 3 ||
		len(receipt.FindingCodes) > 3 || len(receipt.FindingScopeHashes) > 3 {
		return fmt.Errorf("diagnostic verdict is outside bounds")
	}
	if !uniqueAllowedCodes(receipt.FindingCodes) || !uniqueScopeHashes(receipt.FindingScopeHashes) {
		return fmt.Errorf("diagnostic verdict contains invalid bounded values")
	}
	if receipt.Verdict == "PASS" {
		if receipt.FindingCount != 0 || len(receipt.FindingCodes) != 0 || len(receipt.FindingScopeHashes) != 0 {
			return fmt.Errorf("diagnostic PASS must be empty")
		}
		return nil
	}
	if receipt.FindingCount == 0 || len(receipt.FindingCodes) != receipt.FindingCount || len(receipt.FindingScopeHashes) == 0 {
		return fmt.Errorf("diagnostic FAIL is incomplete")
	}
	return nil
}

func uniqueAllowedCodes(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := diagnosticFindingCodes[value]; !ok {
			return false
		}
		if _, ok := seen[value]; ok {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func uniqueScopeHashes(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !diagnosticScopeHash.MatchString(value) {
			return false
		}
		if _, ok := seen[value]; ok {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}
