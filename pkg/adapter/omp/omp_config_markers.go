package omp

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// markerReYml matches one managed section. Matching is line-anchored and
// non-greedy, which keeps adjacent sections distinct and stops a marker glued to
// other text on the same line from opening a section.
//
// Line anchoring alone is NOT enough, and it is worth being precise about why:
// the regex is blind to YAML structure, so a marker line sitting inside a
// literal block scalar reads as ordinary text to YAML but as a section opener
// here. validateMarkerBalance closes that gap by refusing any file whose markers
// appear inside a parsed scalar value.
var markerReYml = regexp.MustCompile(
	`(?ms)^[ \t]*` + regexp.QuoteMeta(markerBeginYml) + `[ \t]*$.*?^[ \t]*` + regexp.QuoteMeta(markerEndYml) + `[ \t]*$\n?`)

var (
	markerBeginLineRe = regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(markerBeginYml) + `[ \t]*$`)
	markerEndLineRe   = regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(markerEndYml) + `[ \t]*$`)
)

// validateMarkerBalance refuses a config whose managed markers do not pair up.
// An orphaned BEGIN is the dangerous case: the rewrite branch would not fire, a
// second BEGIN would be appended, and the next run would then match from the
// first BEGIN to the first END and swallow every user key between them. There is
// no backup on the Generate path, so that loss would be silent and permanent.
func validateMarkerBalance(existing string) error {
	if err := rejectMarkerInsideScalar(existing); err != nil {
		return err
	}

	begins := len(markerBeginLineRe.FindAllString(existing, -1))
	ends := len(markerEndLineRe.FindAllString(existing, -1))
	if begins != ends {
		return fmt.Errorf(
			"%s의 관리 구간 마커가 짝이 맞지 않습니다(BEGIN %d, END %d). 사용자 키 손실을 막기 위해 재작성을 중단합니다",
			configFile, begins, ends)
	}
	if begins > 1 {
		return fmt.Errorf(
			"%s에 관리 구간이 %d개 있습니다. 하나만 남기고 정리한 뒤 다시 실행하세요", configFile, begins)
	}
	return nil
}

// rejectMarkerInsideScalar refuses a config whose marker text lives inside a
// YAML value rather than at document structure level. A correctly placed marker
// is a YAML comment, so it never appears in the parsed node tree at all; text
// that DOES show up in a scalar came from a block or quoted value, where the
// regex would treat it as a section boundary and swallow the user's data.
//
// An unparseable file is refused for the same reason: without a node tree there
// is no way to tell structure from content, and guessing risks the data loss
// this check exists to prevent.
func rejectMarkerInsideScalar(existing string) error {
	if strings.TrimSpace(existing) == "" {
		return nil
	}

	// Every document must be inspected, not just the first. yaml.Unmarshal into
	// a Node decodes document 1 only, so marker text in a scalar of document 2+
	// slipped past this guard while the raw-text regex still matched it.
	decoder := yaml.NewDecoder(strings.NewReader(existing))
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s를 YAML로 읽을 수 없어 관리 구간 재작성을 중단합니다: %w", configFile, err)
		}
		if scalarCarriesMarker(&doc) {
			return fmt.Errorf(
				"%s의 값 안에 관리 구간 마커 문자열이 있습니다. 재작성하면 해당 값이 사라지므로 중단합니다", configFile)
		}
		if markerCommentOutsideRoot(&doc) {
			return fmt.Errorf(
				"%s의 컬렉션 내부에 관리 구간 마커 주석이 있습니다. 어댑터가 쓰지 않는 위치이므로 재작성을 중단합니다", configFile)
		}
	}
}

func scalarCarriesMarker(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.ScalarNode {
		return strings.Contains(node.Value, markerBeginYml) ||
			strings.Contains(node.Value, markerEndYml)
	}
	for _, child := range node.Content {
		if scalarCarriesMarker(child) {
			return true
		}
	}
	return false
}

// commentCarriesMarker reports whether a comment string mentions a marker.
func commentCarriesMarker(comment string) bool {
	return strings.Contains(comment, markerBeginYml) || strings.Contains(comment, markerEndYml)
}

func nodeCommentsCarryMarker(n *yaml.Node) bool {
	return commentCarriesMarker(n.HeadComment) ||
		commentCarriesMarker(n.LineComment) ||
		commentCarriesMarker(n.FootComment)
}

// markerCommentOutsideRoot reports whether a marker comment sits somewhere the
// adapter would never place one.
//
// Position is the whole point here, and a blanket "no marker in comments" rule
// would be wrong: the adapter's own BEGIN/END are themselves comments. Measured
// against go-yaml, a managed section attaches BEGIN as the HeadComment and END as
// the FootComment of the section's key node — a direct child of the root mapping.
// So comments on the document node, the root node, and the root mapping's direct
// children are legitimate anchors.
//
// Anywhere deeper is a position the adapter never writes: a sequence item, a
// member of a flow collection, or a key inside a nested mapping. A marker there
// is user text that the line-anchored regex would still read as a section
// boundary, and rewriting across it both destroys the user's value and leaves the
// document unparseable. Root-level sequences are excluded too, since the adapter
// only ever writes a root mapping.
func markerCommentOutsideRoot(doc *yaml.Node) bool {
	if doc == nil {
		return false
	}

	allowed := map[*yaml.Node]bool{doc: true}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root := doc.Content[0]
		allowed[root] = true
		if root.Kind == yaml.MappingNode {
			for _, child := range root.Content {
				allowed[child] = true
			}
		}
	}

	found := false
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n == nil || found {
			return
		}
		if !allowed[n] && nodeCommentsCarryMarker(n) {
			found = true
			return
		}
		for _, child := range n.Content {
			walk(child)
		}
	}
	walk(doc)
	return found
}
