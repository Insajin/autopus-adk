package run

import (
	"slices"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func sameDesktopV2Parent(actual, expected *string) bool {
	if actual == nil || expected == nil {
		return actual == nil && expected == nil
	}
	return *actual == *expected
}

func validDesktopV2Actions(actions []string) bool {
	if actions == nil {
		return false
	}
	allowed := []string{
		string(desktopobserve.ActionPress),
		string(desktopobserve.ActionRaise),
		string(desktopobserve.ActionShowMenu),
	}
	for index, action := range actions {
		if !slices.Contains(allowed, action) || (index > 0 && actions[index-1] >= action) {
			return false
		}
	}
	return true
}

func validDesktopV2FrameWire(frame *desktopV2FrameWire) bool {
	if frame == nil {
		return true
	}
	if frame.X == nil || frame.Y == nil || frame.Width == nil || frame.Height == nil {
		return false
	}
	return *frame.X >= 0 && *frame.Y >= 0 && *frame.Width > 0 && *frame.Height > 0 &&
		*frame.X <= 100_000 && *frame.Y <= 100_000 &&
		*frame.Width <= 100_000 && *frame.Height <= 100_000
}

func mapDesktopV2Frame(frame *desktopV2FrameWire) *desktopobserve.Frame {
	if frame == nil {
		return nil
	}
	return &desktopobserve.Frame{
		X: *frame.X, Y: *frame.Y, Width: *frame.Width, Height: *frame.Height,
	}
}

func validDesktopV2Landmarks(root desktopV2Node) bool {
	if root.Role != string(desktopobserve.RoleApplication) || root.Name != "Autopus" ||
		root.ParentNodeRef != nil || !desktopV2StateTrue(root, "enabled") || len(root.Children) != 1 {
		return false
	}
	window := root.Children[0]
	if window.Role != string(desktopobserve.RoleWindow) || window.Name != "Autopus" ||
		window.ParentNodeRef == nil || *window.ParentNodeRef != root.NodeRef ||
		!desktopV2StateTrue(window, "focused") {
		return false
	}
	return hasDesktopV2Disclosure(window.Children)
}

func desktopV2StateTrue(node desktopV2Node, key string) bool {
	value, ok := node.SemanticState[key].(bool)
	return ok && value
}

func hasDesktopV2Disclosure(nodes []desktopV2Node) bool {
	for _, node := range nodes {
		if node.Name == "Disclosure" && node.Role == "AXButton" {
			if _, ok := node.SemanticState["expanded"].(bool); ok {
				return true
			}
		}
		if hasDesktopV2Disclosure(node.Children) {
			return true
		}
	}
	return false
}
