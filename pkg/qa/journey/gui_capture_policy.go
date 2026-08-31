package journey

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
)

// validateGUICapturePolicy gates the typed `gui.capture` block.
//
// It lives in its own file because gui_policy.go already carries the origin,
// action, network, and screen-matrix rules, and because capture is the one GUI
// policy that decides what raw media exists on the machine afterwards.
func validateGUICapturePolicy(pack Pack) error {
	policy := pack.GUI.Capture
	if pack.Adapter.ID != "gui-explore" {
		// Only the GUI adapter has a capture runtime. Declaring capture on any
		// other adapter would promise evidence nothing produces.
		if policy.Declared() {
			return validationError("qa_journey_gui_capture_invalid", "gui.capture is only supported by the gui-explore adapter")
		}
		return nil
	}
	if err := capture.ValidatePolicy(policy); err != nil {
		return validationError("qa_journey_gui_capture_invalid", err.Error())
	}
	if !policy.Enabled() {
		return nil
	}
	// publish_raw is already rejected categorically by validateGUIPolicy, so no
	// capture-level combination check is reachable here.
	if policy.HasStream(capture.StreamNetwork) && strings.TrimSpace(pack.GUI.NetworkPolicy.Mode) == "blocked" {
		return validationError("qa_journey_gui_capture_invalid", "gui.capture network stream contradicts gui.network_policy.mode blocked")
	}
	return nil
}
