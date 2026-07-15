package promptlayer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCommandContextProfile_SelectedCommandUsesOnlyDeclaredDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command           string
		required          []ContextProfileName
		conditional       []ContextProfileName
		relevantSpec      bool
		includedDocument  string
		excludedDocuments []string
	}{
		{
			command: "plan", required: []ContextProfileName{ProfileCore, ProfileArchitecture},
			conditional: []ContextProfileName{ProfileSignature, ProfileLearning}, relevantSpec: true,
			includedDocument: "ARCHITECTURE.md", excludedDocuments: []string{".autopus/project/scenarios.md", ".autopus/project/canary.md"},
		},
		{
			command: "test", required: []ContextProfileName{ProfileCore, ProfileTest},
			conditional:      []ContextProfileName{ProfileSignature, ProfileLearning},
			includedDocument: ".autopus/project/scenarios.md", excludedDocuments: []string{".autopus/project/canary.md"},
		},
		{
			command: "canary", required: []ContextProfileName{ProfileCore, ProfileCanary},
			conditional:      []ContextProfileName{ProfileLearning},
			includedDocument: ".autopus/project/canary.md", excludedDocuments: []string{".autopus/project/scenarios.md", ".autopus/context/signatures.md"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()

			profile, ok := ResolveCommandContextProfile(tt.command)
			require.True(t, ok)
			assert.Equal(t, tt.required, profile.Required)
			assert.Equal(t, tt.conditional, profile.Conditional)
			assert.Equal(t, tt.relevantSpec, profile.RelevantSpec)
			assert.Contains(t, profile.RequiredDocuments(), tt.includedDocument)
			for _, document := range tt.excludedDocuments {
				assert.NotContains(t, profile.RequiredDocuments(), document)
			}
		})
	}
}

func TestLoadContextLayer_ProjectEvidenceDefaultsToSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".autopus", "project"), 0o755))
	layer, err := LoadContextLayer(root, ".autopus/project/workspace.md", ContextOptions{})
	require.NoError(t, err)
	assert.Equal(t, KindSnapshot, layer.Kind)

	policy, err := LoadContextLayer(t.TempDir(), "AGENTS.md", ContextOptions{Kind: KindStable})
	require.NoError(t, err)
	assert.Equal(t, KindStable, policy.Kind)
}

func TestLoadContextLayer_RequiredRejectsSymlinkAndNonRegularBeforeRead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "target.md")
	require.NoError(t, os.WriteFile(target, []byte("target"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(root, "linked.md")))
	require.NoError(t, os.Mkdir(filepath.Join(root, "directory.md"), 0o700))

	for _, ref := range []string{"linked.md", "directory.md"} {
		_, err := LoadContextLayer(root, ref, ContextOptions{Required: true})
		require.Error(t, err)
		assert.ErrorContains(t, err, "regular non-symlink")
	}
}
