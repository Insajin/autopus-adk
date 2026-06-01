package journey

import (
	"fmt"
	"net/url"
	"strings"
)

func validateGUIPolicy(pack Pack) error {
	if pack.Adapter.ID != "gui-explore" {
		return nil
	}
	policy := pack.GUI
	if len(policy.AllowedOrigins) == 0 {
		return validationError("qa_journey_gui_policy_invalid", "gui.allowed_origins is required for gui-explore")
	}
	for _, origin := range policy.AllowedOrigins {
		if err := validateOrigin(origin); err != nil {
			return validationError("qa_journey_gui_policy_invalid", err.Error())
		}
	}
	if len(policy.ForbiddenActions) == 0 {
		return validationError("qa_journey_gui_policy_invalid", "gui.forbidden_actions is required for gui-explore")
	}
	for _, action := range policy.ForbiddenActions {
		if strings.TrimSpace(action) == "" {
			return validationError("qa_journey_gui_policy_invalid", "gui.forbidden_actions may not contain empty values")
		}
	}
	switch strings.ToLower(strings.TrimSpace(policy.SelectorStrategy)) {
	case "role-first", "accessibility-first":
	default:
		return validationError("qa_journey_gui_policy_invalid", "gui.selector_strategy must be role-first or accessibility-first")
	}
	switch strings.ToLower(strings.TrimSpace(policy.NetworkPolicy.Mode)) {
	case "summary-only", "blocked", "local-only":
	default:
		return validationError("qa_journey_gui_policy_invalid", "gui.network_policy.mode must be summary-only, blocked, or local-only")
	}
	if policy.NetworkPolicy.RetainHeaders || policy.NetworkPolicy.RetainBodies {
		return validationError("qa_journey_gui_policy_invalid", "gui network policy may not retain raw headers or bodies")
	}
	if policy.ArtifactRetention.PublishRaw {
		return validationError("qa_journey_gui_policy_invalid", "gui artifacts may not publish raw screenshots, traces, or videos")
	}
	if err := validateGUIScreenMatrix(policy.ScreenMatrix); err != nil {
		return validationError("qa_journey_gui_policy_invalid", err.Error())
	}
	return nil
}

func validateGUIScreenMatrix(rows []GUIScreenMatrixRow) error {
	seen := map[string]bool{}
	for _, row := range rows {
		id := strings.TrimSpace(row.ID)
		path := strings.TrimSpace(row.Path)
		if id == "" && path == "" {
			return fmt.Errorf("gui.screen_matrix rows require id or path")
		}
		key := id
		if key == "" {
			key = path
		}
		if seen[key] {
			return fmt.Errorf("gui.screen_matrix contains duplicate screen %q", key)
		}
		seen[key] = true
		if path != "" && invalidGUIRoutePath(path) {
			return fmt.Errorf("gui.screen_matrix.path must be a project-local route path")
		}
		for _, action := range row.RequiredActions {
			if strings.TrimSpace(action) == "" || strings.ContainsAny(action, "\r\n") {
				return fmt.Errorf("gui.screen_matrix.required_actions may not contain empty or multiline values")
			}
		}
	}
	return nil
}

func invalidGUIRoutePath(path string) bool {
	return strings.Contains(path, "://") ||
		strings.HasPrefix(path, "//") ||
		strings.ContainsAny(path, "\r\n\t ")
}

func validateOrigin(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("gui.allowed_origins must contain absolute http(s) origins")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("gui.allowed_origins only supports http and https")
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("gui.allowed_origins must be origins without path, query, or fragment")
	}
	return nil
}
