package orchestra

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestPromptBuilder_BuildReviewerWithManifest(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.SpecContent = "## Requirements\n- Must expose manifest"
	data.CodeContext = "func BuildPrompt() {}"
	data.SnapshotID = "snapshot-a"
	data.SnapshotContent = "quality recall entry"
	data.SnapshotSourceRefs = []string{"quality:L-001"}

	result, manifest, err := pb.BuildReviewerWithManifest(data)
	require.NoError(t, err)

	assert.Contains(t, result, "Must expose manifest")
	assert.Contains(t, result, "quality recall entry")
	assert.Less(t, strings.Index(result, "quality recall entry"), strings.Index(result, "## Topic"))
	assert.Equal(t, "snapshot-a", manifest.SnapshotID)
	assert.Contains(t, manifestEntryIDs(manifest), "orchestra:reviewer:identity")
	assert.Contains(t, manifestEntryIDs(manifest), "orchestra:project-context")
	assert.Contains(t, manifestEntryIDs(manifest), "snapshot-a")
	assert.Contains(t, manifestEntryIDs(manifest), "orchestra:reviewer:task")
	assert.False(t, manifestEntryByID(manifest, "orchestra:reviewer:task").CacheEligible)
}

func TestPromptBuilder_StableManifestHashesRenderedIdentityLayer(t *testing.T) {
	t.Parallel()

	data := newTestPromptData()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)
	result, manifest, err := pb.BuildReviewerWithManifest(data)
	require.NoError(t, err)

	projectContextIndex := strings.Index(result, "## Project Context")
	require.GreaterOrEqual(t, projectContextIndex, 0)
	expectedContent := strings.TrimSpace(result[:projectContextIndex])
	expected, err := promptlayer.Render([]promptlayer.Layer{{
		ID:            "orchestra:reviewer:identity",
		Kind:          promptlayer.KindStable,
		Group:         promptlayer.GroupIdentityRules,
		SourceRef:     "shared/orchestra-reviewer.md.tmpl",
		Content:       expectedContent,
		CacheEligible: true,
	}})
	require.NoError(t, err)

	assert.Equal(t, expected.Manifest.Entries[0].Hash, manifestEntryByID(manifest, "orchestra:reviewer:identity").Hash)
	assert.NotContains(t, expectedContent, "{{")
	assert.Contains(t, expectedContent, "prompt layer manifest")
}

func TestPromptBuilder_ManifestPromptAndLayerOrderStayAligned(t *testing.T) {
	t.Parallel()

	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.SnapshotID = "snapshot-a"
	data.SnapshotContent = "frozen recall"
	result, manifest, err := pb.BuildDebaterR1WithManifest(data)
	require.NoError(t, err)

	orderedNeedles := []string{"Role: Independent Analyst", "## Project Context", "frozen recall", "## Topic"}
	last := -1
	for _, needle := range orderedNeedles {
		idx := strings.Index(result, needle)
		require.GreaterOrEqual(t, idx, 0, needle)
		assert.Greater(t, idx, last, needle)
		last = idx
	}
	assert.Equal(t, []string{
		"orchestra:debater_r1:identity",
		"orchestra:project-context",
		"snapshot-a",
		"orchestra:debater_r1:task",
	}, manifestEntryIDs(manifest))
}

func TestPromptBuilder_SanitizesUnsafeSnapshotManifestRefs(t *testing.T) {
	t.Parallel()

	manifest, err := buildPromptManifest("reviewer", "shared/orchestra-reviewer.md.tmpl", PromptData{
		SnapshotID:         "/Users/example/.ssh/id_rsa",
		SnapshotContent:    "digest-only content",
		SnapshotSourceRefs: []string{"/Users/example/.ssh/id_rsa", "quality:L-001", "token=sk-proj-abcdefghijklmnopqrstuvwxyz"},
	})
	require.NoError(t, err)

	assert.NotContains(t, manifest.SnapshotID, "/Users")
	entry := manifestEntryByID(manifest, manifest.SnapshotID)
	assert.NotContains(t, entry.SourceRef, "/Users")
	assert.NotContains(t, entry.SourceRef, "sk-proj")
	assert.Contains(t, entry.SourceRef, "quality:L-001")
	assert.Contains(t, entry.SourceRef, "snapshot-ref:")
}
