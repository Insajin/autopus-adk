package desktopobserve

import (
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeRefs_AreOpaqueDeterministicAndRawReferenceIndependent(t *testing.T) {
	t.Parallel()

	first := recursiveProjectionFixture()
	second := recursiveProjectionFixture()
	setRawNodeRefs(&first.Root, []string{"AXUIElement:0x123 index=7", "orca-4", "handle:/tmp/a", "node-provider-99"})
	setRawNodeRefs(&second.Root, []string{"AXUIElement:0x999 index=1", "orca-88", "handle:/tmp/b", "node-provider-2"})

	normalizedFirst, err := NormalizeProjection(first, identityRedactor)
	require.NoError(t, err)
	normalizedSecond, err := NormalizeProjection(second, identityRedactor)
	require.NoError(t, err)
	firstRefs := collectNodeRefs(normalizedFirst.Root)
	secondRefs := collectNodeRefs(normalizedSecond.Root)
	assert.Equal(t, firstRefs, secondRefs)
	require.Len(t, firstRefs, 4)
	for _, ref := range firstRefs {
		assert.Regexp(t, opaqueRefPattern, ref)
		assert.False(t, unsafeRefPattern.MatchString(ref), ref)
	}
}

func TestNodeRefs_DuplicateOccurrenceAndRecursiveAncestryStayDistinct(t *testing.T) {
	t.Parallel()

	projection := recursiveProjectionFixture()
	duplicate := projection.Root.Children[0].Children[0]
	duplicate.NodeRef = "raw-duplicate"
	projection.Root.Children[0].Children = append(projection.Root.Children[0].Children, duplicate)
	projection.Root.Children[1].Children = []SemanticNode{duplicate}

	normalized, err := NormalizeProjection(projection, identityRedactor)
	require.NoError(t, err)
	refs := collectNodeRefs(normalized.Root)
	unique := map[string]bool{}
	for _, ref := range refs {
		unique[ref] = true
	}
	assert.Len(t, unique, len(refs), "same-key occurrences and different ancestry need unique refs")

	variant := projection
	setRawNodeRefs(&variant.Root, []string{"one", "two", "three", "four", "five", "six"})
	normalizedVariant, err := NormalizeProjection(variant, identityRedactor)
	require.NoError(t, err)
	want, got := collectNodeRefs(normalized.Root), collectNodeRefs(normalizedVariant.Root)
	sort.Strings(want)
	sort.Strings(got)
	assert.Equal(t, want, got)
}

var (
	opaqueRefPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,}$`)
	unsafeRefPattern = regexp.MustCompile(`(?i)(0x[0-9a-f]+|index|handle|socket|/tmp/|provider|orca-[0-9]+)`)
)

func recursiveProjectionFixture() SemanticProjection {
	projection := semanticFixture()
	projection.Root.Children = []SemanticNode{
		{
			Role: RoleGroup, Name: "Primary",
			Children: []SemanticNode{{Role: RoleStaticText, Name: "Ready"}},
		},
		{Role: RoleGroup, Name: "Secondary"},
	}
	return projection
}

func setRawNodeRefs(node *SemanticNode, refs []string) {
	index := 0
	var walk func(*SemanticNode)
	walk = func(current *SemanticNode) {
		if index < len(refs) {
			current.NodeRef = refs[index]
		}
		index++
		for child := range current.Children {
			walk(&current.Children[child])
		}
	}
	walk(node)
}

func collectNodeRefs(root SemanticNode) []string {
	refs := []string{}
	var walk func(SemanticNode)
	walk = func(node SemanticNode) {
		refs = append(refs, node.NodeRef)
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	return refs
}
