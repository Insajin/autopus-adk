package omp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const maxOMPModelDoctorReceiptBytes = 1 << 20

// @AX:WARN [AUTO]: model doctor receipt loading has cyclomatic complexity 15.
// @AX:REASON [AUTO]: gocyclo reports 15 across rooted-path, file-type, permission, size, decoding, and canonicality checks.
func readOMPModelDoctorReceipt(root string) (OMPModelResolutionReceipt, string) {
	target := filepath.Join(root, filepath.FromSlash(OMPModelReceiptRelativePath))
	path, err := resolveOMPModelOwnedPath(root, OMPModelReceiptRelativePath, false)
	if err != nil {
		if _, statErr := os.Lstat(target); os.IsNotExist(statErr) {
			return OMPModelResolutionReceipt{}, "receipt_missing"
		}
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return OMPModelResolutionReceipt{}, "receipt_missing"
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	file, err := os.Open(path)
	if err != nil {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxOMPModelDoctorReceiptBytes+1))
	if err != nil || len(data) > maxOMPModelDoctorReceiptBytes {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt OMPModelResolutionReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	if err := requireOMPModelDoctorJSONEOF(decoder); err != nil {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	if err := validateOMPModelReceipt(receipt); err != nil {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	canonical, _, err := CanonicalOMPModelResolutionReceipt(receipt)
	if err != nil || canonical.ResolutionDigest != receipt.ResolutionDigest {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	return receipt, ""
}

func readOMPModelDoctorReceiptAt(workspace *ompRootedWorkspace) (OMPModelResolutionReceipt, string) {
	data, info, err := workspace.readFile(OMPModelReceiptRelativePath, maxOMPModelDoctorReceiptBytes)
	if errors.Is(err, os.ErrNotExist) {
		return OMPModelResolutionReceipt{}, "receipt_missing"
	}
	if err != nil || info.Mode().Perm() != 0o600 {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt OMPModelResolutionReceipt
	if decoder.Decode(&receipt) != nil || requireOMPModelDoctorJSONEOF(decoder) != nil ||
		validateOMPModelReceipt(receipt) != nil {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	canonical, _, err := CanonicalOMPModelResolutionReceipt(receipt)
	if err != nil || canonical.ResolutionDigest != receipt.ResolutionDigest {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	return receipt, ""
}

func requireOMPModelDoctorJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return errors.New("unexpected trailing JSON value")
	}
	return err
}

// @AX:WARN [AUTO]: model projection comparison has cyclomatic complexity 25.
// @AX:REASON [AUTO]: gocyclo reports 25 across configured source, profile, route, fallback, and selected-model equivalence checks.
func ompModelDoctorProjectionMatches(receipt OMPModelResolutionReceipt, input OMPModelDoctorInput) bool {
	configuredSource := input.ConfiguredSource
	if configuredSource == "" {
		configuredSource = input.ConfigSource
	}
	if receipt.Profile != input.Profile || receipt.ConfigSource != input.ConfigSource ||
		receipt.ConfigSource != configuredSource ||
		receipt.ProjectOwnershipDigest != input.ProjectOwnershipDigest ||
		receipt.Activation.ConfigHash != input.Activation.ConfigHash ||
		receipt.Activation.ReadbackHash != input.Activation.ReadbackHash ||
		len(receipt.Roles) != len(input.Compilation.Resolutions) {
		return false
	}
	current := make(map[string]OMPModelRouteResolution, len(input.Compilation.Resolutions))
	for _, resolution := range input.Compilation.Resolutions {
		agent := resolution.Agent
		if agent == "" {
			agent = resolution.RouteID
		}
		if agent == "" {
			return false
		}
		if _, duplicate := current[agent]; duplicate {
			return false
		}
		current[agent] = resolution
	}
	for _, role := range receipt.Roles {
		resolution, ok := current[role.Agent]
		if !ok || role.Profile != input.Profile || role.ConfigSource != input.ConfigSource ||
			role.RequestedRole != resolution.RequestedRole || role.EffectiveRole != resolution.RequestedRole ||
			role.Capability != resolution.Capability || role.Provider != resolution.EffectiveProvider ||
			role.Model != resolution.EffectiveModel || role.Thinking != resolution.Thinking ||
			role.Selector != resolution.EffectiveProvider+"/"+resolution.EffectiveModel ||
			role.FamilyDiversity.Status != ompModelDoctorFamilyStatus(resolution) {
			return false
		}
	}
	return true
}

// OMPModelDoctorReceiptConfigSource returns only the fixed config-source enum
// and the secret-free ownership digest needed to select a doctor readback path.
func OMPModelDoctorReceiptConfigSource(root string) (string, string, string) {
	receipt, reason := readOMPModelDoctorReceipt(root)
	if reason != "" {
		return "", "", reason
	}
	if receipt.ConfigSource == "project-managed" {
		ownership, exists, err := readOMPModelProjectOwnership(root)
		if err != nil || !exists || ownership.LedgerDigest != receipt.ProjectOwnershipDigest {
			return "", "", "receipt_invalid"
		}
		return receipt.ConfigSource, ownership.LedgerDigest, ""
	}
	return receipt.ConfigSource, "", ""
}

func ompModelDoctorFamilyStatus(resolution OMPModelRouteResolution) string {
	if resolution.FamilyDiversity.Status == "" {
		return "not_applicable"
	}
	return resolution.FamilyDiversity.Status
}
