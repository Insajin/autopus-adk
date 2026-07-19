package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const deliveryConvergenceDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestDeliveryConvergenceDoctor_WithOpaqueScopedWorktree_ReturnsPathFreeReadyReceipt(t *testing.T) {
	fixture := newDeliveryConvergenceGitFixture(t)
	changeDeliveryConvergenceWorkingDirectory(t, fixture.worktree)

	cmd := newDeliveryConvergenceRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"delivery", "doctor",
		"--repo-scope-ref", "repo-fixture",
		"--phase", "implement",
		"--format", "json",
	})

	require.NoError(t, cmd.Execute())

	payload := decodeDeliveryConvergenceJSON(t, out.Bytes())
	assertDeliveryConvergenceEnvelope(t, payload, "auto delivery doctor")
	data := requireDeliveryConvergenceData(t, payload)
	assert.Equal(t, "codeops.delivery_doctor.v1", data["schema_version"])
	assert.Equal(t, "ready", data["status"])
	assert.Equal(t, "repo-fixture", data["repo_scope_ref"])
	assert.Equal(t, "implement", data["phase"])
	assert.Equal(t, true, data["scoped_worktree"])
	assertDeliveryConvergenceDigest(t, data["harness_digest"])
	assertDeliveryConvergenceDigest(t, data["context_digest"])
	assertDeliveryConvergenceNoKeys(t, payload,
		"repository_path", "work_dir", "resolved_root", "next_phase",
	)
	assertDeliveryConvergencePathFree(t, out.String(), fixture)
}

func TestDeliveryConvergencePrepare_WithSinglePhaseContract_ReturnsBoundedPrompt(t *testing.T) {
	fixture := newDeliveryConvergenceGitFixture(t)
	changeDeliveryConvergenceWorkingDirectory(t, fixture.worktree)

	contract := map[string]any{
		"contract_version":          "codeops.execution.v1",
		"request_id":                "req-e2e",
		"execution_id":              "exec-e2e",
		"workspace_id":              "ws-e2e",
		"runtime_instance_id":       "rt-e2e",
		"repo_connection_id":        "repo-connection-e2e",
		"repo_scope_ref":            "repo-fixture",
		"phase":                     "implement",
		"attempt":                   1,
		"lease_id":                  "lease-e2e",
		"lease_expires_at":          "2030-01-01T00:00:00Z",
		"objective":                 "Change src/value.txt and return a strict phase result.",
		"expected_result_schema":    "codeops.phase_result.v1",
		"execution_contract_digest": deliveryConvergenceDigest,
	}
	contractJSON, err := json.Marshal(contract)
	require.NoError(t, err)

	cmd := newDeliveryConvergenceRootCmd()
	var out bytes.Buffer
	cmd.SetIn(bytes.NewReader(contractJSON))
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"delivery", "prepare", "--contract-stdin", "--format", "json"})

	require.NoError(t, cmd.Execute())

	payload := decodeDeliveryConvergenceJSON(t, out.Bytes())
	assertDeliveryConvergenceEnvelope(t, payload, "auto delivery prepare")
	data := requireDeliveryConvergenceData(t, payload)
	assert.Equal(t, "codeops.delivery_preparation.v1", data["schema_version"])
	assert.Equal(t, "repo-fixture", data["repo_scope_ref"])
	assert.Equal(t, "implement", data["phase"])
	phaseContracts, ok := data["phase_contracts"].([]any)
	require.True(t, ok, "phase_contracts must be a JSON array")
	require.Len(t, phaseContracts, 1, "prepare must return one bounded phase only")
	phaseContract, ok := phaseContracts[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "implement", phaseContract["phase"])
	assert.NotEmpty(t, phaseContract["prompt"])
	assert.Equal(t, "codeops.phase_result.v1", phaseContract["expected_result_schema"])
	assertDeliveryConvergenceNoKeys(t, payload, "next_phase")
	assertDeliveryConvergencePathFree(t, out.String(), fixture)
	assert.NotContains(t, out.String(), "auto pipeline run")
	assert.NotContains(t, out.String(), "@auto go")
	assert.NotContains(t, out.String(), "@auto dev")
}

func TestDeliveryConvergenceDoctor_RejectsNonOpaqueScopeWithoutLeakingPath(t *testing.T) {
	fixture := newDeliveryConvergenceGitFixture(t)
	changeDeliveryConvergenceWorkingDirectory(t, fixture.worktree)

	for _, repoScopeRef := range []string{fixture.repository, "../escape"} {
		t.Run(strings.ReplaceAll(filepath.Base(repoScopeRef), ".", "dot"), func(t *testing.T) {
			cmd := newDeliveryConvergenceRootCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{
				"delivery", "doctor",
				"--repo-scope-ref", repoScopeRef,
				"--phase", "implement",
				"--format", "json",
			})

			require.Error(t, cmd.Execute())
			require.NotEmpty(t, out.Bytes(), "doctor must emit its failure as a structured JSON envelope")
			payload := decodeDeliveryConvergenceJSON(t, out.Bytes())
			assertDeliveryConvergenceEnvelope(t, payload, "auto delivery doctor")
			assert.Equal(t, "error", payload["status"])
			errorPayload, ok := payload["error"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "delivery_scope_invalid", errorPayload["code"])
			assertDeliveryConvergencePathFree(t, out.String(), fixture)
			assert.NotContains(t, out.String(), repoScopeRef)
		})
	}
}

type deliveryConvergenceGitFixture struct {
	repository string
	worktree   string
}

func newDeliveryConvergenceRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "auto", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(newDeliveryCmd())
	return root
}

func decodeDeliveryConvergenceJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	return payload
}

func assertDeliveryConvergenceEnvelope(t *testing.T, payload map[string]any, command string) {
	t.Helper()
	require.Equal(t, cliJSONSchemaVersion, payload["schema_version"])
	require.Equal(t, command, payload["command"])
	require.Contains(t, payload, "status")
	require.Contains(t, payload, "data")
	generatedAt, ok := payload["generated_at"].(string)
	require.True(t, ok)
	_, err := time.Parse(time.RFC3339Nano, generatedAt)
	require.NoError(t, err)
}

func newDeliveryConvergenceGitFixture(t *testing.T) deliveryConvergenceGitFixture {
	t.Helper()
	base := t.TempDir()
	repository := filepath.Join(base, "repository")
	worktree := filepath.Join(base, "worktree")
	require.NoError(t, os.MkdirAll(filepath.Join(repository, ".autopus", "project"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repository, ".autopus", "context"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repository, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "AGENTS.md"), []byte("# Fixture\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "autopus.yaml"), []byte("project: fixture\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repository, ".autopus", "project", "workspace.md"), []byte("# Workspace\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repository, ".autopus", "context", "constraints.yaml"), []byte("constraints: []\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "src", "value.txt"), []byte("old\n"), 0o644))

	runDeliveryConvergenceGit(t, repository, "init")
	runDeliveryConvergenceGit(t, repository, "config", "user.email", "fixture@example.invalid")
	runDeliveryConvergenceGit(t, repository, "config", "user.name", "Autopus Fixture")
	runDeliveryConvergenceGit(t, repository, "add", ".")
	runDeliveryConvergenceGit(t, repository, "commit", "-m", "fixture")
	runDeliveryConvergenceGit(t, repository, "worktree", "add", "-b", "agent/convergence", worktree)

	return deliveryConvergenceGitFixture{repository: repository, worktree: worktree}
}

func runDeliveryConvergenceGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	gitArgs := append([]string{"-C", directory}, args...)
	command := exec.Command("git", gitArgs...)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed:\n%s", strings.Join(args, " "), output)
	return string(output)
}

func changeDeliveryConvergenceWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(directory))
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
}

func requireDeliveryConvergenceData(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	require.True(t, ok, "data must be a JSON object")
	return data
}

func assertDeliveryConvergenceDigest(t *testing.T, value any) {
	t.Helper()
	digest, ok := value.(string)
	require.True(t, ok)
	assert.Regexp(t, regexp.MustCompile(`^sha256:[0-9a-f]{64}$`), digest)
}

func assertDeliveryConvergenceNoKeys(t *testing.T, value any, forbiddenKeys ...string) {
	t.Helper()
	forbidden := make(map[string]struct{}, len(forbiddenKeys))
	for _, key := range forbiddenKeys {
		forbidden[key] = struct{}{}
	}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, nested := range typed {
				_, found := forbidden[key]
				assert.Falsef(t, found, "forbidden key %q must not be emitted", key)
				walk(nested)
			}
		case []any:
			for _, nested := range typed {
				walk(nested)
			}
		}
	}
	walk(value)
}

func assertDeliveryConvergencePathFree(t *testing.T, output string, fixture deliveryConvergenceGitFixture) {
	t.Helper()
	assert.NotContains(t, output, fixture.repository)
	assert.NotContains(t, output, fixture.worktree)
	assert.NotContains(t, output, filepath.Dir(fixture.repository))
}
