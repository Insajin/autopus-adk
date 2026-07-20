package run

import (
	"bytes"
	"encoding/json"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

type orcaAppsResult struct {
	Apps []orcaAppRecord `json:"apps"`
}

type orcaAppRecord struct {
	Name       *string         `json:"name"`
	BundleID   *string         `json:"bundleId"`
	IsRunning  *bool           `json:"isRunning"`
	PID        *int            `json:"pid"`
	LastUsedAt json.RawMessage `json:"lastUsedAt"`
	UseCount   json.RawMessage `json:"useCount"`
}

func decodeOrcaApps(raw []byte, expectedRuntime string) (int, int, error) {
	var result orcaAppsResult
	runtimeID, err := decodeOrcaSuccess(raw, &result, "apps")
	if err != nil || runtimeID != expectedRuntime || result.Apps == nil {
		return 0, 0, desktopobserve.ErrMalformedEnvelope
	}
	pid, matches := 0, 0
	for _, app := range result.Apps {
		if !validOrcaAppRecord(app) {
			return 0, 0, desktopobserve.ErrMalformedEnvelope
		}
		if *app.BundleID != orcaAppBundleID {
			continue
		}
		if *app.Name != orcaAppName || !*app.IsRunning || *app.PID <= 1 {
			return 0, 0, desktopobserve.ErrMalformedEnvelope
		}
		pid = *app.PID
		matches++
	}
	return pid, matches, nil
}

func validOrcaAppRecord(app orcaAppRecord) bool {
	return app.Name != nil && app.BundleID != nil && app.IsRunning != nil && app.PID != nil &&
		len(app.LastUsedAt) != 0 && len(app.UseCount) != 0 &&
		bytes.Equal(bytes.TrimSpace(app.LastUsedAt), []byte("null")) &&
		bytes.Equal(bytes.TrimSpace(app.UseCount), []byte("null"))
}

type orcaWindowsResult struct {
	App     orcaAppRecord `json:"app"`
	Windows []orcaWindow  `json:"windows"`
}

type orcaSnapshotApp struct {
	Name     *string `json:"name"`
	BundleID *string `json:"bundleId"`
	PID      *int    `json:"pid"`
}

type orcaWindow struct {
	Title       *string            `json:"title"`
	App         orcaSnapshotApp    `json:"app"`
	Index       *int               `json:"index"`
	Height      *int               `json:"height"`
	ScreenIndex *int               `json:"screenIndex"`
	Width       *int               `json:"width"`
	Y           *int               `json:"y"`
	IsMinimized *bool              `json:"isMinimized"`
	IsOffscreen *bool              `json:"isOffscreen"`
	IsMain      json.RawMessage    `json:"isMain"`
	Platform    orcaWindowPlatform `json:"platform"`
	ID          *int               `json:"id"`
	X           *int               `json:"x"`
}

type orcaWindowPlatform struct {
	Layer *int     `json:"layer"`
	Alpha *float64 `json:"alpha"`
}

func decodeOrcaWindows(
	raw []byte,
	expectedRuntime string,
	expectedPID int,
) (orcaWindowBinding, int, error) {
	var result orcaWindowsResult
	runtimeID, err := decodeOrcaSuccess(raw, &result, "app", "windows")
	if err != nil || runtimeID != expectedRuntime || !validOrcaAppRecord(result.App) ||
		*result.App.Name != orcaAppName || *result.App.BundleID != orcaAppBundleID ||
		!*result.App.IsRunning || *result.App.PID != expectedPID || result.Windows == nil {
		return orcaWindowBinding{}, 0, desktopobserve.ErrMalformedEnvelope
	}
	for _, window := range result.Windows {
		if !validOrcaWindowSchema(window) {
			return orcaWindowBinding{}, 0, desktopobserve.ErrMalformedEnvelope
		}
	}
	if len(result.Windows) != 1 {
		return orcaWindowBinding{}, len(result.Windows), nil
	}
	window := result.Windows[0]
	if *window.Title != orcaWindowTitle {
		return orcaWindowBinding{}, 0, nil
	}
	if !validOrcaSnapshotApp(window.App, expectedPID) || *window.IsMinimized || *window.IsOffscreen ||
		*window.Index < 0 || *window.ID <= 0 || *window.ScreenIndex < 0 ||
		*window.Width <= 0 || *window.Height <= 0 || *window.Width > 100_000 || *window.Height > 100_000 ||
		*window.X < -100_000 || *window.X > 100_000 || *window.Y < -100_000 || *window.Y > 100_000 ||
		*window.Platform.Layer != 0 || *window.Platform.Alpha != 1 {
		return orcaWindowBinding{}, 0, desktopobserve.ErrMalformedEnvelope
	}
	return orcaWindowBinding{
		id: *window.ID, index: *window.Index, pid: expectedPID,
		x: *window.X, y: *window.Y, width: *window.Width, height: *window.Height,
	}, 1, nil
}

func validOrcaWindowSchema(window orcaWindow) bool {
	return window.Title != nil && window.App.Name != nil && window.App.BundleID != nil &&
		window.App.PID != nil && window.Index != nil && window.Height != nil &&
		window.ScreenIndex != nil && window.Width != nil && window.Y != nil &&
		window.IsMinimized != nil && window.IsOffscreen != nil && len(window.IsMain) != 0 &&
		validNullableOrcaBool(window.IsMain) && window.Platform.Layer != nil &&
		window.Platform.Alpha != nil && window.ID != nil && window.X != nil
}

func validOrcaSnapshotApp(app orcaSnapshotApp, expectedPID int) bool {
	return app.Name != nil && *app.Name == orcaAppName && app.BundleID != nil &&
		*app.BundleID == orcaAppBundleID && app.PID != nil && *app.PID == expectedPID
}

func validNullableOrcaBool(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("true")) ||
		bytes.Equal(trimmed, []byte("false"))
}
