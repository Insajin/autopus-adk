package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

type guiPolicyEvaluation struct {
	confirmed       bool
	unavailable     []string
	blockedAttempts []string
	outsideRequests []string
	missingScreens  []string
	missingActions  []string
	missingEvidence []string
}

func applyGUIPolicyOracle(projectDir string, pack journey.Pack, result *commandResult) (IndexCheck, bool) {
	if pack.Adapter.ID != "gui-explore" {
		return IndexCheck{}, false
	}
	eval := evaluateGUIPolicyEvidence(projectDir, pack, result.GUIGuardReadyPath)
	check := IndexCheck{
		ID:        guiPolicyRuntimeCheckID,
		JourneyID: pack.ID,
		Adapter:   pack.Adapter.ID,
		Expected:  expectedGUIRuntimePolicy(pack),
		Actual:    actualGUIRuntimePolicy(eval),
	}
	if len(eval.unavailable) > 0 || len(eval.blockedAttempts) > 0 || len(eval.outsideRequests) > 0 || len(eval.missingScreens) > 0 || len(eval.missingActions) > 0 || len(eval.missingEvidence) > 0 {
		check.Status = "blocked"
		check.FailureSummary = failureGUIRuntimePolicy(eval)
		result.Status = "blocked"
		result.FailureSummary = check.FailureSummary
		return check, true
	}
	check.Status = "passed"
	return check, true
}

func evaluateGUIPolicyEvidence(projectDir string, pack journey.Pack, guardReadyPath string) guiPolicyEvaluation {
	eval := guiPolicyEvaluation{}
	if !guiGuardReady(guardReadyPath) {
		eval.missingEvidence = append(eval.missingEvidence, "gui_policy_guard.ready")
	}
	graph, err := readDeclaredJSON(projectDir, pack, "journey_graph")
	if err != nil {
		eval.missingEvidence = append(eval.missingEvidence, err.Error())
		return eval
	}
	eval.confirmed = runtimePolicyConfirmed(graph)
	eval.unavailable = append(eval.unavailable, guiAvailabilityFailures(graph)...)
	eval.blockedAttempts = append(eval.blockedAttempts, stoppedPolicyAttempts(graph, pack)...)
	eval.missingScreens, eval.missingActions = evaluateGUIScreenMatrix(graph, pack.GUI.ScreenMatrix)
	network, err := readDeclaredJSON(projectDir, pack, "network_summary")
	if err != nil {
		eval.missingEvidence = append(eval.missingEvidence, err.Error())
		return eval
	}
	eval.outsideRequests = outsideAllowedNetworkRequests(network, pack.GUI.AllowedOrigins)
	if !eval.confirmed && len(eval.blockedAttempts) == 0 {
		eval.missingEvidence = append(eval.missingEvidence, "journey_graph.runtime_policy_enforced")
	}
	return eval
}

func readDeclaredJSON(projectDir string, pack journey.Pack, kind string) (map[string]any, error) {
	for _, artifact := range pack.Artifacts {
		if strings.EqualFold(strings.TrimSpace(artifact.Kind), kind) && strings.TrimSpace(artifact.Path) != "" {
			path := artifact.Path
			if !filepath.IsAbs(path) {
				path = filepath.Join(projectDir, path)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("%s artifact unavailable", kind)
			}
			var doc map[string]any
			if err := json.Unmarshal(body, &doc); err != nil {
				return nil, fmt.Errorf("%s artifact is not valid json", kind)
			}
			return doc, nil
		}
	}
	return nil, fmt.Errorf("%s artifact is not declared", kind)
}

func runtimePolicyConfirmed(doc map[string]any) bool {
	for _, key := range []string{"runtime_policy_enforced", "runtime_enforcement_confirmed", "policy_enforced"} {
		if confirmed, _ := doc[key].(bool); confirmed {
			return true
		}
	}
	nested, _ := doc["policy_enforcement"].(map[string]any)
	for _, key := range []string{"confirmed", "enforced", "runtime_policy_enforced"} {
		if confirmed, _ := nested[key].(bool); confirmed {
			return true
		}
	}
	status := strings.ToLower(strings.TrimSpace(stringValue(nested["status"])))
	return status == "confirmed" || status == "enforced" || status == "passed" || status == "ok"
}

func stoppedPolicyAttempts(doc map[string]any, pack journey.Pack) []string {
	values, ok := doc["stopped_actions"].([]any)
	if !ok {
		return nil
	}
	forbidden := valueSet(pack.GUI.ForbiddenActions)
	attempts := []string{}
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		reason := strings.ToLower(strings.TrimSpace(stringValue(item["reason"])))
		actionClass := strings.ToLower(strings.TrimSpace(stringValue(item["action_class"])))
		if strings.Contains(reason, "off_origin") || strings.Contains(reason, "off-origin") {
			target := firstText(item, "attempted_url", "url", "target")
			attempts = append(attempts, "off_origin_navigation:"+target)
			continue
		}
		if strings.Contains(reason, "forbidden_action") || forbidden[actionClass] {
			if actionClass == "" {
				actionClass = firstText(item, "action", "type")
			}
			attempts = append(attempts, "forbidden_action:"+actionClass)
		}
	}
	return attempts
}

func guiAvailabilityFailures(doc map[string]any) []string {
	availability, ok := doc["availability"].(map[string]any)
	if !ok {
		return nil
	}
	status := strings.ToLower(strings.TrimSpace(stringValue(availability["status"])))
	switch status {
	case "", "available", "ready", "ok", "passed":
		return nil
	default:
		return []string{status}
	}
}

func expectedGUIRuntimePolicy(pack journey.Pack) string {
	return "allowed_origins=" + strings.Join(cleanedList(pack.GUI.AllowedOrigins), ",") +
		"; forbidden_actions=" + strings.Join(cleanedList(pack.GUI.ForbiddenActions), ",")
}

func actualGUIRuntimePolicy(eval guiPolicyEvaluation) string {
	parts := []string{fmt.Sprintf("runtime_policy_enforced=%t", eval.confirmed)}
	parts = append(parts, "target_availability="+joinOrNone(eval.unavailable))
	parts = append(parts, "blocked_attempts="+joinOrNone(eval.blockedAttempts))
	parts = append(parts, "network_outside_allowed="+joinOrNone(eval.outsideRequests))
	parts = append(parts, "missing_screens="+joinOrNone(eval.missingScreens))
	parts = append(parts, "missing_screen_actions="+joinOrNone(eval.missingActions))
	if len(eval.missingEvidence) > 0 {
		parts = append(parts, "missing="+strings.Join(eval.missingEvidence, ", "))
	}
	return strings.Join(parts, "; ")
}

func failureGUIRuntimePolicy(eval guiPolicyEvaluation) string {
	if len(eval.unavailable) > 0 {
		return "gui target was unavailable"
	}
	if len(eval.blockedAttempts) > 0 {
		return "gui runtime policy blocked unsafe action"
	}
	if len(eval.outsideRequests) > 0 {
		return "gui runtime policy detected off-origin network request"
	}
	if len(eval.missingScreens) > 0 || len(eval.missingActions) > 0 {
		return "gui screen matrix coverage was incomplete"
	}
	return "gui runtime policy enforcement was not confirmed"
}

func cleanedList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func valueSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[strings.ToLower(strings.TrimSpace(value))] = true
	}
	return out
}

func guiGuardReady(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func firstText(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(item[key]); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
