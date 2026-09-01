package run

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

// SPEC-QAMESH-013 REQ-4: observe fully, publish selectively.
//
// A real accessibility tree carries user content. Measured: Finder exposes the
// user's folder names and raw AppKit action metadata; Autopus Desktop exposes
// in-flight UI copy. So the published projection contains ONLY nodes whose names
// the pack declared through required_landmarks. Everything else contributes to a
// count and nothing else - no name, no value, no text.
//
// This is also what keeps the existing guards satisfiable by construction rather
// than by luck: the oracle's exact name allowlist
// (pkg/qa/desktopobserve/oracle.go), the 8 KiB typed-evidence bound, and the
// publication scan (pkg/qa/evidence/desktop_publication_scan.go) can all hold
// because the projection never grows with the observed app.

// declaredLandmark is one pack-declared landmark, normalized for matching.
type declaredLandmark struct {
	Role          desktopobserve.Role
	Name          string
	RequiredState desktopobserve.SemanticStateKey
}

// unmatchedLandmark is a declared landmark the observed tree did not contain.
// Typed rather than a "<role>:<name>" string because a window title may contain
// a colon, and a consumer splitting on one would mis-attribute the name.
type unmatchedLandmark struct {
	Role desktopobserve.Role
	Name string
}

// projectionCounts records what was observed but not published, so a reviewer can
// tell "nothing was there" from "plenty was there and we did not publish it".
type projectionCounts struct {
	ObservedNodes  int
	PublishedNodes int
	// UnmatchedLandmarks names the DECLARED landmarks that were absent. Declared
	// names are already known to the project, so naming them leaks nothing.
	UnmatchedLandmarks []unmatchedLandmark
}

// buildDesktopProjection turns a parsed tree plus the pack's declarations into a
// publishable projection.
//
// The application and window nodes come from the tree header lines, never from
// role phrases: those are emitted in the OS locale (REQ-2). Deeper declared
// landmarks are matched by name against the observed nodes.
func buildDesktopProjection(
	tree desktopTree,
	landmarks []declaredLandmark,
	appRef string,
	windowRef string,
	providerRef string,
	refs func(prefix string) (string, error),
) (desktopobserve.SemanticProjection, projectionCounts, error) {
	counts := projectionCounts{ObservedNodes: len(tree.Nodes)}
	appLandmark, windowLandmark, deeper := splitDeclaredLandmarks(landmarks)
	if appLandmark == nil || windowLandmark == nil {
		return desktopobserve.SemanticProjection{}, counts, &treeParseError{
			Reason: "pack must declare one application and one window landmark",
		}
	}

	stateRef, err := refs("state-")
	if err != nil {
		return desktopobserve.SemanticProjection{}, counts, err
	}
	appNodeRef, err := refs("node-")
	if err != nil {
		return desktopobserve.SemanticProjection{}, counts, err
	}
	windowNodeRef, err := refs("node-")
	if err != nil {
		return desktopobserve.SemanticProjection{}, counts, err
	}

	if !nameMatchesHeader(tree.AppName, appLandmark.Name) {
		counts.UnmatchedLandmarks = append(counts.UnmatchedLandmarks,
			unmatchedLandmark{Role: appLandmark.Role, Name: appLandmark.Name})
	}
	if !nameMatchesHeader(tree.WindowTitle, windowLandmark.Name) {
		counts.UnmatchedLandmarks = append(counts.UnmatchedLandmarks,
			unmatchedLandmark{Role: windowLandmark.Role, Name: windowLandmark.Name})
	}

	children, deeperCounts, err := projectDeclaredNodes(tree, deeper, refs)
	if err != nil {
		return desktopobserve.SemanticProjection{}, counts, err
	}
	counts.UnmatchedLandmarks = append(counts.UnmatchedLandmarks, deeperCounts...)

	enabled := true
	// Window focus is a real signal, not an assumption: the provider reports the
	// focused element id, and the tree it reports is the target window's tree. A
	// focused element inside it means the window holds keyboard focus.
	focused := treeReportsFocus(tree)
	window := desktopobserve.SemanticNode{
		NodeRef: windowNodeRef, Role: desktopobserve.RoleWindow, Name: windowLandmark.Name,
		SemanticState: desktopobserve.SemanticState{Focused: &focused},
		Children:      children,
	}
	root := desktopobserve.SemanticNode{
		NodeRef: appNodeRef, Role: desktopobserve.RoleApplication, Name: appLandmark.Name,
		SemanticState: desktopobserve.SemanticState{Enabled: &enabled},
		Children:      []desktopobserve.SemanticNode{window},
	}
	counts.PublishedNodes = 2 + len(children)
	return desktopobserve.SemanticProjection{
		SchemaVersion: desktopobserve.SemanticProjectionSchemaVersion,
		ProviderRef:   providerRef,
		AppRef:        appRef,
		WindowRef:     windowRef,
		StateRef:      stateRef,
		Root:          root,
	}, counts, nil
}

// splitDeclaredLandmarks separates the two canonical landmarks from any deeper
// ones. The journey layer already guarantees exactly one application and one
// window landmark in that order, but this function does not rely on ordering.
func splitDeclaredLandmarks(
	landmarks []declaredLandmark,
) (app *declaredLandmark, window *declaredLandmark, deeper []declaredLandmark) {
	for index := range landmarks {
		switch landmarks[index].Role {
		case desktopobserve.RoleApplication:
			if app == nil {
				app = &landmarks[index]
				continue
			}
		case desktopobserve.RoleWindow:
			if window == nil {
				window = &landmarks[index]
				continue
			}
		}
		deeper = append(deeper, landmarks[index])
	}
	return app, window, deeper
}

// projectDeclaredNodes publishes one node per declared deeper landmark that the
// observed tree actually contains. An undeclared observed node is never
// published, so this loop is bounded by the pack, not by the app.
func projectDeclaredNodes(
	tree desktopTree,
	deeper []declaredLandmark,
	refs func(prefix string) (string, error),
) ([]desktopobserve.SemanticNode, []unmatchedLandmark, error) {
	var nodes []desktopobserve.SemanticNode
	var unmatched []unmatchedLandmark
	for _, landmark := range deeper {
		observed, found := findDeclaredNode(tree, landmark)
		if !found {
			unmatched = append(unmatched, unmatchedLandmark{Role: landmark.Role, Name: landmark.Name})
			continue
		}
		nodeRef, err := refs("node-")
		if err != nil {
			return nil, nil, err
		}
		node := desktopobserve.SemanticNode{
			NodeRef: nodeRef, Role: landmark.Role, Name: landmark.Name,
			SemanticState: declaredNodeState(observed, landmark.RequiredState),
		}
		nodes = append(nodes, node)
	}

	return nodes, unmatched, nil
}

// findDeclaredNode locates the observed node satisfying a declared landmark. The
// role phrase is not consulted: it is localized, so a declared AXRole cannot be
// matched against it (REQ-2). The name is the contract.
func findDeclaredNode(tree desktopTree, landmark declaredLandmark) (desktopTreeNode, bool) {
	for _, node := range tree.Nodes {
		if node.matchesDeclaredName(landmark.Name) {
			return node, true
		}
	}
	return desktopTreeNode{}, false
}

// declaredNodeState maps the renderer's inlined state markers onto the typed
// state the oracle checks. Only the state the pack asked about is reported;
// inventing others would publish observations nobody declared.
func declaredNodeState(
	node desktopTreeNode,
	required desktopobserve.SemanticStateKey,
) desktopobserve.SemanticState {
	state := desktopobserve.SemanticState{}
	switch required {
	case desktopobserve.StateEnabled:
		// The renderer marks the negative case only: an element is "(disabled)"
		// when it is not enabled, and carries no marker when it is.
		value := !node.hasStateMarker("disabled")
		state.Enabled = &value
	case desktopobserve.StateFocused:
		value := node.hasStateMarker("focused")
		state.Focused = &value
	case desktopobserve.StateExpanded:
		value := node.hasStateMarker("expanded")
		state.Expanded = &value
	case desktopobserve.StateSelected:
		value := node.hasStateMarker("selected")
		state.Selected = &value
	}
	return state
}

// nameMatchesHeader compares a declared landmark name against a header value.
// The header carries the label verbatim, so this is equality after trimming; a
// suffix rule here would let "Desktop" satisfy a declaration of "Autopus
// Desktop".
func nameMatchesHeader(observed, declared string) bool {
	return strings.TrimSpace(observed) == strings.TrimSpace(declared)
}

// treeReportsFocus reports whether the focused element id names a node in this
// tree.
func treeReportsFocus(tree desktopTree) bool {
	for _, node := range tree.Nodes {
		if node.ID == tree.FocusedElementID {
			return true
		}
	}
	return false
}
