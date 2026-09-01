// Package regen synthesizes project-local QA Journey Packs from detected
// surface signals and computes a deterministic diff against existing packs.
//
// Unit 1 provides the foundational pure functions: surface analysis, structural
// pack synthesis mirroring the scaffold starter templates, an AI-authority guard
// applied across every surface, deterministic added/changed/removed diffing,
// redaction of all serialized text, and post-approval persistence. Approval
// gating itself lives in Unit 2; apply.go here is a pure write function.
package regen

import "github.com/insajin/autopus-adk/pkg/qa/journey"

// Diff is the deterministic classification of synthesized packs against the
// existing on-disk packs, keyed by journey ID.
//
// There is deliberately no "removed" category. ApplyPacks only ever writes
// accepted packs and never deletes a file, so no diff category may be read as
// a deletion proposal. Unmatched holds the existing packs that this
// re-synthesis did not produce; approval leaves them exactly as they are on
// disk, as it does every other pack. The name states the observation that
// produced the row, not a promise about the file.
type Diff struct {
	AddedCount     int         `json:"added_count"`
	ChangedCount   int         `json:"changed_count"`
	UnmatchedCount int         `json:"unmatched_count"`
	Added          []DiffEntry `json:"added"`
	Changed        []DiffEntry `json:"changed"`
	Unmatched      []DiffEntry `json:"unmatched"`
}

// DiffEntry is one journey-level entry. Category is added|changed|unmatched.
type DiffEntry struct {
	JourneyID     string        `json:"journey_id"`
	Category      string        `json:"category"`
	ChangedFields []FieldChange `json:"changed_fields,omitempty"`
}

// FieldChange records a single field that differs between the existing and
// synthesized pack. Before/After are rendered string forms.
type FieldChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// SynthesizedPack wraps a constructed journey.Pack plus the result of running
// it through journey.Validate and the surface-agnostic AI-authority guard.
// Excluded reports whether the pack was dropped from the proposal, with Reason
// carrying the validation/guard reason code when excluded.
type SynthesizedPack struct {
	Pack     journey.Pack `json:"pack"`
	Surface  string       `json:"surface"`
	Excluded bool         `json:"excluded"`
	Reason   string       `json:"reason,omitempty"`
}

// RegenResult is the full Unit 1 output: present surfaces, the synthesized
// packs (including excluded ones for transparency), and the deterministic diff
// computed over only the accepted packs.
type RegenResult struct {
	Surfaces    []string          `json:"surfaces"`
	Synthesized []SynthesizedPack `json:"synthesized"`
	Diff        Diff              `json:"diff"`
}

// AcceptedPacks returns the journey.Pack values that survived validation and
// the AI-authority guard, in synthesis order.
func (r RegenResult) AcceptedPacks() []journey.Pack {
	packs := make([]journey.Pack, 0, len(r.Synthesized))
	for _, sp := range r.Synthesized {
		if sp.Excluded {
			continue
		}
		packs = append(packs, sp.Pack)
	}
	return packs
}
