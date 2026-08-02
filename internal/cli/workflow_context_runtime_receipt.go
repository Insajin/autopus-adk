package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/companionmanifest"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type WorkflowContextHistoryCredit struct {
	ID          string `json:"id"`
	SourceRef   string `json:"source_ref"`
	PriorHash   string `json:"prior_hash"`
	Action      string `json:"action"`
	Reason      string `json:"reason"`
	TokenBefore int    `json:"token_before"`
	TokenAfter  int    `json:"token_after"`
}

type WorkflowContextArtifactCounts struct {
	Before       int `json:"before"`
	AfterCleanup int `json:"after_cleanup"`
}

type WorkflowContextCleanupReceipt struct {
	Attempted           bool   `json:"attempted"`
	Verified            bool   `json:"verified"`
	Reason              string `json:"reason"`
	UserRootAccessCount int    `json:"user_root_access_count"`
}

type WorkflowContextFallbackReceipt struct {
	Mode               string                                    `json:"mode"`
	Reason             string                                    `json:"reason"`
	Integrity          string                                    `json:"integrity"`
	SnapshotHash       string                                    `json:"snapshot_hash,omitempty"`
	PromptManifestHash string                                    `json:"prompt_manifest_hash,omitempty"`
	FullDocumentRefs   []promptlayer.OMPContextDocumentReference `json:"full_document_refs,omitempty"`
}

type WorkflowContextModeReceipt struct {
	RequestedHistoryMode string `json:"requested_history_mode"`
	EffectiveHistoryMode string `json:"effective_history_mode"`
	EffectiveMemoryMode  string `json:"effective_memory_mode"`
	PreviousHistoryMode  string `json:"previous_history_mode"`
	OverlayHash          string `json:"overlay_hash,omitempty"`
	ReadbackHash         string `json:"readback_hash,omitempty"`
}

type WorkflowContextRuntimeReceipt struct {
	SchemaVersion         string                                    `json:"schema_version"`
	Event                 string                                    `json:"event"`
	Outcome               string                                    `json:"outcome"`
	WorkspaceID           string                                    `json:"workspace_id"`
	SpecID                string                                    `json:"spec_id"`
	TaskID                string                                    `json:"task_id"`
	Phase                 string                                    `json:"phase"`
	SessionID             string                                    `json:"session_id"`
	BindingHash           string                                    `json:"binding_hash,omitempty"`
	OptionsHash           string                                    `json:"options_hash,omitempty"`
	SnapshotHash          string                                    `json:"snapshot_hash,omitempty"`
	PromptManifestHash    string                                    `json:"prompt_manifest_hash,omitempty"`
	FullDocumentRefs      []promptlayer.OMPContextDocumentReference `json:"full_document_refs"`
	RequiredEphemeralRefs []promptlayer.OMPContextHashedReference   `json:"required_ephemeral_hashes"`
	FrozenFindingIDs      []string                                  `json:"frozen_finding_ids"`
	WorkerResultFields    []string                                  `json:"worker_result_fields"`
	HistoryCreditRows     []WorkflowContextHistoryCredit            `json:"history_credit_rows"`
	ShadowCandidateRefs   []promptlayer.OMPContextPlanReference     `json:"shadow_candidate_refs"`
	DocumentOmissions     []string                                  `json:"document_omissions"`
	MemoryInjections      []string                                  `json:"memory_injections"`
	PromotionAttestation  string                                    `json:"promotion_attestation,omitempty"`
	PromotionPolicyDigest string                                    `json:"promotion_policy_digest,omitempty"`
	CanaryDigest          string                                    `json:"canary_digest,omitempty"`
	PromotionCheckedAt    string                                    `json:"promotion_checked_at,omitempty"`
	Capabilities          WorkflowContextCapabilities               `json:"capabilities"`
	RootClass             string                                    `json:"root_class"`
	ArtifactCounts        WorkflowContextArtifactCounts             `json:"artifact_counts"`
	Cleanup               WorkflowContextCleanupReceipt             `json:"cleanup"`
	Mode                  WorkflowContextModeReceipt                `json:"mode"`
	Fallback              WorkflowContextFallbackReceipt            `json:"fallback"`
	ExactMatch            bool                                      `json:"exact_match"`
	PhaseSequence         []string                                  `json:"phase_sequence"`
}

type WorkflowContextReceiptWriter struct {
	WorkspaceRoot string
}

func WorkflowContextReceiptRelativePath(taskID, sessionID string) string {
	return filepath.ToSlash(filepath.Join(".autopus", "runtime", "omp-context", taskID, sessionID, "receipt.json"))
}

// @AX:WARN [AUTO]: receipt persistence validation has cyclomatic complexity 15.
// @AX:REASON [AUTO]: gocyclo reports 15 across schema, identifier, secrecy, rooted-path, and permission checks before atomic write.
func (writer WorkflowContextReceiptWriter) Write(receipt WorkflowContextRuntimeReceipt) error {
	if receipt.SchemaVersion != WorkflowContextRuntimeReceiptSchemaVersion || receipt.Event != "terminal" {
		return fmt.Errorf("invalid OMP context runtime receipt schema or event")
	}
	if receipt.RootClass != config.OMPContextRuntimeNoSession && receipt.RootClass != config.OMPContextRuntimeIsolatedTaskOwned {
		return fmt.Errorf("invalid OMP context runtime receipt root class")
	}
	if err := validateWorkflowContextReceiptID(receipt.TaskID); err != nil {
		return err
	}
	if err := validateWorkflowContextReceiptID(receipt.SessionID); err != nil {
		return err
	}
	if len(receipt.DocumentOmissions) != 0 || len(receipt.MemoryInjections) != 0 {
		return fmt.Errorf("active OMP receipt cannot contain document omissions or memory injections")
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("encode OMP context runtime receipt: %w", err)
	}
	if err := validateWorkflowContextReceiptValues(data); err != nil {
		return err
	}
	rel := WorkflowContextReceiptRelativePath(receipt.TaskID, receipt.SessionID)
	path, err := adapter.SafeWorkspacePath(writer.WorkspaceRoot, rel)
	if err != nil {
		return fmt.Errorf("resolve OMP context receipt: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create OMP context receipt directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("secure OMP context receipt directory: %w", err)
	}
	path, err = adapter.SafeWorkspacePath(writer.WorkspaceRoot, rel)
	if err != nil {
		return fmt.Errorf("recheck OMP context receipt path: %w", err)
	}
	return companionmanifest.WriteAtomic(path, append(data, '\n'))
}

var workflowContextReceiptIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var workflowContextSecretPattern = regexp.MustCompile(`(?i)(^|[^a-z0-9])sk-[a-z0-9]`)

func validateWorkflowContextReceiptID(value string) error {
	if !workflowContextReceiptIDPattern.MatchString(value) || value == "." || value == ".." {
		return fmt.Errorf("invalid OMP context receipt identity")
	}
	return nil
}

func validateWorkflowContextReceiptValues(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("inspect OMP context receipt: %w", err)
	}
	return walkWorkflowContextReceiptValue("receipt", value)
}

// @AX:WARN [AUTO]: recursive receipt secrecy inspection has cyclomatic complexity 20.
// @AX:REASON [AUTO]: gocyclo reports 20 across heterogeneous JSON containers and forbidden secret/path value classes.
func walkWorkflowContextReceiptValue(path string, value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if err := walkWorkflowContextReceiptValue(path+"."+key, child); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range typed {
			if err := walkWorkflowContextReceiptValue(fmt.Sprintf("%s[%d]", path, index), child); err != nil {
				return err
			}
		}
	case string:
		lower := strings.ToLower(typed)
		if filepath.IsAbs(typed) || strings.Contains(lower, "/users/") || strings.Contains(lower, "/home/") {
			return fmt.Errorf("OMP context receipt contains a forbidden absolute path at %s", path)
		}
		if workflowContextSecretPattern.MatchString(typed) || strings.Contains(lower, "token=") || strings.Contains(lower, "api_key") ||
			strings.Contains(lower, "password=") || strings.Contains(lower, "secret=") ||
			strings.Contains(lower, "authorization:") || strings.Contains(lower, "bearer ") {
			return fmt.Errorf("OMP context receipt contains a forbidden secret value at %s", path)
		}
		if strings.Contains(lower, "ignore previous instructions") || strings.Contains(lower, "drop acceptance") {
			return fmt.Errorf("OMP context receipt contains prompt-injection text at %s", path)
		}
	}
	return nil
}
