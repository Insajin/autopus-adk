package evidence

// DesktopObservationProvenance returns the provenance the desktop observation
// contract requires on a published manifest.
//
// The harness, not the project, produces this evidence: the adapter builds the
// manifest, the typed projection, and the artifact. So the governing SPEC and
// acceptance refs are harness facts, and validDesktopObservationStructure pins
// them. Exposing them here gives the producer and the validator one source; the
// alternative was making every project type Autopus's own SPEC id into its
// Journey Pack to get a publishable manifest, which is the magic-constant
// pattern this codebase already removed once for GUI artifact kinds.
func DesktopObservationProvenance() SourceRefs {
	return SourceRefs{
		Adapter:        desktopObservationAdapterID,
		SourceSpec:     desktopObservationSourceSpec,
		JourneyID:      desktopObservationAdapterID,
		StepID:         desktopObservationStepID,
		AcceptanceRefs: []string{"AC-QAMESH12-001"},
	}
}

// DesktopObservationScenarioRef is the scenario_ref the contract pins, kept
// beside the provenance so a producer cannot set one without the other.
func DesktopObservationScenarioRef() string {
	return desktopObservationAdapterID
}
