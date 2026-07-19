package run

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

const (
	desktopV2ProjectionSchema = "autopus.desktop-observe.semantic-projection.v2"
	desktopV2MaxNodes         = 256
	desktopV2MaxDepth         = 32
	desktopV2MaxSafeInteger   = uint64(9_007_199_254_740_991)
)

var (
	desktopV2RolePattern   = regexp.MustCompile(`^AX[A-Za-z]+$`)
	desktopV2DigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	desktopV2RefPattern    = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,95}$`)
	desktopV2SafeNames     = []string{"Autopus", "Disclosure", "Status"}
)

type desktopV2Projection struct {
	SchemaVersion string          `json:"schema_version"`
	ProviderRef   string          `json:"provider_ref"`
	AppRef        string          `json:"app_ref"`
	WindowRef     string          `json:"window_ref"`
	StateRef      string          `json:"state_ref"`
	Digest        string          `json:"digest"`
	Nodes         []desktopV2Node `json:"-"`
}

type desktopV2ProjectionWire struct {
	SchemaVersion string            `json:"schema_version"`
	ProviderRef   string            `json:"provider_ref"`
	AppRef        string            `json:"app_ref"`
	WindowRef     string            `json:"window_ref"`
	StateRef      string            `json:"state_ref"`
	Digest        string            `json:"digest"`
	Nodes         []json.RawMessage `json:"nodes"`
}

type desktopV2Node struct {
	AdvertisedActions []string
	Children          []desktopV2Node
	Frame             *desktopobserve.Frame
	Name              string
	NodeRef           string
	Occurrence        uint64
	ParentNodeRef     *string
	Role              string
	SemanticState     map[string]any
}

type desktopV2NodeWire struct {
	AdvertisedActions []string                   `json:"advertised_actions"`
	Children          []json.RawMessage          `json:"children"`
	Frame             *desktopV2FrameWire        `json:"frame,omitempty"`
	Name              string                     `json:"name"`
	NodeRef           string                     `json:"node_ref"`
	Occurrence        *uint64                    `json:"occurrence"`
	ParentNodeRef     *string                    `json:"parent_node_ref"`
	Role              string                     `json:"role"`
	SemanticState     map[string]json.RawMessage `json:"semantic_state"`
}

type desktopV2FrameWire struct {
	X      *int `json:"x"`
	Y      *int `json:"y"`
	Width  *int `json:"width"`
	Height *int `json:"height"`
}

type desktopV2ProjectionContext struct {
	count int
	refs  map[string]bool
}

func mapDesktopV2Projection(raw []byte) (desktopobserve.SemanticProjection, error) {
	projection, err := decodeDesktopV2Projection(raw)
	if err != nil || validateDesktopV2Digest(projection) != nil {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	if len(projection.Nodes) != 1 || !validDesktopV2Landmarks(projection.Nodes[0]) {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	publicRoot := mapDesktopV2Node(projection.Nodes[0])
	publicProjection := desktopobserve.SemanticProjection{
		SchemaVersion: desktopobserve.SemanticProjectionSchemaVersion,
		ProviderRef:   "provider-local",
		AppRef:        projection.AppRef,
		WindowRef:     projection.WindowRef,
		StateRef:      "state-" + strings.TrimPrefix(projection.StateRef, "state_v2_"),
		Root:          publicRoot,
	}
	return desktopobserve.NormalizeProjection(publicProjection, func(value string) (string, error) {
		if slices.Contains(desktopV2SafeNames, value) {
			return value, nil
		}
		return "", desktopobserve.ErrRedactionFailed
	})
}

func decodeDesktopV2Projection(raw []byte) (desktopV2Projection, error) {
	var wire desktopV2ProjectionWire
	if err := decodeDesktopV2Object(
		raw, &wire, "schema_version", "provider_ref", "app_ref", "window_ref",
		"state_ref", "digest", "nodes",
	); err != nil {
		return desktopV2Projection{}, err
	}
	if wire.SchemaVersion != desktopV2ProjectionSchema || wire.ProviderRef != "provider_rust_go" ||
		wire.AppRef != "autopus-desktop" || wire.WindowRef != "main-window" ||
		!strings.HasPrefix(wire.StateRef, "state_v2_") ||
		!desktopV2DigestPattern.MatchString(strings.TrimPrefix(wire.StateRef, "state_v2_")) ||
		!desktopV2DigestPattern.MatchString(wire.Digest) || len(wire.Nodes) == 0 {
		return desktopV2Projection{}, desktopobserve.ErrMalformedEnvelope
	}
	context := &desktopV2ProjectionContext{refs: make(map[string]bool)}
	nodes := make([]desktopV2Node, 0, len(wire.Nodes))
	for _, rawNode := range wire.Nodes {
		node, err := decodeDesktopV2Node(rawNode, nil, 1, context)
		if err != nil {
			return desktopV2Projection{}, err
		}
		nodes = append(nodes, node)
	}
	return desktopV2Projection{
		SchemaVersion: wire.SchemaVersion, ProviderRef: wire.ProviderRef, AppRef: wire.AppRef,
		WindowRef: wire.WindowRef, StateRef: wire.StateRef, Digest: wire.Digest, Nodes: nodes,
	}, nil
}

func decodeDesktopV2Node(
	raw []byte,
	parent *string,
	depth int,
	context *desktopV2ProjectionContext,
) (desktopV2Node, error) {
	var wire desktopV2NodeWire
	withFrame := []string{"advertised_actions", "children", "frame", "name", "node_ref", "occurrence", "parent_node_ref", "role", "semantic_state"}
	withoutFrame := []string{"advertised_actions", "children", "name", "node_ref", "occurrence", "parent_node_ref", "role", "semantic_state"}
	if err := decodeDesktopV2Object(raw, &wire, withFrame...); err != nil {
		if err := decodeDesktopV2Object(raw, &wire, withoutFrame...); err != nil {
			return desktopV2Node{}, err
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || wire.Children == nil || wire.Occurrence == nil ||
		bytes.Equal(bytes.TrimSpace(fields["frame"]), []byte("null")) {
		return desktopV2Node{}, desktopobserve.ErrMalformedEnvelope
	}
	context.count++
	if depth > desktopV2MaxDepth || context.count > desktopV2MaxNodes ||
		!desktopV2RefPattern.MatchString(wire.NodeRef) || context.refs[wire.NodeRef] ||
		!sameDesktopV2Parent(wire.ParentNodeRef, parent) || *wire.Occurrence > desktopV2MaxSafeInteger ||
		!desktopV2RolePattern.MatchString(wire.Role) || len(wire.Name) > 512 ||
		!slices.Contains(desktopV2SafeNames, wire.Name) || !validDesktopV2Actions(wire.AdvertisedActions) ||
		!validDesktopV2FrameWire(wire.Frame) {
		return desktopV2Node{}, desktopobserve.ErrMalformedEnvelope
	}
	context.refs[wire.NodeRef] = true
	state, err := decodeDesktopV2State(wire.SemanticState)
	if err != nil {
		return desktopV2Node{}, err
	}
	children := make([]desktopV2Node, 0, len(wire.Children))
	for _, rawChild := range wire.Children {
		child, err := decodeDesktopV2Node(rawChild, &wire.NodeRef, depth+1, context)
		if err != nil {
			return desktopV2Node{}, err
		}
		children = append(children, child)
	}
	return desktopV2Node{
		AdvertisedActions: wire.AdvertisedActions, Children: children, Frame: mapDesktopV2Frame(wire.Frame),
		Name: wire.Name, NodeRef: wire.NodeRef, Occurrence: *wire.Occurrence,
		ParentNodeRef: wire.ParentNodeRef, Role: wire.Role, SemanticState: state,
	}, nil
}

func decodeDesktopV2State(raw map[string]json.RawMessage) (map[string]any, error) {
	if raw == nil {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	state := make(map[string]any, len(raw))
	for key, value := range raw {
		if key == "checked" {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		if !slices.Contains([]string{"enabled", "expanded", "focused", "selected", "visible"}, key) {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		trimmed := bytes.TrimSpace(value)
		if !bytes.Equal(trimmed, []byte("true")) && !bytes.Equal(trimmed, []byte("false")) {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		boolean := bytes.Equal(trimmed, []byte("true"))
		state[key] = boolean
	}
	return state, nil
}

func mapDesktopV2Node(node desktopV2Node) desktopobserve.SemanticNode {
	state := desktopobserve.SemanticState{}
	for key, raw := range node.SemanticState {
		value := raw.(bool)
		switch key {
		case "enabled":
			state.Enabled = &value
		case "expanded":
			state.Expanded = &value
		case "focused":
			state.Focused = &value
		case "selected":
			state.Selected = &value
		}
	}
	children := make([]desktopobserve.SemanticNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, mapDesktopV2Node(child))
	}
	actions := make([]desktopobserve.Action, 0, len(node.AdvertisedActions))
	for _, action := range node.AdvertisedActions {
		actions = append(actions, desktopobserve.Action(action))
	}
	return desktopobserve.SemanticNode{
		Role: desktopobserve.Role(node.Role), Name: node.Name, SemanticState: state,
		Frame: node.Frame, AdvertisedActions: actions, Children: children,
	}
}
