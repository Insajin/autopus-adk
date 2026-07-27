// Package e2e provides user-facing scenario-based E2E test infrastructure.
package e2e

import "strings"

// @AX:NOTE [AUTO] @AX:REASON: design choice — "custom-" prefix is a reserved namespace for user-written scenarios; auto-sync never modifies or deprecates them; fan_in=0 (no production callers yet — used only in tests)
// SyncScenarios merges extracted commands into the existing ScenarioSet.
// Rules:
//   - New commands not in existing set are added with status "active".
//   - Existing scenarios whose command was removed are marked "deprecated".
//   - Scenarios with "custom-" prefix are preserved unchanged.
//   - Existing scenarios that match a current command retain their status.
func SyncScenarios(existing *ScenarioSet, commands []Scenario) (*ScenarioSet, error) {
	result := &ScenarioSet{
		ProjectName: existing.ProjectName,
		ProjectType: existing.ProjectType,
		Binary:      existing.Binary,
		Build:       existing.Build,
	}

	// Index current commands by ID for O(1) lookup.
	currentByID := make(map[string]Scenario, len(commands))
	for _, c := range commands {
		currentByID[c.ID] = c
	}

	// Index existing scenarios by ID.
	existingByID := make(map[string]Scenario, len(existing.Scenarios))
	for _, s := range existing.Scenarios {
		existingByID[s.ID] = s
	}

	// Process existing scenarios: preserve custom, deprecate removed, keep others.
	for _, s := range existing.Scenarios {
		if strings.HasPrefix(s.ID, "custom-") {
			// Always preserve custom scenarios unchanged.
			result.Scenarios = append(result.Scenarios, s)
			continue
		}
		if _, found := currentByID[s.ID]; found {
			// Command still exists: retain scenario as-is.
			result.Scenarios = append(result.Scenarios, s)
		} else {
			// Command removed: mark deprecated.
			s.Status = "deprecated"
			result.Scenarios = append(result.Scenarios, s)
		}
	}

	// Add newly detected commands that don't exist in the current set.
	nextNumber := len(result.Scenarios) + 1
	for _, c := range commands {
		if _, exists := existingByID[c.ID]; !exists {
			c.Status = "active"
			if c.Number == 0 {
				c.Number = nextNumber
				nextNumber++
			}
			if strings.TrimSpace(c.Ref) == "" {
				c.Ref = c.DisplayRef()
			}
			result.Scenarios = append(result.Scenarios, c)
		}
	}

	return result, nil
}
