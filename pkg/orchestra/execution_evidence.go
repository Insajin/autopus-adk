package orchestra

func applyProviderRequestEvidence(response *ProviderResponse, request ProviderRequest, backend string) {
	if response == nil {
		return
	}
	if response.Provider == "" {
		response.Provider = request.Provider
	}
	if response.Role == "" {
		response.Role = request.Role
	}
	if response.Attempt == 0 {
		response.Attempt = request.Round
	}
	if response.Attempt == 0 {
		response.Attempt = 1
	}
	if response.ModelFamily == "" {
		response.ModelFamily = request.Config.ModelFamily
	}
	if response.ExecutedBackend == "" {
		response.ExecutedBackend = backend
	}
}
