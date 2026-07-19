package desktopobserve

import "encoding/json"

const (
	ProtocolVersion                 = 1
	MaxEnvelopeBytes                = 8_192
	RuntimeReceiptSchemaVersion     = "qamesh.runtime_receipt.v1"
	SemanticProjectionSchemaVersion = "qamesh.desktop_observation.semantic_projection.v1"
)

type Operation string

const (
	OperationCapabilities Operation = "capabilities"
	OperationGetState     Operation = "get_state"
	OperationListApps     Operation = "list_apps"
	OperationListWindows  Operation = "list_windows"
	OperationPermissions  Operation = "permissions"
)

type RuntimeProvider string

const (
	RuntimeProviderLocal RuntimeProvider = "local"
	RuntimeProviderOrca  RuntimeProvider = "orca"
)

type ScopeKind string

const (
	ScopeProvider    ScopeKind = "provider"
	ScopeApplication ScopeKind = "application"
	ScopeWindow      ScopeKind = "window"
	ScopeState       ScopeKind = "state"
)

type ReceiptScope struct {
	Kind      ScopeKind `json:"kind"`
	PublicRef string    `json:"public_ref"`
}

type ProviderIdentity struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
}

type CapabilityState string

const (
	CapabilitySupported   CapabilityState = "supported"
	CapabilityUnsupported CapabilityState = "unsupported"
)

type CapabilityStatus struct {
	Name   Operation       `json:"name"`
	Status CapabilityState `json:"status"`
}

type RedactionStatus string

const (
	RedactionApplied     RedactionStatus = "applied"
	RedactionNotRequired RedactionStatus = "not_required"
	RedactionFailed      RedactionStatus = "failed"
)

type QuarantineStatus string

const (
	QuarantineEmpty     QuarantineStatus = "empty"
	QuarantineLocalOnly QuarantineStatus = "local_only"
	QuarantineCleared   QuarantineStatus = "cleared"
	QuarantineBlocked   QuarantineStatus = "blocked"
)

type RedactionReceipt struct {
	Status RedactionStatus `json:"status"`
}

type QuarantineReceipt struct {
	Status QuarantineStatus `json:"status"`
}

type Role string

const (
	RoleApplication Role = "AXApplication"
	RoleCanvas      Role = "AXCanvas"
	RoleGroup       Role = "AXGroup"
	RoleStaticText  Role = "AXStaticText"
	RoleWindow      Role = "AXWindow"
)

type Action string

const (
	ActionPress    Action = "AXPress"
	ActionRaise    Action = "AXRaise"
	ActionShowMenu Action = "AXShowMenu"
)

type Frame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type SemanticState struct {
	Enabled  *bool `json:"enabled,omitempty"`
	Expanded *bool `json:"expanded,omitempty"`
	Focused  *bool `json:"focused,omitempty"`
	Selected *bool `json:"selected,omitempty"`
}

type SemanticNode struct {
	NodeRef           string         `json:"node_ref"`
	Role              Role           `json:"role"`
	Name              string         `json:"name"`
	SemanticState     SemanticState  `json:"semantic_state"`
	Frame             *Frame         `json:"frame,omitempty"`
	AdvertisedActions []Action       `json:"advertised_actions,omitempty"`
	Children          []SemanticNode `json:"children,omitempty"`
}

type SemanticProjection struct {
	SchemaVersion string       `json:"schema_version"`
	ProviderRef   string       `json:"provider_ref"`
	AppRef        string       `json:"app_ref"`
	WindowRef     string       `json:"window_ref"`
	StateRef      string       `json:"state_ref"`
	Digest        string       `json:"digest"`
	Root          SemanticNode `json:"root"`
	CanonicalJSON []byte       `json:"-"`
}

type ObservationEvidence struct {
	SemanticProjection  *SemanticProjection  `json:"semantic_projection,omitempty"`
	DeterministicChecks []DeterministicCheck `json:"deterministic_checks"`
	RuntimeReceipt      RuntimeReceipt       `json:"runtime_receipt"`
}

type PermissionResult struct {
	AccessibilityGranted bool `json:"accessibility_granted"`
}

type AppSummary struct {
	AppRef string `json:"app_ref"`
}

type WindowSummary struct {
	WindowRef string `json:"window_ref"`
}

type Request struct {
	ProtocolVersion  int          `json:"protocol_version"`
	RequestID        string       `json:"request_id"`
	Operation        Operation    `json:"operation"`
	Scope            ReceiptScope `json:"scope"`
	ExpectedStateRef string       `json:"expected_state_ref,omitempty"`
}

type ResultStatus string

const (
	ResultPassed ResultStatus = "passed"
	ResultFailed ResultStatus = "failed"
)

type Result struct {
	ProtocolVersion int             `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Status          ResultStatus    `json:"status"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	RuntimeReceipt  RuntimeReceipt  `json:"runtime_receipt"`
}

type Exchange struct {
	Request Request
	Result  Result
}

type SemanticStateKey string

const (
	StateEnabled  SemanticStateKey = "enabled"
	StateFocused  SemanticStateKey = "focused"
	StateSelected SemanticStateKey = "selected"
	StateExpanded SemanticStateKey = "expanded"
)
