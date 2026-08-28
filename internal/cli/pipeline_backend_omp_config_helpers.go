package cli

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func clonePipelineOMPPhaseModels(input map[pipeline.PhaseID]string) map[pipeline.PhaseID]string {
	output := make(map[pipeline.PhaseID]string, len(input))
	for phase, model := range input {
		output[phase] = model
	}
	return output
}

func pipelineOMPCanonicalEnvironment(input []string) []string {
	result := make([]string, 0, len(input))
	for _, entry := range input {
		key, _, _ := strings.Cut(entry, "=")
		if key == pipelineOMPActiveEndpointKey || key == pipelineOMPActiveCredentialKey {
			continue
		}
		result = append(result, entry)
	}
	return result
}
