package worker

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
)

// ParsePhasePlan validates and canonicalizes a server-provided phase plan.
func ParsePhasePlan(phases []string) ([]Phase, error) {
	if len(phases) == 0 {
		return nil, nil
	}

	plan := make([]Phase, 0, len(phases))
	for _, raw := range phases {
		phase, err := ParsePhase(raw)
		if err != nil {
			return nil, err
		}
		plan = append(plan, phase)
	}
	if len(plan) == 0 {
		return nil, nil
	}
	return plan, nil
}

// ParsePhase validates and canonicalizes a single phase name.
func ParsePhase(name string) (Phase, error) {
	switch phase := Phase(strings.ToLower(strings.TrimSpace(name))); phase {
	case PhasePlanner, PhaseExecutor, PhaseTester, PhaseReviewer:
		return phase, nil
	case "":
		return "", fmt.Errorf("empty phase name")
	default:
		return "", fmt.Errorf("unsupported phase %q", name)
	}
}

// ParsePhaseInstructions validates phase instruction overrides from the server.
func ParsePhaseInstructions(instructions map[string]string) (map[Phase]string, error) {
	return parsePhaseTextMap(instructions)
}

// ParsePhasePromptTemplates validates server-provided full prompt templates.
func ParsePhasePromptTemplates(templates map[string]string) (map[Phase]string, error) {
	return parsePhaseTextMap(templates)
}

func parsePhaseTextMap(values map[string]string) (map[Phase]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	parsed := make(map[Phase]string, len(values))
	for rawPhase, value := range values {
		phase, err := ParsePhase(rawPhase)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		parsed[phase] = strings.TrimSpace(value)
	}
	if len(parsed) == 0 {
		return nil, nil
	}
	return parsed, nil
}

func normalizePhasePlan(phases []Phase) []Phase {
	if len(phases) == 0 && a2a.SignedControlPlaneEnforced() {
		return nil
	}
	if len(phases) == 0 {
		return append([]Phase(nil), defaultPipelinePhases...)
	}
	return append([]Phase(nil), phases...)
}

func renderPhasePromptTemplate(template, input string) string {
	if strings.Contains(template, "{{input}}") {
		return strings.ReplaceAll(template, "{{input}}", input)
	}
	return fmt.Sprintf("%s\n\n%s", template, input)
}
