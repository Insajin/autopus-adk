package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func readDesktopTreeFixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "desktop_trees", name))
	require.NoError(t, err)
	return string(body)
}

// AC-QAMESH13-001: a real localized tree. Captured from Finder on macOS with a
// Korean UI locale: 152 elements, 5598 bytes, role phrases such as
// "표준 윈도우" and "셀". The 2048-byte cap the fixture path enforced would have
// rejected this outright.
func TestParseDesktopTree_RealLocalizedFinderTree(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "finder.txt"))
	require.NoError(t, err)

	assert.Equal(t, "com.apple.finder", tree.AppIdentifier)
	assert.Equal(t, "Finder", tree.AppName)
	assert.Equal(t, "응용 프로그램", tree.WindowTitle)
	assert.Equal(t, 90, tree.FocusedElementID)
	assert.Len(t, tree.Nodes, 152)

	assert.Equal(t, 0, tree.Nodes[0].Depth)
	assert.Equal(t, 0, tree.Nodes[0].ID)
	assert.True(t, tree.Nodes[0].matchesDeclaredName("응용 프로그램"),
		"the depth-0 node carries the window label after localized role words")

	// The renderer inlines state as a parenthesised token; it must become a
	// marker rather than part of the name.
	var selected []desktopTreeNode
	for _, node := range tree.Nodes {
		if node.hasStateMarker("selected") {
			selected = append(selected, node)
		}
	}
	require.NotEmpty(t, selected, "Finder marks the active sidebar row selected")
	for _, node := range selected {
		assert.NotContains(t, node.Name, "(selected)")
	}
}

// AC-QAMESH13-002: a real English tree from our own app, whose UI is a web view.
func TestParseDesktopTree_RealAutopusDesktopTree(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "autopus.txt"))
	require.NoError(t, err)

	assert.Equal(t, "co.autopus.desktop", tree.AppIdentifier)
	assert.Equal(t, "Autopus Desktop", tree.AppName)
	assert.Equal(t, "Autopus Desktop", tree.WindowTitle)
	assert.Equal(t, 2, tree.FocusedElementID)
	assert.Len(t, tree.Nodes, 7)

	var fullScreen *desktopTreeNode
	for index, node := range tree.Nodes {
		if strings.Contains(node.RolePhrase, "full screen") {
			fullScreen = &tree.Nodes[index]
		}
	}
	require.NotNil(t, fullScreen)
	assert.Equal(t, "zoom the window", fullScreen.Attributes["Secondary Actions"])
	assert.NotContains(t, fullScreen.Name, "Secondary Actions")

	// A name containing an em dash and non-ASCII text must survive intact; the
	// attribute splitter must not truncate it.
	var htmlContent *desktopTreeNode
	for index, node := range tree.Nodes {
		if node.ID == 2 {
			htmlContent = &tree.Nodes[index]
		}
	}
	require.NotNil(t, htmlContent)
	assert.Contains(t, htmlContent.Name, "AI가 회사를 운영합니다")
}

// AC-QAMESH13-003: identity comes from the header lines, never from role words.
// Both fixtures are asserted through one code path with no locale knowledge.
func TestParseDesktopTree_IdentityIsLocaleIndependent(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]struct{ id, app, window string }{
		"finder.txt":  {"com.apple.finder", "Finder", "응용 프로그램"},
		"autopus.txt": {"co.autopus.desktop", "Autopus Desktop", "Autopus Desktop"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tree, err := parseDesktopTree(readDesktopTreeFixture(t, name))
			require.NoError(t, err)
			assert.Equal(t, want.id, tree.AppIdentifier)
			assert.Equal(t, want.app, tree.AppName)
			assert.Equal(t, want.window, tree.WindowTitle)
		})
	}
}

const minimalDesktopTree = `App=com.example.app (pid 4242)
Window: "Main", App: Example.

0 standard window Main
	1 button Save

The focused UI element is 1 button Save.`

func TestParseDesktopTree_AcceptsMinimalShape(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(minimalDesktopTree)
	require.NoError(t, err)
	assert.Equal(t, 4242, tree.PID)
	require.Len(t, tree.Nodes, 2)
	assert.Equal(t, 1, tree.Nodes[1].Depth)
	assert.True(t, tree.Nodes[1].matchesDeclaredName("Save"))
	assert.False(t, tree.Nodes[1].matchesDeclaredName("Cancel"))
}

// A third real capture, from Slack while the terminal held keyboard focus. The
// provider renders "No UI element is currently focused." instead of naming an
// element, and 378 nodes - both of which the first implementation refused: the
// focus sentence as a malformed envelope, and the node count against a 256 bound
// that predated real observation.
func TestParseDesktopTree_RealUnfocusedThirdPartyTree(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "slack-unfocused.txt"))
	require.NoError(t, err)

	assert.Equal(t, "com.tinyspeck.slackmacgap", tree.AppIdentifier)
	assert.Equal(t, "Slack", tree.AppName)
	assert.Equal(t, "maker-v2(채널) - Aligo - Slack", tree.WindowTitle,
		"a real window title carries parentheses, a hyphen, and non-ASCII text")
	assert.Len(t, tree.Nodes, 378, "well past the 256 bound the fixture era assumed")
	assert.Equal(t, desktopTreeNoFocusID, tree.FocusedElementID)
	assert.False(t, treeReportsFocus(tree),
		"an unfocused app must report no focus, not a phantom focused node")
}

// The focused form and the unfocused form must both parse, and only the focused
// one may report focus. Getting this backwards would let an unfocused window
// satisfy a pack that declared required_state: focused.
func TestParseDesktopTreeFocusLine_BothProviderForms(t *testing.T) {
	t.Parallel()

	id, err := parseDesktopTreeFocusLine("The focused UI element is 7 button Save.")
	require.NoError(t, err)
	assert.Equal(t, 7, id)

	id, err = parseDesktopTreeFocusLine("No UI element is currently focused.")
	require.NoError(t, err)
	assert.Equal(t, desktopTreeNoFocusID, id)

	_, err = parseDesktopTreeFocusLine("Something else entirely.")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "focus line")
}

// Every structural surprise must be named. A dropped node would let an undeclared
// element escape the count REQ-4 depends on, so silence is not an option.
func TestParseDesktopTree_FailsClosedOnMalformedInput(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(string) string
		want   string
	}{
		"carriage return":   {func(s string) string { return strings.Replace(s, "\n", "\r\n", 1) }, "carriage return"},
		"missing app line":  {func(s string) string { return strings.Replace(s, "App=", "Application=", 1) }, "start with App="},
		"missing pid":       {func(s string) string { return strings.Replace(s, " (pid 4242)", "", 1) }, "(pid <n>)"},
		"bad pid":           {func(s string) string { return strings.Replace(s, "4242", "zero", 1) }, "usable process id"},
		"window line shape": {func(s string) string { return strings.Replace(s, `Window: "`, "Win: ", 1) }, `Window: "`},
		"no focus line": {func(s string) string {
			return strings.Replace(s, "\nThe focused UI element is 1 button Save.", "", 1)
		}, "focus line"},
		"depth jump": {func(s string) string {
			return strings.Replace(s, "\t1 button", "\t\t\t1 button", 1)
		}, "depth jumped"},
		"duplicate id": {func(s string) string {
			return strings.Replace(s, "\t1 button Save", "\t0 button Save", 1)
		}, "duplicate element id"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := parseDesktopTree(tc.mutate(minimalDesktopTree))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// A body line that does not open a node is a continuation, because the renderer
// wraps long attribute values. The load-bearing property is that folded text can
// never satisfy a declared landmark: it extends the role phrase or an attribute,
// never the Name that matchesDeclaredName reads.
func TestParseDesktopTree_FoldsContinuationWithoutCreatingMatchableNames(t *testing.T) {
	t.Parallel()

	for name, mutate := range map[string]func(string) string{
		"space indented": func(s string) string { return strings.Replace(s, "\t1 button", "  1 button", 1) },
		"no element id":  func(s string) string { return strings.Replace(s, "\t1 button", "\tone button", 1) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tree, err := parseDesktopTree(mutate(minimalDesktopTree))
			require.NoError(t, err)
			require.Len(t, tree.Nodes, 1, "a continuation must not create a node")
			assert.False(t, tree.Nodes[0].matchesDeclaredName("Save"),
				"folded text must not become a matchable landmark name")
			assert.Contains(t, tree.Nodes[0].RolePhrase, "Save",
				"the fragment is still recorded, just not as a name")
		})
	}
}

// REQ-6: bounds are refused by name, never truncated. Asserting only the message
// substring is what let the node and depth bounds report provider_unavailable
// for a while, so the reason code is asserted too.
func TestParseDesktopTree_RefusesOversizedTreeByName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		body  func(*strings.Builder)
		bound desktopobserve.ObservedTreeBound
	}{
		"node count": {bound: desktopobserve.ObservedTreeBoundNodes, body: func(builder *strings.Builder) {
			builder.WriteString("0 standard window Main\n")
			for index := 1; index <= desktopTreeMaxNodes+5; index++ {
				builder.WriteString("\t" + itoaDesktopTest(index) + " button Item\n")
			}
		}},
		// Depth climbs one level per node, so the depth bound is crossed while the
		// node count is still well inside its own bound.
		"depth": {bound: desktopobserve.ObservedTreeBoundDepth, body: func(builder *strings.Builder) {
			for depth := 0; depth <= desktopTreeMaxDepth+1; depth++ {
				builder.WriteString(strings.Repeat("\t", depth) +
					itoaDesktopTest(depth) + " group Item\n")
			}
		}},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var builder strings.Builder
			builder.WriteString("App=com.example.app (pid 4242)\n")
			builder.WriteString("Window: \"Main\", App: Example.\n\n")
			test.body(&builder)
			builder.WriteString("\nThe focused UI element is 0 standard window Main.")

			_, err := parseDesktopTree(builder.String())
			require.Error(t, err)
			assert.Equal(t, desktopobserve.ReasonObservedTreeBoundExceeded,
				desktopobserve.ReasonCodeOf(err),
				"a crossed bound must not report as provider unavailability")
			assert.Contains(t, err.Error(), string(test.bound))
		})
	}
}

func itoaDesktopTest(value int) string {
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	if digits == "" {
		return "0"
	}
	return digits
}
