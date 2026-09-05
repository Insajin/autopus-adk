package gates

import (
	"fmt"
	"strings"
	"time"
)

// RecordOptions describes one gate run to be captured as evidence.
type RecordOptions struct {
	SpecID      string
	Gate        GateID
	Status      EvidenceStatus
	Partial     bool // the run did not cover its full scope
	InputGlobs  []string
	DynamicDeps []string
	Command     string
	Now         time.Time
}

// BuildEvidence resolves the input closure under root and returns the
// evidence receipt. Every input glob must match at least one file and every
// dynamic dependency must exist; anything else is an invalid recording.
func BuildEvidence(root string, opts RecordOptions) (EvidenceReceipt, error) {
	if _, ok := LookupGate(opts.Gate); !ok {
		return EvidenceReceipt{}, fmt.Errorf("unknown gate %q", opts.Gate)
	}
	if !ValidStatus(opts.Status) {
		return EvidenceReceipt{}, fmt.Errorf("invalid status %q: expected pass, fail, or partial", opts.Status)
	}
	globs := normalizePaths(opts.InputGlobs)
	if len(globs) == 0 {
		return EvidenceReceipt{}, fmt.Errorf("at least one input glob is required")
	}
	for _, glob := range globs {
		matched, err := expandGlob(root, glob)
		if err != nil {
			return EvidenceReceipt{}, err
		}
		if len(matched) == 0 {
			return EvidenceReceipt{}, fmt.Errorf("input glob %q matched no files under %s", glob, root)
		}
	}
	inputs, deps, err := ResolveClosure(root, globs, opts.DynamicDeps)
	if err != nil {
		return EvidenceReceipt{}, err
	}
	return EvidenceReceipt{
		Schema:             EvidenceSchema,
		SpecID:             opts.SpecID,
		Gate:               opts.Gate,
		Status:             opts.Status,
		Complete:           !opts.Partial && opts.Status != StatusPartial,
		InputClosureSHA256: ClosureSHA256(inputs, deps),
		Inputs:             inputs,
		DynamicDeps:        deps,
		InputGlobs:         globs,
		Command:            strings.TrimSpace(opts.Command),
		ObservedAt:         opts.Now.UTC().Format(time.RFC3339),
	}, nil
}
