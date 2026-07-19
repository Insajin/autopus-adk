package desktopobserve

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	canonicalSchemaVersion = "qamesh.desktop_observation.canonical.v1"
	maxSemanticNodes       = 256
	maxSemanticDepth       = 32
	maxFrameLogicalPoint   = 100_000
)

var (
	canonicalAXNamePattern = regexp.MustCompile(`^AX[A-Za-z]+$`)
	canonicalDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Redactor func(string) (string, error)

type canonicalProjection struct {
	Root          canonicalNode `json:"root"`
	SchemaVersion string        `json:"schema_version"`
}

type canonicalNode struct {
	AdvertisedActions []Action        `json:"advertised_actions"`
	Children          []canonicalNode `json:"children"`
	Name              string          `json:"name"`
	Occurrence        int             `json:"occurrence"`
	Role              Role            `json:"role"`
	SemanticState     SemanticState   `json:"semantic_state"`
}

type preparedNode struct {
	public    SemanticNode
	canonical canonicalNode
	primary   string
	full      string
}

func NormalizeProjection(projection SemanticProjection, redact Redactor) (SemanticProjection, error) {
	if redact == nil {
		return SemanticProjection{}, ErrRedactionFailed
	}
	if projection.SchemaVersion != SemanticProjectionSchemaVersion ||
		!safePublicRef(projection.ProviderRef) || !safePublicRef(projection.AppRef) ||
		!safePublicRef(projection.WindowRef) || !safePublicRef(projection.StateRef) {
		return SemanticProjection{}, fmt.Errorf("%w: projection scope", ErrMalformedEnvelope)
	}
	count := 0
	prepared, err := prepareNode(projection.Root, redact, 1, &count)
	if err != nil {
		return SemanticProjection{}, err
	}
	canonical := canonicalProjection{
		SchemaVersion: canonicalSchemaVersion,
		Root:          prepared.canonical,
	}
	canonicalJSON, _ := json.Marshal(canonical)
	digestBytes := sha256.Sum256(canonicalJSON)
	publicScope := strings.Join([]string{
		projection.ProviderRef, projection.AppRef, projection.WindowRef, projection.StateRef,
	}, "\x00")
	assignNodeRefs(&prepared.public, prepared.canonical, publicScope)
	return SemanticProjection{
		SchemaVersion: projection.SchemaVersion,
		ProviderRef:   projection.ProviderRef,
		AppRef:        projection.AppRef,
		WindowRef:     projection.WindowRef,
		StateRef:      projection.StateRef,
		Digest:        hex.EncodeToString(digestBytes[:]),
		Root:          prepared.public,
		CanonicalJSON: canonicalJSON,
	}, nil
}

func prepareNode(node SemanticNode, redact Redactor, depth int, count *int) (preparedNode, error) {
	*count++
	if depth > maxSemanticDepth || *count > maxSemanticNodes || !validRole(node.Role) ||
		!validFrame(node.Frame) {
		return preparedNode{}, fmt.Errorf("%w: semantic node bounds", ErrMalformedEnvelope)
	}
	name, err := normalizeText(node.Name)
	if err != nil {
		return preparedNode{}, err
	}
	name, err = redact(name)
	if err != nil {
		return preparedNode{}, fmt.Errorf("%w: name", ErrRedactionFailed)
	}
	name, err = normalizeText(name)
	if err != nil {
		return preparedNode{}, err
	}
	state, err := normalizeState(node.SemanticState)
	if err != nil {
		return preparedNode{}, err
	}
	actions, err := normalizeActions(node.AdvertisedActions)
	if err != nil {
		return preparedNode{}, err
	}
	children := make([]preparedNode, 0, len(node.Children))
	for _, child := range node.Children {
		prepared, childErr := prepareNode(child, redact, depth+1, count)
		if childErr != nil {
			return preparedNode{}, childErr
		}
		children = append(children, prepared)
	}
	sort.SliceStable(children, func(left, right int) bool {
		if comparison := compareCanonicalText(children[left].primary, children[right].primary); comparison != 0 {
			return comparison < 0
		}
		return compareCanonicalText(children[left].full, children[right].full) < 0
	})
	publicChildren := make([]SemanticNode, 0, len(children))
	canonicalChildren := make([]canonicalNode, 0, len(children))
	occurrences := make(map[string]int)
	for _, child := range children {
		child.canonical.Occurrence = occurrences[child.primary]
		occurrences[child.primary]++
		child.full = marshalCanonicalKey(child.canonical)
		publicChildren = append(publicChildren, child.public)
		canonicalChildren = append(canonicalChildren, child.canonical)
	}
	public := SemanticNode{
		Role: node.Role, Name: name, SemanticState: state, Frame: copyFrame(node.Frame),
		AdvertisedActions: actions, Children: publicChildren,
	}
	canonicalNode := canonicalNode{
		Role: node.Role, Name: name, SemanticState: state,
		AdvertisedActions: actions, Children: canonicalChildren,
	}
	return preparedNode{
		public: public, canonical: canonicalNode, primary: canonicalIdentity(canonicalNode),
		full: marshalCanonicalKey(canonicalNode),
	}, nil
}

func assignNodeRefs(public *SemanticNode, canonical canonicalNode, parentPath string) {
	identity := fmt.Sprintf("%s\x00%s\x00%d", parentPath, canonicalIdentity(canonical), canonical.Occurrence)
	digest := sha256.Sum256([]byte(identity))
	public.NodeRef = "n_" + hex.EncodeToString(digest[:])
	for index := range public.Children {
		assignNodeRefs(&public.Children[index], canonical.Children[index], public.NodeRef)
	}
}

func canonicalIdentity(node canonicalNode) string {
	return marshalCanonicalKey([]any{
		node.Role, node.Name, node.SemanticState, node.AdvertisedActions,
	})
}

func normalizeActions(actions []Action) ([]Action, error) {
	values := make([]Action, 0, len(actions))
	for _, action := range actions {
		normalized, err := normalizeText(string(action))
		if err != nil || !validAction(Action(normalized)) {
			return nil, fmt.Errorf("%w: action", ErrMalformedEnvelope)
		}
		values = append(values, Action(normalized))
	}
	sort.Slice(values, func(left, right int) bool {
		return compareCanonicalText(string(values[left]), string(values[right])) < 0
	})
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result, nil
}

func validAction(action Action) bool {
	return action == ActionPress || action == ActionRaise || action == ActionShowMenu
}

func normalizeState(state SemanticState) (SemanticState, error) {
	return SemanticState{
		Enabled:  copyPointer(state.Enabled),
		Expanded: copyPointer(state.Expanded),
		Focused:  copyPointer(state.Focused),
		Selected: copyPointer(state.Selected),
	}, nil
}

func normalizeText(value string) (string, error) {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("%w: invalid text", ErrRedactionFailed)
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return norm.NFC.String(value), nil
}

func compareCanonicalText(left, right string) int {
	return slices.Compare(utf16.Encode([]rune(left)), utf16.Encode([]rune(right)))
}

func marshalCanonicalKey(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func copyFrame(frame *Frame) *Frame {
	return copyPointer(frame)
}

func validFrame(frame *Frame) bool {
	return frame == nil || frame.X >= 0 && frame.Y >= 0 && frame.Width > 0 && frame.Height > 0 &&
		frame.X <= maxFrameLogicalPoint && frame.Y <= maxFrameLogicalPoint &&
		frame.Width <= maxFrameLogicalPoint && frame.Height <= maxFrameLogicalPoint
}

func copyPointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func validRole(role Role) bool {
	return canonicalAXNamePattern.MatchString(string(role))
}

func validateProjection(projection SemanticProjection) error {
	if !canonicalDigestPattern.MatchString(projection.Digest) {
		return ErrMalformedEnvelope
	}
	normalized, err := NormalizeProjection(projection, func(value string) (string, error) { return value, nil })
	if err != nil || normalized.Digest != projection.Digest ||
		marshalCanonicalKey(normalized.Root) != marshalCanonicalKey(projection.Root) {
		return ErrMalformedEnvelope
	}
	return nil
}
