package workflow_test

import (
	"context"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// S10: the Go runtime owns worktree scheduling. Eight implementation tasks with
// the default cap schedule at most five concurrently; the rest queue.
func TestWorktreeSlotCap_BoundsConcurrency(t *testing.T) {
	tasks := []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8"}

	// cap=0 selects the default cap (5).
	sched := pipeline.ScheduleWorktreeTasksWithCap(tasks, 0)

	if len(sched.ActiveTaskIDs) != 5 {
		t.Fatalf("active = %d, want 5", len(sched.ActiveTaskIDs))
	}
	if len(sched.QueuedTaskIDs) != 3 {
		t.Fatalf("queued = %d, want 3", len(sched.QueuedTaskIDs))
	}
	if sched.Cap != 5 {
		t.Fatalf("cap = %d, want 5", sched.Cap)
	}
	if pipeline.DefaultWorktreeSlotCap != 5 {
		t.Fatalf("DefaultWorktreeSlotCap = %d, want 5", pipeline.DefaultWorktreeSlotCap)
	}
}

// S10: branch naming and worktree reclaim live in the Go WorktreeManager, which
// enforces the max-5 limit — the workflow JS owns sequencing only.
func TestWorktreeManager_EnforcesLimitInGoRuntime(t *testing.T) {
	m := pipeline.NewWorktreeManager()
	ctx := context.Background()

	created := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		path, err := m.Create(ctx, "feat-"+string(rune('a'+i)))
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		created = append(created, path)
	}
	if m.ActiveCount() != 5 {
		t.Fatalf("active count = %d, want 5", m.ActiveCount())
	}

	// The sixth create is rejected by the Go runtime, not by JS.
	if _, err := m.Create(ctx, "feat-overflow"); err == nil {
		t.Fatal("6th worktree create must error at the limit")
	} else if !strings.Contains(err.Error(), "limit") {
		t.Fatalf("error %q must mention the limit", err.Error())
	}

	for _, p := range created {
		_ = m.Remove(ctx, p)
	}
}
