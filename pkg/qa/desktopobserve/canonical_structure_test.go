package desktopobserve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalDigest_PreservesMultiplicity(t *testing.T) {
	t.Parallel()

	one := semanticFixture()
	two := semanticFixture()
	duplicate := two.Root.Children[0]
	duplicate.NodeRef = "provider-duplicate"
	two.Root.Children = append(two.Root.Children, duplicate)

	normalizedOne, err := NormalizeProjection(one, identityRedactor)
	require.NoError(t, err)
	normalizedTwo, err := NormalizeProjection(two, identityRedactor)
	require.NoError(t, err)
	assert.NotEqual(t, normalizedOne.Digest, normalizedTwo.Digest)

	two.Root.Children = two.Root.Children[:1]
	recovered, err := NormalizeProjection(two, identityRedactor)
	require.NoError(t, err)
	assert.Equal(t, normalizedOne.Digest, recovered.Digest)
}

func TestCanonicalDigest_PreservesParentChildAncestry(t *testing.T) {
	t.Parallel()

	underPrimary := ancestryProjection(true)
	underSecondary := ancestryProjection(false)
	primaryDigest, err := NormalizeProjection(underPrimary, identityRedactor)
	require.NoError(t, err)
	secondaryDigest, err := NormalizeProjection(underSecondary, identityRedactor)
	require.NoError(t, err)

	assert.Equal(t, semanticInventory(underPrimary.Root), semanticInventory(underSecondary.Root))
	assert.NotEqual(t, primaryDigest.Digest, secondaryDigest.Digest)
}

func ancestryProjection(primary bool) SemanticProjection {
	projection := semanticFixture()
	leaf := SemanticNode{Role: RoleStaticText, Name: "Ready"}
	left := SemanticNode{Role: RoleGroup, Name: "Primary"}
	right := SemanticNode{Role: RoleGroup, Name: "Secondary"}
	if primary {
		left.Children = []SemanticNode{leaf}
	} else {
		right.Children = []SemanticNode{leaf}
	}
	projection.Root.Children = []SemanticNode{left, right}
	return projection
}

func semanticInventory(root SemanticNode) map[string]int {
	inventory := map[string]int{}
	var walk func(SemanticNode)
	walk = func(node SemanticNode) {
		inventory[string(node.Role)+"\x00"+node.Name]++
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	return inventory
}
