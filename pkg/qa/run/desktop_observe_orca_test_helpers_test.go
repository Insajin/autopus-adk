package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	orcaTestRuntimeID  = "6285a71f-f14e-4654-ab19-13d7a73283a7"
	orcaTestResponseID = "d0deb450-f449-4050-9e86-ff6e5c58b4a6"
	orcaTestSnapshotID = "ED35443E-FB01-4601-A04A-01353D1417A6"
	orcaTestPID        = 17673
	orcaTestWindowID   = 52
)

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

func newHermeticOrcaClient(t *testing.T) (*orcaDesktopClient, *fakeOrcaCommandExecutor) {
	t.Helper()
	executor := &fakeOrcaCommandExecutor{responses: orcaTestResponses(false)}
	client, err := newOrcaDesktopClientWith("/private/test/orca", executor, &countingOrcaReader{})
	require.NoError(t, err)
	return client, executor
}

func orcaTestResponses(expanded bool) map[string][]byte {
	return map[string][]byte{
		"computer capabilities --json":                                           orcaCapabilitiesFixture(),
		"computer permissions --json":                                            orcaPermissionsFixture("granted"),
		"computer list-apps --json":                                              orcaAppsFixture(1),
		"computer list-windows --app co.autopus.desktop --json":                  orcaWindowsFixture(1),
		"computer get-app-state --app co.autopus.desktop --no-screenshot --json": orcaStateFixture(expanded),
	}
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
		"name": orcaAppName, "bundleId": orcaAppBundleID, "isRunning": true,
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
		"title": orcaWindowTitle,
		"app":   map[string]any{"name": orcaAppName, "bundleId": orcaAppBundleID, "pid": orcaTestPID},
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

func orcaFixtureTree(expanded bool) string {
	button := "\t\t\t3 button Autopus disclosure"
	if expanded {
		button = "\t\t\t3 button (expanded) Autopus disclosure"
	}
	return strings.Join([]string{
		fmt.Sprintf("App=%s (pid %d)", orcaAppBundleID, orcaTestPID),
		`Window: "Autopus", App: Autopus Desktop.`, "", "0 standard window Autopus",
		"\t1 scroll area", "\t\t2 HTML content Autopus", button,
		"\t\t\t4 container Autopus fixture status", "\t\t\t\t5 text Ready",
		"\t6 close button", "\t7 full screen button, Secondary Actions: zoom the window",
		"\t8 minimize button", "", "The focused UI element is 2 HTML content Autopus.",
	}, "\n")
}

func orcaStateFixture(expanded bool) []byte {
	return orcaEnvelopeFixture(map[string]any{
		"snapshot": map[string]any{
			"treeText": orcaFixtureTree(expanded),
			"window": map[string]any{
				"id": orcaTestWindowID, "title": orcaWindowTitle, "width": 1460, "height": 980,
				"x": 100, "y": 50, "isMinimized": false, "isOffscreen": false,
				"screenIndex": 0, "platform": map[string]any{"layer": 0},
			},
			"app":              map[string]any{"name": orcaAppName, "bundleId": orcaAppBundleID, "pid": orcaTestPID},
			"focusedElementId": 2, "elementCount": 9, "coordinateSpace": "window",
			"id": orcaTestSnapshotID,
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
