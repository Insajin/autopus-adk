package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeFindingStatuses_KeepsScopedOutVerdictWhenNoProviderVotes(t *testing.T) {
	t.Parallel()

	carried := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen},
		{ID: "F-002", Status: FindingStatusOutOfScope, RepeatOf: "F-001"},
		{ID: "F-003", Status: FindingStatusDeferred},
	}
	providerResults := [][]ReviewFinding{carried, carried}

	merged := MergeFindingStatuses(providerResults, 0.67)

	require.Len(t, merged, 3)
	assert.Equal(t, FindingStatusOpen, merged[0].Status)
	assert.Equal(t, FindingStatusOutOfScope, merged[1].Status,
		"a repeat scoped out in an earlier revision must not return as open work")
	assert.Equal(t, "F-001", merged[1].RepeatOf)
	assert.Equal(t, FindingStatusDeferred, merged[2].Status)
	assert.False(t, IsActiveBlockingFinding(merged[1]))
}

func TestMergeFindingStatuses_ExplicitOpenVoteReopensScopedOutFinding(t *testing.T) {
	t.Parallel()

	providerResults := [][]ReviewFinding{
		{{ID: "F-002", Status: FindingStatusOutOfScope, RepeatOf: "F-001"}},
		{{ID: "F-002", Status: FindingStatusOpen}},
	}

	merged := MergeFindingStatuses(providerResults, 0.67)

	require.Len(t, merged, 1)
	assert.Equal(t, FindingStatusOpen, merged[0].Status,
		"a reviewer reporting the finding as open overrides the carried-over verdict")
}

func TestMergeFindingStatuses_UnknownStatusStillDefaultsToOpen(t *testing.T) {
	t.Parallel()

	merged := MergeFindingStatuses([][]ReviewFinding{{{ID: "F-001"}}}, 0.67)

	require.Len(t, merged, 1)
	assert.Equal(t, FindingStatusOpen, merged[0].Status)
}
