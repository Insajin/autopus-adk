package journey

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// canonicalLandmarks is the mandatory pair every desktop observation pack must
// open with.
func canonicalLandmarks() []DesktopObservationLandmark {
	return []DesktopObservationLandmark{
		{Role: "AXApplication", Name: "Slack", RequiredState: "enabled"},
		{Role: "AXWindow", Name: "maker-v2(채널) - Aligo - Slack", RequiredState: "focused"},
	}
}

func landmarkError(t *testing.T, landmarks []DesktopObservationLandmark) error {
	t.Helper()
	return validateDesktopObservationLandmarks(landmarks, func(message string) error {
		return &ValidationError{Code: "test", Message: message}
	})
}

// SPEC-QAMESH-013 REQ-4 selects published nodes by declared landmark, so a pack
// must be able to declare more than the canonical pair. Exactly two was inherited
// from the fixture era, when the projection was three synthesized nodes, and it
// made the deeper-landmark path unreachable from any valid pack.
func TestDesktopObservationLandmarks_AcceptsAdditionalLandmarks(t *testing.T) {
	t.Parallel()

	landmarks := append(canonicalLandmarks(),
		DesktopObservationLandmark{Role: "AXButton", Name: "도움말", RequiredState: "enabled"},
		DesktopObservationLandmark{Role: "AXCell", Name: "즐겨찾기", RequiredState: "selected"},
	)
	require.NoError(t, landmarkError(t, landmarks))
}

func TestDesktopObservationLandmarks_AcceptsCanonicalPairAlone(t *testing.T) {
	t.Parallel()
	require.NoError(t, landmarkError(t, canonicalLandmarks()))
}

// The pair stays mandatory and positional: app and window identity are derived
// from it, so a pack that omits or reorders it cannot be observed.
func TestDesktopObservationLandmarks_RequiresCanonicalPairFirst(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		landmarks []DesktopObservationLandmark
		want      string
	}{
		"empty": {nil, "must begin with"},
		"application only": {
			canonicalLandmarks()[:1], "must begin with",
		},
		"reordered": {
			[]DesktopObservationLandmark{canonicalLandmarks()[1], canonicalLandmarks()[0]},
			"required_landmarks[AXApplication]",
		},
		"window state wrong": {
			[]DesktopObservationLandmark{
				canonicalLandmarks()[0],
				{Role: "AXWindow", Name: "W", RequiredState: "enabled"},
			},
			"required_landmarks[AXWindow]",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := landmarkError(t, tc.landmarks)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// An additional landmark may only require a state the provider's rendered tree
// actually reports. Accepting an unobservable state would let a pack fail on
// something the harness never looks for.
func TestDesktopObservationLandmarks_RejectsMalformedAdditionalLandmark(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		extra DesktopObservationLandmark
		want  string
	}{
		"unobservable state": {
			DesktopObservationLandmark{Role: "AXButton", Name: "Help", RequiredState: "pressed"},
			"required_state must be one of",
		},
		"empty state": {
			DesktopObservationLandmark{Role: "AXButton", Name: "Help"},
			"required_state must be one of",
		},
		"role with a space": {
			DesktopObservationLandmark{Role: "AX Button", Name: "Help", RequiredState: "enabled"},
			"landmark role must be a short alias",
		},
		"empty role": {
			DesktopObservationLandmark{Name: "Help", RequiredState: "enabled"},
			"landmark role must be a short alias",
		},
		"empty name": {
			DesktopObservationLandmark{Role: "AXButton", RequiredState: "enabled"},
			"landmark name must be",
		},
		"multiline name": {
			DesktopObservationLandmark{Role: "AXButton", Name: "Help\nMe", RequiredState: "enabled"},
			"landmark name must be",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := landmarkError(t, append(canonicalLandmarks(), tc.extra))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// The published projection carries one node per declared landmark, so the count
// is what keeps it under the 8 KiB typed evidence bound regardless of app size.
func TestDesktopObservationLandmarks_BoundsLandmarkCount(t *testing.T) {
	t.Parallel()

	landmarks := canonicalLandmarks()
	for len(landmarks) <= desktopObservationLandmarkMax {
		landmarks = append(landmarks, DesktopObservationLandmark{
			Role: "AXButton", Name: "Help", RequiredState: "enabled",
		})
	}
	err := landmarkError(t, landmarks)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most")
}

// A real window title contains a non-breaking space. Go's unicode.IsPrint counts
// only the ASCII space as printable among spaces, so the measured Finder title
// was rejected as unprintable and the landmark became undeclarable.
func TestDesktopObservationLandmarks_AcceptsUnicodeSpaceInName(t *testing.T) {
	t.Parallel()

	landmarks := canonicalLandmarks()
	landmarks[1].Name = "맥북판매의 Mac\u00a0Studio"
	require.NoError(t, landmarkError(t, landmarks))

	landmarks[1].Name = "line\u2028break"
	require.Error(t, landmarkError(t, landmarks), "a line separator must stay rejected")
}
