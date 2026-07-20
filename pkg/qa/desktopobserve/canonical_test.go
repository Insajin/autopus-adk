package desktopobserve

import (
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeProjection_NFCLineEndingsActionsAndMultiplicity(t *testing.T) {
	t.Parallel()

	projection := semanticFixture()
	projection.Root.Name = "Autopus\r\nCafe\u0301"
	projection.Root.AdvertisedActions = []Action{ActionShowMenu, ActionPress, ActionPress}
	duplicate := projection.Root.Children[0]
	duplicate.NodeRef = "node-provider-duplicate"
	projection.Root.Children = append(projection.Root.Children, duplicate)

	normalized, err := NormalizeProjection(projection, identityRedactor)
	require.NoError(t, err)
	assert.Equal(t, "Autopus\nCafé", normalized.Root.Name)
	assert.Equal(t, []Action{ActionPress, ActionShowMenu}, normalized.Root.AdvertisedActions)
	require.Len(t, normalized.Root.Children, 2)
	assert.NotEqual(t, normalized.Root.Children[0].NodeRef, normalized.Root.Children[1].NodeRef)
	assert.NotEmpty(t, normalized.Digest)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}$`), normalized.Digest)
}

func TestNormalizeProjection_RedactsBeforeDigestAndEmitsNoPayloadOnFailure(t *testing.T) {
	t.Parallel()

	raw := semanticFixture()
	raw.Root.Name = "private account title"
	redacted, err := NormalizeProjection(raw, func(value string) (string, error) {
		if value == "private account title" {
			return "Autopus", nil
		}
		return value, nil
	})
	require.NoError(t, err)
	allowed, err := NormalizeProjection(semanticFixture(), identityRedactor)
	require.NoError(t, err)
	assert.Equal(t, allowed.Digest, redacted.Digest)
	assert.Equal(t, "Autopus", redacted.Root.Name)

	failed, err := NormalizeProjection(raw, func(string) (string, error) {
		return "", errors.New("redaction failed")
	})
	require.Error(t, err)
	assert.Empty(t, failed.Digest)
	assert.Empty(t, failed.Root)
}

func TestNormalizeProjection_RefsFrameAndProviderOrderDoNotAffectDigest(t *testing.T) {
	t.Parallel()

	first := semanticFixture()
	second := semanticFixture()
	second.ProviderRef = "provider-other"
	second.AppRef = "app-other"
	second.WindowRef = "window-other"
	second.StateRef = "state-other"
	second.Root.NodeRef = "root-other"
	second.Root.Frame = &Frame{X: 900, Y: 400, Width: 1, Height: 2}
	second.Root.Children[0].NodeRef = "child-other"
	second.Root.Children[0].Frame = &Frame{X: 19, Y: 27, Width: 901, Height: 703}
	second.Root.Children = append(second.Root.Children, SemanticNode{
		NodeRef:       "status-other",
		Role:          RoleStaticText,
		Name:          "Ready",
		SemanticState: SemanticState{Enabled: boolPointer(true)},
	})
	first.Root.Children = append([]SemanticNode{second.Root.Children[1]}, first.Root.Children...)
	assert.Equal(t, RoleStaticText, first.Root.Children[0].Role)
	assert.Equal(t, RoleWindow, second.Root.Children[0].Role)

	normalizedFirst, err := NormalizeProjection(first, identityRedactor)
	require.NoError(t, err)
	normalizedSecond, err := NormalizeProjection(second, identityRedactor)
	require.NoError(t, err)
	assert.Equal(t, normalizedFirst.Digest, normalizedSecond.Digest)
	assert.Equal(t, normalizedFirst.CanonicalJSON, normalizedSecond.CanonicalJSON)
}

func TestNormalizeProjection_RejectsOutOfRangeWindowRelativeFrames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		frame Frame
	}{
		{name: "negative x", frame: Frame{X: -1, Y: 0, Width: 1, Height: 1}},
		{name: "negative y", frame: Frame{X: 0, Y: -1, Width: 1, Height: 1}},
		{name: "zero width", frame: Frame{X: 0, Y: 0, Width: 0, Height: 1}},
		{name: "zero height", frame: Frame{X: 0, Y: 0, Width: 1, Height: 0}},
		{name: "x above bound", frame: Frame{X: 100_001, Y: 0, Width: 1, Height: 1}},
		{name: "height above bound", frame: Frame{X: 0, Y: 0, Width: 1, Height: 100_001}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			projection := semanticFixture()
			projection.Root.Frame = &test.frame
			_, err := NormalizeProjection(projection, identityRedactor)
			require.ErrorIs(t, err, ErrMalformedEnvelope)
		})
	}
}

func TestNormalizeProjection_SemanticChangeChangesAndRecoveryRestoresDigest(t *testing.T) {
	t.Parallel()

	before := semanticFixture()
	before.Root.Children[0].SemanticState.Expanded = boolPointer(false)
	changed := semanticFixture()
	changed.Root.Children[0].SemanticState.Expanded = boolPointer(true)
	recovered := semanticFixture()
	recovered.Root.Children[0].SemanticState.Expanded = boolPointer(false)

	normalizedBefore, err := NormalizeProjection(before, identityRedactor)
	require.NoError(t, err)
	normalizedChanged, err := NormalizeProjection(changed, identityRedactor)
	require.NoError(t, err)
	normalizedRecovered, err := NormalizeProjection(recovered, identityRedactor)
	require.NoError(t, err)

	assert.NotEqual(t, normalizedBefore.Digest, normalizedChanged.Digest)
	assert.Equal(t, normalizedBefore.Digest, normalizedRecovered.Digest)
}

func TestNormalizeProjection_UnknownAdvertisedActionFailsClosed(t *testing.T) {
	t.Parallel()

	projection := semanticFixture()
	projection.Root.AdvertisedActions = []Action{ActionPress, Action("AXUnsafe")}
	normalized, err := NormalizeProjection(projection, identityRedactor)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedEnvelope)
	assert.Empty(t, normalized.Digest)
	assert.Empty(t, normalized.Root)
}

func semanticFixture() SemanticProjection {
	return SemanticProjection{
		SchemaVersion: SemanticProjectionSchemaVersion,
		ProviderRef:   "provider-local",
		AppRef:        "autopus-desktop",
		WindowRef:     "main-window",
		StateRef:      "state-001",
		Root: SemanticNode{
			NodeRef:           "node-app",
			Role:              RoleApplication,
			Name:              "Autopus",
			SemanticState:     SemanticState{Enabled: boolPointer(true)},
			Frame:             &Frame{X: 0, Y: 0, Width: 1440, Height: 900},
			AdvertisedActions: []Action{ActionShowMenu, ActionPress},
			Children: []SemanticNode{{
				NodeRef:           "node-window",
				Role:              RoleWindow,
				Name:              "Autopus",
				SemanticState:     SemanticState{Focused: boolPointer(true)},
				Frame:             &Frame{X: 20, Y: 30, Width: 900, Height: 700},
				AdvertisedActions: []Action{ActionRaise},
			}},
		},
	}
}

func identityRedactor(value string) (string, error) {
	return value, nil
}

func boolPointer(value bool) *bool {
	return &value
}
