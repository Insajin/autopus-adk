package omp

import (
	"github.com/insajin/autopus-adk/pkg/config"
)

// ompProjectionRoleSpec is one agent-owned OMP model role in canonical order.
type ompProjectionRoleSpec struct {
	agent      string
	role       string
	capability string
}

// ompProjectionRoleSpecs mirrors the policy contract: exactly one
// autopus_<agent> role per canonical agent, ordered like CanonicalAgentNames.
var ompProjectionRoleSpecs = func() []ompProjectionRoleSpec {
	agents := config.CanonicalAgentNames()
	roles := config.OMPAgentRoleMapping()
	capabilities := config.OMPAgentCapabilityMapping()
	specs := make([]ompProjectionRoleSpec, 0, len(agents))
	for _, agent := range agents {
		specs = append(specs, ompProjectionRoleSpec{
			agent: agent, role: roles[agent], capability: capabilities[agent],
		})
	}
	return specs
}()
