// Package workflow holds the shared schema contract for the harness workflow
// route. The manifest (route_a.schema.json) is the machine-authoritative source
// for phase-id, retry, budget, and result-type sets; this package parses it.
//
// This package MUST NOT import pkg/content or internal/cli.
package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// PhaseDef is a single workflow phase as declared in route_a.schema.json.
// ResultType carries the verdict_source for the deterministic gate phase
// (e.g. "exit_code"); it is "" for non-gate phases.
type PhaseDef struct {
	ID          string `json:"id"`
	Retry       int    `json:"retry"`
	Budget      int    `json:"budget"`
	ResultType  string `json:"result_type"`
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	VerifyVotes int    `json:"verify_votes"`
	FanOutCap   int    `json:"fan_out_cap"`
	Synthesis   bool   `json:"synthesis"`
}

// Schema is the parsed manifest with phases in execution order.
type Schema struct {
	Phases []PhaseDef `json:"phases"`
}

// rawPhase tolerates either "result_type" or "verdict_source" as the
// result-type field so the JSON manifest can use the more descriptive
// "verdict_source" key for the gate phase.
type rawPhase struct {
	ID            string `json:"id"`
	Retry         int    `json:"retry"`
	Budget        int    `json:"budget"`
	ResultType    string `json:"result_type"`
	VerdictSource string `json:"verdict_source"`
	Model         string `json:"model"`
	Effort        string `json:"effort"`
	VerifyVotes   int    `json:"verify_votes"`
	FanOutCap     int    `json:"fan_out_cap"`
	Synthesis     bool   `json:"synthesis"`
}

type rawSchema struct {
	Phases []rawPhase `json:"phases"`
}

// ParseSchema unmarshals route_a.schema.json bytes into a Schema, preserving
// phase array order as execution order.
func ParseSchema(data []byte) (Schema, error) {
	var raw rawSchema
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return Schema{}, fmt.Errorf("parse workflow schema: %w", err)
	}
	if len(raw.Phases) == 0 {
		return Schema{}, fmt.Errorf("parse workflow schema: no phases declared")
	}
	s := Schema{Phases: make([]PhaseDef, 0, len(raw.Phases))}
	for i, rp := range raw.Phases {
		if rp.ID == "" {
			return Schema{}, fmt.Errorf("parse workflow schema: phase %d has empty id", i)
		}
		// Defense-in-depth (Q-SEC-01): phase ids are interpolated into the
		// generated workflow JS, so reject any id outside a safe token charset
		// (alnum/underscore/hyphen) before it can break or inject into that
		// trust surface. Fail closed at the SoT parse boundary.
		if !isSafePhaseID(rp.ID) {
			return Schema{}, fmt.Errorf("parse workflow schema: phase %d has unsafe id %q (allowed: [A-Za-z0-9_-])", i, rp.ID)
		}
		// JS-injection trust boundary (REQ-011, S6): model and effort strings
		// are interpolated into generated workflow JS, so reject any value
		// outside the closed whitelist before it reaches that surface.
		if !isSafeAgentModel(rp.Model) {
			return Schema{}, fmt.Errorf("parse workflow schema: phase %d has unsafe model %q (not whitelisted)", i, rp.Model)
		}
		if !isSafeEffort(rp.Effort) {
			return Schema{}, fmt.Errorf("parse workflow schema: phase %d has unsafe effort %q", i, rp.Effort)
		}
		// Bounded depth (REQ-004, S4): verify votes, fan-out, and retry are hard
		// capped; values above the ceiling are rejected, never silently clamped.
		if err := validateDepthCaps(rp.ID, rp.VerifyVotes, rp.FanOutCap, rp.Retry); err != nil {
			return Schema{}, err
		}
		rt := rp.ResultType
		if rt == "" {
			rt = rp.VerdictSource
		}
		s.Phases = append(s.Phases, PhaseDef{
			ID:          rp.ID,
			Retry:       rp.Retry,
			Budget:      rp.Budget,
			ResultType:  rt,
			Model:       rp.Model,
			Effort:      rp.Effort,
			VerifyVotes: rp.VerifyVotes,
			FanOutCap:   rp.FanOutCap,
			Synthesis:   rp.Synthesis,
		})
	}
	return s, nil
}

// isSafePhaseID reports whether id contains only token-safe characters
// (ASCII letters, digits, underscore, hyphen). This blocks quotes, newlines,
// backslashes, and braces that could break the generated workflow JS.
func isSafePhaseID(id string) bool {
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// LoadSchema reads and parses route_a.schema.json from path.
func LoadSchema(path string) (Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Schema{}, fmt.Errorf("read workflow schema %q: %w", path, err)
	}
	return ParseSchema(data)
}

// PhaseIDs returns the ordered phase-ids.
func (s Schema) PhaseIDs() []string {
	ids := make([]string, len(s.Phases))
	for i, p := range s.Phases {
		ids[i] = p.ID
	}
	return ids
}

// RetrySet returns retry counts keyed by phase-id.
func (s Schema) RetrySet() map[string]int {
	m := make(map[string]int, len(s.Phases))
	for _, p := range s.Phases {
		m[p.ID] = p.Retry
	}
	return m
}

// BudgetSet returns budgets keyed by phase-id.
func (s Schema) BudgetSet() map[string]int {
	m := make(map[string]int, len(s.Phases))
	for _, p := range s.Phases {
		m[p.ID] = p.Budget
	}
	return m
}

// ResultTypeSet returns result-types keyed by phase-id.
func (s Schema) ResultTypeSet() map[string]string {
	m := make(map[string]string, len(s.Phases))
	for _, p := range s.Phases {
		m[p.ID] = p.ResultType
	}
	return m
}
