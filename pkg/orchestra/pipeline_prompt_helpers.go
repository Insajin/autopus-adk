package orchestra

import "fmt"

func providerResultsToResponses(results []ProviderResult) []ProviderResponse {
	responses := make([]ProviderResponse, 0, len(results))
	for _, result := range results {
		response := result.Response
		if response.Provider == "" {
			response.Provider = result.Provider
			response.Output = result.Output
			response.Usage = result.Usage
			response.UsageCapability = result.UsageCapability
		}
		responses = append(responses, response)
	}
	return responses
}

// buildPromptBuilder creates a PromptBuilder, panicking on error (templates are embedded).
func buildPromptBuilder() *PromptBuilder {
	promptBuilder, err := NewPromptBuilder()
	if err != nil {
		panic(fmt.Sprintf("pipeline: failed to create prompt builder: %v", err))
	}
	return promptBuilder
}

func withPromptSchema(
	base PromptData,
	schema *SchemaBuilder,
	role string,
	providers []ProviderConfig,
) (PromptData, error) {
	data := base
	if providersSupportCLISchema(providers) {
		data.SchemaMethod = ""
		data.SchemaJSON = ""
		return data, nil
	}

	embedded, err := schema.EmbedInPrompt(role)
	if err != nil {
		return PromptData{}, fmt.Errorf("pipeline: embed %s schema in prompt: %w", role, err)
	}
	data.SchemaMethod = "prompt"
	data.SchemaJSON = embedded
	return data, nil
}
