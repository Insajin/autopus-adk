package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

// ompRoleModelPolicyIndependenceNote states the one fact operators misread:
// the OMP role model selection is not quality.default. Selecting an OMP
// profile never rewrites quality.default, and a global balanced quality value
// says nothing about which OMP profile is active.
const ompRoleModelPolicyIndependenceNote = "omp role model selection is independent of quality.default; " +
	"change it with: auto platform omp profile apply <name>"

const (
	ompRoleModelPolicySourceBuiltin  = "builtin"
	ompRoleModelPolicySourceExplicit = "explicit_profile"
	ompRoleModelPolicySourceUnknown  = "unresolved"
)

// writeOMPRoleModelPolicyStatus reports the OMP role model selection as its own
// block. It only reads the loaded config: no adapter is invoked and no value is
// mutated.
func writeOMPRoleModelPolicyStatus(out io.Writer, policy config.RoleModelPolicyConf) {
	enabled := strings.TrimSpace(policy.Profile) != ""
	fmt.Fprintf(out, "omp.role_model_policy.enabled = %t\n", enabled)
	if !enabled {
		fmt.Fprintf(out, "omp.role_model_policy.note = %s\n", ompRoleModelPolicyIndependenceNote)
		return
	}
	fmt.Fprintf(out, "omp.role_model_policy.profile = %s\n", policy.Profile)
	fmt.Fprintf(
		out, "omp.role_model_policy.family = %s\n",
		ompProfileDisplayValue(policy.Family, "default"),
	)
	mode := policy.ConfigMode
	if profile, explicit := policy.Profiles[policy.Profile]; explicit {
		mode = profile.ConfigMode
	}
	fmt.Fprintf(
		out, "omp.role_model_policy.config_mode = %s\n",
		ompProfileDisplayValue(mode, config.RoleModelConfigModeOverlay),
	)
	fmt.Fprintf(out, "omp.role_model_policy.source = %s\n", ompRoleModelPolicySource(policy))
	fmt.Fprintf(
		out, "omp.role_model_policy.agent_overrides = %s\n",
		ompProfileDisplayValue(strings.Join(ompRoleModelPolicyOverriddenAgents(policy), ", "), "none"),
	)
	fmt.Fprintf(out, "omp.role_model_policy.note = %s\n", ompRoleModelPolicyIndependenceNote)
}

// ompRoleModelPolicySource distinguishes a quality-derived built-in selection
// from an explicit profile definition, which always wins over the built-in.
func ompRoleModelPolicySource(policy config.RoleModelPolicyConf) string {
	if _, explicit := policy.Profiles[policy.Profile]; explicit {
		return ompRoleModelPolicySourceExplicit
	}
	if config.IsBuiltinRoleModelProfileName(policy.Profile) {
		return ompRoleModelPolicySourceBuiltin
	}
	return ompRoleModelPolicySourceUnknown
}

func ompRoleModelPolicyOverriddenAgents(policy config.RoleModelPolicyConf) []string {
	agents := make([]string, 0, len(policy.Agents))
	for agent, override := range policy.Agents {
		if len(override.Candidates) > 0 {
			agents = append(agents, agent)
		}
	}
	sort.Strings(agents)
	return agents
}
