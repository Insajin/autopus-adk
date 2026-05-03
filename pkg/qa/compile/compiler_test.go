package compile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromProjectCompilesTaggedAcceptance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-EXAMPLE-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	body := "Given a deterministic check\n```qamesh-check\nadapter: go-test\ncommand: [\"go\", \"test\", \"./...\"]\nexpected:\n  exit_code: 0\nacceptance_refs: [\"AC-EXAMPLE-001\"]\n```\n"
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "acceptance.md"), []byte(body), 0o644))

	candidates := FromProject(dir)
	require.Len(t, candidates, 1)
	assert.Equal(t, "go-test", candidates[0].Adapter)
	assert.Equal(t, "compiled", candidates[0].Source)
	assert.Equal(t, []string{"AC-EXAMPLE-001"}, candidates[0].AcceptanceRefs)
	assert.Equal(t, 0, candidates[0].OracleThresholds["exit_code"])
}

func TestFromProjectRejectsUnsafeTaggedAcceptance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-EXAMPLE-002")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	body := "Given an unsafe check\n```qamesh-check\nadapter: custom-command\ncommand: [\"sh\", \"-c\", \"rm -rf /\"]\nexpected:\n  exit_code: 0\n```\n"
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "acceptance.md"), []byte(body), 0o644))

	candidates := FromProject(dir)
	require.Len(t, candidates, 1)
	assert.True(t, candidates[0].ManualOrDeferred)
	assert.Equal(t, "qa_compiler_command_unsafe", candidates[0].ErrorCode)
}

func TestFromProjectRejectsUnsafeTaggedAcceptanceArtifactsAndEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		code string
	}{
		{
			name: "absolute artifact",
			body: "```qamesh-check\nadapter: go-test\ncommand: [\"go\", \"test\", \"./...\"]\nartifacts: [\"/tmp/raw-secret.log\"]\n```\n",
			code: "qa_compiler_artifact_path_invalid",
		},
		{
			name: "env ref",
			body: "```qamesh-check\nadapter: go-test\ncommand: [\"go\", \"test\", \"./...\"]\nenv: [\"SECRET_TOKEN\"]\n```\n",
			code: "qa_compiler_env_not_allowlisted",
		},
		{
			name: "bad env allowlist",
			body: "```qamesh-check\nadapter: go-test\ncommand: [\"go\", \"test\", \"./...\"]\nenv_allowlist: [\"SECRET_TOKEN=value\"]\n```\n",
			code: "qa_compiler_env_not_allowlisted",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-EXAMPLE-UNSAFE")
			require.NoError(t, os.MkdirAll(specDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(specDir, "acceptance.md"), []byte(tt.body), 0o644))

			candidates := FromProject(dir)

			require.Len(t, candidates, 1)
			assert.True(t, candidates[0].ManualOrDeferred)
			assert.Equal(t, tt.code, candidates[0].ErrorCode)
		})
	}
}

func TestFromProjectCompilesScenarioCommands(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	projectDir := filepath.Join(dir, ".autopus", "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	body := "- command: `go test ./...`\n- command: `pytest`\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "scenarios.md"), []byte(body), 0o644))

	candidates := FromProject(dir)
	require.Len(t, candidates, 2)
	assert.Equal(t, "go-test", candidates[0].Adapter)
	assert.Equal(t, "pytest", candidates[1].Adapter)
	assert.Equal(t, "compiled", candidates[0].Source)
}

func TestFromProjectMarksInvalidCheckAsDeferred(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-EXAMPLE-003")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	body := "```qamesh-check\nadapter: [not-valid]\n```\n"
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "acceptance.md"), []byte(body), 0o644))

	candidates := FromProject(dir)
	require.Len(t, candidates, 1)
	assert.True(t, candidates[0].ManualOrDeferred)
	assert.Equal(t, "qa_compiler_parse_invalid", candidates[0].ErrorCode)
}

func TestFromProjectDefersExternalAdapters(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-EXAMPLE-004")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	body := "```qamesh-check\nadapter: maestro\ncommand: [\"maestro\", \"test\", \"flow.yaml\"]\n```\n"
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "acceptance.md"), []byte(body), 0o644))

	candidates := FromProject(dir)

	require.Len(t, candidates, 1)
	assert.True(t, candidates[0].ManualOrDeferred)
	assert.Equal(t, "qa_compiler_deferred_to_SPEC-QAMESH-003", candidates[0].ErrorCode)
}

func TestFromProjectDefersAIAndProductionSessionChecks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-EXAMPLE-005")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	body := "```qamesh-check\nadapter: go-test\ncommand: [\"go\", \"test\", \"./...\"]\npass_fail_authority: ai\nsource: production_session\n```\n"
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "acceptance.md"), []byte(body), 0o644))

	candidates := FromProject(dir)

	require.Len(t, candidates, 1)
	assert.Equal(t, "go-test", candidates[0].Adapter)
	assert.Equal(t, "ai", candidates[0].PassFailAuthority)
	assert.Equal(t, "production_session", candidates[0].InputSource)
	assert.True(t, candidates[0].ManualOrDeferred)
	assert.Equal(t, "qa_compiler_deferred_to_SPEC-QAMESH-003", candidates[0].ErrorCode)
}
