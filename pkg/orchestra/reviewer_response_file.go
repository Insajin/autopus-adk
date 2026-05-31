package orchestra

import "strings"

func requiresReviewerResponseFile(req ProviderRequest, pi paneInfo) bool {
	return pi.responseFile != "" && (isReviewerRole(req.Role) || isReviewerRole(pi.role))
}

func requiresResponseFileCompletion(pi paneInfo) bool {
	return pi.responseFile != "" && isReviewerRole(pi.role)
}

func isReviewerRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "reviewer")
}

func reviewerResponseFileMissingError(timedOut bool) string {
	if timedOut {
		return "reviewer pane timed out before writing response file"
	}
	return "reviewer pane completed without writing response file"
}
