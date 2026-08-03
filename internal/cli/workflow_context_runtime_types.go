package cli

import (
	"context"
	"encoding/json"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

const (
	WorkflowContextRuntimeReceiptSchemaVersion = promptlayer.OMPContextReceiptSchemaVersion

	WorkflowContextEventPreCompaction  = "pre_compaction"
	WorkflowContextEventCompacted      = "compacted"
	WorkflowContextEventPostCompaction = "post_compaction"

	WorkflowContextOutcomeAdmitted = "admitted"
	WorkflowContextOutcomeFallback = "fallback"
	WorkflowContextOutcomeBlocked  = "blocked"

	WorkflowContextDispatchOptimized     = "optimized"
	WorkflowContextDispatchCanonicalFull = "canonical_full"
	WorkflowContextFallbackNone          = "none"
)

type WorkflowContextEffectivePolicy struct {
	Profile             string `json:"profile"`
	HistoryMode         string `json:"history_mode"`
	MemoryMode          string `json:"memory_mode"`
	HistoryTargetTokens int    `json:"history_target_tokens"`
	Fallback            string `json:"fallback"`
	CapabilityPolicy    string `json:"capability_policy"`
	RuntimeRootPolicy   string `json:"runtime_root_policy"`
	MutationScope       string `json:"mutation_scope"`
}

func workflowContextPolicyFromConfig(cfg *config.HarnessConfig) (WorkflowContextEffectivePolicy, bool, error) {
	if cfg == nil {
		return WorkflowContextEffectivePolicy{}, false, nil
	}
	name, profile, ok, err := cfg.OMPContextPolicy.SelectedOMPContextProfile()
	if err != nil || !ok {
		return WorkflowContextEffectivePolicy{}, ok, err
	}
	return WorkflowContextEffectivePolicy{
		Profile: name, HistoryMode: profile.HistoryMode, MemoryMode: profile.MemoryMode,
		HistoryTargetTokens: profile.HistoryTargetTokens, Fallback: profile.Fallback,
		CapabilityPolicy: profile.CapabilityPolicy, RuntimeRootPolicy: profile.RuntimeRootPolicy,
		MutationScope: profile.MutationScope,
	}, true, nil
}

type WorkflowContextCapabilities struct {
	Version             string    `json:"version"`
	ExecutableIdentity  bool      `json:"executable_identity"`
	SettingsSchema      bool      `json:"settings_schema"`
	OverlayReadback     bool      `json:"overlay_readback"`
	PreCompactionEvent  bool      `json:"pre_compaction_event"`
	PostCompactionEvent bool      `json:"post_compaction_event"`
	CanonicalInjection  bool      `json:"canonical_injection"`
	AdmissionBlocking   bool      `json:"admission_blocking"`
	NoSession           bool      `json:"no_session"`
	IsolatedTaskRoot    bool      `json:"isolated_task_root"`
	CleanupReadback     bool      `json:"cleanup_readback"`
	MemoryInterception  bool      `json:"memory_interception"`
	AuthNoneLoopback    bool      `json:"auth_none_loopback"`
	ProbeSource         string    `json:"probe_source"`
	CheckedAt           time.Time `json:"checked_at"`
}

type WorkflowContextRuntimeEvent struct {
	Kind               string
	HistoryAfterTokens map[string]int
}

type WorkflowContextProcessDriver interface {
	ArtifactCount(context.Context) (int, error)
	Run(context.Context, func(WorkflowContextRuntimeEvent) error) error
	Cleanup(context.Context) error
}

type WorkflowContextBridgeBinding struct {
	SchemaVersion string `json:"schema_version"`
	BindingHash   string `json:"binding_hash"`
	OptionsHash   string `json:"options_hash"`
	SessionHash   string `json:"session_hash"`
	NonceHash     string `json:"nonce_hash"`
}

type WorkflowContextDispatchAck struct {
	SchemaVersion    string `json:"schema_version"`
	BindingHash      string `json:"binding_hash"`
	OptionsHash      string `json:"options_hash"`
	SessionHash      string `json:"session_hash"`
	NonceHash        string `json:"nonce_hash"`
	ProviderObserved bool   `json:"provider_observed"`
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: managed drivers extend the process contract with bridge binding and observed dispatch acknowledgement.
// @AX:REASON [AUTO]: the supervisor wrapper, concrete RPC driver, and live canary depend on this fail-closed admission interface.
type WorkflowContextManagedProcessDriver interface {
	WorkflowContextProcessDriver
	Bind(context.Context, WorkflowContextBridgeBinding) error
	Dispatch(context.Context, WorkflowContextDispatch) (WorkflowContextDispatchAck, error)
}

type WorkflowContextOverlayRequest struct {
	HistoryMode string
	MemoryMode  string
	Reason      string
}

type WorkflowContextOverlayReadback struct {
	RequestedHistoryMode string `json:"requested_history_mode"`
	EffectiveHistoryMode string `json:"effective_history_mode"`
	EffectiveMemoryMode  string `json:"effective_memory_mode"`
	PreviousHistoryMode  string `json:"previous_history_mode"`
	OverlayHash          string `json:"overlay_hash"`
	ReadbackHash         string `json:"readback_hash"`
	Reason               string `json:"reason,omitempty"`
}

type WorkflowContextOverlayController interface {
	Apply(context.Context, WorkflowContextOverlayRequest) (WorkflowContextOverlayReadback, error)
}

type WorkflowContextCanonicalSource interface {
	Rebuild(context.Context, promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error)
}

type WorkflowContextRuntimeRequest struct {
	Policy          WorkflowContextEffectivePolicy
	Capabilities    WorkflowContextCapabilities
	Binding         promptlayer.OMPContextBindingInput
	RootClass       string
	Driver          WorkflowContextProcessDriver
	Overlay         WorkflowContextOverlayController
	CanonicalSource WorkflowContextCanonicalSource
	Promotion       promptlayer.OMPContextPromotionEvidenceV1
	ReceiptWriter   *WorkflowContextReceiptWriter
}

type WorkflowContextDispatch struct {
	Mode      string
	Delivery  promptlayer.ContextDeliveryResult
	Transient promptlayer.OMPContextTransientView
	Ephemeral promptlayer.OMPContextEphemeral
}

func (WorkflowContextDispatch) MarshalJSON() ([]byte, error) {
	return nil, promptlayer.ErrOMPContextBodySerialization
}

type WorkflowContextDispatchFunc func(context.Context, WorkflowContextDispatch) error

var _ json.Marshaler = WorkflowContextDispatch{}
