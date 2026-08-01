package rulecond_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// hookFiredFixture wires the three shipped hook-fired rules plus one edit-scoped
// fixture rule into a manifest with their real bodies.
func hookFiredFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	for _, name := range []string{"lore-commit", "shell-portability", "worktree-safety"} {
		f.writeBody(name+".md", sourceRuleBody(t, name))
	}
	f.writeBody("fixture-go-edit.md", "# Fixture\n\nGo edits follow the Go style guide.\n")
	f.writeManifest(
		bashRule("lore-commit", condLoreCommit, "lore-commit.md"),
		bashRule("shell-portability", condShellPortable, "shell-portability.md"),
		bashRule("worktree-safety", condWorktreeSafety, "worktree-safety.md"),
		editRule("fixture-go-edit", condGoEditFixture, "fixture-go-edit.md"),
	)
	return f
}

// TestFire_LoreCommitFiresOnCommitCommand is the S1 oracle for
// REQ-CONDRULE-FIRE-01.
func TestFire_LoreCommitFiresOnCommitCommand(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.writeBody("lore-commit.md", sourceRuleBody(t, "lore-commit"))
	f.writeManifest(bashRule("lore-commit", condLoreCommit, "lore-commit.md"))

	stdout, stderr := f.fire(bashPayload(`git commit -m "feat(x): y"`))

	ctx := injectedContext(t, stdout)
	assert.Contains(t, ctx, loreCommitSignature,
		"additionalContext must carry the lore-commit body")
	assert.Equal(t, []string{"lore-commit"}, firedRuleNames(t, stdout))
	assert.Empty(t, stderr)
}

// TestFire_HeterogeneousToolInputs is the S2 oracle for INV-001 and INV-007.
func TestFire_HeterogeneousToolInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		want    []string
	}{
		{name: "git commit", payload: bashPayload(`git commit -m "fix: y"`), want: []string{"lore-commit"}},
		{name: "timeout prefix", payload: bashPayload("timeout 540 auto spec review"), want: []string{"shell-portability"}},
		{name: "gtimeout prefix", payload: bashPayload("gtimeout 30 go test ./..."), want: []string{"shell-portability"}},
		{name: "git gc", payload: bashPayload("git gc --prune=now"), want: []string{"worktree-safety"}},
		{
			name:    "git repack and commit",
			payload: bashPayload("git repack -ad && git commit -m x"),
			want:    []string{"lore-commit", "worktree-safety"},
		},
		{name: "plain ls", payload: bashPayload("ls -la"), want: []string{}},
		{name: "npm run gc", payload: bashPayload("npm run gc"), want: []string{}},
		{
			name:    "echo mentioning git commit is an accepted false positive",
			payload: bashPayload(`echo "git commit is required"`),
			want:    []string{"lore-commit"},
		},
		{
			name:    "edit go file",
			payload: filePayload("Edit", "/repo/pkg/rulecond/fire.go"),
			want:    []string{"fixture-go-edit"},
		},
		{name: "edit markdown file", payload: filePayload("Edit", "/repo/README.md"), want: []string{}},
		{
			name:    "read go file yields no condition subject",
			payload: filePayload("Read", "/repo/pkg/rulecond/fire.go"),
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := hookFiredFixture(t)
			stdout, stderr := f.fire(tt.payload)
			assert.Equal(t, tt.want, firedRuleNames(t, stdout))
			assert.Empty(t, stderr)
			if len(tt.want) == 0 {
				assert.Empty(t, stdout, "a non-match must produce empty stdout")
			}
		})
	}
}

// TestFire_AdvisoryOnly is the REQ-CONDRULE-FIRE-06 oracle.
func TestFire_AdvisoryOnly(t *testing.T) {
	t.Parallel()

	f := hookFiredFixture(t)
	stdout, _ := f.fire(bashPayload("git gc --prune=now && git commit -m x"))

	require.NotEmpty(t, stdout)
	assert.NotContains(t, stdout, "permissionDecision")
	assert.NotContains(t, stdout, `"continue"`)
	assert.NotContains(t, stdout, "\"deny\"")
	assert.NotContains(t, stdout, "\"ask\"")
}

// TestFire_ManifestLocationIsFixed is the REQ-CONDRULE-FIRE-11 oracle: no
// environment variable, flag, or stdin field may redirect the manifest.
func TestFire_ManifestLocationIsFixed(t *testing.T) {
	decoyRoot := t.TempDir()
	decoyManifest := filepath.Join(decoyRoot, "hostile.json")
	require.NoError(t, os.WriteFile(decoyManifest,
		[]byte(`{"version":"1","rules":[{"name":"hostile","event":"PreToolUse","matcher":"Bash","conditions":[".*"],"body":"hostile.md"}]}`),
		0o644))

	for _, key := range []string{
		"AUTOPUS_CONDITIONAL_MANIFEST",
		"AUTOPUS_RULES_MANIFEST",
		"AUTOPUS_CONDRULE_MANIFEST",
	} {
		t.Setenv(key, decoyManifest)
	}

	f := newFixture(t)
	f.writeBody("lore-commit.md", sourceRuleBody(t, "lore-commit"))
	f.writeManifest(bashRule("lore-commit", condLoreCommit, "lore-commit.md"))

	payload := toolPayload("Bash", map[string]any{"command": "git commit -m x"})
	stdout, _ := f.fire(payload)

	assert.Equal(t, []string{"lore-commit"}, firedRuleNames(t, stdout),
		"the manifest must resolve only at the fixed project-relative location")
	assert.Equal(t, ".claude/hooks/autopus/conditional-rules.json",
		filepath.ToSlash(rulecond.ManifestRelPath))
	assert.Equal(t, ".claude/hooks/autopus/conditional",
		filepath.ToSlash(rulecond.BodyRootRelPath))
}
