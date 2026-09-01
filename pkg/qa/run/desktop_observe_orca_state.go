package run

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"

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

// decodeOrcaState turns the provider's observed tree into a publishable
// projection.
//
// This replaces a fixture conformance path: the previous implementation required
// treeText to be exactly 14 lines matching a specific UI, extracted one boolean,
// and returned a hardcoded three-node projection. No shipped application
// produced that tree, so the lane could not pass for anyone.
//
// Observation is complete; publication is selective. See
// desktop_observe_projection_build.go for why - real trees carry user content.
func decodeOrcaState(
	raw []byte,
	expectedRuntime string,
	binding orcaWindowBinding,
	random io.Reader,
	target desktopProviderTarget,
) (desktopobserve.SemanticProjection, error) {
	var result orcaStateResult
	runtimeID, err := decodeOrcaSuccess(raw, &result, "snapshot", "screenshot", "screenshotStatus")
	if err != nil || runtimeID != expectedRuntime || !validOrcaSnapshot(result, binding, target) {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	tree, err := parseDesktopTree(*result.Snapshot.TreeText)
	if err != nil {
		return desktopobserve.SemanticProjection{}, err
	}
	// The provider reported this tree for the window we bound, so a mismatch
	// means the provider answered about something else.
	if tree.PID != binding.pid || tree.AppIdentifier != target.ProviderAppID {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	// refFailed distinguishes our own CSPRNG failing from anything the builder
	// diagnoses about the observation. A raw CSPRNG error would travel into the
	// provider error text an operator sees, and nothing about a local entropy
	// failure is publishable, so it is replaced by the envelope sentinel. Every
	// other builder error keeps its reason code and propagates unchanged.
	refFailed := false
	projection, counts, err := buildDesktopProjection(
		tree, target.Landmarks, target.AppRef, target.WindowRef, "provider-orca",
		func(prefix string) (string, error) {
			ref, refErr := orcaFreshRef(random, prefix)
			if refErr != nil {
				refFailed = true
			}
			return ref, refErr
		},
	)
	if refFailed {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	if err != nil {
		return desktopobserve.SemanticProjection{}, err
	}
	// A declared landmark the observed tree does not contain is reported as
	// exactly that: REQ-5 forbids provider_unavailable here, and a malformed
	// envelope would route to a protocol mismatch. Returning the projection
	// anyway would let the landmark oracle judge an incomplete observation.
	if len(counts.UnmatchedLandmarks) > 0 {
		absent := counts.UnmatchedLandmarks[0]
		return desktopobserve.SemanticProjection{},
			desktopobserve.DeclaredLandmarkNotFound(absent.Role, absent.Name)
	}
	return projection, nil
}

// orcaTreeMaxBytes bounds the rendered tree text. It is derived from the tree
// parser's node bound - desktopTreeMaxNodes = 256 and desktopTreeMaxDepth = 32
// live in desktop_observe_tree.go, which enforces them - at 128 bytes per
// rendered line, the widest measured real line being a localized ~100-byte
// content node. Deriving it is what keeps the byte bound from shadowing the node
// bound: an app with 255 real nodes has to be refused by the node bound, not by
// an incidental byte cap. Do not flatten this to a literal. It stays under the
// 64 KiB orcaMaxOutputBytes envelope cap, so the refusal is reachable.
const orcaTreeMaxBytes = desktopTreeMaxNodes * 128

// orcaSnapshotFault reports why an Orca state envelope is unusable, or nil when
// it is sound. It returns an error rather than a bool because SPEC-QAMESH-013
// REQ-6 requires an oversized tree to be refused by the name of the bound it
// crossed; collapsing that into a bare malformed envelope is how a size refusal
// surfaced as a protocol mismatch.
func orcaSnapshotFault(
	result orcaStateResult,
	binding orcaWindowBinding,
	target desktopProviderTarget,
) error {
	if !validOrcaSnapshotEnvelope(result, binding, target) {
		return desktopobserve.ErrMalformedEnvelope
	}
	// Refuse, never truncate or sample: a shortened tree would let a partial
	// observation report a pass.
	if size := len(*result.Snapshot.TreeText); size > orcaTreeMaxBytes {
		return desktopobserve.ObservedTreeBoundExceeded(
			desktopobserve.ObservedTreeBoundBytes, orcaTreeMaxBytes, size)
	}
	return nil
}

func validOrcaSnapshot(
	result orcaStateResult,
	binding orcaWindowBinding,
	target desktopProviderTarget,
) bool {
	return orcaSnapshotFault(result, binding, target) == nil
}

// validOrcaSnapshotEnvelope checks envelope integrity only: the screenshot is
// absent and declared skipped, coordinates are window-relative, the snapshot id
// is fresh, the app and window are the ones the pack named, and nothing was
// truncated.
//
// The treeText length, focused element id, and element count value pins it used
// to carry described one test fixture. Measured real trees are 406 characters /
// 484 bytes with 7 elements and 5598 characters / 7364 bytes with 152, so those
// pins refused every stock macOS app. Presence of the two counters is still
// required - that is wire shape, not a fixture value.
func validOrcaSnapshotEnvelope(
	result orcaStateResult,
	binding orcaWindowBinding,
	target desktopProviderTarget,
) bool {
	snapshot := result.Snapshot
	window := snapshot.Window
	if len(result.Screenshot) == 0 || !bytes.Equal(bytes.TrimSpace(result.Screenshot), []byte("null")) ||
		result.ScreenshotStatus.Reason == nil || *result.ScreenshotStatus.Reason != "no_screenshot_flag" ||
		result.ScreenshotStatus.State == nil || *result.ScreenshotStatus.State != "skipped" ||
		snapshot.TreeText == nil || len(*snapshot.TreeText) == 0 ||
		snapshot.FocusedElement == nil || snapshot.ElementCount == nil ||
		snapshot.CoordinateSpace == nil || *snapshot.CoordinateSpace != "window" ||
		snapshot.ID == nil || !orcaUUIDPattern.MatchString(*snapshot.ID) ||
		!validOrcaSnapshotApp(snapshot.App, binding.pid, target.ProviderAppID, target.AppName) {
		return false
	}
	// target.WindowTitle == "" refuses instead of matching. A target that never
	// got injected would otherwise accept an untitled window, which is the
	// fixture behavior this SPEC removed wearing a different name. Equality, not
	// a suffix: the envelope carries the title verbatim, so a suffix rule would
	// let "Desktop" satisfy a declaration of "Autopus Desktop".
	if window.ID == nil || *window.ID != binding.id ||
		target.WindowTitle == "" || window.Title == nil || *window.Title != target.WindowTitle ||
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

func orcaFreshRef(random io.Reader, prefix string) (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value), nil
}
