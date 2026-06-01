package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findJourneyPackRow(t *testing.T, rows []JourneyPackRow, lane string) JourneyPackRow {
	t.Helper()
	for _, row := range rows {
		if row.Lane == lane {
			return row
		}
	}
	require.Failf(t, "missing journey pack row", "lane=%s", lane)
	return JourneyPackRow{}
}

func assertReleaseGap(t *testing.T, rows []SetupGapRow, lane string, class SetupGapClass, blocking bool, severity Severity) {
	t.Helper()
	for _, row := range rows {
		if row.Lane == lane && row.SetupGapClass == class {
			assert.Equal(t, blocking, row.Blocking)
			assert.Equal(t, severity, row.Severity)
			assert.False(t, row.InventedCommand)
			return
		}
	}
	require.Failf(t, "missing setup gap", "lane=%s class=%s", lane, class)
}

func assertNoReleaseGap(t *testing.T, rows []SetupGapRow, lane string) {
	t.Helper()
	for _, row := range rows {
		if row.Lane == lane {
			require.Failf(t, "unexpected setup gap", "lane=%s class=%s", lane, row.SetupGapClass)
		}
	}
}

func siblingSpecIDs(rows []SiblingSpec) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.SpecID)
	}
	return out
}

func roadmapLaneIDs(rows []RoadmapLane) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Lane)
	}
	return out
}

func findRoadmapLane(t *testing.T, rows []RoadmapLane, lane string) RoadmapLane {
	t.Helper()
	for _, row := range rows {
		if row.Lane == lane {
			return row
		}
	}
	require.Failf(t, "missing roadmap lane", "lane=%s", lane)
	return RoadmapLane{}
}

func findLaneRow(t *testing.T, rows []LaneRow, lane string) LaneRow {
	t.Helper()
	for _, row := range rows {
		if row.Lane == lane {
			return row
		}
	}
	require.Failf(t, "missing lane row", "lane=%s", lane)
	return LaneRow{}
}
