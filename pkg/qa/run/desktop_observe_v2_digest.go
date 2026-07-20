package run

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/unicode/norm"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

type desktopV2CanonicalContent struct {
	Nodes         []desktopV2CanonicalNode `json:"nodes"`
	SchemaVersion string                   `json:"schema_version"`
}

type desktopV2CanonicalNode struct {
	AdvertisedActions []string                 `json:"advertised_actions"`
	Children          []desktopV2CanonicalNode `json:"children"`
	Name              string                   `json:"name"`
	Occurrence        uint64                   `json:"occurrence"`
	Role              string                   `json:"role"`
	SemanticState     map[string]any           `json:"semantic_state"`
}

type desktopV2CanonicalBase struct {
	AdvertisedActions []string                 `json:"advertised_actions"`
	Children          []desktopV2CanonicalNode `json:"children"`
	Name              string                   `json:"name"`
	Role              string                   `json:"role"`
	SemanticState     map[string]any           `json:"semantic_state"`
}

type desktopV2PreparedCanonical struct {
	base    desktopV2CanonicalBase
	primary string
	sortKey string
}

func validateDesktopV2Digest(projection desktopV2Projection) error {
	canonical, err := desktopV2CanonicalBytes(projection)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != projection.Digest {
		return desktopobserve.ErrMalformedEnvelope
	}
	return nil
}

func desktopV2CanonicalBytes(projection desktopV2Projection) ([]byte, error) {
	nodes, err := canonicalizeDesktopV2Siblings(projection.Nodes)
	if err != nil {
		return nil, err
	}
	return json.Marshal(desktopV2CanonicalContent{
		Nodes: nodes, SchemaVersion: "autopus.desktop-observe.canonical.v2",
	})
}

func canonicalizeDesktopV2Siblings(nodes []desktopV2Node) ([]desktopV2CanonicalNode, error) {
	prepared := make([]desktopV2PreparedCanonical, 0, len(nodes))
	for _, node := range nodes {
		actions := make([]string, 0, len(node.AdvertisedActions))
		for _, action := range node.AdvertisedActions {
			actions = append(actions, normalizeDesktopV2Text(action))
		}
		sort.Slice(actions, func(i, j int) bool { return compareDesktopV2Text(actions[i], actions[j]) < 0 })
		actions = slices.Compact(actions)
		children, err := canonicalizeDesktopV2Siblings(node.Children)
		if err != nil {
			return nil, err
		}
		state := make(map[string]any, len(node.SemanticState))
		for key, value := range node.SemanticState {
			state[normalizeDesktopV2Text(key)] = value
		}
		base := desktopV2CanonicalBase{
			AdvertisedActions: actions, Children: children, Name: normalizeDesktopV2Text(node.Name),
			Role: normalizeDesktopV2Text(node.Role), SemanticState: state,
		}
		primaryRaw, err := json.Marshal([]any{base.Role, base.Name, base.SemanticState, base.AdvertisedActions})
		if err != nil {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		sortRaw, err := json.Marshal(base)
		if err != nil {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		prepared = append(prepared, desktopV2PreparedCanonical{
			base: base, primary: string(primaryRaw), sortKey: string(sortRaw),
		})
	}
	sort.SliceStable(prepared, func(i, j int) bool {
		if comparison := compareDesktopV2Text(prepared[i].primary, prepared[j].primary); comparison != 0 {
			return comparison < 0
		}
		return compareDesktopV2Text(prepared[i].sortKey, prepared[j].sortKey) < 0
	})
	occurrences := make(map[string]uint64)
	canonical := make([]desktopV2CanonicalNode, 0, len(prepared))
	for _, item := range prepared {
		canonical = append(canonical, desktopV2CanonicalNode{
			AdvertisedActions: item.base.AdvertisedActions, Children: item.base.Children,
			Name: item.base.Name, Occurrence: occurrences[item.primary], Role: item.base.Role,
			SemanticState: item.base.SemanticState,
		})
		occurrences[item.primary]++
	}
	return canonical, nil
}

func normalizeDesktopV2Text(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return norm.NFC.String(value)
}

func compareDesktopV2Text(left, right string) int {
	return slices.Compare(utf16.Encode([]rune(left)), utf16.Encode([]rune(right)))
}
