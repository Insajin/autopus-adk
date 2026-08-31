package capture

import (
	"fmt"
	"sort"
	"strings"
)

// Screenshot cadences.
const (
	ScreenshotOff       = "off"
	ScreenshotOnFailure = "on-failure"
	ScreenshotPerStep   = "per-step"
)

// Replay policies.
const (
	ReplayOff      = "off"
	ReplayOptional = "optional"
	ReplayRequired = "required"
)

// Policy is the journey-declared capture contract, read from `gui.capture` in a
// Journey Pack. It lives here so the harness, the validator, and the producer
// share one vocabulary instead of agreeing by convention.
type Policy struct {
	Mode            string   `yaml:"mode,omitempty" json:"mode,omitempty"`
	Streams         []string `yaml:"streams,omitempty" json:"streams,omitempty"`
	Screenshot      string   `yaml:"screenshot,omitempty" json:"screenshot,omitempty"`
	ConsoleSeverity string   `yaml:"console_severity,omitempty" json:"console_severity,omitempty"`
	RetainLocal     bool     `yaml:"retain_local,omitempty" json:"retain_local,omitempty"`
	ReplayScript    string   `yaml:"replay_script,omitempty" json:"replay_script,omitempty"`
	// Unknown collects any key the contract does not define. A misspelled
	// capture field used to disable evidence silently; now it fails the pack.
	Unknown map[string]any `yaml:",inline" json:"-"`
}

// Declared reports whether the pack opted into typed capture at all. Packs
// written before this contract existed leave the block empty and keep working.
func (p Policy) Declared() bool {
	return strings.TrimSpace(p.Mode) != "" || len(p.Streams) > 0 ||
		strings.TrimSpace(p.Screenshot) != "" || strings.TrimSpace(p.ConsoleSeverity) != "" ||
		strings.TrimSpace(p.ReplayScript) != "" || p.RetainLocal || len(p.Unknown) > 0
}

// Enabled reports whether capture evidence is expected from this journey.
func (p Policy) Enabled() bool {
	return p.Declared() && p.Mode != ModeOff
}

// HasStream reports whether the policy declared a stream.
func (p Policy) HasStream(stream string) bool {
	for _, declared := range p.Streams {
		if declared == stream {
			return true
		}
	}
	return false
}

// ValidatePolicy statically validates a declared capture policy.
func ValidatePolicy(policy Policy) error {
	if !policy.Declared() {
		return nil
	}
	if len(policy.Unknown) > 0 {
		return fmt.Errorf("gui.capture contains unknown fields: %s", strings.Join(sortedKeys(policy.Unknown), ", "))
	}
	if !validMode(policy.Mode) {
		return fmt.Errorf("gui.capture.mode must be off, on-failure, or always")
	}
	seen := map[string]bool{}
	for _, stream := range policy.Streams {
		if !validStream(stream) {
			return fmt.Errorf("gui.capture.streams contains unsupported stream %q", stream)
		}
		if seen[stream] {
			return fmt.Errorf("gui.capture.streams repeats %q", stream)
		}
		seen[stream] = true
	}
	if policy.Mode == ModeOff && len(policy.Streams) > 0 {
		return fmt.Errorf("gui.capture.mode off may not declare streams")
	}
	if err := validatePolicyCadence(policy); err != nil {
		return err
	}
	// Traces and videos are raw media. They may exist only as local-only
	// evidence, so declaring them without local retention is a contradiction.
	for _, stream := range []string{StreamTrace, StreamVideo} {
		if policy.HasStream(stream) && !policy.RetainLocal {
			return fmt.Errorf("gui.capture.streams %q requires gui.capture.retain_local: true", stream)
		}
	}
	return nil
}

func validatePolicyCadence(policy Policy) error {
	switch policy.Screenshot {
	case "", ScreenshotOff:
	case ScreenshotOnFailure, ScreenshotPerStep:
		if !policy.HasStream(StreamScreenshot) {
			return fmt.Errorf("gui.capture.screenshot requires the screenshot stream")
		}
		if !policy.RetainLocal {
			return fmt.Errorf("gui.capture.screenshot requires gui.capture.retain_local: true")
		}
	default:
		return fmt.Errorf("gui.capture.screenshot must be off, on-failure, or per-step")
	}
	switch policy.ConsoleSeverity {
	case "", SeverityInfo, SeverityWarning, SeverityError:
	default:
		return fmt.Errorf("gui.capture.console_severity must be info, warning, or error")
	}
	switch policy.ReplayScript {
	case "", ReplayOff, ReplayOptional, ReplayRequired:
	default:
		return fmt.Errorf("gui.capture.replay_script must be off, optional, or required")
	}
	return nil
}

// Conform cross-checks a validated index against the journey policy and returns
// every disagreement. An empty result means the producer delivered exactly the
// evidence the pack promised.
//
// `always` requires the declared streams on every step; `on-failure` requires
// them only where a step actually failed, so a green run stays cheap.
func Conform(index Index, policy Policy) []string {
	var findings []string
	if index.Mode != policy.Mode {
		findings = append(findings, fmt.Sprintf("capture mode %q does not match policy %q", index.Mode, policy.Mode))
	}
	for _, stream := range index.Streams {
		if !policy.HasStream(stream) {
			findings = append(findings, fmt.Sprintf("capture stream %q was not declared by the policy", stream))
		}
	}
	findings = append(findings, conformStreams(index, policy)...)
	findings = append(findings, conformSteps(index, policy)...)
	if policy.ReplayScript == ReplayRequired && index.Replay == nil {
		findings = append(findings, "policy requires a replay reference but none was captured")
	}
	return findings
}

func conformStreams(index Index, policy Policy) []string {
	var findings []string
	for _, stream := range []string{StreamTrace, StreamVideo} {
		if !policy.HasStream(stream) || !requiresAlways(index, policy) {
			continue
		}
		if !hasMediaKind(index.Media, stream) {
			findings = append(findings, fmt.Sprintf("policy declared the %s stream but no %s media was captured", stream, stream))
		}
	}
	return findings
}

func conformSteps(index Index, policy Policy) []string {
	var findings []string
	for _, step := range index.Steps {
		if !stepNeedsEvidence(step, policy, requiresAlways(index, policy)) {
			continue
		}
		if policy.HasStream(StreamConsole) && step.Console == nil {
			findings = append(findings, fmt.Sprintf("step %q is missing console evidence", step.StepID))
		}
		if policy.HasStream(StreamNetwork) && step.Network == nil {
			findings = append(findings, fmt.Sprintf("step %q is missing network evidence", step.StepID))
		}
		if needsScreenshot(step, policy) && step.Screenshot == nil {
			findings = append(findings, fmt.Sprintf("step %q is missing a screenshot", step.StepID))
		}
		findings = append(findings, conformConsoleSeverity(step, policy)...)
	}
	return findings
}

// conformConsoleSeverity rejects messages quieter than the declared threshold:
// a producer must not pad evidence with noise the pack chose not to retain.
func conformConsoleSeverity(step Step, policy Policy) []string {
	if step.Console == nil || policy.ConsoleSeverity == "" {
		return nil
	}
	threshold := severityRank(policy.ConsoleSeverity)
	var findings []string
	for _, message := range step.Console.Messages {
		if severityRank(message.Severity) < threshold {
			findings = append(findings, fmt.Sprintf("step %q retained %s console output below the %s threshold", step.StepID, message.Severity, policy.ConsoleSeverity))
			break
		}
	}
	return findings
}

func stepNeedsEvidence(step Step, policy Policy, always bool) bool {
	if step.Status == StatusSkipped {
		return false
	}
	if always {
		return true
	}
	_ = policy
	return step.Status == StatusFailed || step.Status == StatusBlocked
}

func needsScreenshot(step Step, policy Policy) bool {
	switch policy.Screenshot {
	case ScreenshotPerStep:
		return step.Status != StatusSkipped
	case ScreenshotOnFailure:
		return step.Status == StatusFailed || step.Status == StatusBlocked
	default:
		return false
	}
}

func requiresAlways(index Index, policy Policy) bool {
	return policy.Mode == ModeAlways && index.Mode == ModeAlways
}

func hasMediaKind(media []Media, kind string) bool {
	for _, entry := range media {
		if entry.Kind == kind {
			return true
		}
	}
	return false
}

func severityRank(severity string) int {
	switch severity {
	case SeverityError:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
