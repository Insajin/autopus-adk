package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewOrchestraBrainstormCmd_Flags verifies that brainstorm cmd registers
// all expected flags: strategy, providers, timeout, and judge.
func TestNewOrchestraBrainstormCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraBrainstormCmd()
	require.NotNil(t, cmd)

	assert.NotNil(t, cmd.Flags().Lookup("strategy"), "strategy flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("providers"), "providers flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("timeout"), "timeout flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("judge"), "judge flag must exist")
}

// TestBuildBrainstormPrompt_ContainsSCAMPER verifies that the brainstorm prompt
// includes SCAMPER framework keywords for divergent thinking.
func TestBuildBrainstormPrompt_ContainsSCAMPER(t *testing.T) {
	t.Parallel()

	prompt := buildBrainstormPrompt("test feature")

	assert.Contains(t, prompt, "SCAMPER", "prompt must reference SCAMPER framework")
	assert.Contains(t, prompt, "Substitute", "prompt must include Substitute lens")
	assert.Contains(t, prompt, "Combine", "prompt must include Combine lens")
	assert.Contains(t, prompt, "Adapt", "prompt must include Adapt lens")
	assert.Contains(t, prompt, "Eliminate", "prompt must include Eliminate lens")
}

// TestBuildBrainstormPrompt_ContainsFeature verifies that the feature description
// is embedded in the generated brainstorm prompt.
func TestBuildBrainstormPrompt_ContainsFeature(t *testing.T) {
	t.Parallel()

	feature := "auto-complete for terminal commands"
	prompt := buildBrainstormPrompt(feature)

	assert.Contains(t, prompt, feature, "prompt must embed the feature description")
}

// TestBuildBrainstormPrompt_ContainsHMW verifies that the prompt includes
// HMW (How Might We) question format for reframing constraints.
func TestBuildBrainstormPrompt_ContainsHMW(t *testing.T) {
	t.Parallel()

	prompt := buildBrainstormPrompt("any feature")

	assert.Contains(t, prompt, "HMW", "prompt must reference HMW questions")
	assert.Contains(t, prompt, "How Might We", "prompt must include How Might We phrasing")
}

// TestNewOrchestraBrainstormCmd_DefaultTimeout verifies that the brainstorm
// cmd has the default timeout of 300 seconds (higher than other orchestra
// commands due to SCAMPER+HMW+ICE complexity and extended thinking).
func TestNewOrchestraBrainstormCmd_DefaultTimeout(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraBrainstormCmd()
	require.NotNil(t, cmd)

	flag := cmd.Flags().Lookup("timeout")
	require.NotNil(t, flag, "timeout flag must exist")
	assert.Equal(t, "300", flag.DefValue, "default timeout must be 300 seconds for brainstorm")
}

// TestNewOrchestraBrainstormCmd_UseAndShort verifies the command Use and Short fields.
func TestNewOrchestraBrainstormCmd_UseAndShort(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraBrainstormCmd()
	require.NotNil(t, cmd)

	assert.Contains(t, cmd.Use, "brainstorm", "command Use must contain brainstorm")
	assert.NotEmpty(t, cmd.Short, "command Short description must not be empty")
}

// TestNewOrchestraBrainstormCmd_ExactArgs verifies that the command requires
// exactly one positional argument.
func TestNewOrchestraBrainstormCmd_ExactArgs(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraBrainstormCmd()
	require.NotNil(t, cmd)

	// No args — should fail arg validation
	assert.Error(t, cmd.Args(cmd, []string{}), "command must reject zero args")
	// Two args — should fail arg validation
	assert.Error(t, cmd.Args(cmd, []string{"a", "b"}), "command must reject two args")
	// Exactly one arg — must pass
	assert.NoError(t, cmd.Args(cmd, []string{"feature description"}), "command must accept exactly one arg")
}

// TestBuildBrainstormPrompt_ContainsICE verifies that the brainstorm prompt
// instructs the judge model to apply ICE scoring.
func TestBuildBrainstormPrompt_ContainsICE(t *testing.T) {
	t.Parallel()

	prompt := buildBrainstormPrompt("search feature")

	assert.Contains(t, prompt, "ICE", "prompt must reference ICE scoring for judge integration")
}

// TestBuildBrainstormPrompt_EmptyFeature verifies that an empty feature string
// still produces a non-empty, structurally valid prompt.
func TestBuildBrainstormPrompt_EmptyFeature(t *testing.T) {
	t.Parallel()

	prompt := buildBrainstormPrompt("")

	assert.NotEmpty(t, prompt, "prompt must not be empty even for empty feature input")
	assert.Contains(t, prompt, "SCAMPER", "prompt structure must remain intact for empty input")
}

// TestNewOrchestraBrainstormCmd_RoundsFlag verifies --rounds flag exists.
func TestNewOrchestraBrainstormCmd_RoundsFlag(t *testing.T) {
	t.Parallel()
	cmd := newOrchestraBrainstormCmd()
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("rounds"), "rounds flag must exist")
}
