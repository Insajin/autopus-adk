package workflow

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

// TaskOwnership is one planner task's file ownership. It is used to ENFORCE that
// an executor's merged changes stay within its assigned files, turning the
// planner's disjoint-file decomposition into a hard merge-time guarantee.
type TaskOwnership struct {
	ID    string   `json:"id"`
	Files []string `json:"files"`
}

// ParsePlanOwnership extracts task file-ownership from a persisted planner result.
// It accepts either {"tasks":[...]} or {"plan":{"tasks":[...]}} — the dispatcher
// persists the segment-A return value, which wraps the plan under "plan".
func ParsePlanOwnership(data []byte) ([]TaskOwnership, error) {
	var direct struct {
		Tasks []TaskOwnership `json:"tasks"`
	}
	if err := json.Unmarshal(data, &direct); err == nil && len(direct.Tasks) > 0 {
		return direct.Tasks, nil
	}
	var wrapped struct {
		Plan struct {
			Tasks []TaskOwnership `json:"tasks"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Plan.Tasks, nil
}

// ownsFile reports whether relpath (repo-relative, from git status) is owned by
// one of taskFiles. Planner file paths are LLM-generated and may carry an
// absolute or extra prefix, so the comparison is a path-component-aligned suffix
// match against the authoritative repo-relative path.
func ownsFile(taskFiles []string, relpath string) bool {
	r := filepath.ToSlash(filepath.Clean(relpath))
	for _, f := range taskFiles {
		tf := filepath.ToSlash(filepath.Clean(f))
		if tf == r || strings.HasSuffix(tf, "/"+r) {
			return true
		}
	}
	return false
}

// assignWorktreesToTasks assigns each worktree (index into changedSets) 1:1 to
// the task it performed, returning a slice of task indices (-1 = unassigned).
//
// It greedily takes the highest file-overlap (worktree, task) pairs first,
// claiming both sides so no two worktrees share a task and no worktree gets two
// tasks. This is what makes ownership a HARD guarantee even when an executor
// strays into another task's files: a strayed worktree has equal raw overlap
// with two tasks, but the rightful owner of each task (a worktree that changed
// only/mostly that task's files) is assigned first and consumes it, forcing the
// strayed worktree onto the task it actually owns. Ties resolve by lowest
// worktree index then lowest task index for determinism.
func assignWorktreesToTasks(changedSets [][]string, tasks []TaskOwnership) []int {
	assign := make([]int, len(changedSets))
	for i := range assign {
		assign[i] = -1
	}
	taskTaken := make([]bool, len(tasks))

	type pair struct{ wt, task, overlap int }
	var pairs []pair
	for i, changed := range changedSets {
		for j, t := range tasks {
			o := 0
			for _, f := range changed {
				if ownsFile(t.Files, f) {
					o++
				}
			}
			if o > 0 {
				pairs = append(pairs, pair{i, j, o})
			}
		}
	}
	sort.Slice(pairs, func(a, b int) bool {
		if pairs[a].overlap != pairs[b].overlap {
			return pairs[a].overlap > pairs[b].overlap
		}
		if pairs[a].wt != pairs[b].wt {
			return pairs[a].wt < pairs[b].wt
		}
		return pairs[a].task < pairs[b].task
	})
	for _, p := range pairs {
		if assign[p.wt] == -1 && !taskTaken[p.task] {
			assign[p.wt] = p.task
			taskTaken[p.task] = true
		}
	}
	return assign
}
