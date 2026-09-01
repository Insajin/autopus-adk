package readiness_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/readiness"
)

// D11 regression: auto qa release writes one run index per lane, so a projection
// built from the single newest index reports one lane's checks as the whole
// release and contradicts the lane_statuses in the same payload.
func TestProject_CountsChecksAcrossEveryRunIndexOfTheRelease(t *testing.T) {
	t.Parallel()

	root := copyFixture(t)
	secondRun := filepath.Join(root, "qa", "run-index-canary.json")
	cloneRunIndex(t, filepath.Join(root, "qa", "run-index.json"), secondRun, func(doc map[string]any) {
		doc["run_id"] = "qa-canary"
		doc["lane"] = "canary-explicit"
		doc["checks"] = []any{
			map[string]any{"id": "canary-1", "status": "failed"},
			map[string]any{"id": "canary-2", "status": "passed"},
		}
	})
	patchJSON(t, filepath.Join(root, "qa", "release-index.json"), func(doc map[string]any) {
		rows, _ := doc["lane_rows"].([]any)
		last, _ := rows[len(rows)-1].(map[string]any)
		last["run_index_path"] = "qa/run-index-canary.json"
		// Drop the release's own summary so the derived one, which is what
		// contradicted lane_statuses, is the string under test.
		delete(doc, "trend_summary")
	})

	result, err := readiness.Project(context.Background(), portableInput(t, root))
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	counts := result.Projection.CheckCounts
	// Primary index: passed/failed/skipped. Second index: failed + passed.
	if counts.Total != 5 || counts.Passed != 2 || counts.Failed != 2 || counts.Skipped != 1 {
		t.Fatalf("check counts = %#v, want total=5 passed=2 failed=2 skipped=1", counts)
	}
	if got := result.Projection.TrendSummary; got != "2 failed deterministic check across 6 lanes" {
		t.Fatalf("trend summary = %q, want it to agree with the aggregated counts", got)
	}
}

// A sibling run index the release names is evidence like any other: if it would
// be refused on its own, aggregation must not launder it into the projection.
func TestProject_FailsClosedOnUnsafeSiblingRunIndex(t *testing.T) {
	t.Parallel()

	root := copyFixture(t)
	secondRun := filepath.Join(root, "qa", "run-index-canary.json")
	cloneRunIndex(t, filepath.Join(root, "qa", "run-index.json"), secondRun, func(doc map[string]any) {
		doc["run_id"] = "qa-canary"
		doc["source_refs"] = []any{"qamesh://source/portable-shop", "sk-prod-1234567890abcdef1234567890abcdef"}
	})
	patchJSON(t, filepath.Join(root, "qa", "release-index.json"), func(doc map[string]any) {
		rows, _ := doc["lane_rows"].([]any)
		last, _ := rows[len(rows)-1].(map[string]any)
		last["run_index_path"] = "qa/run-index-canary.json"
	})

	result, err := readiness.Project(context.Background(), portableInput(t, root))
	if err == nil {
		t.Fatal("projection accepted a release whose sibling run index carries a credential")
	}
	assertFailClosed(t, result,
		[]string{"unsafe_secret:credential_or_token"},
		[]string{"sk-prod-1234567890abcdef1234567890abcdef"})
}

// A release that names a run index which is not on disk is broken, not partially
// projectable: silently skipping it would recreate the under-reporting defect.
func TestProject_FailsClosedOnMissingSiblingRunIndex(t *testing.T) {
	t.Parallel()

	root := copyFixture(t)
	patchJSON(t, filepath.Join(root, "qa", "release-index.json"), func(doc map[string]any) {
		rows, _ := doc["lane_rows"].([]any)
		last, _ := rows[len(rows)-1].(map[string]any)
		last["run_index_path"] = "qa/run-index-gone.json"
	})

	result, err := readiness.Project(context.Background(), portableInput(t, root))
	if err == nil {
		t.Fatal("projection accepted a release naming a run index that does not exist")
	}
	assertFailClosed(t, result, []string{"invalid_ref:run_index_unreadable"}, nil)
}

// A run-index ref pointing outside the workspace cannot be resolved as portable
// evidence. The blocker names the ref kind and the actual condition rather than
// asserting a user directory the path need not contain.
func TestProject_FailsClosedOnRunIndexRefOutsideWorkspace(t *testing.T) {
	t.Parallel()

	root := copyFixture(t)
	patchJSON(t, filepath.Join(root, "qa", "release-index.json"), func(doc map[string]any) {
		rows, _ := doc["lane_rows"].([]any)
		last, _ := rows[len(rows)-1].(map[string]any)
		last["run_index_path"] = "/var/tmp/elsewhere/run-index.json"
	})

	result, err := readiness.Project(context.Background(), portableInput(t, root))
	if err == nil {
		t.Fatal("projection accepted a run-index ref outside the workspace")
	}
	assertFailClosed(t, result, []string{"unsafe_ref:run_index_path_outside_workspace"}, nil)
}

// D11, second form: a release index that records a passed or failed lane but
// names no run index for it. Skipping such rows is exactly what let the
// projection report one lane's checks as the whole release while its own
// lane_statuses listed several, so the incomplete index fails closed.
func TestProject_FailsClosedWhenAnExecutedLaneNamesNoRunIndex(t *testing.T) {
	t.Parallel()

	root := copyFixture(t)
	patchJSON(t, filepath.Join(root, "qa", "release-index.json"), func(doc map[string]any) {
		for _, row := range doc["lane_rows"].([]any) {
			entry := row.(map[string]any)
			if entry["status"] == "passed" {
				entry["run_index_path"] = ""
			}
		}
	})

	result, err := readiness.Project(context.Background(), portableInput(t, root))
	if err == nil {
		t.Fatal("projection accepted a release whose passed lane names no run index")
	}
	assertFailClosed(t, result, []string{"invalid_ref:lane_run_index_missing"}, nil)
}

// Lanes that never ran legitimately carry no run-index ref and must not trip
// the completeness check.
func TestProject_AllowsNonExecutedLanesWithoutARunIndex(t *testing.T) {
	t.Parallel()

	root := copyFixture(t)
	if _, err := readiness.Project(context.Background(), portableInput(t, root)); err != nil {
		t.Fatalf("deferred, skipped, blocked and setup_gap lanes must not require a run index: %v", err)
	}
}

func cloneRunIndex(t *testing.T, src, dst string, mutate func(map[string]any)) {
	t.Helper()
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
	patchJSON(t, dst, mutate)
}
