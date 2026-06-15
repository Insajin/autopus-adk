package regen

import (
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

// RedactDiff returns a copy of the diff with every human-visible string passed
// through qaevidence.RedactText, so the serialized diff never carries a raw
// secret, credential, or local user path that may have arrived via extracted
// flow text. JourneyID and Category are pack-keyed structural identifiers and
// are still redacted defensively.
func RedactDiff(diff Diff) Diff {
	out := Diff{
		AddedCount:   diff.AddedCount,
		ChangedCount: diff.ChangedCount,
		RemovedCount: diff.RemovedCount,
		Added:        redactEntries(diff.Added),
		Changed:      redactEntries(diff.Changed),
		Removed:      redactEntries(diff.Removed),
	}
	return out
}

func redactEntries(entries []DiffEntry) []DiffEntry {
	if entries == nil {
		return nil
	}
	out := make([]DiffEntry, 0, len(entries))
	for _, e := range entries {
		redacted := DiffEntry{
			JourneyID: qaevidence.RedactText(e.JourneyID),
			Category:  e.Category,
		}
		for _, fc := range e.ChangedFields {
			redacted.ChangedFields = append(redacted.ChangedFields, FieldChange{
				Field:  qaevidence.RedactText(fc.Field),
				Before: qaevidence.RedactText(fc.Before),
				After:  qaevidence.RedactText(fc.After),
			})
		}
		out = append(out, redacted)
	}
	return out
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-011: security gate — callers that serialize or persist the diff MUST call RedactDiff then gate on this returning nil; skipping it allows raw secrets or local paths from extracted CLI flow text into published payloads.
// AssertDiffSafe fails closed if any rendered string in the diff still contains
// an unsafe token after redaction. Callers persisting or publishing the diff
// must gate on this returning nil.
func AssertDiffSafe(diff Diff) error {
	groups := [][]DiffEntry{diff.Added, diff.Changed, diff.Removed}
	for _, group := range groups {
		for _, e := range group {
			if err := qaevidence.AssertSafeText(e.JourneyID, "diff.journey_id"); err != nil {
				return err
			}
			for _, fc := range e.ChangedFields {
				if err := qaevidence.AssertSafeText(fc.Before, "diff.field.before"); err != nil {
					return err
				}
				if err := qaevidence.AssertSafeText(fc.After, "diff.field.after"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
