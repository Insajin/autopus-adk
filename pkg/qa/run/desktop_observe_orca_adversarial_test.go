package run

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestOrcaDecoder_RejectsMalformedMissingUnknownDuplicateAndOversized(t *testing.T) {
	valid := orcaCapabilitiesFixture()
	tests := []struct {
		name string
		raw  []byte
	}{
		{name: "malformed", raw: []byte(`{"id":`)},
		{name: "missing", raw: mutateOrcaJSON(valid, func(value map[string]any) {
			delete(value["result"].(map[string]any), "provider")
		})},
		{name: "unknown", raw: mutateOrcaJSON(valid, func(value map[string]any) {
			value["private_path"] = "/Users/private/secret"
		})},
		{name: "provider mismatch", raw: mutateOrcaJSON(valid, func(value map[string]any) {
			value["result"].(map[string]any)["provider"] = "other-provider"
		})},
		{name: "protocol mismatch", raw: mutateOrcaJSON(valid, func(value map[string]any) {
			value["result"].(map[string]any)["protocolVersion"] = 2
		})},
		{name: "nested missing", raw: mutateOrcaJSON(valid, func(value map[string]any) {
			supports := value["result"].(map[string]any)["supports"].(map[string]any)
			delete(supports["observation"].(map[string]any), "screenshot")
		})},
		{name: "duplicate", raw: bytes.Replace(valid, []byte(`"ok":true`),
			[]byte(`"ok":true,"ok":true`), 1)},
		{name: "oversized", raw: bytes.Repeat([]byte("x"), orcaMaxOutputBytes+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identity, runtimeID, err := decodeOrcaCapabilities(test.raw)
			assert.Error(t, err)
			assert.Empty(t, identity)
			assert.Empty(t, runtimeID)
			assert.NotContains(t, err.Error(), "/Users/private/secret")
		})
	}
}

func TestOrcaDecoder_PreservesStrictDuplicateUnknownAndSizeTaxonomy(t *testing.T) {
	valid := orcaCapabilitiesFixture()
	duplicate := bytes.Replace(valid, []byte(`"ok":true`), []byte(`"ok":true,"ok":true`), 1)
	assert.ErrorIs(t, decodeOrcaObject(duplicate, &orcaEnvelope{}, "id", "ok", "result", "_meta"),
		desktopobserve.ErrDuplicateKey)
	unknown := mutateOrcaJSON(valid, func(value map[string]any) { value["unknown"] = true })
	assert.ErrorIs(t, decodeOrcaObject(unknown, &orcaEnvelope{}, "id", "ok", "result", "_meta"),
		desktopobserve.ErrUnknownField)
	assert.ErrorIs(t, decodeOrcaObject(bytes.Repeat([]byte("x"), orcaMaxOutputBytes+1),
		&orcaEnvelope{}, "id", "ok", "result", "_meta"), desktopobserve.ErrEnvelopeTooLarge)
}

// The envelope guards are unchanged by the fixture removal: each case below
// refused as a malformed envelope before this SPEC and still does, for the same
// reason.
//
// Two cases moved out rather than changing meaning here. "unsafe tree path" and
// "unsafe action" spliced a filesystem path and a mutating action into the tree
// and relied on the exact-14-line fixture comparison to reject them. That
// comparison is gone, and the risk they guarded - observed content reaching
// published evidence - is now structural rather than incidental: REQ-4 publishes
// only pack-declared names. Asserting an error here would assert the fixture pin
// again, so both risks are pinned by
// TestOrcaStateDecoder_UndeclaredTreeContentIsObservedButNeverPublished, which
// proves the path and the action are observed, counted, and absent from the
// projection.
func TestOrcaStateDecoder_RejectsScreenshotTruncationAndEnvelopeDrift(t *testing.T) {
	binding, target := orcaStateTestBinding(), orcaStateTestTarget()
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "screenshot present", mutate: func(value map[string]any) {
			result := value["result"].(map[string]any)
			result["screenshot"] = map[string]any{"path": "/private/screenshot.png"}
		}},
		{name: "truncated", mutate: func(value map[string]any) {
			snapshot(value)["truncation"].(map[string]any)["truncated"] = true
		}},
		{name: "unknown snapshot field", mutate: func(value map[string]any) {
			snapshot(value)["rawTree"] = "private"
		}},
		{name: "missing tree", mutate: func(value map[string]any) {
			delete(snapshot(value), "treeText")
		}},
		// The elementCount == 9 and focusedElementId == 2 value pins were fixture
		// values and are gone; their presence is still wire shape.
		{name: "missing element count", mutate: func(value map[string]any) {
			delete(snapshot(value), "elementCount")
		}},
		{name: "missing focused element", mutate: func(value map[string]any) {
			delete(snapshot(value), "focusedElementId")
		}},
		{name: "pid drift", mutate: func(value map[string]any) {
			snapshot(value)["app"].(map[string]any)["pid"] = orcaTestPID + 1
		}},
		{name: "window drift", mutate: func(value map[string]any) {
			snapshot(value)["window"].(map[string]any)["id"] = orcaTestWindowID + 1
		}},
		{name: "runtime drift", mutate: func(value map[string]any) {
			value["_meta"].(map[string]any)["runtimeId"] = "d0deb450-f449-4050-9e86-ff6e5c58b4a6"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := mutateOrcaJSON(orcaStateFixture(orcaTreeFixture{}), test.mutate)
			projection, err := decodeOrcaState(
				raw, orcaTestRuntimeID, binding, &countingOrcaReader{}, target,
			)
			assert.Error(t, err)
			assert.Empty(t, projection)
			assert.NotContains(t, err.Error(), "/private")
			assert.NotContains(t, err.Error(), orcaFixtureStatusText,
				"a refusal must not quote observed content back")
		})
	}
}

// REQ-6, AC-QAMESH13-009: an app with more elements than the harness will carry
// is refused by the name of the bound it crossed.
//
// The byte bound is exercised elsewhere with a blob; this one has to be a
// legitimately shaped tree, or it would be refused for its shape and prove
// nothing about the node bound. 257 tab-indented node lines are roughly 3 KiB,
// so the derived byte bound cannot fire first - that ordering is the reason
// orcaTreeMaxBytes is derived from the node bound rather than picked.
func TestOrcaStateDecoder_NodeBoundRefusesByNameWithoutTruncating(t *testing.T) {
	binding, target := orcaStateTestBinding(), orcaStateTestTarget()
	oversized := orcaTreeFixture{PadNodes: desktopTreeMaxNodes - orcaFixtureNodeCount + 1}
	require.Equal(t, desktopTreeMaxNodes+1, oversized.elementCount())
	raw := orcaStateFixture(oversized)

	var envelope orcaStateResult
	_, err := decodeOrcaSuccess(raw, &envelope, "snapshot", "screenshot", "screenshotStatus")
	require.NoError(t, err)
	require.Less(t, len(*envelope.Snapshot.TreeText), orcaTreeMaxBytes,
		"the byte bound must not shadow the node bound")
	require.NoError(t, orcaSnapshotFault(envelope, binding, target),
		"the envelope is sound; only the tree is too large")

	projection, err := decodeOrcaState(
		raw, orcaTestRuntimeID, binding, &countingOrcaReader{}, target,
	)
	require.Error(t, err)
	assert.Empty(t, projection, "REQ-6 forbids reporting a pass over a partial observation")
	assert.Equal(t, desktopobserve.ReasonObservedTreeBoundExceeded,
		desktopobserve.ReasonCodeOf(err))
	assert.Contains(t, err.Error(), string(desktopobserve.ObservedTreeBoundNodes))
	assert.NotErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)
}

func TestOrcaTargets_AmbiguousAppAndWindowFailClosed(t *testing.T) {
	_, matches, err := decodeOrcaApps(
		orcaAppsFixture(2), orcaTestRuntimeID, orcaFixtureAppID, orcaFixtureAppName,
	)
	require.NoError(t, err)
	assert.Equal(t, 2, matches)

	_, matches, err = decodeOrcaWindows(
		orcaWindowsFixture(2), orcaTestRuntimeID, orcaTestPID,
		orcaFixtureAppID, orcaFixtureAppName, orcaFixtureWindowTitle,
	)
	require.NoError(t, err)
	assert.Equal(t, 2, matches)

	wrongWindow := mutateOrcaJSON(orcaWindowsFixture(1), func(value map[string]any) {
		result := value["result"].(map[string]any)
		result["windows"].([]any)[0].(map[string]any)["title"] = "Other"
	})
	_, matches, err = decodeOrcaWindows(
		wrongWindow, orcaTestRuntimeID, orcaTestPID,
		orcaFixtureAppID, orcaFixtureAppName, orcaFixtureWindowTitle,
	)
	require.NoError(t, err)
	assert.Zero(t, matches)

	wrongPID := mutateOrcaJSON(orcaWindowsFixture(1), func(value map[string]any) {
		result := value["result"].(map[string]any)
		result["windows"].([]any)[0].(map[string]any)["app"].(map[string]any)["pid"] =
			orcaTestPID + 1
	})
	_, _, err = decodeOrcaWindows(
		wrongPID, orcaTestRuntimeID, orcaTestPID,
		orcaFixtureAppID, orcaFixtureAppName, orcaFixtureWindowTitle,
	)
	assert.Error(t, err)
}

func TestOrcaDesktopClient_ContextTimeoutIsBoundedAndSanitized(t *testing.T) {
	executor := &fakeOrcaCommandExecutor{block: true}
	client, err := newOrcaDesktopClientWith("/private/provider/orca", executor, &countingOrcaReader{})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = client.Handshake(ctx)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Less(t, time.Since(started), 500*time.Millisecond)
	assert.NotContains(t, err.Error(), "/private/provider/orca")
}

func snapshot(value map[string]any) map[string]any {
	return value["result"].(map[string]any)["snapshot"].(map[string]any)
}
