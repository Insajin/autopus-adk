package journey

import (
	"slices"
	"strings"
	"unicode"
)

const (
	desktopObservationAdapterID  = "desktop-accessibility-observe"
	desktopObservationPolicyCode = "qa_journey_desktop_observation_policy_invalid"

	// Refs and landmark names are bounded so a pack cannot smuggle a payload
	// through a field that ends up in evidence, logs, and provider requests.
	desktopObservationRefMaxLen  = 64
	desktopObservationNameMaxLen = 96
)

var (
	desktopObservationOperations = []string{
		"capabilities",
		"permissions",
		"list_apps",
		"list_windows",
		"get_state",
	}
	// The observation contract is a specific two-landmark shape: the application
	// must be enabled and its window focused. Roles and required states are
	// structural; only the accessibility names are project identity.
	desktopObservationLandmarkShape = []struct{ Role, RequiredState string }{
		{Role: "AXApplication", RequiredState: "enabled"},
		{Role: "AXWindow", RequiredState: "focused"},
	}
	desktopObservationPlatforms = []string{"macos"}
)

// validateDesktopObservationPolicy enforces the read-only observation contract.
//
// It validates the SHAPE of the target, not its identity: any project may point
// the lane at its own app. The trust anchor is not the app name — the local
// runtime provider still refuses to start without a signature-verified release
// artifact (pkg/qa/run/desktop_observe_process.go) — so pinning "autopus-desktop"
// here bought no safety and made desktop-native unreachable for every project
// that is not Autopus itself.
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
	return validateDesktopObservationTarget(pack.DesktopObservation, invalid)
}

func validateDesktopObservationTarget(
	policy DesktopObservationPolicy,
	invalid func(string) error,
) error {
	if !slices.Contains(desktopObservationPlatforms, policy.Platform) {
		return invalid("desktop observation platform must be one of: " +
			strings.Join(desktopObservationPlatforms, ", "))
	}
	if !slices.Equal(policy.Operations, desktopObservationOperations) {
		return invalid("desktop observation operations must match the read-only sequence")
	}
	if !safeDesktopObservationRef(policy.AppRef) {
		return invalid("desktop observation app_ref must be a short alias of " +
			"letters, digits, dot, underscore, or hyphen")
	}
	if !safeDesktopObservationRef(policy.WindowRef) {
		return invalid("desktop observation window_ref must be a short alias of " +
			"letters, digits, dot, underscore, or hyphen")
	}
	return validateDesktopObservationLandmarks(policy.RequiredLandmarks, invalid)
}

// validateDesktopObservationLandmarks keeps the canonical landmark structure:
// exactly one application landmark that must be enabled followed by one window
// landmark that must be focused. Only the names are project-supplied.
func validateDesktopObservationLandmarks(
	landmarks []DesktopObservationLandmark,
	invalid func(string) error,
) error {
	if len(landmarks) != len(desktopObservationLandmarkShape) {
		return invalid("desktop observation required_landmarks must declare " +
			"exactly one application landmark and one window landmark")
	}
	for index, want := range desktopObservationLandmarkShape {
		got := landmarks[index]
		if got.Role != want.Role || got.RequiredState != want.RequiredState {
			return invalid("desktop observation required_landmarks[" + want.Role +
				"] must require state " + want.RequiredState)
		}
		if !safeDesktopObservationName(got.Name) {
			return invalid("desktop observation landmark name must be a short " +
				"printable single-line label")
		}
	}
	return nil
}

// safeDesktopObservationRef accepts an opaque alias the provider can match
// against. Path separators, whitespace, quoting, and control characters are
// rejected so the ref cannot alter a provider request or an evidence path.
func safeDesktopObservationRef(value string) bool {
	if value == "" || len(value) > desktopObservationRefMaxLen {
		return false
	}
	for index, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
			continue
		case (char == '.' || char == '_' || char == '-') && index > 0:
			continue
		default:
			return false
		}
	}
	return true
}

// safeDesktopObservationName accepts a human accessibility label, which may
// contain spaces, but never control characters or surrounding whitespace.
func safeDesktopObservationName(value string) bool {
	if value == "" || len(value) > desktopObservationNameMaxLen {
		return false
	}
	if strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if !unicode.IsPrint(char) {
			return false
		}
	}
	return true
}

func emptyDesktopObservationCommand(command Command) bool {
	return command.Run == "" &&
		command.Argv == nil &&
		command.CWD == "" &&
		command.Timeout == "" &&
		command.EnvAllowlist == nil
}
