package workflow

// This file owns the quality-binding DATA types and the pure render overlay
// (REQ-015). pkg/workflow does NOT compute the per-phase quality values; the CLI
// dispatch layer (a different package) computes the complete binding and injects
// it here. OverlayPhases is a pure function over schema + injected binding.

// PhaseBinding is the complete per-phase quality binding the dispatch layer
// computes for a single phase. All fields are kept (no omitempty) so the
// serialized JSON is deterministic regardless of zero values.
type PhaseBinding struct {
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	VerifyVotes int    `json:"verify_votes"`
	FanOutCap   int    `json:"fan_out_cap"`
	Synthesis   bool   `json:"synthesis"`
}

// QualityBinding maps phase-id to its computed PhaseBinding.
type QualityBinding struct {
	Phases map[string]PhaseBinding `json:"phases"`
}

// OverlayPhases builds the rendered per-phase view in schema order. Each phase
// starts from its schema baseline (the PhaseDef values). When b is non-nil and
// carries an entry for a phase id, that phase's quality fields are replaced
// wholesale with the binding's values — the dispatch computes the complete
// per-phase binding, so this is a replace, not a merge. Phases without a binding
// entry keep their baseline (deterministic gate phases stay empty).
func OverlayPhases(s Schema, b *QualityBinding) []RenderedPhase {
	out := make([]RenderedPhase, 0, len(s.Phases))
	for _, p := range s.Phases {
		rp := RenderedPhase{
			ID:          p.ID,
			Model:       p.Model,
			Effort:      p.Effort,
			VerifyVotes: p.VerifyVotes,
			FanOutCap:   p.FanOutCap,
			Synthesis:   p.Synthesis,
		}
		if b != nil {
			if pb, ok := b.Phases[p.ID]; ok {
				rp.Model = pb.Model
				rp.Effort = pb.Effort
				rp.VerifyVotes = pb.VerifyVotes
				rp.FanOutCap = pb.FanOutCap
				rp.Synthesis = pb.Synthesis
			}
		}
		out = append(out, rp)
	}
	return out
}
