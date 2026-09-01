package run

import (
	"fmt"
	"strconv"
	"strings"
)

// SPEC-QAMESH-013 REQ-1, REQ-2: parse the provider's observed accessibility tree
// into a real hierarchy.
//
// The Orca CLI exposes the tree only as rendered text (`snapshot.treeText`); it
// offers no structured node array and no flag that produces one. That text is
// emitted in the operating system's locale, so role phrases differ per machine -
// measured: "standard window" versus "표준 윈도우", "scroll area" versus
// "스크롤 영역". Nothing here compares a role phrase against a literal. Application
// and window identity come from the two header lines, which are locale-stable.

// desktopTreeNode is one observed element. RolePhrase stays opaque on purpose.
type desktopTreeNode struct {
	ID         int
	Depth      int
	RolePhrase string
	Name       string
	// StateMarkers are the parenthesised tokens the renderer inlines into the
	// role phrase, e.g. "selected", "expanded".
	StateMarkers []string
	// Attributes are the trailing ", Key: value" segments, e.g. Text, Value,
	// "Secondary Actions".
	Attributes map[string]string
	// LastAttribute is the key the renderer emitted last, so a wrapped value can
	// be folded back into the right place.
	LastAttribute string
}

// desktopTree is the parsed observation.
type desktopTree struct {
	// AppIdentifier is the platform identifier the provider reported, e.g. a
	// macOS bundle id. It is compared against the pack's provider_app_id and is
	// never published.
	AppIdentifier string
	PID           int
	// AppName and WindowTitle are the accessibility labels the pack declares
	// through its landmarks.
	AppName          string
	WindowTitle      string
	FocusedElementID int
	Nodes            []desktopTreeNode
}

// Observation bounds, aligned to what the provider itself renders: Orca reports
// maxNodes 1200 and maxDepth 64 in every snapshot's truncation block, so a larger
// tree cannot reach us and a smaller ceiling would refuse trees the provider was
// willing to describe.
//
// These bound PARSE cost, not published evidence. The published projection is
// bounded by the pack's declared landmarks (SPEC-QAMESH-013 REQ-4), so it does
// not grow with the app; the 256-node and depth-32 limits in
// pkg/qa/desktopobserve/canonical.go still guard that, and the 8 KiB typed
// evidence bound remains the privacy-relevant one.
//
// The previous 256 predates real observation, when the projection was three
// synthesized nodes. Measured element counts: Autopus Desktop 7-22 by load
// state, Finder 152, Slack 378 - so 256 refused a mainstream app.
const (
	desktopTreeMaxNodes = 1200
	desktopTreeMaxDepth = 64
)

// treeParseError names the malformed condition. A bare "malformed" gives an
// operator nothing to act on, which is how a window-title mismatch was once
// reported as a provider lifecycle failure.
type treeParseError struct {
	Reason string
	Line   int
}

func (e *treeParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("desktop tree line %d: %s", e.Line, e.Reason)
	}
	return "desktop tree: " + e.Reason
}

func treeError(line int, format string, args ...any) error {
	return &treeParseError{Reason: fmt.Sprintf(format, args...), Line: line}
}

// parseDesktopTree parses the rendered accessibility tree. It fails closed:
// every structural surprise is an error rather than a silently dropped node,
// because a dropped node would let an undeclared element escape the count that
// REQ-4 relies on.
func parseDesktopTree(text string) (desktopTree, error) {
	if strings.ContainsAny(text, "\r\x00") {
		return desktopTree{}, treeError(0, "carriage return or NUL is not allowed")
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 5 {
		return desktopTree{}, treeError(0, "too few lines to carry a header, a body, and a focus line")
	}
	tree := desktopTree{}
	var err error
	if tree.AppIdentifier, tree.PID, err = parseDesktopTreeAppLine(lines[0]); err != nil {
		return desktopTree{}, err
	}
	if tree.WindowTitle, tree.AppName, err = parseDesktopTreeWindowLine(lines[1]); err != nil {
		return desktopTree{}, err
	}
	if strings.TrimSpace(lines[2]) != "" {
		return desktopTree{}, treeError(3, "expected a blank line between the header and the body")
	}
	body, focusLine, err := splitDesktopTreeBody(lines)
	if err != nil {
		return desktopTree{}, err
	}
	if tree.FocusedElementID, err = parseDesktopTreeFocusLine(focusLine); err != nil {
		return desktopTree{}, err
	}
	if tree.Nodes, err = parseDesktopTreeNodes(body); err != nil {
		return desktopTree{}, err
	}
	return tree, nil
}

// parseDesktopTreeAppLine reads `App=<identifier> (pid <n>)`.
func parseDesktopTreeAppLine(line string) (string, int, error) {
	rest, ok := strings.CutPrefix(line, "App=")
	if !ok {
		return "", 0, treeError(1, "expected the header to start with App=")
	}
	identifier, tail, ok := strings.Cut(rest, " (pid ")
	if !ok {
		return "", 0, treeError(1, "expected ` (pid <n>)` after the app identifier")
	}
	digits, ok := strings.CutSuffix(tail, ")")
	if !ok {
		return "", 0, treeError(1, "expected the pid to be closed by `)`")
	}
	pid, convErr := strconv.Atoi(digits)
	if convErr != nil || pid <= 1 {
		return "", 0, treeError(1, "pid %q is not a usable process id", digits)
	}
	if strings.TrimSpace(identifier) == "" {
		return "", 0, treeError(1, "app identifier is empty")
	}
	return identifier, pid, nil
}

// parseDesktopTreeWindowLine reads `Window: "<title>", App: <name>.`
func parseDesktopTreeWindowLine(line string) (string, string, error) {
	rest, ok := strings.CutPrefix(line, `Window: "`)
	if !ok {
		return "", "", treeError(2, `expected the window line to start with Window: "`)
	}
	title, tail, ok := strings.Cut(rest, `", App: `)
	if !ok {
		return "", "", treeError(2, "expected `\", App: ` between the window title and the app name")
	}
	name, ok := strings.CutSuffix(tail, ".")
	if !ok {
		return "", "", treeError(2, "expected the window line to end with a period")
	}
	if title == "" || strings.TrimSpace(name) == "" {
		return "", "", treeError(2, "window title and app name must both be present")
	}
	return title, name, nil
}

// splitDesktopTreeBody separates the node lines from the trailing focus line.
// The renderer emits a blank line before the focus line; anything else means the
// shape changed and guessing would risk parsing the focus line as a node.
func splitDesktopTreeBody(lines []string) ([]string, string, error) {
	last := len(lines) - 1
	for last > 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	if last < 4 {
		return nil, "", treeError(0, "no focus line found")
	}
	focusLine := lines[last]
	if strings.TrimSpace(lines[last-1]) != "" {
		return nil, "", treeError(last, "expected a blank line before the focus line")
	}
	return lines[3 : last-1], focusLine, nil
}

// desktopTreeNoFocusID marks a tree whose window holds no focused element. It is
// not a valid element id, so no node can match it and treeReportsFocus is false.
const desktopTreeNoFocusID = -1

// parseDesktopTreeFocusLine reads the trailing focus sentence.
//
// The provider renders two forms and both are legitimate: an app holding keyboard
// focus reports "The focused UI element is <id> ...", and one that does not
// reports "No UI element is currently focused." Measured on Slack while the
// terminal held focus. Refusing the second form rejected a correctly observed
// unfocused app as a malformed envelope, which is a real state reported as a
// protocol fault.
func parseDesktopTreeFocusLine(line string) (int, error) {
	if strings.TrimSpace(line) == "No UI element is currently focused." {
		return desktopTreeNoFocusID, nil
	}
	rest, ok := strings.CutPrefix(line, "The focused UI element is ")
	if !ok {
		return 0, treeError(0, "expected a focus line naming the focused element or stating that none is focused")
	}
	digits, _, _ := strings.Cut(rest, " ")
	id, convErr := strconv.Atoi(digits)
	if convErr != nil || id < 0 {
		return 0, treeError(0, "focused element id %q is not a number", digits)
	}
	return id, nil
}
