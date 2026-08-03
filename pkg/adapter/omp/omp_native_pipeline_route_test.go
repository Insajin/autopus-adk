package omp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestOMPNativePipelineRoute_GeneratesSingleShellFreeAutoOwner(t *testing.T) {
	root := t.TempDir()
	cfg := optedInOMPContextBridgeConfig()
	require.NoError(t, config.Save(root, cfg))
	_, err := NewWithRoot(root).Generate(context.Background(), cfg)
	require.NoError(t, err)

	path := filepath.Join(root, ".omp", "extensions", "autopus-pipeline.ts")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	source := string(body)
	require.Contains(t, source, `registerCommand("auto"`)
	require.Equal(t, 1, countLiteral(source, `registerCommand("auto"`))
	require.Contains(t, source, `process.env.AUTOPUS_OMP_MANAGED_INNER === "1"`)
	require.Contains(t, source, `["pipeline", "run", specID, "--platform", "omp"]`)
	require.Contains(t, source, "shell: false")
	require.NotContains(t, source, "promotion")
	require.NotContains(t, source, "history_rows")
	require.NotContains(t, source, "capabilities")
	require.NotContains(t, source, "exec(")
	require.NotContains(t, source, "sh -c")
	require.NoError(t, NewWithRoot(root).Clean(context.Background()))
	require.NoFileExists(t, path)
}

func TestOMPNativePipelineRoute_IdentityIsGeneratedSourceOfTruth(t *testing.T) {
	identity := ExpectedOMPNativePipelineRouteSourceIdentity()
	require.Equal(t, ".omp/extensions/autopus-pipeline.ts", identity.TargetPath)
	require.Regexp(t, `^[0-9a-f]{64}$`, identity.SHA256)
	require.Positive(t, identity.Size)

	bridge := ExpectedOMPContextBridgeSourceIdentity()
	require.Equal(t, ".omp/extensions/autopus-context.ts", bridge.TargetPath)
	require.Regexp(t, `^[0-9a-f]{64}$`, bridge.SHA256)
	require.Positive(t, bridge.Size)
}

func TestOMPNativePipelineRoute_RegistersOnlyOutsideManagedInnerRuntime(t *testing.T) {
	bun, err := exec.LookPath("bun")
	if err != nil {
		t.Skip("bun is required to execute the generated TypeScript route contract")
	}
	harness := `
let registrations = 0;
const api = { registerCommand() { registrations++; } };
delete process.env.AUTOPUS_OMP_MANAGED_INNER;
autopusPipelineRoute(api as any);
if (registrations !== 1) throw new Error("outer route was not registered exactly once");
registrations = 0;
process.env.AUTOPUS_OMP_MANAGED_INNER = "1";
autopusPipelineRoute(api as any);
if (registrations !== 0) throw new Error("managed inner route registered recursively");
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bun, "run", "-")
	cmd.Stdin = strings.NewReader(ompNativePipelineRouteSource + harness)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func countLiteral(source, literal string) int {
	count := 0
	for {
		index := len(source)
		for i := 0; i+len(literal) <= len(source); i++ {
			if source[i:i+len(literal)] == literal {
				index = i
				break
			}
		}
		if index == len(source) {
			return count
		}
		count++
		source = source[index+len(literal):]
	}
}
