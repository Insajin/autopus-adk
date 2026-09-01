package run

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

// desktopTreeNodeStart matches a line that opens a new node: tab indentation
// followed by the element id.
//
// A body line that does not match is a CONTINUATION of the previous node, not a
// malformed line. Measured on Finder: an accessibility action value carries
// embedded newlines, so one node spans several physical lines:
//
//	\t\t132 container, Secondary Actions: name:다음으로 이동
//	 target:0x0
//	 selector:(null), name:도구 막대에서 유겨기
//
// Rejecting those would reject a stock macOS app. One node per line was the
// assumption real data disproved.
var desktopTreeNodeStart = regexp.MustCompile(`^\t*[0-9]+ `)

// SPEC-QAMESH-013 REQ-1: node lines. Depth is the leading tab count; any other
// leading whitespace on a node-opening line is refused because a space-indented
// renderer would collapse distinct depths and silently reparent nodes.
func parseDesktopTreeNodes(body []string) ([]desktopTreeNode, error) {
	nodes := make([]desktopTreeNode, 0, len(body))
	seen := make(map[int]struct{}, len(body))
	previousDepth := -1
	for index, line := range body {
		lineNumber := index + 4
		if !desktopTreeNodeStart.MatchString(line) {
			if len(nodes) == 0 {
				return nil, treeError(lineNumber, "the tree body must open with a node line")
			}
			// Continuation of a wrapped attribute value. Folding it back keeps the
			// value whole and never creates a phantom node.
			appendDesktopTreeContinuation(&nodes[len(nodes)-1], line)
			continue
		}
		depth, rest, err := splitDesktopTreeIndent(line, lineNumber)
		if err != nil {
			return nil, err
		}
		// REQ-6: a crossed bound is refused by the name of the bound, not as a
		// generic parse fault. A bare treeError carries no reason code, so it
		// routes to provider_unavailable - the misreport this SPEC removes. The
		// limit comes from the constant so the bound and the reported limit
		// cannot drift. Every other treeError here stays bare on purpose: a
		// malformed tree is not a bound violation.
		if depth > desktopTreeMaxDepth {
			return nil, desktopobserve.ObservedTreeBoundExceeded(
				desktopobserve.ObservedTreeBoundDepth, desktopTreeMaxDepth, depth)
		}
		// A jump of more than one means a node was dropped by the renderer or by
		// us; either way the hierarchy is no longer trustworthy.
		if previousDepth >= 0 && depth > previousDepth+1 {
			return nil, treeError(lineNumber, "depth jumped from %d to %d", previousDepth, depth)
		}
		if previousDepth < 0 && depth != 0 {
			return nil, treeError(lineNumber, "the first node must be at depth 0")
		}
		node, err := parseDesktopTreeNodeBody(rest, depth, lineNumber)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[node.ID]; duplicate {
			return nil, treeError(lineNumber, "duplicate element id %d", node.ID)
		}
		seen[node.ID] = struct{}{}
		nodes = append(nodes, node)
		previousDepth = depth
		if len(nodes) > desktopTreeMaxNodes {
			return nil, desktopobserve.ObservedTreeBoundExceeded(
				desktopobserve.ObservedTreeBoundNodes, desktopTreeMaxNodes, len(nodes))
		}
	}
	if len(nodes) == 0 {
		return nil, treeError(0, "the tree body declared no nodes")
	}
	return nodes, nil
}

// appendDesktopTreeContinuation folds a wrapped line back into the node it
// belongs to. It lands on the attribute the renderer was emitting when it
// wrapped; with no attribute it extends the role phrase, never the declared-name
// match surface, so a wrapped value can never satisfy a landmark by accident.
func appendDesktopTreeContinuation(node *desktopTreeNode, line string) {
	fragment := strings.TrimSpace(line)
	if fragment == "" {
		return
	}
	if node.Attributes != nil && node.LastAttribute != "" {
		node.Attributes[node.LastAttribute] += " " + fragment
		return
	}
	node.RolePhrase = strings.TrimSpace(node.RolePhrase + " " + fragment)
}

func splitDesktopTreeIndent(line string, lineNumber int) (int, string, error) {
	depth := 0
	for depth < len(line) && line[depth] == '\t' {
		depth++
	}
	rest := line[depth:]
	if strings.HasPrefix(rest, " ") {
		return 0, "", treeError(lineNumber, "indentation must be tabs only")
	}
	return depth, rest, nil
}

// parseDesktopTreeNodeBody reads `<id> <role phrase>[, <Key>: <value>]...`.
//
// The role phrase and the name are not separable without knowing the locale's
// role vocabulary, so both are kept: RolePhrase holds the whole phrase and Name
// holds the trailing portion the renderer appends after the role words. When the
// two cannot be told apart, Name is empty and only the phrase is recorded -
// REQ-4 matching is by declared name, so an unnamed node simply never matches.
func parseDesktopTreeNodeBody(rest string, depth, lineNumber int) (desktopTreeNode, error) {
	digits, tail, ok := strings.Cut(rest, " ")
	if !ok {
		return desktopTreeNode{}, treeError(lineNumber, "expected `<id> <role phrase>`")
	}
	id, convErr := strconv.Atoi(digits)
	if convErr != nil || id < 0 {
		return desktopTreeNode{}, treeError(lineNumber, "element id %q is not a number", digits)
	}
	phrase, attributes, lastKey := splitDesktopTreeAttributes(tail)
	markers, phrase := extractDesktopTreeStateMarkers(phrase)
	return desktopTreeNode{
		ID: id, Depth: depth, RolePhrase: strings.TrimSpace(phrase),
		Name: desktopTreeNodeName(phrase), StateMarkers: markers,
		Attributes: attributes, LastAttribute: lastKey,
	}, nil
}

// splitDesktopTreeAttributes peels trailing ", Key: value" segments and reports
// which key appeared last in the rendered line. Only a known key starts a
// segment, so a comma inside a name does not truncate it - measured names contain
// commas and em dashes. The last key matters because that is where a wrapped
// value continues on the following physical line.
func splitDesktopTreeAttributes(tail string) (string, map[string]string, string) {
	attributes := map[string]string{}
	lastKey, lastIndex := "", -1
	for _, key := range []string{"Secondary Actions", "Value", "Text", "Description"} {
		marker := ", " + key + ": "
		index := strings.Index(tail, marker)
		if index < 0 {
			continue
		}
		if index > lastIndex {
			lastKey, lastIndex = key, index
		}
	}
	if lastKey == "" {
		return tail, nil, ""
	}
	for _, key := range []string{"Secondary Actions", "Value", "Text", "Description"} {
		marker := ", " + key + ": "
		head, value, found := strings.Cut(tail, marker)
		if !found {
			continue
		}
		attributes[key] = strings.TrimSpace(value)
		tail = head
	}
	return tail, attributes, lastKey
}

// extractDesktopTreeStateMarkers lifts parenthesised tokens such as
// "(selected)" or "(expanded)" out of the role phrase.
func extractDesktopTreeStateMarkers(phrase string) ([]string, string) {
	var markers []string
	for {
		open := strings.Index(phrase, "(")
		if open < 0 {
			break
		}
		close := strings.Index(phrase[open:], ")")
		if close < 0 {
			break
		}
		close += open
		token := strings.TrimSpace(phrase[open+1 : close])
		if token != "" {
			markers = append(markers, token)
		}
		phrase = strings.TrimSpace(phrase[:open]) + " " + strings.TrimSpace(phrase[close+1:])
	}
	return markers, strings.Join(strings.Fields(phrase), " ")
}

// desktopTreeNodeName returns the phrase as the candidate name. The renderer
// concatenates role words and the accessibility label with no delimiter, so the
// full phrase is the only honest candidate; a declared landmark matches when the
// phrase ends with the declared name.
func desktopTreeNodeName(phrase string) string {
	return strings.Join(strings.Fields(phrase), " ")
}

// matchesDeclaredName reports whether an observed node satisfies a declared
// landmark name. The comparison is a suffix match because the renderer prefixes
// localized role words to the label: "0 표준 윈도우 응용 프로그램" carries the label
// "응용 프로그램". Equality would require translating role words, which REQ-2
// forbids.
func (node desktopTreeNode) matchesDeclaredName(declared string) bool {
	declared = strings.TrimSpace(declared)
	if declared == "" {
		return false
	}
	if node.Name == declared {
		return true
	}
	return strings.HasSuffix(node.Name, " "+declared)
}

// hasStateMarker reports whether the renderer inlined a state token.
func (node desktopTreeNode) hasStateMarker(marker string) bool {
	for _, candidate := range node.StateMarkers {
		if strings.EqualFold(candidate, marker) {
			return true
		}
	}
	return false
}
