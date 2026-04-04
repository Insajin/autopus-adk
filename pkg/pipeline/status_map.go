package pipeline

// phaseSequence defines the canonical ordering of pipeline phases.
var phaseSequence = []string{"phase1", "phase1.5", "phase2", "phase3", "phase4"}

// MapCheckpointToPhases converts a Checkpoint to DashboardData for rendering.
//
// Phase mapping rules:
//   - Phases before the current phase are marked "done".
//   - The current phase is marked "running", unless any task in that phase
//     has CheckpointStatusFailed, in which case it is marked "failed".
//   - Phases after the current phase are marked "pending".
//   - If the checkpoint phase is not recognized, all phases are "pending".
func MapCheckpointToPhases(cp *Checkpoint) DashboardData {
	currentIdx := -1
	for i, p := range phaseSequence {
		if p == cp.Phase {
			currentIdx = i
			break
		}
	}

	result := DashboardData{
		Phases: make(map[string]PhaseStatus, len(phaseSequence)),
		Agents: make(map[string]string),
	}

	for i, p := range phaseSequence {
		result.Phases[p] = resolvePhaseStatus(i, currentIdx, cp.TaskStatus)
	}

	return result
}

// resolvePhaseStatus returns the PhaseStatus for a single phase given its
// index relative to the current phase index and the task status map.
func resolvePhaseStatus(idx, currentIdx int, taskStatus map[string]CheckpointStatus) PhaseStatus {
	switch {
	case currentIdx < 0:
		// Unrecognized phase — default all to pending.
		return PhasePending
	case idx < currentIdx:
		return PhaseDone
	case idx == currentIdx:
		if hasFailedTask(taskStatus) {
			return PhaseFailed
		}
		return PhaseRunning
	default:
		return PhasePending
	}
}

// hasFailedTask reports whether any task in the map has CheckpointStatusFailed.
func hasFailedTask(taskStatus map[string]CheckpointStatus) bool {
	for _, s := range taskStatus {
		if s == CheckpointStatusFailed {
			return true
		}
	}
	return false
}
