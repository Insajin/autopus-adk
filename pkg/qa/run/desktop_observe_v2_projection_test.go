package run

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestDesktopV2CanonicalDigest_MatchesRustGoldenVector(t *testing.T) {
	t.Parallel()
	appRef := "application_fixture_00"
	windowRef := "window_fixture_00"
	projection := desktopV2Projection{Nodes: []desktopV2Node{{
		AdvertisedActions: []string{}, Name: "Autopus", NodeRef: appRef, Role: "AXApplication",
		SemanticState: map[string]any{"enabled": true},
		Children: []desktopV2Node{{
			AdvertisedActions: []string{}, Name: "Autopus", NodeRef: windowRef,
			ParentNodeRef: &appRef, Role: "AXWindow", SemanticState: map[string]any{"focused": true},
			Children: []desktopV2Node{
				{AdvertisedActions: []string{}, Name: "Cafe\u0301\r\nReady", NodeRef: "text_fixture_00",
					Occurrence: 7, ParentNodeRef: &windowRef, Role: "AXStaticText",
					SemanticState: map[string]any{"visible": true}},
				{AdvertisedActions: []string{"AXPress"}, Name: "Retry", NodeRef: "retry_fixture_01",
					Occurrence: 9, ParentNodeRef: &windowRef, Role: "AXButton",
					SemanticState: map[string]any{"enabled": true}},
				{AdvertisedActions: []string{"AXPress"}, Name: "Retry", NodeRef: "retry_fixture_00",
					Occurrence: 4, ParentNodeRef: &windowRef, Role: "AXButton",
					SemanticState: map[string]any{"enabled": true}},
			},
		}},
	}}}
	canonical, err := desktopV2CanonicalBytes(projection)
	require.NoError(t, err)
	digest := sha256.Sum256(canonical)
	assert.Equal(t, "8e7104e0c5dfe7f39ae3a2aa9282020b5cc1d6375f231d6a9ef1fb73cc6d2ce4", hex.EncodeToString(digest[:]))
}

func TestDesktopV2ProjectionAdapter_PreservesRecursiveMultiplicityAndSemanticChange(t *testing.T) {
	t.Parallel()
	beforeRaw := desktopV2ProjectionResult(t, desktopV2RecursiveRoot(false))
	afterRaw := desktopV2ProjectionResult(t, desktopV2RecursiveRoot(true))
	before, err := mapDesktopV2Projection(beforeRaw)
	require.NoError(t, err)
	after, err := mapDesktopV2Projection(afterRaw)
	require.NoError(t, err)
	assert.NotEqual(t, before.Digest, after.Digest)
	assert.Equal(t, 3, countPublicDesktopNodesByName(before.Root, "Disclosure"))
	refs := collectPublicDesktopRefs(before.Root, nil)
	assert.Len(t, refs, 6)

	visibleRoot := desktopV2RecursiveRoot(false)
	visibleRoot.Children[0].Children[1].SemanticState["visible"] = false
	visibleRaw := desktopV2ProjectionResult(t, visibleRoot)
	visibleProjection, err := mapDesktopV2Projection(visibleRaw)
	require.NoError(t, err)
	assert.Equal(t, before.Digest, visibleProjection.Digest, "visible is the explicit safe private-only state")
	assert.NotEqual(t, privateDesktopV2Digest(t, beforeRaw), privateDesktopV2Digest(t, visibleRaw))
}

func TestDesktopV2ProjectionAdapter_RejectsTamperingAndUnmappableTreeData(t *testing.T) {
	t.Parallel()
	valid := desktopV2ProjectionResult(t, desktopV2RecursiveRoot(false))
	framedRoot := desktopV2RecursiveRoot(false)
	framedRoot.Children[0].Frame = &desktopobserve.Frame{X: 4, Y: 8, Width: 800, Height: 600}
	withFrame := desktopV2ProjectionResult(t, framedRoot)
	tests := []struct {
		name string
		raw  []byte
	}{
		{"private digest tamper", bytes.Replace(valid, []byte(`"digest":"`), []byte(`"digest":"f`), 1)},
		{"unknown safe name", bytes.Replace(valid, []byte(`"name":"Status"`), []byte(`"name":"Private User Title"`), 1)},
		{"unmappable checked state", bytes.Replace(valid, []byte(`"visible":true`), []byte(`"checked":"mixed"`), 1)},
		{"null boolean state", bytes.Replace(valid, []byte(`"expanded":false`), []byte(`"expanded":null`), 1)},
		{"null occurrence", bytes.Replace(valid, []byte(`"occurrence":0`), []byte(`"occurrence":null`), 1)},
		{"null frame coordinate", bytes.Replace(withFrame, []byte(`"x":4`), []byte(`"x":null`), 1)},
		{"missing frame coordinate", bytes.Replace(withFrame, []byte(`"x":4,`), nil, 1)},
		{"unknown action", bytes.Replace(valid, []byte(`"AXPress"`), []byte(`"AXDelete"`), 1)},
		{"duplicate node ref", bytes.Replace(valid, []byte(`disclosure_fixture_01`), []byte(`disclosure_fixture_00`), 1)},
		{"broken parent ref", bytes.Replace(valid, []byte(`"parent_node_ref":"group_fixture_00"`), []byte(`"parent_node_ref":"window_fixture_00"`), 1)},
		{"unknown node field", bytes.Replace(valid, []byte(`"name":"Status"`), []byte(`"pid":4242,"name":"Status"`), 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projection, err := mapDesktopV2Projection(test.raw)
			assert.Error(t, err)
			assert.Empty(t, projection.Digest)
			assert.NotContains(t, err.Error(), "4242")
		})
	}
}

func privateDesktopV2Digest(t *testing.T, raw []byte) string {
	t.Helper()
	var wire desktopV2TestProjection
	require.NoError(t, json.Unmarshal(raw, &wire))
	return wire.Digest
}

func collectPublicDesktopRefs(node desktopobserve.SemanticNode, refs []string) []string {
	refs = append(refs, node.NodeRef)
	for _, child := range node.Children {
		refs = collectPublicDesktopRefs(child, refs)
	}
	return refs
}
