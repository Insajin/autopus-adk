// Package orchestra tests provider subprocess argv construction correctness.
// SPEC-ORCH-021 REQ-014: subprocess argv delivers the prompt in the form the
// provider CLI actually accepts (gemini --print value slot, codex exec +
// --output-schema), never a form that fails argument parsing before the prompt
// is read.
package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// indexOf returns the first index of target in args, or -1 if absent.
func indexOf(args []string, target string) int {
	for i, a := range args {
		if a == target {
			return i
		}
	}
	return -1
}

// TestBuildSubprocessArgs_GeminiPrintValueSlot covers S15: the gemini (`agy`)
// subprocess argv passes the prompt as the VALUE of --print (not a bare
// valueless --print that exits with `flag needs an argument: -print`).
func TestBuildSubprocessArgs_GeminiPrintValueSlot(t *testing.T) {
	cfg := ProviderConfig{Binary: "agy", Args: []string{"--print", ""}, PromptViaArgs: true}
	req := ProviderRequest{Config: cfg, Prompt: "PROMPTXYZ"}

	args := buildSubprocessArgs(req)

	printIdx := indexOf(args, "--print")
	require.GreaterOrEqual(t, printIdx, 0, "argv must contain --print")
	// The prompt must occupy the slot immediately after --print (its value).
	require.Less(t, printIdx+1, len(args), "--print must not be the last element (valueless)")
	assert.Equal(t, "PROMPTXYZ", args[printIdx+1], "prompt must be the --print value")

	// There must be NO valueless --print: --print is neither the last element
	// nor immediately followed by another flag.
	next := args[printIdx+1]
	assert.NotEmpty(t, next, "--print value slot must not be empty")
	assert.NotEqual(t, "--", next[:1], "--print must not be followed by another flag")
}

// TestBuildSubprocessArgs_CodexExecSchema covers S16: the codex subprocess argv
// begins with `exec` and contains --output-schema immediately followed by the
// schema path when a schema path is supplied for a structured role.
func TestBuildSubprocessArgs_CodexExecSchema(t *testing.T) {
	cfg := ProviderConfig{
		Binary:     "codex",
		Args:       []string{"exec", "--sandbox", "workspace-write", "-m", "gpt-5.5"},
		SchemaFlag: "--output-schema",
	}
	req := ProviderRequest{Config: cfg, Prompt: "P", SchemaPath: "/tmp/s.json", Role: "reviewer"}

	args := buildSubprocessArgs(req)

	require.NotEmpty(t, args)
	assert.Equal(t, "exec", args[0], "codex subprocess argv must begin with exec")

	schemaIdx := indexOf(args, "--output-schema")
	require.GreaterOrEqual(t, schemaIdx, 0, "argv must contain --output-schema")
	require.Less(t, schemaIdx+1, len(args), "--output-schema must carry a value")
	assert.Equal(t, "/tmp/s.json", args[schemaIdx+1], "schema path must follow --output-schema")
}
