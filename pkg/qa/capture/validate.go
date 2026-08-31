package capture

import (
	"fmt"
	"regexp"
	"strings"
)

// Structural bounds. These cap what a producer can push into published evidence
// and into a rendered report; exceeding a bound is a contract violation, not a
// silent truncation, so the producer learns about it.
const (
	MaxSteps                = 200
	MaxActionsPerStep       = 200
	MaxConsolePerStep       = 200
	MaxNetworkPerStep       = 500
	MaxMediaEntries         = 200
	MaxTextLen              = 2000
	MaxRefLen               = 512
	MaxReplayCommandLen     = 32
	MaxSpecRefs             = 64
	maxHTTPStatus           = 599
	minRecognizedHTTPStatus = 100
)

var (
	digestRe     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	stepIDRe     = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,120}$`)
	originRefRe  = regexp.MustCompile(`^origin:[0-9]{1,3}(/[^\s?#]*)?$`)
	httpMethodRe = regexp.MustCompile(`^[A-Z]{3,10}$`)
	// shellMetaRe rejects replay commands that would need a shell to run.
	shellMetaRe = regexp.MustCompile("[;&|<>$`\n\r\\\\]")
)

// Validate checks the index against the contract independently of any journey
// policy. Policy conformance is a separate gate in policy.go.
func Validate(index Index) error {
	if index.SchemaVersion != IndexSchemaVersion {
		return fmt.Errorf("unsupported schema_version %q", index.SchemaVersion)
	}
	if strings.TrimSpace(index.JourneyID) == "" {
		return fmt.Errorf("missing required field journey_id")
	}
	if !validMode(index.Mode) {
		return fmt.Errorf("unsupported mode %q", index.Mode)
	}
	for _, stream := range index.Streams {
		if !validStream(stream) {
			return fmt.Errorf("unsupported stream %q", stream)
		}
	}
	if len(index.Steps) > MaxSteps {
		return fmt.Errorf("steps exceed %d entries", MaxSteps)
	}
	if index.Mode != ModeOff && len(index.Steps) == 0 {
		return fmt.Errorf("capture mode %q requires at least one step", index.Mode)
	}
	if err := validateSteps(index.Steps); err != nil {
		return err
	}
	if err := validateMedia(index.Media); err != nil {
		return err
	}
	if err := validateReplay(index.Replay); err != nil {
		return err
	}
	return validateTotals(index)
}

func validateSteps(steps []Step) error {
	seen := make(map[string]bool, len(steps))
	for position, step := range steps {
		if !stepIDRe.MatchString(step.StepID) {
			return fmt.Errorf("steps[%d].step_id must match %s", position, stepIDRe)
		}
		if seen[step.StepID] {
			return fmt.Errorf("steps[%d] repeats step_id %q", position, step.StepID)
		}
		seen[step.StepID] = true
		// A dense 1..n order is what makes the filmstrip trustworthy: a gap
		// means the producer dropped a step instead of reporting it.
		if step.Order != position+1 {
			return fmt.Errorf("steps[%d].order must be %d, got %d", position, position+1, step.Order)
		}
		if err := validateStepBody(position, step); err != nil {
			return err
		}
	}
	return nil
}

func validateStepBody(position int, step Step) error {
	switch step.Status {
	case StatusPassed, StatusSkipped:
	case StatusFailed, StatusBlocked:
		if strings.TrimSpace(step.FailureSummary) == "" {
			return fmt.Errorf("steps[%d].failure_summary is required for status %q", position, step.Status)
		}
	default:
		return fmt.Errorf("steps[%d].status %q is unsupported", position, step.Status)
	}
	if len(step.Actions) > MaxActionsPerStep {
		return fmt.Errorf("steps[%d].actions exceed %d entries", position, MaxActionsPerStep)
	}
	for index, action := range step.Actions {
		if strings.TrimSpace(action.API) == "" {
			return fmt.Errorf("steps[%d].actions[%d].api is required", position, index)
		}
	}
	if step.Screenshot != nil {
		if err := validateMediaRef(*step.Screenshot); err != nil {
			return fmt.Errorf("steps[%d].screenshot: %w", position, err)
		}
	}
	if err := validateConsole(position, step.Console); err != nil {
		return err
	}
	return validateNetwork(position, step.Network)
}

func validateConsole(position int, console *ConsoleSummary) error {
	if console == nil {
		return nil
	}
	if len(console.Messages) > MaxConsolePerStep {
		return fmt.Errorf("steps[%d].console.messages exceed %d entries", position, MaxConsolePerStep)
	}
	for index, message := range console.Messages {
		switch message.Severity {
		case SeverityInfo, SeverityWarning, SeverityError:
		default:
			return fmt.Errorf("steps[%d].console.messages[%d].severity %q is unsupported", position, index, message.Severity)
		}
		if strings.TrimSpace(message.Text) == "" {
			return fmt.Errorf("steps[%d].console.messages[%d].text is required", position, index)
		}
	}
	return nil
}

// validateTotals rejects a mismatch between headline numbers and step evidence.
// Consumers render totals directly, so a wrong total is a wrong report.
func validateTotals(index Index) error {
	want := ComputeTotals(index)
	if index.Totals != want {
		return fmt.Errorf("totals disagree with steps: got %+v, want %+v", index.Totals, want)
	}
	return nil
}

// ComputeTotals derives the headline counters from step and media evidence.
func ComputeTotals(index Index) Totals {
	totals := Totals{Steps: len(index.Steps)}
	for _, step := range index.Steps {
		totals.Actions += len(step.Actions)
		if step.Console != nil {
			totals.ConsoleErrors += step.Console.Errors
		}
		if step.Network != nil {
			totals.NetworkFailures += step.Network.Failures
		}
		if step.Screenshot != nil {
			totals.Screenshots++
			totals.MediaBytes += step.Screenshot.Bytes
		}
	}
	for _, entry := range index.Media {
		totals.MediaBytes += entry.Bytes
	}
	return totals
}

func validMode(mode string) bool {
	switch mode {
	case ModeOff, ModeOnFailure, ModeAlways:
		return true
	}
	return false
}

func validStream(stream string) bool {
	switch stream {
	case StreamScreenshot, StreamConsole, StreamNetwork, StreamTrace, StreamVideo:
		return true
	}
	return false
}
