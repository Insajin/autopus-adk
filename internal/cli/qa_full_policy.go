package cli

import (
	"strings"

	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
)

type qaFullPolicy struct {
	Orchestrator          string   `json:"orchestrator"`
	RunnerSelection       string   `json:"runner_selection"`
	RunnerAdapters        []string `json:"runner_adapters"`
	UserChoiceRequiredFor []string `json:"user_choice_required_for"`
	PlaywrightRole        string   `json:"playwright_role,omitempty"`
}

func qaFullPolicyForPlan(plan qarelease.Plan) qaFullPolicy {
	adapters := []string{}
	lanes := []string{}
	for _, pack := range plan.JourneyPacks {
		adapters = append(adapters, pack.Adapter)
		lanes = append(lanes, pack.Lane)
	}
	lanes = append(lanes, plan.BlockerRules.MustLanes...)
	lanes = append(lanes, plan.BlockerRules.OptionalLanes...)
	return qaFullPolicyForAdapters(adapters, lanes)
}

func qaFullPolicyForRun(index qarelease.Index) qaFullPolicy {
	lanes := []string{}
	for _, row := range index.LaneRows {
		if row.LanePolicy == qarelease.LanePolicyDeferred {
			continue
		}
		lanes = append(lanes, row.Lane)
	}
	return qaFullPolicyForAdapters(nil, lanes)
}

func qaFullPolicyForCandidates(candidates []qaFullProjectCandidate) qaFullPolicy {
	adapters := []string{}
	lanes := []string{}
	for _, candidate := range candidates {
		reason := strings.ToLower(candidate.Reason)
		if strings.Contains(reason, "playwright") {
			adapters = append(adapters, "playwright")
			lanes = append(lanes, "browser-staging")
		}
		if strings.Contains(reason, "desktop gui") {
			adapters = append(adapters, "gui-explore")
			lanes = append(lanes, "gui-explore")
		}
		if strings.Contains(reason, "package") {
			adapters = append(adapters, "node-script")
			lanes = append(lanes, "fast")
		}
		if strings.Contains(reason, "go module") {
			adapters = append(adapters, "go-test")
			lanes = append(lanes, "fast")
		}
	}
	policy := qaFullPolicyForAdapters(adapters, lanes)
	if !stringSliceContains(policy.UserChoiceRequiredFor, "project-dir") {
		policy.UserChoiceRequiredFor = append([]string{"project-dir"}, policy.UserChoiceRequiredFor...)
	}
	return policy
}

func qaFullPolicyForAdapters(adapters, lanes []string) qaFullPolicy {
	adapters = uniqueNonEmptyStrings(adapters)
	choices := []string{"execution"}
	if stringSliceContains(lanes, "browser-staging") || stringSliceContains(lanes, "gui-explore") {
		choices = append(choices, "environment", "credentials")
	}
	if stringSliceContains(lanes, "canary-explicit") {
		choices = append(choices, "canary-command")
	}
	if stringSliceContains(lanes, "mobile-readiness") {
		choices = append(choices, "mobile-device-cloud")
	}
	policy := qaFullPolicy{
		Orchestrator:          "qamesh",
		RunnerSelection:       "auto-detected-from-project-journey-packs",
		RunnerAdapters:        adapters,
		UserChoiceRequiredFor: uniqueNonEmptyStrings(choices),
	}
	if stringSliceContains(adapters, "playwright") || stringSliceContains(adapters, "gui-explore") || stringSliceContains(lanes, "browser-staging") || stringSliceContains(lanes, "gui-explore") {
		policy.PlaywrightRole = "runner adapter for browser and GUI journeys, not a competing QA mode"
	}
	return policy
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
