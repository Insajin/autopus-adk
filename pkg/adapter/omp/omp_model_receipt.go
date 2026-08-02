package omp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	OMPModelReceiptSchemaVersion = "autopus.omp-model-resolution.v1"
	OMPModelReceiptRelativePath  = ".autopus/omp-model-resolution-v1.json"
)

type OMPModelActivationReceipt struct {
	Argv         []string `json:"argv"`
	ConfigHash   string   `json:"config_hash"`
	ReadbackHash string   `json:"readback_hash"`
}

type OMPModelFallbackAttemptReceipt struct {
	Selector string `json:"selector"`
	Reason   string `json:"reason"`
}

type OMPModelFamilyDiversityReceipt struct {
	Status                      string `json:"status"`
	ExecutorFamily              string `json:"executor_family"`
	EffectiveFamily             string `json:"effective_family"`
	Reason                      string `json:"reason"`
	IndependentProviderEvidence bool   `json:"independent_provider_evidence"`
}

type OMPModelRoleReceipt struct {
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

type OMPModelSafetyReceipt struct {
	ApprovalMode  string `json:"approval_mode"`
	IsolationMode string `json:"isolation_mode"`
	Source        string `json:"source"`
}

type OMPModelResolutionReceipt struct {
	SchemaVersion          string                    `json:"schema_version"`
	OMPVersion             string                    `json:"omp_version"`
	CatalogFingerprint     string                    `json:"catalog_fingerprint"`
	Profile                string                    `json:"profile"`
	ConfigSource           string                    `json:"config_source"`
	ProjectOwnershipDigest string                    `json:"project_ownership_digest,omitempty"`
	Activation             OMPModelActivationReceipt `json:"activation"`
	Roles                  []OMPModelRoleReceipt     `json:"roles"`
	Safety                 OMPModelSafetyReceipt     `json:"safety"`
	GeneratedAt            time.Time                 `json:"generated_at"`
	ResolutionDigest       string                    `json:"resolution_digest"`
}

type OMPModelReceiptWriteInput struct {
	WorkspaceRoot   string
	Receipt         OMPModelResolutionReceipt
	ForbiddenValues []string
}

type OMPModelReceiptWriteEvidence struct {
	Receipt OMPModelResolutionReceipt
	Bytes   []byte
}

type ompModelResolutionBody struct {
	SchemaVersion          string                    `json:"schema_version"`
	OMPVersion             string                    `json:"omp_version"`
	CatalogFingerprint     string                    `json:"catalog_fingerprint"`
	Profile                string                    `json:"profile"`
	ConfigSource           string                    `json:"config_source"`
	ProjectOwnershipDigest string                    `json:"project_ownership_digest,omitempty"`
	Activation             OMPModelActivationReceipt `json:"activation"`
	Roles                  []OMPModelRoleReceipt     `json:"roles"`
	Safety                 OMPModelSafetyReceipt     `json:"safety"`
}

func CanonicalOMPModelResolutionReceipt(receipt OMPModelResolutionReceipt) (OMPModelResolutionReceipt, []byte, error) {
	receipt.SchemaVersion = OMPModelReceiptSchemaVersion
	receipt.ResolutionDigest = ""
	receipt.GeneratedAt = receipt.GeneratedAt.UTC()
	receipt.Activation.Argv = append([]string(nil), receipt.Activation.Argv...)
	receipt.Roles = append([]OMPModelRoleReceipt(nil), receipt.Roles...)
	for index := range receipt.Roles {
		receipt.Roles[index].FallbackAttempts = append([]OMPModelFallbackAttemptReceipt(nil), receipt.Roles[index].FallbackAttempts...)
		if receipt.Roles[index].FallbackAttempts == nil {
			receipt.Roles[index].FallbackAttempts = []OMPModelFallbackAttemptReceipt{}
		}
		if receipt.Roles[index].FamilyDiversity.Status == "" {
			receipt.Roles[index].FamilyDiversity.Status = "not_applicable"
		}
	}
	sort.Slice(receipt.Roles, func(i, j int) bool {
		if receipt.Roles[i].Agent == receipt.Roles[j].Agent {
			return receipt.Roles[i].EffectiveRole < receipt.Roles[j].EffectiveRole
		}
		return receipt.Roles[i].Agent < receipt.Roles[j].Agent
	})
	if err := validateOMPModelReceipt(receipt); err != nil {
		return OMPModelResolutionReceipt{}, nil, err
	}
	body := ompModelResolutionBody{
		SchemaVersion:          receipt.SchemaVersion,
		OMPVersion:             receipt.OMPVersion,
		CatalogFingerprint:     receipt.CatalogFingerprint,
		Profile:                receipt.Profile,
		ConfigSource:           receipt.ConfigSource,
		ProjectOwnershipDigest: receipt.ProjectOwnershipDigest,
		Activation:             receipt.Activation,
		Roles:                  receipt.Roles,
		Safety:                 receipt.Safety,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return OMPModelResolutionReceipt{}, nil, fmt.Errorf("marshal OMP resolution body: %w", err)
	}
	receipt.ResolutionDigest = OMPModelSHA256(bodyBytes)
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return OMPModelResolutionReceipt{}, nil, fmt.Errorf("marshal OMP resolution receipt: %w", err)
	}
	data = append(data, '\n')
	return receipt, data, nil
}

func WriteOMPModelResolutionReceipt(input OMPModelReceiptWriteInput) (evidence OMPModelReceiptWriteEvidence, returnErr error) {
	workspace, err := openOMPRootedWorkspace(input.WorkspaceRoot)
	if err != nil {
		return OMPModelReceiptWriteEvidence{}, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, workspace.Close()) }()
	return writeOMPModelResolutionReceiptAt(workspace, input)
}

func writeOMPModelResolutionReceiptAt(
	workspace *ompRootedWorkspace,
	input OMPModelReceiptWriteInput,
) (OMPModelReceiptWriteEvidence, error) {
	if input.Receipt.GeneratedAt.IsZero() {
		input.Receipt.GeneratedAt = time.Now().UTC()
	}
	receipt, data, err := CanonicalOMPModelResolutionReceipt(input.Receipt)
	if err != nil {
		return OMPModelReceiptWriteEvidence{}, err
	}
	for _, forbidden := range input.ForbiddenValues {
		if forbidden != "" && strings.Contains(string(data), forbidden) {
			return OMPModelReceiptWriteEvidence{}, fmt.Errorf("receipt contains forbidden secret material")
		}
	}
	if err := workspace.atomicWrite(OMPModelReceiptRelativePath, data, 0o600); err != nil {
		return OMPModelReceiptWriteEvidence{}, err
	}
	return OMPModelReceiptWriteEvidence{Receipt: receipt, Bytes: append([]byte(nil), data...)}, nil
}

// @AX:WARN [AUTO]: model receipt validation has cyclomatic complexity 20.
// @AX:REASON [AUTO]: gocyclo reports 20 across schema, projection, provenance, digest, route, and fallback invariants.
func validateOMPModelReceipt(receipt OMPModelResolutionReceipt) error {
	if receipt.SchemaVersion != OMPModelReceiptSchemaVersion || receipt.GeneratedAt.IsZero() {
		return fmt.Errorf("receipt schema and generated_at are required")
	}
	if !strings.HasPrefix(receipt.OMPVersion, "omp/") || !validOMPModelHash(receipt.CatalogFingerprint) {
		return fmt.Errorf("receipt OMP version or catalog fingerprint is invalid")
	}
	if receipt.Profile == "" || receipt.ConfigSource == "" || len(receipt.Roles) == 0 {
		return fmt.Errorf("receipt profile, config source, and roles are required")
	}
	if receipt.ConfigSource == "project-managed" {
		if !validOMPModelHash(receipt.ProjectOwnershipDigest) {
			return fmt.Errorf("receipt project ownership digest is invalid")
		}
	} else if receipt.ConfigSource != "overlay" || receipt.ProjectOwnershipDigest != "" {
		return fmt.Errorf("receipt config source is invalid")
	}
	if !validOMPModelHash(receipt.Activation.ConfigHash) || !validOMPModelHash(receipt.Activation.ReadbackHash) {
		return fmt.Errorf("receipt activation hashes are invalid")
	}
	if err := validateOMPModelReceiptArgv(receipt.Activation.Argv); err != nil {
		return err
	}
	for _, role := range receipt.Roles {
		if err := validateOMPModelRoleReceipt(role); err != nil {
			return err
		}
	}
	all, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("validate receipt fields: %w", err)
	}
	lower := strings.ToLower(string(all))
	for _, marker := range []string{"authorization:", "bearer ", "api_key", "api-key", "password", "secret", "access_token", "refresh_token"} {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("receipt contains secret-like material")
		}
	}
	return nil
}

func validateOMPModelReceiptArgv(argv []string) error {
	if len(argv) == 0 || len(argv) > 16 {
		return fmt.Errorf("receipt activation argv is invalid")
	}
	for _, value := range argv {
		if value == "" || strings.ContainsAny(value, "\x00\r\n\t") || filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, ".."+string(filepath.Separator)) {
			return fmt.Errorf("receipt activation argv contains an unsafe path or value")
		}
	}
	return nil
}

func validateOMPModelRoleReceipt(role OMPModelRoleReceipt) error {
	required := []string{role.Agent, role.Profile, role.ConfigSource, role.RequestedRole, role.EffectiveRole,
		role.Capability, role.Provider, role.Model, role.Selector, role.Thinking, role.FamilyDiversity.Status, role.SafetySource}
	for _, value := range required {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\x00\r\n") {
			return fmt.Errorf("receipt role contains an invalid required value")
		}
	}
	if role.Selector != role.Provider+"/"+role.Model {
		return fmt.Errorf("receipt selector does not match provider/model")
	}
	for _, attempt := range role.FallbackAttempts {
		if attempt.Selector == "" || attempt.Reason == "" {
			return fmt.Errorf("receipt fallback attempt is incomplete")
		}
	}
	return nil
}
