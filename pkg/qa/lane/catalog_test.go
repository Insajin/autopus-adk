package lane_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/adapter"
	"github.com/insajin/autopus-adk/pkg/qa/lane"
)

func TestReleaseLanesAreStableAndUnique(t *testing.T) {
	t.Parallel()

	lanes := lane.Release()
	require.NotEmpty(t, lanes)
	assert.Equal(t, "fast", lanes[0], "gate order is part of the JSON contract")

	seen := map[string]bool{}
	for _, id := range lanes {
		require.False(t, seen[id], id)
		seen[id] = true
		assert.True(t, lane.IsRelease(id))
	}
	assert.False(t, lane.IsRelease("totally-bogus-lane"))
}

// The list of unimplemented lanes must stay honest in both directions: it may
// only name release lanes, and it may not name a lane an adapter can actually
// run. If an adapter starts declaring one, wire the lane up instead of leaving
// it flagged as having no executor.
func TestLanesWithoutExecutorAreReleaseLanesNoAdapterDeclares(t *testing.T) {
	t.Parallel()

	declared := map[string]string{}
	for _, metadata := range adapter.Registry() {
		for _, id := range metadata.DefaultLanes {
			declared[id] = metadata.ID
		}
	}
	for _, id := range lane.WithoutExecutor() {
		assert.True(t, lane.IsRelease(id), "%s is not a release lane", id)
		assert.False(t, lane.HasExecutor(id))
		assert.Empty(t, declared[id], "adapter %s declares %s: wire the lane up", declared[id], id)
	}
	for _, id := range lane.Release() {
		if declared[id] != "" {
			assert.True(t, lane.HasExecutor(id), "%s is declared by adapter %s", id, declared[id])
		}
	}
}
