package journey

import (
	"slices"
	"strconv"
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
	// desktopObservationLandmarkMax bounds how many landmarks a pack may declare.
	// The published projection is one node per declared landmark, so this is what
	// keeps it under the 8 KiB typed evidence bound regardless of app size.
	desktopObservationLandmarkMax = 24

	// provider_app_id is never published, so it is not bounded by the evidence
	// budget the refs answer to. It is bounded anyway because it is
	// interpolated into an argv element handed to an external provider CLI:
	// 128 bytes is well past any real reverse-DNS bundle identifier and far
	// short of anything that could be a smuggled payload.
	desktopObservationProviderAppIDMaxLen = 128
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
	// Absence is called out separately from malformation: REQ-3 requires the
	// setup gap to name the missing field, and there is deliberately no
	// compiled-in identifier to fall back to.
	if policy.ProviderAppID == "" {
		return invalid("desktop observation provider_app_id is required: the pack must " +
			"declare the platform identifier of the app under observation")
	}
	if !safeDesktopObservationProviderAppID(policy.ProviderAppID) {
		return invalid("desktop observation provider_app_id must be at most " +
			strconv.Itoa(desktopObservationProviderAppIDMaxLen) + " characters of letters, " +
			"digits, dot, underscore, or hyphen, not beginning with a separator")
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

// desktopObservationExtraLandmarkStates are the states an additional landmark may
// require. They are the states the provider's rendered tree actually reports, so a
// pack cannot declare a state the harness has no way to observe.
var desktopObservationExtraLandmarkStates = []string{"enabled", "focused", "selected", "expanded"}

// validateDesktopObservationLandmarks requires the canonical pair first - one
// application landmark that must be enabled, then one window landmark that must be
// focused - and permits additional landmarks after them.
//
// The pair is what the app and window identity are derived from, so it stays
// mandatory and positional. Additional landmarks are how a project states what it
// actually cares about inside the window; without them the oracle can only assert
// "the app is running and its window is focused", and SPEC-QAMESH-013 REQ-4's
// selective publication would have nothing to select. Exactly two was inherited
// from the fixture era, when the projection was three synthesized nodes.
func validateDesktopObservationLandmarks(
	landmarks []DesktopObservationLandmark,
	invalid func(string) error,
) error {
	if len(landmarks) < len(desktopObservationLandmarkShape) {
		return invalid("desktop observation required_landmarks must begin with " +
			"one application landmark and one window landmark")
	}
	if len(landmarks) > desktopObservationLandmarkMax {
		return invalid("desktop observation required_landmarks must declare at most " +
			strconv.Itoa(desktopObservationLandmarkMax) + " landmarks")
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
	for _, extra := range landmarks[len(desktopObservationLandmarkShape):] {
		if !safeDesktopObservationRef(extra.Role) {
			return invalid("desktop observation landmark role must be a short alias of " +
				"letters, digits, dot, underscore, or hyphen")
		}
		if !slices.Contains(desktopObservationExtraLandmarkStates, extra.RequiredState) {
			return invalid("desktop observation landmark required_state must be one of: " +
				strings.Join(desktopObservationExtraLandmarkStates, ", "))
		}
		if !safeDesktopObservationName(extra.Name) {
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

// safeDesktopObservationProviderAppID accepts the platform identifier the
// provider resolves. It is deliberately NOT validated with
// safeDesktopObservationRef: that grammar exists to keep a ref safe as an
// evidence path component, and it is also applied to values that are published,
// whereas provider_app_id is request-only. The two grammars happen to overlap,
// but they answer different questions and must be free to diverge.
//
// The value is interpolated into a single argv element handed to an external
// provider CLI. That is not a shell, so quoting is not the threat; the threats
// are argument injection and a value the provider cannot round-trip. So the
// grammar is an ASCII allowlist — letters, digits, dot, underscore, hyphen —
// which excludes whitespace, quotes, backticks, `$`, `;`, path separators, NUL,
// every other control character, and all non-ASCII. A leading dot, underscore,
// or hyphen is rejected because a leading hyphen turns the value into a flag
// and a leading dot makes it path-relative if a provider ever resolves it as
// one. Reverse-DNS bundle identifiers such as `co.autopus.desktop` and
// `com.apple.finder` satisfy this unchanged.
func safeDesktopObservationProviderAppID(value string) bool {
	if value == "" || len(value) > desktopObservationProviderAppIDMaxLen {
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

// safeDesktopObservationName accepts a human accessibility label. It may contain
// spaces but never a control character, a line break, or surrounding whitespace.
//
// Unicode space separators are accepted deliberately. Go's unicode.IsPrint counts
// only the ASCII space as printable among spaces, so it rejects U+00A0 - and a
// measured macOS Finder window title is "맥북판매의 Mac\u00a0Studio". Rejecting a
// real window title as unprintable made the landmark undeclarable, which is the
// lane being unreachable again for a new reason. Line separators stay rejected:
// the label must remain single-line.
func safeDesktopObservationName(value string) bool {
	if value == "" || len(value) > desktopObservationNameMaxLen {
		return false
	}
	if strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if unicode.Is(unicode.Zs, char) {
			continue
		}
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
