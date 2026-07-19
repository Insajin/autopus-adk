package run

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

type orcaStateResult struct {
	Snapshot         orcaSnapshot         `json:"snapshot"`
	Screenshot       json.RawMessage      `json:"screenshot"`
	ScreenshotStatus orcaScreenshotStatus `json:"screenshotStatus"`
}

type orcaSnapshot struct {
	TreeText        *string            `json:"treeText"`
	Window          orcaSnapshotWindow `json:"window"`
	App             orcaSnapshotApp    `json:"app"`
	FocusedElement  *int               `json:"focusedElementId"`
	ElementCount    *int               `json:"elementCount"`
	CoordinateSpace *string            `json:"coordinateSpace"`
	ID              *string            `json:"id"`
	Truncation      orcaTruncation     `json:"truncation"`
}

type orcaSnapshotWindow struct {
	ID          *int                 `json:"id"`
	Index       *int                 `json:"index,omitempty"`
	Title       *string              `json:"title"`
	Width       *int                 `json:"width"`
	Height      *int                 `json:"height"`
	X           *int                 `json:"x"`
	Y           *int                 `json:"y"`
	IsMinimized *bool                `json:"isMinimized"`
	IsOffscreen *bool                `json:"isOffscreen"`
	ScreenIndex *int                 `json:"screenIndex"`
	Platform    orcaSnapshotPlatform `json:"platform"`
}

type orcaSnapshotPlatform struct {
	Layer *int `json:"layer"`
}

type orcaTruncation struct {
	Truncated       *bool `json:"truncated"`
	MaxDepth        *int  `json:"maxDepth"`
	MaxNodes        *int  `json:"maxNodes"`
	MaxDepthReached *bool `json:"maxDepthReached"`
}

type orcaScreenshotStatus struct {
	Reason *string `json:"reason"`
	State  *string `json:"state"`
}

func decodeOrcaState(
	raw []byte,
	expectedRuntime string,
	binding orcaWindowBinding,
	random io.Reader,
) (desktopobserve.SemanticProjection, error) {
	var result orcaStateResult
	runtimeID, err := decodeOrcaSuccess(raw, &result, "snapshot", "screenshot", "screenshotStatus")
	if err != nil || runtimeID != expectedRuntime || !validOrcaSnapshot(result, binding) {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	expanded, err := parseOrcaFixtureTree(*result.Snapshot.TreeText, binding.pid)
	if err != nil {
		return desktopobserve.SemanticProjection{}, err
	}
	stateRef, err := orcaFreshRef(random, "state-")
	if err != nil {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	appNodeRef, err := orcaFreshRef(random, "node-")
	if err != nil {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	windowNodeRef, err := orcaFreshRef(random, "node-")
	if err != nil {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	disclosureNodeRef, err := orcaFreshRef(random, "node-")
	if err != nil {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	enabled, focused := true, true
	return desktopobserve.SemanticProjection{
		SchemaVersion: desktopobserve.SemanticProjectionSchemaVersion,
		ProviderRef:   "provider-orca",
		AppRef:        "autopus-desktop",
		WindowRef:     "main-window",
		StateRef:      stateRef,
		Root: desktopobserve.SemanticNode{
			NodeRef: appNodeRef, Role: desktopobserve.RoleApplication, Name: "Autopus",
			SemanticState: desktopobserve.SemanticState{Enabled: &enabled},
			Children: []desktopobserve.SemanticNode{{
				NodeRef: windowNodeRef, Role: desktopobserve.RoleWindow, Name: "Autopus",
				SemanticState: desktopobserve.SemanticState{Focused: &focused},
				Children: []desktopobserve.SemanticNode{{
					NodeRef: disclosureNodeRef, Role: desktopobserve.Role("AXButton"), Name: "Disclosure",
					SemanticState:     desktopobserve.SemanticState{Enabled: &enabled, Expanded: &expanded},
					AdvertisedActions: []desktopobserve.Action{desktopobserve.ActionPress},
				}},
			}},
		},
	}, nil
}

func validOrcaSnapshot(result orcaStateResult, binding orcaWindowBinding) bool {
	snapshot := result.Snapshot
	window := snapshot.Window
	if len(result.Screenshot) == 0 || !bytes.Equal(bytes.TrimSpace(result.Screenshot), []byte("null")) ||
		result.ScreenshotStatus.Reason == nil || *result.ScreenshotStatus.Reason != "no_screenshot_flag" ||
		result.ScreenshotStatus.State == nil || *result.ScreenshotStatus.State != "skipped" ||
		snapshot.TreeText == nil || len(*snapshot.TreeText) == 0 || len(*snapshot.TreeText) > 2_048 ||
		snapshot.FocusedElement == nil || *snapshot.FocusedElement != 2 ||
		snapshot.ElementCount == nil || *snapshot.ElementCount != 9 ||
		snapshot.CoordinateSpace == nil || *snapshot.CoordinateSpace != "window" ||
		snapshot.ID == nil || !orcaUUIDPattern.MatchString(*snapshot.ID) ||
		!validOrcaSnapshotApp(snapshot.App, binding.pid) {
		return false
	}
	if window.ID == nil || *window.ID != binding.id || window.Title == nil || *window.Title != orcaWindowTitle ||
		window.Width == nil || *window.Width != binding.width || window.Height == nil || *window.Height != binding.height ||
		window.X == nil || *window.X != binding.x || window.Y == nil || *window.Y != binding.y ||
		window.IsMinimized == nil || *window.IsMinimized || window.IsOffscreen == nil || *window.IsOffscreen ||
		window.ScreenIndex == nil || *window.ScreenIndex < 0 || window.Platform.Layer == nil || *window.Platform.Layer != 0 ||
		(window.Index != nil && *window.Index != binding.index) {
		return false
	}
	truncation := snapshot.Truncation
	return truncation.Truncated != nil && !*truncation.Truncated &&
		truncation.MaxDepth != nil && *truncation.MaxDepth == 64 &&
		truncation.MaxNodes != nil && *truncation.MaxNodes == 1_200 &&
		truncation.MaxDepthReached != nil && !*truncation.MaxDepthReached
}

func parseOrcaFixtureTree(tree string, pid int) (bool, error) {
	if strings.ContainsAny(tree, "\r\x00") {
		return false, desktopobserve.ErrMalformedEnvelope
	}
	lines := strings.Split(tree, "\n")
	if len(lines) != 14 || lines[0] != fmt.Sprintf("App=%s (pid %d)", orcaAppBundleID, pid) ||
		lines[1] != `Window: "Autopus", App: Autopus Desktop.` || lines[2] != "" ||
		lines[3] != "0 standard window Autopus" || lines[4] != "\t1 scroll area" ||
		lines[5] != "\t\t2 HTML content Autopus" ||
		lines[7] != "\t\t\t4 container Autopus fixture status" ||
		lines[8] != "\t\t\t\t5 text Ready" || lines[9] != "\t6 close button" ||
		lines[10] != "\t7 full screen button, Secondary Actions: zoom the window" ||
		lines[11] != "\t8 minimize button" || lines[12] != "" ||
		lines[13] != "The focused UI element is 2 HTML content Autopus." {
		return false, desktopobserve.ErrMalformedEnvelope
	}
	switch lines[6] {
	case "\t\t\t3 button Autopus disclosure":
		return false, nil
	case "\t\t\t3 button (expanded) Autopus disclosure":
		return true, nil
	default:
		return false, desktopobserve.ErrMalformedEnvelope
	}
}

func orcaFreshRef(random io.Reader, prefix string) (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value), nil
}
