package run

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func orcaStateTestBinding() orcaWindowBinding {
	return orcaWindowBinding{
		id: orcaTestWindowID, index: 0, pid: orcaTestPID, x: 100, y: 50, width: 1460, height: 980,
	}
}

// orcaStateTestTarget is the identity the state fixture answers about. It is
// supplied, not compiled in, which is the whole point of REQ-3.
//
// It is built by the same targetFromRequest the runner uses, so a decoder-level
// test cannot be given a target the runner would never produce - notably one
// whose declared landmarks disagree with the oracle policy judging the result.
func orcaStateTestTarget() desktopProviderTarget {
	return targetFromRequest(orcaFixtureRunRequest())
}

// orcaStateResultWithTree splices a tree into the state envelope. The envelope
// checks never read tree content, only its length, so any tree exercises the
// bound.
func orcaStateResultWithTree(t *testing.T, tree string) orcaStateResult {
	t.Helper()
	raw := mutateOrcaJSON(orcaStateFixture(orcaTreeFixture{}), func(value map[string]any) {
		snapshot(value)["treeText"] = tree
	})
	var result orcaStateResult
	_, err := decodeOrcaSuccess(raw, &result, "snapshot", "screenshot", "screenshotStatus")
	require.NoError(t, err)
	return result
}

// SPEC-QAMESH-013 REQ-6, AC-QAMESH13-009: the 2048-byte treeText cap was a
// fixture artifact that refused every stock macOS app. Both captured real trees
// now clear the envelope check, and a tree past the derived bound is refused by
// the name of the bound it crossed rather than as a malformed envelope.
func TestOrcaSnapshotFault_AcceptsRealTreeSizesAndNamesTheByteBound(t *testing.T) {
	t.Parallel()

	binding, target := orcaStateTestBinding(), orcaStateTestTarget()

	// 5598 characters of mostly Korean role phrases, which is 7364 UTF-8 bytes -
	// the cap counted bytes, so a localized tree hit it about 1.3x harder than
	// its character count suggests.
	finder := readDesktopTreeFixture(t, "finder.txt")
	require.Len(t, finder, 7_364)
	require.Greater(t, len(finder), 2_048, "this is the tree the removed cap rejected")
	require.NoError(t, orcaSnapshotFault(orcaStateResultWithTree(t, finder), binding, target))
	assert.True(t, validOrcaSnapshot(orcaStateResultWithTree(t, finder), binding, target))

	autopus := readDesktopTreeFixture(t, "autopus.txt")
	require.Len(t, autopus, 484)
	require.NoError(t, orcaSnapshotFault(orcaStateResultWithTree(t, autopus), binding, target))

	// The bound has to stay above any legitimate 256-node tree and below the
	// envelope cap. Above, or it is a new fixture pin; below, or it is dead code
	// that decodeOrcaObject reaches first.
	assert.Greater(t, orcaTreeMaxBytes, len(finder)*2)
	assert.Less(t, orcaTreeMaxBytes, orcaMaxOutputBytes)

	oversized := strings.Repeat("x", orcaTreeMaxBytes+1)
	err := orcaSnapshotFault(orcaStateResultWithTree(t, oversized), binding, target)
	require.Error(t, err)
	assert.Equal(t, desktopobserve.ReasonObservedTreeBoundExceeded,
		desktopobserve.ReasonCodeOf(err))
	assert.NotErrorIs(t, err, desktopobserve.ErrMalformedEnvelope,
		"a size refusal must not degrade into a protocol mismatch")
	assert.Contains(t, err.Error(), string(desktopobserve.ObservedTreeBoundBytes))
	assert.Contains(t, err.Error(), strconv.Itoa(len(oversized)),
		"the refusal reports the observed size instead of truncating to the bound")
	assert.False(t, validOrcaSnapshot(orcaStateResultWithTree(t, oversized), binding, target))
}

// Envelope integrity is untouched by the pin removal: an absent-and-skipped
// screenshot, window-relative coordinates, a fresh snapshot id, the bound app
// and window, and an untruncated capture. Each still refuses as a malformed
// envelope, and none of them is reported as a bound.
func TestOrcaSnapshotFault_StillRefusesMalformedEnvelopes(t *testing.T) {
	t.Parallel()

	binding, target := orcaStateTestBinding(), orcaStateTestTarget()
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "screenshot present", mutate: func(value map[string]any) {
			value["result"].(map[string]any)["screenshot"] = map[string]any{"path": "shot.png"}
		}},
		{name: "screenshot claimed captured", mutate: func(value map[string]any) {
			status := value["result"].(map[string]any)["screenshotStatus"].(map[string]any)
			status["state"] = "captured"
		}},
		{name: "screenshot reason drift", mutate: func(value map[string]any) {
			status := value["result"].(map[string]any)["screenshotStatus"].(map[string]any)
			status["reason"] = "requested"
		}},
		{name: "truncated", mutate: func(value map[string]any) {
			snapshot(value)["truncation"].(map[string]any)["truncated"] = true
		}},
		{name: "max depth drift", mutate: func(value map[string]any) {
			snapshot(value)["truncation"].(map[string]any)["maxDepth"] = 32
		}},
		{name: "max nodes drift", mutate: func(value map[string]any) {
			snapshot(value)["truncation"].(map[string]any)["maxNodes"] = 1_201
		}},
		{name: "max depth reached", mutate: func(value map[string]any) {
			snapshot(value)["truncation"].(map[string]any)["maxDepthReached"] = true
		}},
		{name: "coordinate space drift", mutate: func(value map[string]any) {
			snapshot(value)["coordinateSpace"] = "screen"
		}},
		{name: "snapshot id not a uuid", mutate: func(value map[string]any) {
			snapshot(value)["id"] = "snapshot-1"
		}},
		{name: "empty tree", mutate: func(value map[string]any) {
			snapshot(value)["treeText"] = ""
		}},
		{name: "app pid drift", mutate: func(value map[string]any) {
			snapshot(value)["app"].(map[string]any)["pid"] = orcaTestPID + 1
		}},
		{name: "window id drift", mutate: func(value map[string]any) {
			snapshot(value)["window"].(map[string]any)["id"] = orcaTestWindowID + 1
		}},
		// The declared window title now drives this check, so a provider answering
		// about a different window is refused rather than silently accepted.
		{name: "window title drift", mutate: func(value map[string]any) {
			snapshot(value)["window"].(map[string]any)["title"] = "Some Other Window"
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			raw := mutateOrcaJSON(orcaStateFixture(orcaTreeFixture{}), test.mutate)
			var result orcaStateResult
			_, err := decodeOrcaSuccess(raw, &result, "snapshot", "screenshot", "screenshotStatus")
			require.NoError(t, err)

			fault := orcaSnapshotFault(result, binding, target)
			require.ErrorIs(t, fault, desktopobserve.ErrMalformedEnvelope)
			assert.NotEqual(t, desktopobserve.ReasonObservedTreeBoundExceeded,
				desktopobserve.ReasonCodeOf(fault))
			assert.False(t, validOrcaSnapshot(result, binding, target))
		})
	}
}

// The removed pins are gone for real: the fixture's own element count and
// focused element id may now hold any value a real app reports, while their
// presence stays required as wire shape.
func TestOrcaSnapshotFault_AcceptsRealElementCountsAndFocusedIDs(t *testing.T) {
	t.Parallel()

	binding, target := orcaStateTestBinding(), orcaStateTestTarget()
	for _, drift := range []func(map[string]any){
		func(value map[string]any) { snapshot(value)["elementCount"] = 152 },
		func(value map[string]any) { snapshot(value)["focusedElementId"] = 90 },
		func(value map[string]any) { snapshot(value)["elementCount"] = 7 },
		func(value map[string]any) { snapshot(value)["focusedElementId"] = 0 },
	} {
		raw := mutateOrcaJSON(orcaStateFixture(orcaTreeFixture{}), drift)
		var result orcaStateResult
		_, err := decodeOrcaSuccess(raw, &result, "snapshot", "screenshot", "screenshotStatus")
		require.NoError(t, err)
		require.NoError(t, orcaSnapshotFault(result, binding, target))
	}
}
