package omp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
)

const maxOMPModelDoctorReceiptBytes = 1 << 20

// @AX:WARN [AUTO]: model doctor receipt loading has cyclomatic complexity 15.
// @AX:REASON [AUTO]: gocyclo reports 15 across rooted-path, file-type, permission, size, decoding, and canonicality checks.
func readOMPModelDoctorReceipt(root string) (receipt OMPModelResolutionReceipt, reason string) {
	workspace, err := openOMPRootedWorkspace(root)
	if err != nil {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	defer func() {
		if workspace.Close() != nil {
			receipt = OMPModelResolutionReceipt{}
			reason = "receipt_invalid"
		}
	}()
	return readOMPModelDoctorReceiptAt(workspace)
}

func readOMPModelDoctorReceiptAt(workspace *ompRootedWorkspace) (OMPModelResolutionReceipt, string) {
	data, _, err := workspace.readOwnerOnlyFile(OMPModelReceiptRelativePath, maxOMPModelDoctorReceiptBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return OMPModelResolutionReceipt{}, "receipt_missing"
	}
	if err != nil {
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
		receipt.Activation.ReadbackHash != input.Activation.ReadbackHash {
		return false
	}
	current := make(map[string]OMPModelRouteResolution, len(input.Compilation.Resolutions))
	for _, resolution := range input.Compilation.Resolutions {
		if resolution.Status != "selected" || resolution.EffectiveSelector == "" {
			continue
		}
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
	if len(receipt.Roles) != len(current) {
		return false
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
func OMPModelDoctorReceiptConfigSource(root string) (configSource string, ownershipDigest string, reason string) {
	workspace, err := openOMPRootedWorkspace(root)
	if err != nil {
		return "", "", "receipt_invalid"
	}
	defer func() {
		if workspace.Close() != nil {
			configSource = ""
			ownershipDigest = ""
			reason = "receipt_invalid"
		}
	}()
	receipt, reason := readOMPModelDoctorReceiptAt(workspace)
	if reason != "" {
		return "", "", reason
	}
	if receipt.ConfigSource == "project-managed" {
		ownership, exists, err := readOMPModelProjectOwnershipAt(workspace)
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
