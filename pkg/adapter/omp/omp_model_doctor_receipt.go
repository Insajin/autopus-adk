package omp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
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
	if rejectDuplicateOMPModelReceiptJSON(data) != nil {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt OMPModelResolutionReceipt
	if decoder.Decode(&receipt) != nil || requireOMPModelDoctorJSONEOF(decoder) != nil {
		return OMPModelResolutionReceipt{}, "receipt_invalid"
	}
	if receipt.CatalogTrust == "" {
		if !legacyOMPModelReceiptDigestMatches(receipt) {
			return OMPModelResolutionReceipt{}, "receipt_invalid"
		}
		normalizeOMPModelReceiptTrust(&receipt)
		if validateOMPModelReceipt(receipt) != nil {
			return OMPModelResolutionReceipt{}, "receipt_invalid"
		}
		return receipt, ""
	}
	if validateOMPModelReceipt(receipt) != nil {
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

type legacyOMPModelRoleReceipt struct {
	Agent            string                           `json:"agent"`
	Profile          string                           `json:"profile"`
	ConfigSource     string                           `json:"config_source"`
	RequestedRole    string                           `json:"requested_role"`
	EffectiveRole    string                           `json:"effective_role"`
	Capability       string                           `json:"capability"`
	Provider         string                           `json:"provider"`
	Model            string                           `json:"model"`
	Selector         string                           `json:"selector"`
	Thinking         string                           `json:"thinking"`
	FallbackAttempts []OMPModelFallbackAttemptReceipt `json:"fallback_attempts"`
	FallbackReason   string                           `json:"fallback_reason"`
	DegradedReason   string                           `json:"degraded_reason"`
	FamilyDiversity  OMPModelFamilyDiversityReceipt   `json:"family_diversity"`
	SafetySource     string                           `json:"safety_source"`
}

type legacyOMPModelResolutionBody struct {
	SchemaVersion          string                      `json:"schema_version"`
	OMPVersion             string                      `json:"omp_version"`
	CatalogFingerprint     string                      `json:"catalog_fingerprint"`
	Profile                string                      `json:"profile"`
	ConfigSource           string                      `json:"config_source"`
	ProjectOwnershipDigest string                      `json:"project_ownership_digest,omitempty"`
	Activation             OMPModelActivationReceipt   `json:"activation"`
	Roles                  []legacyOMPModelRoleReceipt `json:"roles"`
	Safety                 OMPModelSafetyReceipt       `json:"safety"`
	GeneratedAt            time.Time                   `json:"generated_at"`
}

func legacyOMPModelReceiptDigestMatches(receipt OMPModelResolutionReceipt) bool {
	roles := make([]legacyOMPModelRoleReceipt, 0, len(receipt.Roles))
	for _, role := range receipt.Roles {
		if role.EvidenceClass != "" || role.EffectiveFamily != "" {
			return false
		}
		roles = append(roles, legacyOMPModelRoleReceipt{
			Agent: role.Agent, Profile: role.Profile, ConfigSource: role.ConfigSource,
			RequestedRole: role.RequestedRole, EffectiveRole: role.EffectiveRole,
			Capability: role.Capability, Provider: role.Provider, Model: role.Model,
			Selector: role.Selector, Thinking: role.Thinking,
			FallbackAttempts: role.FallbackAttempts, FallbackReason: role.FallbackReason,
			DegradedReason: role.DegradedReason, FamilyDiversity: role.FamilyDiversity,
			SafetySource: role.SafetySource,
		})
	}
	body := legacyOMPModelResolutionBody{
		SchemaVersion: receipt.SchemaVersion, OMPVersion: receipt.OMPVersion,
		CatalogFingerprint: receipt.CatalogFingerprint, Profile: receipt.Profile,
		ConfigSource: receipt.ConfigSource, ProjectOwnershipDigest: receipt.ProjectOwnershipDigest,
		Activation: receipt.Activation, Roles: roles, Safety: receipt.Safety,
		GeneratedAt: receipt.GeneratedAt,
	}
	data, err := json.Marshal(body)
	return err == nil && OMPModelSHA256(data) == receipt.ResolutionDigest
}

// @AX:WARN [AUTO]: model projection comparison has cyclomatic complexity 25.
// @AX:REASON [AUTO]: gocyclo reports 25 across configured source, profile, route, fallback, and selected-model equivalence checks.
func ompModelDoctorProjectionMatches(receipt OMPModelResolutionReceipt, input OMPModelDoctorInput) bool {
	configuredSource := input.ConfiguredSource
	if configuredSource == "" {
		configuredSource = input.ConfigSource
	}
	probeTrust := input.Probe.CatalogTrust
	if probeTrust == "" {
		probeTrust = config.RoleModelCatalogTrustStrict
	}
	if receipt.Profile != input.Profile || receipt.ConfigSource != input.ConfigSource ||
		receipt.CatalogTrust != probeTrust ||
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
			role.EvidenceClass != resolution.EvidenceClass ||
			role.EffectiveFamily != resolution.EffectiveFamily ||
			role.Selector != resolution.EffectiveProvider+"/"+resolution.EffectiveModel ||
			role.FamilyDiversity.Status != ompModelDoctorFamilyStatus(resolution) ||
			role.FamilyDiversity.ExecutorFamily != resolution.FamilyDiversity.Executor ||
			role.FamilyDiversity.EffectiveFamily != resolution.FamilyDiversity.Reviewer ||
			role.FamilyDiversity.Reason != resolution.FamilyDiversity.Reason ||
			role.FamilyDiversity.IndependentProviderEvidence != resolution.IndependentProviderEvidence {
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
