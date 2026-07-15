package worker

import "strings"

// SetRequiredContext configures the frozen snapshot that is reattached after
// every phase transition and is never passed to the compressor.
func (pe *PipelineExecutor) SetRequiredContext(context string) {
	pe.requiredContext = strings.TrimSpace(context)
}

// SetPersistentTask configures the original task contract that must survive
// planner output and every later compressed phase handoff.
func (pe *PipelineExecutor) SetPersistentTask(task string) {
	pe.persistentTask = strings.TrimSpace(task)
}

func (pe *PipelineExecutor) attachRequiredContext(prompt string) string {
	if pe.requiredContext == "" {
		return prompt
	}
	return pe.requiredContext + "\n\n# Ephemeral Phase Context\n\n" + prompt
}

func (pe *PipelineExecutor) attachPersistentTask(prompt string) string {
	if pe.persistentTask == "" {
		return prompt
	}
	return pe.persistentTask + "\n\n# Current Phase Context\n\n" + prompt
}
