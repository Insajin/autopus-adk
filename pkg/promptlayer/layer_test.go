package promptlayer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDeterministicLayerOrdering(t *testing.T) {
	t.Parallel()

	layers := []Layer{
		{ID: "user-request", Kind: KindEphemeral, Group: GroupUserRequest, SourceRef: "user", Content: "implement prompt manifests"},
		{ID: "rules", Kind: KindStable, Group: GroupIdentityRules, SourceRef: "AGENTS.md", Content: "follow project rules", CacheEligible: true},
		{ID: "snapshot-a", Kind: KindSnapshot, Group: GroupFrozenSnapshot, SourceRef: "quality-recall", Content: "prior review finding"},
		{ID: "skills", Kind: KindStable, Group: GroupStableSkillIndex, SourceRef: "skills", Content: "auto-go, review", CacheEligible: true},
		{ID: "task-context", Kind: KindEphemeral, Group: GroupTaskContext, SourceRef: "tool", Content: "latest tool output"},
		{ID: "method", Kind: KindStable, Group: GroupMethodologyTools, SourceRef: "tdd", Content: "red green refactor", CacheEligible: true},
	}

	first, err := Render(layers)
	require.NoError(t, err)
	second, err := Render(layers)
	require.NoError(t, err)

	assert.Equal(t, first.Prompt, second.Prompt)
	require.Len(t, first.Manifest.Entries, 6)
	assert.Equal(t, []string{"rules", "method", "skills", "snapshot-a", "task-context", "user-request"}, manifestIDs(first.Manifest))
	assert.Equal(t, first.Manifest.Entries[0].Hash, second.Manifest.Entries[0].Hash)
	assert.True(t, first.Manifest.Entries[0].CacheEligible)
	assert.Greater(t, first.Manifest.Entries[0].TokenEstimate, 0)
}

func TestFrozenSnapshotLayerDoesNotDriftWithoutRebuild(t *testing.T) {
	t.Parallel()

	snapshot := SnapshotLayer("snapshot-a", "quality-recall:L-001", "cached recall result")
	first, err := Render([]Layer{snapshot, {ID: "task", Kind: KindEphemeral, Group: GroupUserRequest, SourceRef: "user", Content: "first task"}})
	require.NoError(t, err)

	second, err := Render([]Layer{snapshot, {ID: "task", Kind: KindEphemeral, Group: GroupUserRequest, SourceRef: "user", Content: "second task"}})
	require.NoError(t, err)

	assert.Equal(t, "snapshot-a", first.Manifest.SnapshotID)
	assert.Equal(t, "snapshot-a", second.Manifest.SnapshotID)
	assert.Equal(t, entryByID(first.Manifest, "snapshot-a").Hash, entryByID(second.Manifest, "snapshot-a").Hash)
	assert.NotContains(t, second.Prompt, "new matching record")
}

func TestCompareManifestsIsolatesEphemeralChanges(t *testing.T) {
	t.Parallel()

	stable := Layer{ID: "rules", Kind: KindStable, Group: GroupIdentityRules, SourceRef: "rules", Content: "stable rules", CacheEligible: true}
	a, err := Render([]Layer{stable, {ID: "task", Kind: KindEphemeral, Group: GroupUserRequest, SourceRef: "user", Content: "task A"}})
	require.NoError(t, err)
	b, err := Render([]Layer{stable, {ID: "task", Kind: KindEphemeral, Group: GroupUserRequest, SourceRef: "user", Content: "task B"}})
	require.NoError(t, err)

	changes := CompareManifests(a.Manifest, b.Manifest)

	require.Len(t, changes, 1)
	assert.Equal(t, "task", changes[0].ID)
	assert.Equal(t, InvalidationEphemeralChanged, changes[0].Reason)
	assert.Equal(t, entryByID(a.Manifest, "rules").Hash, entryByID(b.Manifest, "rules").Hash)
}

func TestCompareManifestsReportsRemovedLayers(t *testing.T) {
	t.Parallel()

	previous, err := Render([]Layer{
		{ID: "rules", Kind: KindStable, Group: GroupIdentityRules, SourceRef: "rules", Content: "stable rules", CacheEligible: true},
		{ID: "snapshot-a", Kind: KindSnapshot, Group: GroupFrozenSnapshot, SourceRef: "quality:L-001", Content: "snapshot"},
	})
	require.NoError(t, err)
	current, err := Render([]Layer{
		{ID: "rules", Kind: KindStable, Group: GroupIdentityRules, SourceRef: "rules", Content: "stable rules", CacheEligible: true},
	})
	require.NoError(t, err)

	changes := CompareManifests(previous.Manifest, current.Manifest)

	require.Len(t, changes, 1)
	assert.Equal(t, "snapshot-a", changes[0].ID)
	assert.Equal(t, InvalidationSnapshotRebuild, changes[0].Reason)
	assert.NotEmpty(t, changes[0].PreviousHash)
	assert.Empty(t, changes[0].CurrentHash)
}

func TestCompareManifestsReportsMetadataChanges(t *testing.T) {
	t.Parallel()

	previous, err := Render([]Layer{
		{ID: "rules", Kind: KindStable, Group: GroupIdentityRules, SourceRef: "rules", Content: "stable rules", CacheEligible: true},
	})
	require.NoError(t, err)
	current, err := Render([]Layer{
		{ID: "rules", Kind: KindStable, Group: GroupIdentityRules, SourceRef: "rules-v2", Content: "stable rules", CacheEligible: true},
	})
	require.NoError(t, err)

	changes := CompareManifests(previous.Manifest, current.Manifest)

	require.Len(t, changes, 1)
	assert.Equal(t, "rules", changes[0].ID)
	assert.Equal(t, InvalidationStableSourceChanged, changes[0].Reason)
	assert.Equal(t, changes[0].PreviousHash, changes[0].CurrentHash)
}

func manifestIDs(m Manifest) []string {
	ids := make([]string, 0, len(m.Entries))
	for _, entry := range m.Entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

func entryByID(m Manifest, id string) ManifestEntry {
	for _, entry := range m.Entries {
		if entry.ID == id {
			return entry
		}
	}
	return ManifestEntry{}
}
