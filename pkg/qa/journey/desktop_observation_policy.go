package journey

import "slices"

const (
	desktopObservationAdapterID  = "desktop-accessibility-observe"
	desktopObservationPolicyCode = "qa_journey_desktop_observation_policy_invalid"
)

var (
	desktopObservationOperations = []string{
		"capabilities",
		"permissions",
		"list_apps",
		"list_windows",
		"get_state",
	}
	desktopObservationLandmarks = []DesktopObservationLandmark{
		{Role: "AXApplication", Name: "Autopus", RequiredState: "enabled"},
		{Role: "AXWindow", Name: "Autopus", RequiredState: "focused"},
	}
)

func validateDesktopObservationPolicy(pack Pack) error {
	if pack.Adapter.ID != desktopObservationAdapterID {
		return nil
	}
	invalid := func(message string) error {
		return validationError(desktopObservationPolicyCode, message)
	}
	if pack.Surface != "desktop" {
		return invalid("desktop observation surface must be desktop")
	}
	if !slices.Equal(pack.Lanes, []string{"desktop-native"}) {
		return invalid("desktop observation lane must be exactly desktop-native")
	}
	if pack.PassFailAuthority != "deterministic" {
		return invalid("desktop observation pass/fail authority must be deterministic")
	}
	if !emptyDesktopObservationCommand(pack.Command) {
		return invalid("desktop observation journey must not declare a command")
	}
	if len(pack.Artifacts) != 0 {
		return invalid("desktop observation journey must not declare artifacts")
	}

	policy := pack.DesktopObservation
	if policy.Platform != "macos" {
		return invalid("desktop observation platform must be macos")
	}
	if !slices.Equal(policy.Operations, desktopObservationOperations) {
		return invalid("desktop observation operations must match the read-only sequence")
	}
	if policy.AppRef != "autopus-desktop" {
		return invalid("desktop observation app_ref must be autopus-desktop")
	}
	if policy.WindowRef != "main-window" {
		return invalid("desktop observation window_ref must be main-window")
	}
	if !slices.Equal(policy.RequiredLandmarks, desktopObservationLandmarks) {
		return invalid("desktop observation required_landmarks must match the canonical landmarks")
	}
	return nil
}

func emptyDesktopObservationCommand(command Command) bool {
	return command.Run == "" &&
		command.Argv == nil &&
		command.CWD == "" &&
		command.Timeout == "" &&
		command.EnvAllowlist == nil
}
