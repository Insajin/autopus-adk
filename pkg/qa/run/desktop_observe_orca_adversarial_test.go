package run

import (
	"bytes"
	"context"
	"errors"
	"strings"
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

func TestOrcaStateDecoder_RejectsScreenshotTruncationUnsafeTreeAndTargetDrift(t *testing.T) {
	binding := orcaWindowBinding{
		id: orcaTestWindowID, index: 0, pid: orcaTestPID, x: 100, y: 50, width: 1460, height: 980,
	}
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
		{name: "unsafe tree path", mutate: func(value map[string]any) {
			snapshot(value)["treeText"] = strings.Replace(
				orcaFixtureTree(false), "Ready", "/Users/private/token", 1)
		}},
		{name: "unsafe action", mutate: func(value map[string]any) {
			snapshot(value)["treeText"] = strings.Replace(
				orcaFixtureTree(false), "Autopus disclosure", "Autopus disclosure, click", 1)
		}},
		{name: "unknown snapshot field", mutate: func(value map[string]any) {
			snapshot(value)["rawTree"] = "private"
		}},
		{name: "missing tree", mutate: func(value map[string]any) {
			delete(snapshot(value), "treeText")
		}},
		{name: "element count drift", mutate: func(value map[string]any) {
			snapshot(value)["elementCount"] = 10
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
			raw := mutateOrcaJSON(orcaStateFixture(false), test.mutate)
			projection, err := decodeOrcaState(
				raw, orcaTestRuntimeID, binding, &countingOrcaReader{},
			)
			assert.Error(t, err)
			assert.Empty(t, projection)
			assert.NotContains(t, err.Error(), "/Users/private")
			assert.NotContains(t, err.Error(), "token")
		})
	}
}

func TestOrcaTargets_AmbiguousAppAndWindowFailClosed(t *testing.T) {
	_, matches, err := decodeOrcaApps(orcaAppsFixture(2), orcaTestRuntimeID)
	require.NoError(t, err)
	assert.Equal(t, 2, matches)

	_, matches, err = decodeOrcaWindows(orcaWindowsFixture(2), orcaTestRuntimeID, orcaTestPID)
	require.NoError(t, err)
	assert.Equal(t, 2, matches)

	wrongWindow := mutateOrcaJSON(orcaWindowsFixture(1), func(value map[string]any) {
		result := value["result"].(map[string]any)
		result["windows"].([]any)[0].(map[string]any)["title"] = "Other"
	})
	_, matches, err = decodeOrcaWindows(wrongWindow, orcaTestRuntimeID, orcaTestPID)
	require.NoError(t, err)
	assert.Zero(t, matches)

	wrongPID := mutateOrcaJSON(orcaWindowsFixture(1), func(value map[string]any) {
		result := value["result"].(map[string]any)
		result["windows"].([]any)[0].(map[string]any)["app"].(map[string]any)["pid"] =
			orcaTestPID + 1
	})
	_, _, err = decodeOrcaWindows(wrongPID, orcaTestRuntimeID, orcaTestPID)
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
