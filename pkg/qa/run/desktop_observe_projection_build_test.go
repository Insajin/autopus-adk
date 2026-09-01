package run

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

// sequentialRefs is deterministic so a projection can be asserted exactly.
func sequentialRefs() func(string) (string, error) {
	counter := 0
	return func(prefix string) (string, error) {
		counter++
		return prefix + strings.Repeat("0", 60) + strconv.Itoa(1000+counter), nil
	}
}

func autopusLandmarks() []declaredLandmark {
	return []declaredLandmark{
		{Role: desktopobserve.RoleApplication, Name: "Autopus Desktop", RequiredState: desktopobserve.StateEnabled},
		{Role: desktopobserve.RoleWindow, Name: "Autopus Desktop", RequiredState: desktopobserve.StateFocused},
	}
}

// AC-QAMESH13-007: the load-bearing privacy property. Finder's tree carries the
// user's folder names and raw AppKit action metadata. None of it may be
// published, while the observation must still be counted.
func TestBuildDesktopProjection_NeverPublishesUndeclaredContent(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "finder.txt"))
	require.NoError(t, err)

	landmarks := []declaredLandmark{
		{Role: desktopobserve.RoleApplication, Name: "Finder", RequiredState: desktopobserve.StateEnabled},
		{Role: desktopobserve.RoleWindow, Name: "응용 프로그램", RequiredState: desktopobserve.StateFocused},
	}
	projection, counts, err := buildDesktopProjection(
		tree, landmarks, "finder-app", "finder-window", "provider-orca", sequentialRefs(),
	)
	require.NoError(t, err)

	assert.Equal(t, 152, counts.ObservedNodes)
	assert.Equal(t, 2, counts.PublishedNodes, "only the two declared landmarks are published")
	assert.Empty(t, counts.UnmatchedLandmarks)

	// Every string in the projection must be either a declared name or a ref.
	rendered := renderProjectionStrings(projection)
	for _, leaked := range []string{
		"즐겨찾기", "데스크탑", "최근 항목", "다운로드", // user-visible sidebar entries
		"target:0x0", "selector:(null)", // raw AppKit action metadata
		"윤곽체", "스크롤 영역", "셀", // localized role phrases
	} {
		assert.NotContains(t, rendered, leaked,
			"observed but undeclared content must never reach the projection")
	}
	assert.Contains(t, rendered, "Finder")
	assert.Contains(t, rendered, "응용 프로그램")
}

// AC-QAMESH13-002 continued: our own app projects the two declared landmarks.
func TestBuildDesktopProjection_ProjectsDeclaredLandmarks(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "autopus.txt"))
	require.NoError(t, err)

	projection, counts, err := buildDesktopProjection(
		tree, autopusLandmarks(), "autopus-desktop", "main-window", "provider-orca", sequentialRefs(),
	)
	require.NoError(t, err)
	assert.Empty(t, counts.UnmatchedLandmarks)

	assert.Equal(t, desktopobserve.SemanticProjectionSchemaVersion, projection.SchemaVersion)
	assert.Equal(t, "autopus-desktop", projection.AppRef)
	assert.Equal(t, "main-window", projection.WindowRef)
	assert.Equal(t, desktopobserve.RoleApplication, projection.Root.Role)
	assert.Equal(t, "Autopus Desktop", projection.Root.Name)
	require.NotNil(t, projection.Root.SemanticState.Enabled)
	assert.True(t, *projection.Root.SemanticState.Enabled)

	require.Len(t, projection.Root.Children, 1)
	window := projection.Root.Children[0]
	assert.Equal(t, desktopobserve.RoleWindow, window.Role)
	assert.Equal(t, "Autopus Desktop", window.Name)
	require.NotNil(t, window.SemanticState.Focused)
	assert.True(t, *window.SemanticState.Focused,
		"the tree reports a focused element inside this window")
}

// AC-QAMESH13-008: a declared landmark that is absent must be reported as
// absent, by name, and must not be silently dropped or reported as a pass.
func TestBuildDesktopProjection_ReportsUnmatchedDeclaredLandmark(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "autopus.txt"))
	require.NoError(t, err)

	landmarks := autopusLandmarks()
	landmarks[1].Name = "Autopus" // the real window title is "Autopus Desktop"

	_, counts, err := buildDesktopProjection(
		tree, landmarks, "autopus-desktop", "main-window", "provider-orca", sequentialRefs(),
	)
	require.NoError(t, err)
	require.Len(t, counts.UnmatchedLandmarks, 1)
	assert.Equal(t, desktopobserve.RoleWindow, counts.UnmatchedLandmarks[0].Role)
	assert.Equal(t, "Autopus", counts.UnmatchedLandmarks[0].Name)
}

// A prefix must not satisfy a declaration. The header carries the label
// verbatim, so equality is the honest rule there.
func TestBuildDesktopProjection_HeaderMatchIsExact(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "autopus.txt"))
	require.NoError(t, err)

	landmarks := autopusLandmarks()
	landmarks[0].Name = "Desktop"
	_, counts, err := buildDesktopProjection(
		tree, landmarks, "a", "w", "provider-orca", sequentialRefs(),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, counts.UnmatchedLandmarks,
		"a suffix of the app name must not satisfy the declaration")
}

// A deeper declared landmark is published with the state the pack asked about,
// and its state comes from the renderer's inlined marker.
func TestBuildDesktopProjection_PublishesDeclaredDeeperLandmark(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "finder.txt"))
	require.NoError(t, err)

	landmarks := []declaredLandmark{
		{Role: desktopobserve.RoleApplication, Name: "Finder", RequiredState: desktopobserve.StateEnabled},
		{Role: desktopobserve.RoleWindow, Name: "응용 프로그램", RequiredState: desktopobserve.StateFocused},
		{Role: desktopobserve.Role("AXCell"), Name: "즐겨찾기", RequiredState: desktopobserve.StateSelected},
	}
	projection, counts, err := buildDesktopProjection(
		tree, landmarks, "finder-app", "finder-window", "provider-orca", sequentialRefs(),
	)
	require.NoError(t, err)
	assert.Empty(t, counts.UnmatchedLandmarks)
	assert.Equal(t, 3, counts.PublishedNodes)

	require.Len(t, projection.Root.Children, 1)
	children := projection.Root.Children[0].Children
	require.Len(t, children, 1)
	assert.Equal(t, "즐겨찾기", children[0].Name)
	require.NotNil(t, children[0].SemanticState.Selected)
	assert.False(t, *children[0].SemanticState.Selected,
		"that row carries no (selected) marker in the captured tree")
}

// A pack without the two canonical landmarks cannot produce a projection. The
// journey layer already enforces this; the builder must not paper over a caller
// that bypassed it.
func TestBuildDesktopProjection_RequiresCanonicalLandmarks(t *testing.T) {
	t.Parallel()

	tree, err := parseDesktopTree(readDesktopTreeFixture(t, "autopus.txt"))
	require.NoError(t, err)

	_, _, err = buildDesktopProjection(
		tree,
		[]declaredLandmark{{Role: desktopobserve.RoleApplication, Name: "Autopus Desktop"}},
		"a", "w", "provider-orca", sequentialRefs(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one application and one window")
}

// renderProjectionStrings flattens every string a projection would serialize, so
// a leak test cannot be fooled by nesting.
func renderProjectionStrings(projection desktopobserve.SemanticProjection) string {
	var builder strings.Builder
	builder.WriteString(projection.ProviderRef + "\n" + projection.AppRef + "\n")
	builder.WriteString(projection.WindowRef + "\n" + projection.StateRef + "\n")
	var walk func(node desktopobserve.SemanticNode)
	walk = func(node desktopobserve.SemanticNode) {
		builder.WriteString(node.NodeRef + "\n" + string(node.Role) + "\n" + node.Name + "\n")
		for _, action := range node.AdvertisedActions {
			builder.WriteString(string(action) + "\n")
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(projection.Root)
	return builder.String()
}
