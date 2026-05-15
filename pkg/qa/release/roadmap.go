package release

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: roadmap payload is the governance surface for release lanes and sibling SPEC dependencies.
// @AX:REASON: downgraded from ANCHOR during sync; grep fan-in is below threshold, but CLI --roadmap and roadmap tests still rely on this contract.
func Roadmap() RoadmapPayload {
	profiles := profilePolicies()
	lanes := make([]RoadmapLane, 0, len(ReleaseLanes()))
	for _, lane := range ReleaseLanes() {
		catalog := laneByID(lane)
		lanes = append(lanes, RoadmapLane{
			Lane:                 lane,
			OwnerSpec:            catalog.OwnerSpec,
			OwnerRepo:            catalog.OwnerRepo,
			ImplementationState:  catalog.ImplementationState,
			SiblingDependency:    siblingDependencyForLane(lane),
			ReadinessContract:    catalog.ReadinessContract,
			LanePolicyByProfile:  lanePolicyByProfile(profiles, lane),
			LaunchBlockingPolicy: lanePolicyByProfile(profiles, lane),
		})
	}
	return RoadmapPayload{
		SchemaVersion: RoadmapSchemaVersion,
		GeneratedAt:   "",
		Lanes:         lanes,
		SiblingSpecs:  SiblingSpecs(),
		Profiles:      profiles,
	}
}

func RoadmapAt(generatedAt string) RoadmapPayload {
	payload := Roadmap()
	payload.GeneratedAt = generatedAt
	return payload
}

func lanePolicyByProfile(profiles map[string]ProfilePolicy, lane string) map[string]LanePolicy {
	out := map[string]LanePolicy{}
	for profile, policy := range profiles {
		out[profile] = lanePolicy(policy, lane)
	}
	return out
}

func siblingDependencyForLane(lane string) string {
	switch lane {
	case "browser-staging", "desktop-native":
		return "SPEC-QAMESH-005"
	case "mobile-readiness":
		return "SPEC-QAMESH-006"
	case "evidence-dashboard":
		return "SPEC-QAMESH-007"
	default:
		return "none"
	}
}
