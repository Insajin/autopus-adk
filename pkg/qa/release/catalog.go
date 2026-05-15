package release

import "slices"

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-004: canonical release lane IDs and order drive plan, execution, roadmap, and tests.
// @AX:REASON: BuildPlan, Execute, Roadmap, and release guidance expect these lane IDs and ordering to remain stable.
func ReleaseLanes() []string {
	return []string{"fast", "browser-staging", "desktop-native", "gui-explore", "mobile-readiness", "canary-explicit", "evidence-dashboard"}
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: severity ordering defines threshold comparisons in the release blocker matrix.
func SeverityOrder() []Severity {
	return []Severity{SeverityNone, SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: redaction rule labels must match command-preview masking behavior and JSON contract expectations.
func RedactionRules() []string {
	return []string{"token-flag", "env-secret", "credential-url", "private-path"}
}

func ValidateProfile(profile string) error {
	if _, ok := profilePolicies()[profile]; !ok {
		return ErrInvalidProfile
	}
	return nil
}

func profilePolicy(profile string) (ProfilePolicy, error) {
	policies := profilePolicies()
	policy, ok := policies[profile]
	if !ok {
		return ProfilePolicy{}, ErrInvalidProfile
	}
	return policy, nil
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: profile lane buckets encode launch blocking policy for prelaunch, release-candidate, and postdeploy-smoke.
func profilePolicies() map[string]ProfilePolicy {
	return map[string]ProfilePolicy{
		"prelaunch": {
			MustLanes:     []string{"fast", "browser-staging", "desktop-native", "gui-explore"},
			OptionalLanes: []string{"canary-explicit"},
			DeferredLanes: []string{"mobile-readiness", "evidence-dashboard"},
		},
		"release-candidate": {
			MustLanes:     []string{"fast", "browser-staging", "desktop-native", "canary-explicit"},
			OptionalLanes: []string{"gui-explore", "evidence-dashboard"},
			DeferredLanes: []string{"mobile-readiness"},
		},
		"postdeploy-smoke": {
			MustLanes:     []string{"canary-explicit"},
			OptionalLanes: []string{"fast", "browser-staging", "gui-explore", "evidence-dashboard"},
			DeferredLanes: []string{"desktop-native", "mobile-readiness"},
		},
	}
}

func lanePolicy(policy ProfilePolicy, lane string) LanePolicy {
	switch {
	case slices.Contains(policy.MustLanes, lane):
		return LanePolicyMust
	case slices.Contains(policy.OptionalLanes, lane):
		return LanePolicyOptional
	default:
		return LanePolicyDeferred
	}
}

func LaneCatalog() []LaneCatalogRow {
	rows := make([]LaneCatalogRow, 0, len(ReleaseLanes()))
	for _, lane := range ReleaseLanes() {
		rows = append(rows, laneCatalogRow(lane))
	}
	return rows
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: lane catalog rows hardcode owner SPEC, readiness contract, and implementation state for release governance.
func laneCatalogRow(lane string) LaneCatalogRow {
	switch lane {
	case "fast":
		return laneRow(lane, "SPEC-QAMESH-002", "ready", "project-qa-run", []string{"prelaunch", "release-candidate", "postdeploy-smoke"})
	case "browser-staging":
		return laneRow(lane, "SPEC-QAMESH-005", "planned", "project-local-journey-pack", []string{"prelaunch", "release-candidate", "postdeploy-smoke"})
	case "desktop-native":
		return laneRow(lane, "SPEC-QAMESH-005", "planned", "project-local-journey-pack", []string{"prelaunch", "release-candidate", "postdeploy-smoke"})
	case "gui-explore":
		return laneRow(lane, "SPEC-QAMESH-003", "ready", "explicit-gui-journey-pack", []string{"prelaunch", "release-candidate", "postdeploy-smoke"})
	case "mobile-readiness":
		return laneRow(lane, "SPEC-QAMESH-006", "planned", "sibling-readiness-contract", []string{"prelaunch", "release-candidate", "postdeploy-smoke"})
	case "canary-explicit":
		return laneRow(lane, "SPEC-QAMESH-004", "ready", "explicit-journey-pack", []string{"prelaunch", "release-candidate", "postdeploy-smoke"})
	case "evidence-dashboard":
		return laneRow(lane, "SPEC-QAMESH-007", "planned", "sibling-readiness-contract", []string{"prelaunch", "release-candidate", "postdeploy-smoke"})
	default:
		return laneRow(lane, "SPEC-QAMESH-004", "planned", "unknown", nil)
	}
}

func laneRow(lane, ownerSpec, state, readiness string, profiles []string) LaneCatalogRow {
	return LaneCatalogRow{
		Lane:                lane,
		OwnerSpec:           ownerSpec,
		OwnerRepo:           "autopus-adk",
		ReadinessContract:   readiness,
		ImplementationState: state,
		SupportedProfiles:   profiles,
	}
}

func SiblingSpecs() []SiblingSpec {
	return []SiblingSpec{
		{SpecID: "SPEC-QAMESH-005", OwnerRepo: "autopus-adk", Lanes: []string{"browser-staging", "desktop-native"}, Status: "planned", Relationship: "sibling"},
		{SpecID: "SPEC-QAMESH-006", OwnerRepo: "autopus-adk", Lanes: []string{"mobile-readiness"}, Status: "planned", Relationship: "sibling"},
		{SpecID: "SPEC-QAMESH-007", OwnerRepo: "autopus-adk", Lanes: []string{"evidence-dashboard"}, Status: "planned", Relationship: "sibling"},
	}
}

func laneByID(lane string) LaneCatalogRow {
	for _, row := range LaneCatalog() {
		if row.Lane == lane {
			return row
		}
	}
	return laneCatalogRow(lane)
}
