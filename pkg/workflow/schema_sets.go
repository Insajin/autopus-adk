package workflow

// This file holds the per-phase accessor maps that complement schema.go. They
// are split out to keep schema.go under the 300-line source limit.

// ModelSet returns agent model identifiers keyed by phase-id.
func (s Schema) ModelSet() map[string]string {
	m := make(map[string]string, len(s.Phases))
	for _, p := range s.Phases {
		m[p.ID] = p.Model
	}
	return m
}

// EffortSet returns effort tiers keyed by phase-id.
func (s Schema) EffortSet() map[string]string {
	m := make(map[string]string, len(s.Phases))
	for _, p := range s.Phases {
		m[p.ID] = p.Effort
	}
	return m
}

// DepthSet returns the bounded DepthProfile for each phase keyed by phase-id.
func (s Schema) DepthSet() map[string]DepthProfile {
	m := make(map[string]DepthProfile, len(s.Phases))
	for _, p := range s.Phases {
		m[p.ID] = DepthProfile{
			VerifyVotes: p.VerifyVotes,
			FanOutCap:   p.FanOutCap,
			Synthesis:   p.Synthesis,
			Retry:       p.Retry,
		}
	}
	return m
}
