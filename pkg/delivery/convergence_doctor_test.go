package delivery

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type convergenceGitFixture struct {
	repository string
	worktree   string
}

func TestConvergenceDoctorReturnsPathFreeScopedReceipt(t *testing.T) {
	fixture := newConvergenceGitFixture(t)

	receipt, err := Doctor(DoctorOptions{
		WorkingDirectory: fixture.worktree,
		RepoScopeRef:     "repo-fixture",
		Phase:            PhaseImplement,
	})

	require.NoError(t, err)
	assert.Equal(t, DeliveryDoctorSchemaV1, receipt.SchemaVersion)
	assert.Equal(t, "ready", receipt.Status)
	assert.True(t, receipt.ScopedWorktree)
	assert.Regexp(t, digestPattern, receipt.HarnessDigest)
	assert.Regexp(t, digestPattern, receipt.ContextDigest)
	encoded, marshalErr := json.Marshal(receipt)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(encoded), fixture.repository)
	assert.NotContains(t, string(encoded), fixture.worktree)
}

func TestConvergenceDoctorRejectsInvalidScopeAndHarness(t *testing.T) {
	fixture := newConvergenceGitFixture(t)

	tests := []struct {
		name   string
		mutate func(t *testing.T)
		opts   DoctorOptions
		code   string
	}{
		{
			name: "absolute scope",
			opts: DoctorOptions{WorkingDirectory: fixture.worktree, RepoScopeRef: fixture.repository, Phase: PhaseImplement},
			code: ReasonScopeInvalid,
		},
		{
			name: "unknown phase",
			opts: DoctorOptions{WorkingDirectory: fixture.worktree, RepoScopeRef: "repo-fixture", Phase: "invent"},
			code: ReasonScopeInvalid,
		},
		{
			name: "repository root is not isolated",
			opts: DoctorOptions{WorkingDirectory: fixture.repository, RepoScopeRef: "repo-fixture", Phase: PhaseImplement},
			code: ReasonScopeInvalid,
		},
		{
			name: "empty harness",
			mutate: func(t *testing.T) {
				require.NoError(t, os.WriteFile(filepath.Join(fixture.worktree, "AGENTS.md"), nil, 0o644))
			},
			opts: DoctorOptions{WorkingDirectory: fixture.worktree, RepoScopeRef: "repo-fixture", Phase: PhaseImplement},
			code: ReasonHarnessInvalid,
		},
		{
			name: "invalid yaml alias",
			mutate: func(t *testing.T) {
				content := []byte("value: &shared safe\ncopy: *shared\n")
				require.NoError(t, os.WriteFile(filepath.Join(fixture.worktree, "autopus.yaml"), content, 0o644))
			},
			opts: DoctorOptions{WorkingDirectory: fixture.worktree, RepoScopeRef: "repo-fixture", Phase: PhaseImplement},
			code: ReasonHarnessInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.mutate != nil {
				test.mutate(t)
			}
			_, err := Doctor(test.opts)
			require.Error(t, err)
			assert.Equal(t, test.code, ConvergenceReasonCode(err))
		})
	}
}

func TestConvergenceDoctorRejectsUnsafeHarnessFiles(t *testing.T) {
	t.Run("oversized", func(t *testing.T) {
		fixture := newConvergenceGitFixture(t)
		oversized := bytes.Repeat([]byte("x"), maximumHarnessFileBytes+1)
		require.NoError(t, os.WriteFile(filepath.Join(fixture.worktree, "AGENTS.md"), oversized, 0o644))
		_, err := Doctor(DoctorOptions{fixture.worktree, "repo-fixture", PhaseImplement})
		require.Error(t, err)
		assert.Equal(t, ReasonHarnessInvalid, ConvergenceReasonCode(err))
	})

	t.Run("symlink", func(t *testing.T) {
		fixture := newConvergenceGitFixture(t)
		name := filepath.Join(fixture.worktree, "AGENTS.md")
		require.NoError(t, os.Remove(name))
		require.NoError(t, os.Symlink(filepath.Join(fixture.repository, "AGENTS.md"), name))
		_, err := Doctor(DoctorOptions{fixture.worktree, "repo-fixture", PhaseImplement})
		require.Error(t, err)
		assert.Equal(t, ReasonHarnessInvalid, ConvergenceReasonCode(err))
	})
}

func TestConvergenceDoctorHelpersFailClosed(t *testing.T) {
	for _, value := range []string{"", "repo-a", "repo--bad", "repo-Bad", "repo-a/b", "repo-" + strings.Repeat("a", 66)} {
		assert.Error(t, ValidateOpaqueRepoScopeRef(value), value)
	}
	for _, value := range []string{"repo-ab", "repo-a_b.c-1"} {
		assert.NoError(t, ValidateOpaqueRepoScopeRef(value), value)
	}

	fixture := newConvergenceGitFixture(t)
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(fixture.worktree))
	t.Cleanup(func() { _ = os.Chdir(previous) })
	root, err := scopedWorktreeRoot("")
	require.NoError(t, err)
	assert.True(t, sameCanonicalPath(fixture.worktree, root))
	_, err = scopedWorktreeRoot(filepath.Join(fixture.worktree, "missing"))
	assert.Error(t, err)
	fake := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fake, ".git"), []byte("not git"), 0o644))
	_, err = scopedWorktreeRoot(fake)
	assert.Error(t, err)
	_, err = runGit(fake, "rev-parse", "--show-toplevel")
	assert.Error(t, err)

	t.Setenv("GIT_DIR", "/private/forbidden")
	for _, entry := range safeGitEnvironment() {
		assert.False(t, strings.HasPrefix(entry, "GIT_DIR="))
	}
	assert.True(t, sameCanonicalPath(fixture.worktree, filepath.Clean(fixture.worktree)))
	assert.False(t, sameCanonicalPath(fixture.worktree, filepath.Join(fixture.worktree, "missing")))
	assert.Equal(t, filepath.Join(fixture.worktree, ".git"), resolveGitPath(fixture.worktree, ".git"))
	_, err = readScopedRegularBounded(fixture.worktree, "../AGENTS.md")
	assert.Error(t, err)
	_, err = readScopedRegularBounded(fixture.worktree, ".autopus")
	assert.Error(t, err)
	assert.Equal(t, filepath.Clean("/absolute"), resolveGitPath(fixture.worktree, "/absolute"))
	assert.Equal(t, ReasonContractInvalid, ConvergenceReasonCode(assert.AnError))
}

func TestConvergenceYAMLValidationRejectsAmbiguousNodes(t *testing.T) {
	assert.True(t, validHarnessContent("AGENTS.md", []byte("anything")))
	assert.False(t, validHarnessContent("autopus.yaml", []byte("- one\n- two\n")))
	assert.False(t, validHarnessContent("autopus.yaml", []byte("broken: [\n")))

	assert.True(t, validYAMLNode(&yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode}}}))
	assert.False(t, validYAMLNode(&yaml.Node{Kind: yaml.AliasNode}))
	assert.False(t, validYAMLNode(&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode}}}))
	assert.False(t, validYAMLNode(&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.SequenceNode}, {Kind: yaml.ScalarNode},
	}}))
	assert.False(t, validYAMLNode(&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "same"}, {Kind: yaml.ScalarNode},
		{Kind: yaml.ScalarNode, Value: "same"}, {Kind: yaml.ScalarNode},
	}}))
}

func newConvergenceGitFixture(t *testing.T) convergenceGitFixture {
	t.Helper()
	base := t.TempDir()
	repository := filepath.Join(base, "repository")
	worktree := filepath.Join(base, "worktree")
	require.NoError(t, os.MkdirAll(filepath.Join(repository, ".autopus", "project"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repository, ".autopus", "context"), 0o755))
	writeConvergenceFixtureFile(t, repository, "AGENTS.md", "# Fixture\n")
	writeConvergenceFixtureFile(t, repository, "autopus.yaml", "project: fixture\n")
	writeConvergenceFixtureFile(t, repository, ".autopus/project/workspace.md", "# Workspace\n")
	writeConvergenceFixtureFile(t, repository, ".autopus/context/constraints.yaml", "constraints: []\n")
	runConvergenceGit(t, repository, "init")
	runConvergenceGit(t, repository, "config", "user.email", "fixture@example.invalid")
	runConvergenceGit(t, repository, "config", "user.name", "Autopus Fixture")
	runConvergenceGit(t, repository, "add", ".")
	runConvergenceGit(t, repository, "commit", "-m", "fixture")
	runConvergenceGit(t, repository, "worktree", "add", "-b", "agent/convergence", worktree)
	return convergenceGitFixture{repository: repository, worktree: worktree}
}

func writeConvergenceFixtureFile(t *testing.T, root, relative, content string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o755))
	require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
}

func runConvergenceGit(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	args := append([]string{"-C", directory}, arguments...)
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed: %s", strings.Join(arguments, " "), output)
}
