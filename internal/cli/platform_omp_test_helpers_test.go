package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

type ompCLIJSONEnvelope struct {
	Status jsonEnvelopeStatus `json:"status"`
	Data   json.RawMessage    `json:"data"`
}

type ompCLIFakeRunner struct {
	catalog []byte
	fail    bool
	calls   []string
}

func (runner *ompCLIFakeRunner) Run(_ context.Context, executable string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, executable+" "+strings.Join(args, " "))
	if runner.fail {
		return nil, errors.New("fixture unavailable")
	}
	switch strings.Join(args, " ") {
	case "--version":
		return []byte("omp/17.1.8\n"), nil
	case "models --json --no-extensions":
		return append([]byte(nil), runner.catalog...), nil
	case "config list --json":
		return []byte(`{"compaction":{},"memory":{}}`), nil
	default:
		if len(args) >= 4 && args[len(args)-1] == "--json" && args[len(args)-3] == "config" && args[len(args)-2] == "get" {
			return []byte(`{}`), nil
		}
		return nil, fmt.Errorf("unexpected fixture args: %s", strings.Join(args, " "))
	}
}

func writeSelectedOMPProfile(t *testing.T) (string, *ompCLIFakeRunner, config.RoleModelProfileConf) {
	t.Helper()
	root := t.TempDir()
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	catalog := mustOMPCatalog(t, runner.catalog)
	proposal, err := buildOMPProfileProposal("balanced", catalog)
	require.NoError(t, err)
	cfg := config.DefaultFullConfig("omp-selected")
	cfg.Platforms = []string{"omp"}
	cfg.RoleModelPolicy = config.RoleModelPolicyConf{
		Version: config.RoleModelPolicyVersionV1, Profile: "balanced",
		Profiles: map[string]config.RoleModelProfileConf{"balanced": proposal.profile},
	}
	require.NoError(t, config.Save(root, cfg))
	return root, runner, proposal.profile
}

func writeStaleOMPModelReceipt(t *testing.T, root string) {
	t.Helper()
	hash := "sha256:" + strings.Repeat("a", 64)
	receipt := omp.OMPModelResolutionReceipt{
		OMPVersion: "omp/17.1.8", CatalogFingerprint: hash, Profile: "balanced", ConfigSource: "overlay",
		Activation: omp.OMPModelActivationReceipt{Argv: []string{"omp"}, ConfigHash: hash, ReadbackHash: hash},
		Roles: []omp.OMPModelRoleReceipt{{
			Agent: "executor", Profile: "balanced", ConfigSource: "overlay", RequestedRole: "task", EffectiveRole: "task",
			Capability: config.CapabilityCodingToolUse, Provider: "openai", Model: "beta-coder",
			Selector: "openai/beta-coder", Thinking: "high",
			FamilyDiversity: omp.OMPModelFamilyDiversityReceipt{Status: "not_applicable"}, SafetySource: "user_effective",
		}},
		GeneratedAt: time.Now().UTC(),
	}
	_, err := omp.WriteOMPModelResolutionReceipt(omp.OMPModelReceiptWriteInput{WorkspaceRoot: root, Receipt: receipt})
	require.NoError(t, err)
}

func mustOMPCatalog(t *testing.T, data []byte) omp.OMPModelCatalog {
	t.Helper()
	catalog, reason := omp.NormalizeOMPModelCatalog(data, 1<<20)
	require.Equal(t, "catalog_ready", reason)
	return catalog
}

func ompCLIReadyCatalogJSON() []byte {
	return []byte(`{"models":[
		{"provider":"anthropic","id":"alpha-reasoner","family":"anthropic","capabilities":["deep_reasoning","independent_dissent"],"thinking":["high","xhigh"],"auth_enabled":true,"keyless":false,"disabled":false},
		{"provider":"openai","id":"beta-coder","family":"openai","capabilities":["coding_tool_use","fast_validation","independent_dissent","deterministic_transform"],"thinking":["low","medium","high"],"auth_enabled":true,"keyless":false,"disabled":false},
		{"provider":"google","id":"gamma-vision","family":"google","capabilities":["vision_design"],"thinking":["high"],"auth_enabled":true,"keyless":false,"disabled":false},
		{"provider":"openai","id":"disabled-coder","family":"openai","capabilities":["coding_tool_use"],"thinking":["high"],"auth_enabled":true,"keyless":false,"disabled":true},
		{"provider":"openai","id":"unauthorized-coder","family":"openai","capabilities":["coding_tool_use"],"thinking":["high"],"auth_enabled":false,"keyless":false,"disabled":false}
	]}`)
}

func executeOMPSubcommand(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute(), out.String())
	return out.String()
}

func mustFileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func assertCommandHelpContains(t *testing.T, args []string, expected ...string) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	for _, value := range expected {
		assert.Contains(t, out.String(), value)
	}
}
