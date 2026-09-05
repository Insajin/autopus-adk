package cli

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// requireSpecID rejects lead-time subrecords without a SPEC before any
// validation or file is touched.
func requireSpecID(p recordParams) error {
	if p.specID == "" {
		return fmt.Errorf("telemetry record %s: --spec-id is required", p.action)
	}
	return nil
}

// writeLeadTimeRecord opens a short-lived recorder, applies one subrecord,
// and flushes it. Each CLI invocation is its own process, mirroring recordAgent.
func writeLeadTimeRecord(baseDir string, p recordParams, apply func(*telemetry.Recorder)) error {
	rec, err := telemetry.NewRecorder(baseDir, p.specID)
	if err != nil {
		return fmt.Errorf("telemetry record %s: %w", p.action, err)
	}
	apply(rec)
	_ = rec.Finalize("")
	return nil
}

// recordMilestone appends a milestone event (e.g. first_vertical_slice).
func recordMilestone(baseDir string, p recordParams) error {
	if err := requireSpecID(p); err != nil {
		return err
	}
	if p.name == "" {
		return fmt.Errorf("telemetry record milestone: --name is required")
	}
	return writeLeadTimeRecord(baseDir, p, func(rec *telemetry.Recorder) { rec.RecordMilestone(p.name) })
}

// recordAction appends a reread or rerun event with its reason.
func recordAction(baseDir string, p recordParams) error {
	if err := requireSpecID(p); err != nil {
		return err
	}
	if err := telemetry.ValidateActionKind(p.kind); err != nil {
		return fmt.Errorf("telemetry record action: %w", err)
	}
	if p.target == "" {
		return fmt.Errorf("telemetry record action: --target is required")
	}
	if p.reason == "" {
		return fmt.Errorf("telemetry record action: --reason is required")
	}
	return writeLeadTimeRecord(baseDir, p, func(rec *telemetry.Recorder) { rec.RecordAction(p.kind, p.target, p.reason) })
}

// recordDefect appends a defect event; --files counts files touched by the fix.
func recordDefect(baseDir string, p recordParams) error {
	if err := requireSpecID(p); err != nil {
		return err
	}
	if p.defectID == "" {
		return fmt.Errorf("telemetry record defect: --id is required")
	}
	if p.discoveredPhase == "" {
		return fmt.Errorf("telemetry record defect: --discovered-phase is required")
	}
	if p.files < 0 {
		return fmt.Errorf("telemetry record defect: --files must not be negative")
	}
	return writeLeadTimeRecord(baseDir, p, func(rec *telemetry.Recorder) {
		rec.RecordDefect(telemetry.DefectRecord{
			ID:              p.defectID,
			DiscoveredPhase: p.discoveredPhase,
			FixedPhase:      p.fixedPhase,
			FilesTouched:    p.files,
			Escaped:         p.escaped,
			Repeat:          p.repeat,
		})
	})
}

// recordGate appends a gate applicability decision. Mandatory safety gates
// are rejected as not_applicable at the boundary so an omission can never be
// disguised as a decision.
func recordGate(baseDir string, p recordParams) error {
	if err := requireSpecID(p); err != nil {
		return err
	}
	if p.gate == "" {
		return fmt.Errorf("telemetry record gate: --gate is required")
	}
	if err := telemetry.ValidateGateApplicability(p.gate, p.applicability); err != nil {
		return fmt.Errorf("telemetry record gate: %w", err)
	}
	return writeLeadTimeRecord(baseDir, p, func(rec *telemetry.Recorder) {
		rec.RecordGate(telemetry.GateRecord{Gate: p.gate, Applicability: p.applicability, Resolved: p.resolved})
	})
}

// recordEstimate appends the planned lead-time range.
func recordEstimate(baseDir string, p recordParams) error {
	if err := requireSpecID(p); err != nil {
		return err
	}
	if err := telemetry.ValidateEstimate(p.estimateMin, p.estimateMax); err != nil {
		return fmt.Errorf("telemetry record estimate: %w", err)
	}
	return writeLeadTimeRecord(baseDir, p, func(rec *telemetry.Recorder) { rec.RecordEstimate(p.estimateMin, p.estimateMax) })
}
