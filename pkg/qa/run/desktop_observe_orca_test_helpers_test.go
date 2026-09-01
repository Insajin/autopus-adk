package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

const (
	orcaTestRuntimeID  = "6285a71f-f14e-4654-ab19-13d7a73283a7"
	orcaTestResponseID = "d0deb450-f449-4050-9e86-ff6e5c58b4a6"
	orcaTestSnapshotID = "ED35443E-FB01-4601-A04A-01353D1417A6"
	orcaTestPID        = 17673
	orcaTestWindowID   = 52
)

// orcaFixtureLandmarks is what the fixture pack declares. Three landmarks, not
// two: the third proves REQ-4 publishes a declared node below the window, which
// is the branch that was unreachable from a provider while decodeOrcaState
// synthesized its own two.
func orcaFixtureLandmarks() []desktopobserve.LandmarkRequirement {
	return []desktopobserve.LandmarkRequirement{
		{
			Role: desktopobserve.RoleApplication, Name: orcaFixtureAppName,
			RequiredState: desktopobserve.StateEnabled,
		},
		{
			Role: desktopobserve.RoleWindow, Name: orcaFixtureWindowTitle,
			RequiredState: desktopobserve.StateFocused,
		},
		{
			Role: desktopobserve.Role("AXButton"), Name: orcaFixtureButtonName,
			RequiredState: desktopobserve.StateEnabled,
		},
	}
}

// orcaFixtureRunRequest is the run request a pack observing the captured app
// would produce. It is separate from desktopRunRequest because the two speak
// different identities on purpose: the fake-client parity tests share one
// projection fixture named "Autopus", while the Orca path carries the real
// captured labels, which contain a space.
func orcaFixtureRunRequest() DesktopObservationRunRequest {
	landmarks := orcaFixtureLandmarks()
	names := make([]string, 0, len(landmarks))
	for _, landmark := range landmarks {
		names = append(names, landmark.Name)
	}
	request := desktopRunRequest(desktopobserve.RuntimeProviderOrca)
	request.ProviderAppID = orcaFixtureAppID
	request.AppName = orcaFixtureAppName
	request.WindowTitle = orcaFixtureWindowTitle
	request.AppRef = "autopus-desktop"
	request.WindowRef = "main-window"
	request.Policy = desktopobserve.OraclePolicy{
		AllowedNames: names, MinimumLandmarks: landmarks,
	}
	return request
}

type fakeOrcaCommandExecutor struct {
	mu        sync.Mutex
	responses map[string][]byte
	calls     []string
	err       error
	block     bool
}

func (executor *fakeOrcaCommandExecutor) Run(
	ctx context.Context,
	_ string,
	arguments []string,
) ([]byte, error) {
	executor.mu.Lock()
	executor.calls = append(executor.calls, strings.Join(arguments, " "))
	response := append([]byte(nil), executor.responses[strings.Join(arguments, " ")]...)
	err, block := executor.err, executor.block
	executor.mu.Unlock()
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return response, err
}

func (executor *fakeOrcaCommandExecutor) recordedCalls() []string {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	return append([]string(nil), executor.calls...)
}

type countingOrcaReader struct {
	value byte
}

func (reader *countingOrcaReader) Read(value []byte) (int, error) {
	for index := range value {
		reader.value++
		value[index] = reader.value
	}
	return len(value), nil
}

// newHermeticOrcaClient returns a client already addressing the fixture's app.
// The runner injects the target in production (SPEC-QAMESH-013 REQ-3), so a
// client-level test has to do the same; without it every request fails closed on
// an unresolved target, which is the behaviour orcaUnsetTargetFailsClosed pins.
func newHermeticOrcaClient(t *testing.T) (*orcaDesktopClient, *fakeOrcaCommandExecutor) {
	t.Helper()
	return newTargetedOrcaClient(t, orcaTreeFixture{})
}

func newTargetedOrcaClient(
	t *testing.T,
	fixture orcaTreeFixture,
) (*orcaDesktopClient, *fakeOrcaCommandExecutor) {
	t.Helper()
	executor := &fakeOrcaCommandExecutor{responses: orcaTestResponses(fixture)}
	client, err := newOrcaDesktopClientWith("/private/test/orca", executor, &countingOrcaReader{})
	require.NoError(t, err)
	client.applyTarget(orcaStateTestTarget())
	return client, executor
}

// orcaObserveOnce drives the client's binding sequence in the order the runner
// uses. GetState refuses unless the app and window are already bound, so a test
// that wants one observation cannot skip to it.
func orcaObserveOnce(
	t *testing.T,
	client *orcaDesktopClient,
) desktopobserve.SemanticProjection {
	t.Helper()
	target := orcaStateTestTarget()
	ctx := context.Background()
	_, err := client.Handshake(ctx)
	require.NoError(t, err)
	_, err = client.Permissions(ctx)
	require.NoError(t, err)
	_, err = client.ListApps(ctx)
	require.NoError(t, err)
	_, err = client.ListWindows(ctx, target.AppRef)
	require.NoError(t, err)
	projection, err := client.GetState(ctx, target.AppRef, target.WindowRef)
	require.NoError(t, err)
	return projection
}

// orcaTestResponses keys the provider's replies by the exact argv the client
// issues, so the --app value is derived from the fixture identity rather than
// spelled out twice.
func orcaTestResponses(fixture orcaTreeFixture) map[string][]byte {
	responses := map[string][]byte{
		"computer capabilities --json": orcaCapabilitiesFixture(),
		"computer permissions --json":  orcaPermissionsFixture("granted"),
		"computer list-apps --json":    orcaAppsFixture(1),
	}
	// The two target-scoped commands carry the pack's provider_app_id, so their
	// keys are built from the same constant the target uses. Spelling the bundle
	// id out here again is how the old fixture drifted from the code it tested.
	responses[fmt.Sprintf("computer list-windows --app %s --json", orcaFixtureAppID)] =
		orcaWindowsFixture(1)
	responses[fmt.Sprintf(
		"computer get-app-state --app %s --no-screenshot --json", orcaFixtureAppID,
	)] = orcaStateFixture(fixture)
	return responses
}

func orcaEnvelopeFixture(result any) []byte {
	return mustOrcaJSON(map[string]any{
		"id": orcaTestResponseID, "ok": true, "result": result,
		"_meta": map[string]any{"runtimeId": orcaTestRuntimeID},
	})
}

func orcaCapabilitiesFixture() []byte {
	return orcaEnvelopeFixture(map[string]any{
		"platform": "darwin", "provider": "orca-computer-use-macos",
		"protocolVersion": 1, "providerVersion": "1.0.0",
		"supports": map[string]any{
			"apps": map[string]any{"list": true, "pids": true, "bundleIds": true},
			"observation": map[string]any{
				"ocr": false, "annotatedScreenshot": false, "elementFrames": true, "screenshot": true,
			},
			"windows": map[string]any{
				"list": true, "moveResize": false, "focus": false, "targetByIndex": true, "targetById": true,
			},
			"actions": map[string]any{
				"pressKey": true, "hotkey": true, "typeText": true, "click": true, "pasteText": true,
				"scroll": true, "setValue": true, "drag": true, "performAction": true,
			},
			"surfaces": map[string]any{"dialogs": false, "menus": false, "menubar": false, "dock": false},
		},
	})
}

func orcaPermissionsFixture(accessibility string) []byte {
	return orcaEnvelopeFixture(map[string]any{
		"platform": "darwin", "helperAppPath": "/private/provider/helper.app",
		"openedSettings": false, "launchedHelper": false, "nextStep": nil,
		"permissions": []any{
			map[string]any{"id": "accessibility", "status": accessibility},
			map[string]any{"id": "screenshots", "status": "granted"},
		},
	})
}

func orcaAppFixture() map[string]any {
	return map[string]any{
		"name": orcaFixtureAppName, "bundleId": orcaFixtureAppID, "isRunning": true,
		"pid": orcaTestPID, "lastUsedAt": nil, "useCount": nil,
	}
}

func orcaAppsFixture(matches int) []byte {
	apps := []any{map[string]any{
		"name": "Other", "bundleId": "com.example.other", "isRunning": true,
		"pid": 123, "lastUsedAt": nil, "useCount": nil,
	}}
	for range matches {
		apps = append(apps, orcaAppFixture())
	}
	return orcaEnvelopeFixture(map[string]any{"apps": apps})
}

func orcaWindowFixture() map[string]any {
	return map[string]any{
		"title": orcaFixtureWindowTitle,
		"app": map[string]any{
			"name": orcaFixtureAppName, "bundleId": orcaFixtureAppID, "pid": orcaTestPID,
		},
		"index": 0, "height": 980, "screenIndex": 0, "width": 1460, "y": 50,
		"isMinimized": false, "isOffscreen": false, "isMain": nil,
		"platform": map[string]any{"layer": 0, "alpha": 1}, "id": orcaTestWindowID, "x": 100,
	}
}

func orcaWindowsFixture(matches int) []byte {
	windows := make([]any, 0, matches)
	for range matches {
		windows = append(windows, orcaWindowFixture())
	}
	return orcaEnvelopeFixture(map[string]any{"app": orcaAppFixture(), "windows": windows})
}

// orcaStateFixture wraps a tree fixture in the state envelope. The snapshot's
// identity, focused element id, and element count are all derived from the same
// fixture, so the envelope can never claim something the tree contradicts - the
// combination validOrcaSnapshotEnvelope exists to refuse.
func orcaStateFixture(fixture orcaTreeFixture) []byte {
	spec := fixture.withDefaults()
	return orcaEnvelopeFixture(map[string]any{
		"snapshot": map[string]any{
			"treeText": spec.render(),
			"window": map[string]any{
				"id": orcaTestWindowID, "title": spec.WindowTitle, "width": 1460, "height": 980,
				"x": 100, "y": 50, "isMinimized": false, "isOffscreen": false,
				"screenIndex": 0, "platform": map[string]any{"layer": 0},
			},
			"app": map[string]any{
				"name": spec.AppName, "bundleId": spec.AppID, "pid": spec.PID,
			},
			"focusedElementId": spec.FocusedID, "elementCount": spec.elementCount(),
			"coordinateSpace": "window", "id": orcaTestSnapshotID,
			"truncation": map[string]any{
				"truncated": false, "maxDepth": 64, "maxNodes": 1200, "maxDepthReached": false,
			},
		},
		"screenshot":       nil,
		"screenshotStatus": map[string]any{"reason": "no_screenshot_flag", "state": "skipped"},
	})
}

func mustOrcaJSON(value any) []byte {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return body
}

func mutateOrcaJSON(raw []byte, mutate func(map[string]any)) []byte {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		panic(err)
	}
	mutate(value)
	return mustOrcaJSON(value)
}
