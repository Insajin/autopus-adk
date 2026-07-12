package orchestra

import "strings"

func requiresReviewerResponseFile(req ProviderRequest, pi paneInfo) bool {
	provider := pi.provider
	if provider.Name == "" && provider.Binary == "" {
		provider = req.Config
	}
	return pi.responseFile != "" && reviewerRequiresResponseFile(provider) && (isReviewerRole(req.Role) || isReviewerRole(pi.role))
}

func requiresResponseFileCompletion(pi paneInfo) bool {
	if pi.responseFile == "" || !reviewerRequiresResponseFile(pi.provider) {
		return false
	}
	return isReviewerRole(pi.role) || isCodexInteractiveProvider(pi.provider)
}

func isReviewerRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "reviewer")
}

func reviewerRequiresResponseFile(provider ProviderConfig) bool {
	return !usesAntigravityPromptInteractive(provider)
}

func reviewerResponseFileMissingError(timedOut bool) string {
	if timedOut {
		return "reviewer pane timed out before writing response file"
	}
	return "reviewer pane completed without writing response file"
}
