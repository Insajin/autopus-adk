package workflow

import "encoding/json"

// MinVersion is the minimum claude-code version this Primary workflow route
// requires (REQ-006).
const MinVersion = "2.1.154"

const (
	// StatusAvailable / StatusUnavailable are the per-primitive probe states.
	StatusAvailable   = "available"
	StatusUnavailable = "unavailable"
	// OverallPass / OverallFail are the capability gate verdicts.
	OverallPass = "pass"
	OverallFail = "fail"
)

// RequiredPrimitives are hard-gated: any unavailable one fails the gate.
var RequiredPrimitives = []string{"claude", "agent", "schema", "phase"}

// AdvisoryPrimitives are probed and reported but never affect the verdict.
var AdvisoryPrimitives = []string{"parallel", "isolation", "budget", "agent-model-override"}

// Prober is the injectable capability-probe seam. The production implementation
// inspects the claude-code runtime; tests inject a fake.
type Prober interface {
	// Probe reports whether a named workflow primitive is available.
	Probe(primitive string) bool
	// Version returns the probed claude-code version string (dotted ints).
	Version() string
}

// PrimitiveStatus is a single probed primitive in the capability report.
// Gating is true for required primitives, false for advisory ones.
type PrimitiveStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Gating bool   `json:"gating"`
}

// CapabilityReport is the structured `auto workflow doctor` output.
type CapabilityReport struct {
	Primitives []PrimitiveStatus `json:"primitives"`
	Version    string            `json:"version"`
	VersionOK  bool              `json:"version_ok"`
	Overall    string            `json:"overall"`
}

// EvaluateCapabilities probes required and advisory primitives plus the version
// pin and produces the capability report. Overall is "fail" iff any required
// primitive is unavailable OR the version is below MinVersion. Advisory
// primitives are reported with Gating=false and never change Overall.
//
// Named EvaluateCapabilities (not Evaluate) because the gate evaluator
// EvaluateGate shares this package; Go forbids two same-named funcs.
func EvaluateCapabilities(p Prober) CapabilityReport {
	report := CapabilityReport{Version: p.Version()}
	report.VersionOK = versionAtLeast(report.Version, MinVersion)

	failed := !report.VersionOK

	for _, name := range RequiredPrimitives {
		status := StatusAvailable
		if !p.Probe(name) {
			status = StatusUnavailable
			failed = true
		}
		report.Primitives = append(report.Primitives, PrimitiveStatus{
			Name:   name,
			Status: status,
			Gating: true,
		})
	}

	for _, name := range AdvisoryPrimitives {
		status := StatusAvailable
		if !p.Probe(name) {
			status = StatusUnavailable
		}
		report.Primitives = append(report.Primitives, PrimitiveStatus{
			Name:   name,
			Status: status,
			Gating: false,
		})
	}

	report.Overall = OverallPass
	if failed {
		report.Overall = OverallFail
	}
	return report
}

// EncodeJSON serializes the capability report for CLI stdout consumption.
func (r CapabilityReport) EncodeJSON() ([]byte, error) {
	return json.Marshal(r)
}
